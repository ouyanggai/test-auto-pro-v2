// T007 runtime 客户端：Go 与 form-runtime 执行服务（无头 Chromium）之间的唯一连接器。
// 职责（docs/s/02-执行任务清单.md T007）：
//   - 连接 runtime、发送命令、接收页面转发请求并回传响应；
//   - 页面目标请求必须交给现有 TargetForwarder（*target.Client），不新增第二个目标 HTTP 客户端；
//   - 复用现有账号 SID 与重登（SessionProvider）；SID 失效只自动换新会话重放一次；
//   - runtime 连接断开时，按目标请求是否已经写出决定不确定或失败，绝不自动重发原写请求。
package runtimeexec

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"test-auto-pro-v2/internal/adapter/target"
)

// SessionProvider 与执行器侧同语义：Current 取缓存会话，Refresh 强制重登。
type SessionProvider interface {
	Current(ctx context.Context, account string) (target.Session, error)
	Refresh(ctx context.Context, account string) (target.Session, error)
}

// TargetForwarder 是唯一目标出口的页面转发面；*target.Client.ForwardRuntimeRequest 满足。
type TargetForwarder interface {
	ForwardRuntimeRequest(ctx context.Context, session target.Session, request target.PageForwardRequest) (*target.PageForwardResponse, error)
}

// ErrUncertain 表示连接断开时已有目标请求写出：写是否生效无法判定，必须走结果待确认。
var ErrUncertain = errors.New("runtime 连接断开且已有目标请求写出，结果待确认")

// ErrFailedCleanly 表示连接断开时没有任何目标请求写出：无副作用失败。
var ErrFailedCleanly = errors.New("runtime 连接断开且没有任何目标请求写出")

// SessionSpec 是创建执行会话的输入：账号唯一键 + 运行编号（一次节点动作一个会话）。
type SessionSpec struct {
	AccountKey string
	RunID      string
	PathRunID  string
	StepID     string
}

// SessionHandle 是执行服务返回的会话事实。
type SessionHandle struct {
	SessionID string
	Status    string
}

// Client 是执行服务客户端；零值不可用，必须经 NewClient 构造。
type Client struct {
	baseURL  *url.URL
	http     *http.Client
	forward  TargetForwarder
	sessions SessionProvider
	// forwardTimeout 是单笔页面请求转发到目标平台的超时；目标存在约 30 秒慢请求，取 90 秒上限。
	forwardTimeout time.Duration
}

// NewClient 构造 runtime 客户端；baseURL 是执行服务地址（如 http://127.0.0.1:19011）。
func NewClient(baseURL string, forward TargetForwarder, sessions SessionProvider) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("执行服务地址不合法：%s", baseURL)
	}
	return &Client{
		baseURL:        parsed,
		http:           &http.Client{Timeout: 60 * time.Second},
		forward:        forward,
		sessions:       sessions,
		forwardTimeout: 90 * time.Second,
	}, nil
}

// CreateSession 创建执行会话（执行服务随即装载固定路由并注册桥接）。
func (c *Client) CreateSession(ctx context.Context, spec SessionSpec) (*SessionHandle, error) {
	body, err := json.Marshal(map[string]string{
		"accountKey": spec.AccountKey,
		"runId":      spec.RunID,
		"pathRunId":  spec.PathRunID,
		"stepId":     spec.StepID,
	})
	if err != nil {
		return nil, err
	}
	var result SessionHandle
	if err := c.doJSON(ctx, http.MethodPost, "/runtime/session", body, &result); err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.SessionID) == "" {
		return nil, fmt.Errorf("执行服务未返回会话编号")
	}
	return &result, nil
}

