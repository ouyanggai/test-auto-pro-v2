// T008/T012 runtime 执行器单测：以脚本化假执行服务驱动真实 RuntimeExecutor，
// 锁定装载信封形状（必须携带全部协议编号、可被协议校验接受）与结果载荷解析。
// 回归背景：RunAction 曾把已编码信封塞进外层 Payload 二次包装，协议校验必然拒绝，
// 整条 runtime 动作链路在装载第一步即失败且无测试覆盖。
package runtimeexec_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"

	"test-auto-pro-v2/internal/runtimeexec"
)

// runnerHarness 汇集执行器测试的假服务与捕获结果。
type runnerHarness struct {
	mu          sync.Mutex
	loadBodies  []string
	loadPayload map[string]any
}

// newRunnerServer 启动假执行服务：记录 load 请求原文并返回成功结果。
func newRunnerServer(t *testing.T, harness *runnerHarness) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/runtime/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"sessionId":"sess-runner","status":"ready"}`))
	})
	mux.HandleFunc("/runtime/session/sess-runner/load", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<20)
		n, _ := r.Body.Read(buf)
		harness.mu.Lock()
		harness.loadBodies = append(harness.loadBodies, string(buf[:n]))
		harness.mu.Unlock()
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"ready":true}`))
	})
	mux.HandleFunc("/runtime/session/sess-runner/execute", func(w http.ResponseWriter, r *http.Request) {
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
				raw, _ := json.Marshal(map[string]any{"status": "success", "message": "完成", "requestTrace": []any{}})
				_ = ws.WriteJSON(map[string]any{
					"version": runtimeexec.ProtocolVersion, "kind": "result", "requestId": env.RequestID,
					"sessionId": env.SessionID, "runId": env.RunID, "pathRunID": env.PathRunID, "stepId": env.StepID,
					"payload": json.RawMessage(raw),
				})
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// TestRunActionSendsValidLoadEnvelope 动作编排的装载信封必须是完整协议信封：
// 编号齐全、能通过 ValidateMessage，且载荷携带模式、模板与工具流程名称。
func TestRunActionSendsValidLoadEnvelope(t *testing.T) {
	harness := &runnerHarness{}
	server := newRunnerServer(t, harness)
	client, err := runtimeexec.NewClient(server.URL, &fakeForwarder{}, &fakeSessions{})
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	executor := runtimeexec.NewRuntimeExecutor(client)
	result, err := executor.RunAction(context.Background(), runtimeexec.RuntimeRunRequest{
		RunID: "run-9", PathRunID: "pr-9", StepID: "step-9", Account: "acct",
		Action:       "submit",
		ToolFlowName: "工具流程",
		Session:      runtimeexec.SessionFacts{UserID: "u-1", Account: "acct", DisplayName: "张三"},
		Initiator:    runtimeexec.ActorIdentity{UserID: "u-1", Name: "张三"},
		Template:     json.RawMessage(`{"list":[],"config":{}}`),
		Values:       json.RawMessage(`{"reason":"x"}`),
		BaseURL:      "http://127.0.0.1:1",
	})
	if err != nil {
		t.Fatalf("RunAction 失败：%v", err)
	}
	if result.Status != "success" {
		t.Fatalf("期望 success，实际 %s", result.Status)
	}
	harness.mu.Lock()
	defer harness.mu.Unlock()
	if len(harness.loadBodies) != 1 {
		t.Fatalf("装载应恰好发生一次，实际 %d", len(harness.loadBodies))
	}
	envelope, err := runtimeexec.ValidateMessage([]byte(harness.loadBodies[0]))
	if err != nil {
		t.Fatalf("装载请求必须是合法协议信封：%v（原文：%s）", err, harness.loadBodies[0])
	}
	if envelope.Kind != runtimeexec.KindLoad || envelope.SessionID != "sess-runner" {
		t.Fatalf("装载信封类型/会话不符：%s/%s", envelope.Kind, envelope.SessionID)
	}
	var payload struct {
		Mode         string          `json:"mode"`
		ToolFlowName string          `json:"toolFlowName"`
		Template     json.RawMessage `json:"template"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("装载载荷解析失败：%v", err)
	}
	if payload.Mode != "initiate" || payload.ToolFlowName != "工具流程" || !strings.Contains(string(payload.Template), "list") {
		t.Fatalf("装载载荷字段不符：%+v", payload)
	}
}
