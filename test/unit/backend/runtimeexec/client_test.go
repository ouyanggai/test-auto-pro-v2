// T007 runtime 客户端单测：以脚本化假执行服务（HTTP + WebSocket）驱动真实 Client，
// 核对命令往返、SID 失效换会话重放一次、断连按写出事实二分与路径允许清单拦截。
package runtimeexec_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/runtimeexec"
)

// fakeSessions 是可编程的会话提供者：记录 Refresh 次数，返回预置会话。
type fakeSessions struct {
	mu       sync.Mutex
	current  target.Session
	refresh  target.Session
	refreshN int
}

func (s *fakeSessions) Current(_ context.Context, _ string) (target.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refreshN > 0 {
		return s.refresh, nil
	}
	return s.current, nil
}

func (s *fakeSessions) Refresh(_ context.Context, _ string) (target.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshN++
	return s.refresh, nil
}

// fakeForwarder 是可编程的目标转发出口：按调用次序返回预置结果。
type fakeForwarder struct {
	mu       sync.Mutex
	calls    []target.PageForwardRequest
	sids     []string
	respones []*target.PageForwardResponse
	errs     []error
}

func (f *fakeForwarder) ForwardRuntimeRequest(_ context.Context, session target.Session, request target.PageForwardRequest) (*target.PageForwardResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	index := len(f.calls)
	f.calls = append(f.calls, request)
	f.sids = append(f.sids, session.SID)
	if index < len(f.errs) && f.errs[index] != nil {
		return nil, f.errs[index]
	}
	if index < len(f.respones) && f.respones[index] != nil {
		return f.respones[index], nil
	}
	return &target.PageForwardResponse{StatusCode: 200, Headers: map[string]string{"content-type": "application/json"}, Body: []byte(`{"isSuccess":true}`)}, nil
}

// sessionExpiredError 构造带 TransportResponded 的会话失效错误（与真实目标响应形状一致）。
func sessionExpiredError() error {
	return &target.Error{Kind: target.ErrorSessionExpired, Transport: target.TransportResponded}
}

