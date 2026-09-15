// T011 完成检测失败路径集成验证（真实执行服务 + 真实无头页面 + 假目标对端）：
// ① 写响应丢失（Go 侧转发失败 httpStatus=0）→ 动作判定 uncertain；
// ② 硬超时（目标迟迟不响应且检测超时）→ 判定 timeout。
// 假对端采用单一读循环分发（结果 → 通道；转发请求 → 响应脚本），避免多读竞争。
package runtimeexec_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// fakePeerScript 决定对一笔真实提交转发的响应：status=0 表示 Go 转发失败（写响应丢失）；
// hang=true 表示永不响应（制造在途挂起）。
type fakePeerScript struct {
	onRealSubmit func() (status int, hang bool)
}

// currentSessionID 记录最近创建的会话编号，供超时诊断拉取状态（单会话测试场景）。
var currentSessionID string

// startExecutorWithTimeout 拉起动作硬超时可配置的执行服务，返回基地址与停止函数。
func startExecutorWithTimeout(t *testing.T, actionTimeoutMs int) (string, func()) {
	t.Helper()
	if reason := executorEnvMissing(); reason != "" {
		t.Skipf("执行服务运行条件缺失：%s", reason)
	}
	port := freePort(t)
	cmd := exec.Command("node", "--experimental-strip-types", "src/server.ts")
	cmd.Dir = "../../../form-runtime/executor"
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("EXECUTOR_PORT=%d", port),
		fmt.Sprintf("EXECUTOR_ACTION_TIMEOUT_MS=%d", actionTimeoutMs),
		"EXECUTOR_IDLE_WINDOW_MS=200",
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动执行服务失败：%v", err)
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForExecutorTCP(t, base)
	stopped := false
	return base, func() {
		if stopped {
			return
		}
		stopped = true
		_ = cmd.Process.Kill()
	}
}

// waitForExecutorTCP 等待执行服务就绪（base 地址版本，避免与同包辅助重名）。
func waitForExecutorTCP(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(base + "/runtime/session/none/status")
		if err == nil {
			_ = response.Body.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("执行服务 30 秒内未就绪（%s）", base)
}

// createExecutorSession 创建执行会话并返回会话编号。
func createExecutorSession(t *testing.T, base string) string {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{
		"accountKey": "acct-t011", "runId": "run-f", "pathRunId": "pr-f", "stepId": "step-f",
	})
	response, err := http.Post(base+"/runtime/session", "application/json", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("创建会话失败：%v", err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 {
		t.Fatalf("创建会话应成功，实际 HTTP %d：%s", response.StatusCode, raw)
	}
	var result struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(raw, &result) != nil || result.SessionID == "" {
		t.Fatalf("会话编号解析失败：%s", raw)
	}
	currentSessionID = result.SessionID
	return result.SessionID
}

// startFakeGoPeer 拨号会话 WS 并启动唯一读循环：结果进通道，转发请求按脚本应答。
// 必须在装载/执行命令之前启动。
func startFakeGoPeer(t *testing.T, base, sessionID string, script fakePeerScript) (*websocket.Conn, <-chan string) {
	t.Helper()
	wsURL := strings.Replace(base, "http", "ws", 1) + "/runtime/session/" + sessionID + "/execute"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("假对端连接失败：%v", err)
	}
	results := make(chan string, 16)
	go func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				close(results)
				return
			}
			var envelope struct {
				Kind      string `json:"kind"`
				RequestID string `json:"requestId"`
				SessionID string `json:"sessionId"`
				RunID     string `json:"runId"`
				PathRunID string `json:"pathRunId"`
				StepID    string `json:"stepId"`
				Payload   struct {
					URL    string          `json:"url"`
					Body   json.RawMessage `json:"body"`
					Status string          `json:"status"`
				} `json:"payload"`
			}
			if json.Unmarshal(raw, &envelope) != nil {
				continue
			}
			switch envelope.Kind {
			case "result":
				// result 载荷直接就是 {status,...}，没有 body 字段。
				results <- envelope.Payload.Status
			case "request_forward":
				// envelope.Payload.URL/Body 已由外层解析：URL 是目标地址，Body 是 base64 保真正文
				// （JSON 字符串字面量，先解引号再解码）。
				var bodyText string
				_ = json.Unmarshal(envelope.Payload.Body, &bodyText)
				decoded, _ := base64.StdEncoding.DecodeString(bodyText)
				var parsedBody struct {
					FormDataMongoVo json.RawMessage `json:"formDataMongoVo"`
				}
				_ = json.Unmarshal(decoded, &parsedBody)
				text := `{"message":"success","isSuccess":true,"data":{}}`
				status := 200
				hang := false
				if strings.Contains(envelope.Payload.URL, "/web/flowInstanceApi/submit") && len(parsedBody.FormDataMongoVo) > 0 && script.onRealSubmit != nil {
					status, hang = script.onRealSubmit()
				}
				if hang {
					continue
				}
				// 必须回显原信封编号：空编号会被协议校验拒绝，转发将永远挂起。
				_ = conn.WriteJSON(map[string]any{
					"version": "exec-engine/v1", "kind": "request_response", "requestId": envelope.RequestID,
					"sessionId": envelope.SessionID, "runId": envelope.RunID, "pathRunId": envelope.PathRunID, "stepId": envelope.StepID,
					"payload": map[string]any{
						"httpStatus":   status,
						"headers":      map[string]string{"content-type": "application/json"},
						"body":         base64.StdEncoding.EncodeToString([]byte(text)),
						"bodyEncoding": "base64",
					},
				})
			}
		}
	}()
	t.Cleanup(func() { _ = conn.Close() })
	return conn, results
}