// Load 装载身份、流程和表单；payload 按 T008/T009 的装载载荷形状传入。
// mode 取 initiate 或 approve。
func (c *Client) Load(ctx context.Context, sessionID, requestID string, cmd Message) (json.RawMessage, error) {
	envelope, err := EncodeMessage(cmd)
	if err != nil {
		return nil, err
	}
	var result json.RawMessage
	if err := c.doJSON(ctx, http.MethodPost, "/runtime/session/"+sessionID+"/load", envelope, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Destroy 关闭执行会话并释放 Context；会话不存在时执行服务返回 404，同样视为已关闭。
func (c *Client) Destroy(ctx context.Context, sessionID string) error {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + "/runtime/session/" + sessionID + "/destroy"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("销毁会话失败：HTTP %d", response.StatusCode)
	}
	return nil
}

// ExecuteCallbacks 接收动作过程中的事件（页面异常、选择要求、阶段日志）。
type ExecuteCallbacks struct {
	OnEvent func(msg *Message)
}

// Execute 通过 WebSocket 发送业务命令并驱动转发循环，直到 runtime 返回结果消息。
// 返回的 *Message 是 kind=result 的协议信封；连接断开按写出事实返回 ErrUncertain/ErrFailedCleanly。
func (c *Client) Execute(ctx context.Context, sessionID, account string, cmd Message, callbacks ExecuteCallbacks) (*Message, error) {
	// 发送前先走同一套协议校验：缺编号、未知动作的命令在本地即被拒绝。
	if _, err := EncodeMessage(cmd); err != nil {
		return nil, err
	}
	dialURL := *c.baseURL
	switch dialURL.Scheme {
	case "https":
		dialURL.Scheme = "wss"
	default:
		dialURL.Scheme = "ws"
	}
	dialURL.Path = strings.TrimRight(dialURL.Path, "/") + "/runtime/session/" + sessionID + "/execute"
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, dialURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("连接执行服务失败：%w", err)
	}
	defer conn.Close()

	state := &executeState{account: account}
	var writeMu sync.Mutex
	send := func(message Message) error {
		// 统一经 EncodeMessage 补协议版本并复核；直接序列化会漏掉 version 被对端拒绝。
		raw, err := EncodeMessage(message)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(websocket.TextMessage, raw)
	}
	if err := send(cmd); err != nil {
		return nil, fmt.Errorf("发送执行命令失败：%w", err)
	}
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			// 连接断开：按「是否已有目标请求写出」二分，绝不重发原命令。
			if state.written {
				return nil, fmt.Errorf("%w（%v）", ErrUncertain, err)
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("%w（%v）", ErrFailedCleanly, err)
		}
		message, err := ValidateMessage(raw)
		if err != nil {
			return nil, err
		}
		switch message.Kind {
		case KindRequestForward:
			response, err := c.handleForward(ctx, state, message)
			payload := requestResponsePayloadOf(response, err)
			if err := send(Message{
				Kind:      KindRequestResp,
				RequestID: message.RequestID,
				SessionID: message.SessionID,
				RunID:     message.RunID,
				PathRunID: message.PathRunID,
				StepID:    message.StepID,
				Payload:   payload,
			}); err != nil {
				if state.written {
					return nil, fmt.Errorf("%w（%v）", ErrUncertain, err)
				}
				return nil, fmt.Errorf("%w（%v）", ErrFailedCleanly, err)
			}
		case KindEvent:
			if callbacks.OnEvent != nil {
				callbacks.OnEvent(message)
			}
		case KindResult:
			return message, nil
		default:
			return nil, fmt.Errorf("runtime 发来不接受的消息类型：%s", message.Kind)
		}
	}
}

// executeState 记录一次 Execute 过程中「目标请求是否已写出」的事实。
type executeState struct {
	account string
	written bool
}