// startFakeExecutor 启动假执行服务：返回基地址、会话编号与 WS 行为的可编程钩子。
// onExecute 在收到 execute 命令后被调用（wsConn 已可读写）；为空时直接回 success 结果。
func startFakeExecutor(t *testing.T, onExecute func(ws *websocket.Conn, env *runtimeexec.Message)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/runtime/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"sessionId":"sess-fake","status":"ready"}`))
	})
	mux.HandleFunc("/runtime/session/sess-fake/execute", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		for {
			_, raw, err := ws.ReadMessage()
			if err != nil {
				return
			}
			env, err := runtimeexec.ValidateMessage(raw)
			if err != nil {
				return
			}
			if env.Kind == runtimeexec.KindExecute {
				if onExecute == nil {
					raw, _ := json.Marshal(map[string]any{"status": "success"})
					_ = ws.WriteJSON(map[string]any{
						"version": runtimeexec.ProtocolVersion, "kind": "result", "requestId": env.RequestID,
						"sessionId": env.SessionID, "runId": env.RunID, "pathRunID": env.PathRunID, "stepId": env.StepID,
						"payload": json.RawMessage(raw),
					})
					continue
				}
				onExecute(ws, env)
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// executeCommand 构造一条最小 execute 命令。
func executeCommand() runtimeexec.Message {
	return runtimeexec.Message{
		Kind: runtimeexec.KindExecute, RequestID: "req-1", SessionID: "sess-fake",
		RunID: "run-1", PathRunID: "pr-1", StepID: "step-1",
		Payload: json.RawMessage(`{"action":"submit"}`),
	}
}

// readEnvelope 从假执行服务侧读取一条 Go 发来的协议消息。
func readEnvelope(t *testing.T, ws *websocket.Conn) *runtimeexec.Message {
	t.Helper()
	_, raw, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("读取消息失败：%v", err)
	}
	env, err := runtimeexec.ValidateMessage(raw)
	if err != nil {
		t.Fatalf("假执行服务收到非法消息：%v", err)
	}
	return env
}

// TestClientExecuteRoundtrip 命令→转发→回填→结果全链路：
// 页面写请求经假出口发出，客户端回填响应，runtime 返回结果后 Execute 正常返回。
func TestClientExecuteRoundtrip(t *testing.T) {
	server := startFakeExecutor(t, func(ws *websocket.Conn, env *runtimeexec.Message) {
		// 模拟页面拦截到一笔目标写请求。
		forward := runtimeexec.Message{
			Version: runtimeexec.ProtocolVersion,
			Kind:    runtimeexec.KindRequestForward, RequestID: "fwd-1", SessionID: env.SessionID,
			RunID: env.RunID, PathRunID: env.PathRunID, StepID: env.StepID,
			Payload: json.RawMessage(`{"method":"POST","url":"https://target.example.com/web/flowInstanceApi/submit","headers":{"content-type":"application/json"},"body":"eyJhIjoxfQ==","bodyEncoding":"base64","action":"submit","writeSequence":1}`),
		}
		_ = ws.WriteJSON(forward)
		response := readEnvelope(t, ws)
		if response.Kind != runtimeexec.KindRequestResp {
			t.Fatalf("期望 request_response，收到 %s", response.Kind)
		}
		var payload struct {
			HTTPStatus int            `json:"httpStatus"`
			Body       string         `json:"body"`
			Encoding   string         `json:"bodyEncoding"`
			Headers    map[string]any `json:"headers"`
		}
		if err := json.Unmarshal(response.Payload, &payload); err != nil {
			t.Fatalf("request_response 载荷解析失败：%v", err)
		}
		if payload.HTTPStatus != 200 {
			t.Fatalf("期望回填 200，实际 %d", payload.HTTPStatus)
		}
		decoded, err := base64.StdEncoding.DecodeString(payload.Body)
		if err != nil || string(decoded) != `{"isSuccess":true}` {
			t.Fatalf("回填正文不符合预期：%s (%v)", payload.Body, err)
		}
		raw, _ := json.Marshal(map[string]any{"status": "success", "businessSummary": map[string]any{"forwarded": 1}})
		_ = ws.WriteJSON(map[string]any{
			"version": runtimeexec.ProtocolVersion, "kind": "result", "requestId": env.RequestID,
			"sessionId": env.SessionID, "runId": env.RunID, "pathRunID": env.PathRunID, "stepId": env.StepID,
			"payload": json.RawMessage(raw),
		})
	})
	sessions := &fakeSessions{current: target.Session{SID: "sid-old", Summary: target.AccountSummary{Account: "acct"}}}
	forwarder := &fakeForwarder{}
	client, err := runtimeexec.NewClient(server.URL, forwarder, sessions)
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	result, err := client.Execute(context.Background(), "sess-fake", "acct", executeCommand(), runtimeexec.ExecuteCallbacks{})
	if err != nil {
		t.Fatalf("Execute 失败：%v", err)
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result.Payload, &payload); err != nil {
		t.Fatalf("结果载荷解析失败：%v", err)
	}
	if payload.Status != "success" {
		t.Fatalf("期望 success，实际 %s", payload.Status)
	}
	if len(forwarder.calls) != 1 {
		t.Fatalf("目标出口应被调用一次，实际 %d", len(forwarder.calls))
	}
	if forwarder.sids[0] != "sid-old" {
		t.Fatalf("转发必须使用当前会话 SID，实际 %s", forwarder.sids[0])
	}
	if string(forwarder.calls[0].Body) != `{"a":1}` {
		t.Fatalf("转发正文必须按 base64 解码保真：%s", forwarder.calls[0].Body)
	}
}

// TestClientSessionExpiredReplaysOnceWithRefresh SID 失效：自动重登一次并重放同一请求，第二次成功。
func TestClientSessionExpiredReplaysOnceWithRefresh(t *testing.T) {
	server := startFakeExecutor(t, func(ws *websocket.Conn, env *runtimeexec.Message) {
		forward := runtimeexec.Message{
			Version: runtimeexec.ProtocolVersion,
			Kind:    runtimeexec.KindRequestForward, RequestID: "fwd-1", SessionID: env.SessionID,
			RunID: env.RunID, PathRunID: env.PathRunID, StepID: env.StepID,
			Payload: json.RawMessage(`{"method":"POST","url":"https://target.example.com/web/flowInstanceApi/submit","headers":{},"action":"submit"}`),
		}
		_ = ws.WriteJSON(forward)
		if resp := readEnvelope(t, ws); resp.Kind != runtimeexec.KindRequestResp {
			t.Fatalf("期望 request_response")
		}
		raw, _ := json.Marshal(map[string]any{"status": "success"})
		_ = ws.WriteJSON(map[string]any{
			"version": runtimeexec.ProtocolVersion, "kind": "result", "requestId": env.RequestID,
			"sessionId": env.SessionID, "runId": env.RunID, "pathRunID": env.PathRunID, "stepId": env.StepID,
			"payload": json.RawMessage(raw),
		})
	})
	sessions := &fakeSessions{current: target.Session{SID: "sid-old", Summary: target.AccountSummary{Account: "acct"}}, refresh: target.Session{SID: "sid-new", Summary: target.AccountSummary{Account: "acct"}}}
	forwarder := &fakeForwarder{errs: []error{sessionExpiredError()}}
	client, err := runtimeexec.NewClient(server.URL, forwarder, sessions)
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	result, err := client.Execute(context.Background(), "sess-fake", "acct", executeCommand(), runtimeexec.ExecuteCallbacks{})
	if err != nil {
		t.Fatalf("Execute 失败：%v", err)
	}
	if !strings.Contains(string(result.Payload), "success") {
		t.Fatalf("重放后应返回 success：%s", result.Payload)
	}
	if len(forwarder.calls) != 2 {
		t.Fatalf("会话失效后必须且只能重放一次，实际调用 %d 次", len(forwarder.calls))
	}
	if forwarder.sids[0] != "sid-old" || forwarder.sids[1] != "sid-new" {
		t.Fatalf("重放必须使用新会话 SID：%v", forwarder.sids)
	}
	if sessions.refreshN != 1 {
		t.Fatalf("只允许自动重登一次，实际 %d", sessions.refreshN)
	}
}

// TestClientDisconnectAfterWrittenIsUncertain 写请求已写出后连接断开：必须返回不确定，不重发。
func TestClientDisconnectAfterWrittenIsUncertain(t *testing.T) {
	server := startFakeExecutor(t, func(ws *websocket.Conn, env *runtimeexec.Message) {
		forward := runtimeexec.Message{
			Version: runtimeexec.ProtocolVersion,
			Kind:    runtimeexec.KindRequestForward, RequestID: "fwd-1", SessionID: env.SessionID,
			RunID: env.RunID, PathRunID: env.PathRunID, StepID: env.StepID,
			Payload: json.RawMessage(`{"method":"POST","url":"https://target.example.com/web/flowInstanceApi/submit","headers":{},"action":"submit"}`),
		}
		_ = ws.WriteJSON(forward)
		if resp := readEnvelope(t, ws); resp.Kind != runtimeexec.KindRequestResp {
			t.Fatalf("期望 request_response")
		}
		// 已写出后直接断开，不发送 result。
	})
	sessions := &fakeSessions{current: target.Session{SID: "sid-old", Summary: target.AccountSummary{Account: "acct"}}}
	forwarder := &fakeForwarder{}
	client, err := runtimeexec.NewClient(server.URL, forwarder, sessions)
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	_, err = client.Execute(context.Background(), "sess-fake", "acct", executeCommand(), runtimeexec.ExecuteCallbacks{})
	if !errors.Is(err, runtimeexec.ErrUncertain) {
		t.Fatalf("写出后断开必须返回 ErrUncertain，实际 %v", err)
	}
}

// TestClientDisconnectBeforeWriteFailsCleanly 未发出任何写请求就断开：无副作用失败。
func TestClientDisconnectBeforeWriteFailsCleanly(t *testing.T) {
	server := startFakeExecutor(t, func(_ *websocket.Conn, _ *runtimeexec.Message) {
		// 不做任何事：直接由 defer ws.Close() 断开。
	})
	sessions := &fakeSessions{current: target.Session{SID: "sid-old", Summary: target.AccountSummary{Account: "acct"}}}
	forwarder := &fakeForwarder{}
	client, err := runtimeexec.NewClient(server.URL, forwarder, sessions)
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	_, err = client.Execute(context.Background(), "sess-fake", "acct", executeCommand(), runtimeexec.ExecuteCallbacks{})
	if !errors.Is(err, runtimeexec.ErrFailedCleanly) {
		t.Fatalf("未写出断开必须返回 ErrFailedCleanly，实际 %v", err)
	}
	if len(forwarder.calls) != 0 {
		t.Fatalf("不应调用目标出口，实际 %d 次", len(forwarder.calls))
	}
}

// TestClientRejectsForwardOutsideAllowlist 路径不在允许清单：客户端直接回 httpStatus=0，不触碰目标出口。
func TestClientRejectsForwardOutsideAllowlist(t *testing.T) {
	server := startFakeExecutor(t, func(ws *websocket.Conn, env *runtimeexec.Message) {
		forward := runtimeexec.Message{
			Version: runtimeexec.ProtocolVersion,
			Kind:    runtimeexec.KindRequestForward, RequestID: "fwd-1", SessionID: env.SessionID,
			RunID: env.RunID, PathRunID: env.PathRunID, StepID: env.StepID,
			Payload: json.RawMessage(`{"method":"GET","url":"https://target.example.com/not/allowed"}`),
		}
		_ = ws.WriteJSON(forward)
		response := readEnvelope(t, ws)
		var payload struct {
			HTTPStatus int `json:"httpStatus"`
		}
		if err := json.Unmarshal(response.Payload, &payload); err != nil {
			t.Fatalf("载荷解析失败：%v", err)
		}
		if payload.HTTPStatus != 0 {
			t.Fatalf("未登记路径必须以 httpStatus=0 拒绝，实际 %d", payload.HTTPStatus)
		}
		raw, _ := json.Marshal(map[string]any{"status": "success"})
		_ = ws.WriteJSON(map[string]any{
			"version": runtimeexec.ProtocolVersion, "kind": "result", "requestId": env.RequestID,
			"sessionId": env.SessionID, "runId": env.RunID, "pathRunID": env.PathRunID, "stepId": env.StepID,
			"payload": json.RawMessage(raw),
		})
	})
	sessions := &fakeSessions{current: target.Session{SID: "sid-old", Summary: target.AccountSummary{Account: "acct"}}}
	forwarder := &fakeForwarder{}
	client, err := runtimeexec.NewClient(server.URL, forwarder, sessions)
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	if _, err := client.Execute(context.Background(), "sess-fake", "acct", executeCommand(), runtimeexec.ExecuteCallbacks{}); err != nil {
		t.Fatalf("Execute 失败：%v", err)
	}
	if len(forwarder.calls) != 0 {
		t.Fatalf("未登记路径不允许触达目标出口，实际 %d 次", len(forwarder.calls))
	}
}

// TestClientCreateSession 走本地 HTTP 创建会话并解析会话编号。
func TestClientCreateSession(t *testing.T) {
	server := startFakeExecutor(t, nil)
	sessions := &fakeSessions{current: target.Session{SID: "sid", Summary: target.AccountSummary{Account: "acct"}}}
	client, err := runtimeexec.NewClient(server.URL, &fakeForwarder{}, sessions)
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	handle, err := client.CreateSession(ctx, runtimeexec.SessionSpec{AccountKey: "acct", RunID: "run", PathRunID: "pr", StepID: "s"})
	if err != nil {
		t.Fatalf("创建会话失败：%v", err)
	}
	if handle.SessionID != "sess-fake" {
		t.Fatalf("会话编号不符：%s", handle.SessionID)
	}
}