// sendExecuteCommand 发送一条业务命令。
func sendExecuteCommand(t *testing.T, conn *websocket.Conn, sessionID, action string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"action": action})
	envelope, err := json.Marshal(map[string]any{
		"version": "exec-engine/v1", "kind": "execute", "requestId": fmt.Sprintf("req-%d", time.Now().UnixNano()),
		"sessionId": sessionID, "runId": "run-f", "pathRunId": "pr-f", "stepId": "step-f",
		"payload": json.RawMessage(payload),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, envelope); err != nil {
		t.Fatalf("发送命令失败：%v", err)
	}
}

// loadExecutorSession 走注入路径装载最小表单（离线模板，无目标读取）。
func loadExecutorSession(t *testing.T, base, sessionID string) {
	t.Helper()
	payload := map[string]any{
		"version": "exec-engine/v1", "kind": "load", "requestId": "req-load", "sessionId": sessionID,
		"runId": "run-f", "pathRunId": "pr-f", "stepId": "step-f",
		"payload": map[string]any{
			"mode":          "initiate",
			"renderType":    "formmaking",
			"template":      map[string]any{"list": []map[string]any{{"type": "input", "model": "reason", "options": map[string]any{"remote": false, "required": true}}}, "config": map[string]any{}},
			"values":        map[string]any{"reason": "失败路径"},
			"permissions":   []string{"reason"},
			"changedFields": []string{},
			"toolFlowName":  "T011 失败路径验证",
		},
	}
	encoded, _ := json.Marshal(payload)
	response, err := http.Post(base+"/runtime/session/"+sessionID+"/load", "application/json", strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("装载失败：%v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("装载应成功，实际 HTTP %d", response.StatusCode)
	}
}

// waitVerdict 等待结果通道中的下一个判定；超时时附带服务端轨迹帮助定位。
func waitVerdict(t *testing.T, base string, results <-chan string, timeout time.Duration) string {
	t.Helper()
	select {
	case verdict, ok := <-results:
		if !ok {
			t.Fatalf("执行服务通道已关闭")
		}
		return verdict
	case <-time.After(timeout):
		response, err := http.Get(base + "/runtime/session/" + currentSessionID + "/status")
		var body string
		if err != nil {
			body = err.Error()
		} else {
			raw, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			body = string(raw)
		}
		t.Fatalf("等待结果超时：status=%s", body)
		return ""
	}
}

// TestCompletionLostWriteIsUncertain 真实提交的响应丢失（转发失败）→ 判定 uncertain。
func TestCompletionLostWriteIsUncertain(t *testing.T) {
	base, stop := startExecutorWithTimeout(t, 180000)
	defer stop()
	sessionID := createExecutorSession(t, base)
	conn, results := startFakeGoPeer(t, base, sessionID, fakePeerScript{
		onRealSubmit: func() (int, bool) {
			// 写响应丢失：Go 转发失败（httpStatus=0）→ 路由本地中止，页面收到网络错误。
			return 0, false
		},
	})
	loadExecutorSession(t, base, sessionID)
	sendExecuteCommand(t, conn, sessionID, "submit")
	if verdict := waitVerdict(t, base, results, 60*time.Second); verdict != "uncertain" {
		t.Fatalf("写响应丢失必须判定 uncertain，实际 %s", verdict)
	}
}

// TestCompletionHardTimeout 页面链路挂起 → 判定 timeout（动作硬超时压缩为 4 秒）。
func TestCompletionHardTimeout(t *testing.T) {
	base, stop := startExecutorWithTimeout(t, 4000)
	defer stop()
	sessionID := createExecutorSession(t, base)
	conn, results := startFakeGoPeer(t, base, sessionID, fakePeerScript{
		onRealSubmit: func() (int, bool) {
			return 200, true // 故意挂起：在途请求永不落定
		},
	})
	loadExecutorSession(t, base, sessionID)
	sendExecuteCommand(t, conn, sessionID, "submit")
	if verdict := waitVerdict(t, base, results, 60*time.Second); verdict != "timeout" {
		t.Fatalf("硬超时必须判定 timeout，实际 %s", verdict)
	}
}