// handleForward 校验并转发一笔页面目标请求；SID 失效只换新会话重放一次。
// 写出事实只认已登记写端点：读请求无论成败都不产生「可能已写出」的结论。
func (c *Client) handleForward(ctx context.Context, state *executeState, message *Message) (*target.PageForwardResponse, error) {
	forwarded, err := ParseForwardedRequest(message.Payload)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(forwarded.URL)
	if err != nil {
		return nil, err
	}
	isWrite := target.IsKnownWritePath(parsed.Path)
	session, err := c.sessions.Current(ctx, state.account)
	if err != nil {
		return nil, fmt.Errorf("取得账号 %s 会话失败：%w", state.account, err)
	}
	response, err := c.forwardOnce(ctx, session, forwarded, parsed)
	if err == nil {
		if isWrite {
			state.written = true
		}
		return response, nil
	}
	if !isSessionExpiredError(err) {
		// 传输失败分类决定写出事实：响应已到达或中断都算已写出，连接阶段失败不算。
		if isWrite {
			if targetErr, ok := err.(*target.Error); ok {
				switch targetErr.Transport {
				case target.TransportResponded, target.TransportInterrupted:
					state.written = true
				}
			}
		}
		return nil, err
	}
	// SID 失效发生在目标业务之前（现有纪律）：换新会话后只重放一次。
	refreshed, refreshErr := c.sessions.Refresh(ctx, state.account)
	if refreshErr != nil {
		return nil, fmt.Errorf("会话失效且重登失败：%w", refreshErr)
	}
	response, err = c.forwardOnce(ctx, refreshed, forwarded, parsed)
	if err == nil && isWrite {
		state.written = true
	}
	return response, err
}

// forwardOnce 发出一次页面转发请求。
func (c *Client) forwardOnce(ctx context.Context, session target.Session, forwarded *ForwardedRequest, parsed *url.URL) (*target.PageForwardResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.forwardTimeout)
	defer cancel()
	body, err := forwarded.decodedBody()
	if err != nil {
		return nil, err
	}
	return c.forward.ForwardRuntimeRequest(callCtx, session, target.PageForwardRequest{
		Method:       forwarded.Method,
		Path:         parsed.Path,
		Query:        parsed.Query(),
		Header:       forwarded.Headers,
		Body:         body,
		CustomerCode: session.CustomerCode,
	})
}

// requestResponsePayloadOf 把转发结果转成 request_response 载荷；失败以 httpStatus=0 表达，
// runtime 据此本地中止对应请求，不会直连目标平台。
func requestResponsePayloadOf(response *target.PageForwardResponse, err error) json.RawMessage {
	if err != nil {
		return json.RawMessage(`{"httpStatus":0}`)
	}
	bodyEncoding := "utf8"
	var body string
	if len(response.Body) > 0 {
		body = base64.StdEncoding.EncodeToString(response.Body)
		bodyEncoding = "base64"
	}
	raw, encodeErr := json.Marshal(map[string]any{
		"httpStatus":   response.StatusCode,
		"headers":      response.Headers,
		"body":         body,
		"bodyEncoding": bodyEncoding,
	})
	if encodeErr != nil {
		return json.RawMessage(`{"httpStatus":0}`)
	}
	return raw
}

// isSessionExpiredError 判定目标返回的会话失效错误（HTTP 401 或 RESP401/AUTH_401 包络）。
func isSessionExpiredError(err error) bool {
	var targetErr *target.Error
	return errors.As(err, &targetErr) && targetErr.Kind == target.ErrorSessionExpired
}

// decodedBody 按 bodyEncoding 解出原始正文字节；base64 保真二进制，utf8 为文本直通。
func (r *ForwardedRequest) decodedBody() ([]byte, error) {
	if r.Body == "" {
		return nil, nil
	}
	switch strings.ToLower(r.BodyEncoding) {
	case "", "utf8":
		return []byte(r.Body), nil
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(r.Body)
		if err != nil {
			return nil, fmt.Errorf("转发正文 base64 解码失败：%w", err)
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("转发正文编码不支持：%s", r.BodyEncoding)
	}
}

// doJSON 发出本地 HTTP 调用（执行服务环回接口，不经目标传输层）。
func (c *Client) doJSON(ctx context.Context, method, path string, body []byte, result any) error {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + path
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return err
	}
	request.Header.Set("content-type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("执行服务返回 HTTP %d：%s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	if result != nil && len(data) > 0 {
		return json.Unmarshal(data, result)
	}
	return nil
}
