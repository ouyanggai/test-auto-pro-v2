package runtimeexec_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/runtimeexec"
	"test-auto-pro-v2/internal/service"
)

// TestRuntimeForwardLogChain 经本地 WebSocket、真实 HTTP 传输、落盘及详情解析验证请求身份和错误保真。
// 只连接 httptest 服务；两笔写入都是内存响应，不连接目标平台、不启动浏览器。
func TestRuntimeForwardLogChain(t *testing.T) {
	original := "  目标 错误原话\n\t" + strings.Repeat("详细原因", 100) + "  "
	var mu sync.Mutex
	var paths, bodies []string
	readCount := 0
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		paths, bodies = append(paths, r.URL.Path), append(bodies, string(body))
		if strings.HasSuffix(r.URL.Path, "/findById") {
			readCount++
		}
		firstRead := strings.HasSuffix(r.URL.Path, "/findById") && readCount == 1
		mu.Unlock()
		w.Header().Set("content-type", "application/json")
		switch {
		case firstRead:
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"isSuccess":false,"code":"RESP401","message":"会话失效"}`)
		case strings.HasSuffix(r.URL.Path, "/save"):
			_ = json.NewEncoder(w).Encode(map[string]any{"isSuccess": false, "message": original, "code": "POSTSCRIPT_ERROR"})
		default:
			_, _ = io.WriteString(w, `{"isSuccess":true,"data":{"id":"aux-new"}}`)
		}
	}))
	t.Cleanup(targetServer.Close)
	router := logging.NewRouter(t.TempDir(), time.Now)
	targetClient, err := target.NewClient(target.ClientConfig{BaseURL: targetServer.URL + "/api", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	targetClient.SetNetworkLogger(logging.NewLogger(router, time.Now))
	server := startFakeExecutor(t, func(ws *websocket.Conn, env *runtimeexec.Message) {
		for index, path := range []string{"/web/formProxy/findById", "/web/flowInstanceApi/transpond", "/web/flowInstancePostscript/save"} {
			payload, _ := json.Marshal(map[string]any{"method": "POST", "url": "https://target.example" + path, "requestSeq": index + 7, "headers": map[string]string{"content-type": "application/json"}, "body": `{"data":{"flowInstanceId":"aux-new","text":"当前附言"}}`, "bodyEncoding": "utf8"})
			forward := *env
			forward.Kind, forward.RequestID, forward.Payload = runtimeexec.KindRequestForward, "forward-"+path, payload
			if err := ws.WriteJSON(forward); err != nil {
				t.Error(err)
				return
			}
			response := readEnvelope(t, ws)
			if response.Kind != runtimeexec.KindRequestResp {
				t.Errorf("转发未回填：%s", response.Kind)
				return
			}
		}
		result := *env
		result.Kind = runtimeexec.KindResult
		result.Payload, _ = json.Marshal(map[string]any{"status": "business_failed", "message": original})
		_ = ws.WriteJSON(result)
	})
	sessions := &fakeSessions{current: target.Session{SID: "sid-test-old"}, refresh: target.Session{SID: "sid-test-new"}}
	client, err := runtimeexec.NewClient(server.URL, targetClient, sessions)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Destroy(context.Background(), "sess-fake") })
	scope := logging.Scope{PlanID: "50", PlanName: "中文计划", ExecutionPathID: "77", ExecutionPathName: "中文路径", RunID: "912", RunSeq: "3", RunStartedAt: "2026-10-15 10-00-00", PathRunID: "801", StepID: "5", Attempt: "2", Phase: "submit"}
	client.SetSessionLogScope("sess-fake", scope)
	command := executeCommand()
	command.RunID, command.PathRunID, command.StepID = "912", "801", "5"
	if _, err := client.Execute(context.Background(), "sess-fake", "acct", command, runtimeexec.ExecuteCallbacks{}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	gotPaths, gotBodies := append([]string(nil), paths...), append([]string(nil), bodies...)
	mu.Unlock()
	if len(gotPaths) != 4 || gotPaths[0] != gotPaths[1] || !strings.HasSuffix(gotPaths[2], "/transpond") || !strings.HasSuffix(gotPaths[3], "/save") || sessions.refreshN != 1 {
		t.Fatalf("只允许重放失效读取，两笔业务写各一次：%v", gotPaths)
	}
	if !strings.Contains(gotBodies[2], "当前附言") || !strings.Contains(gotBodies[3], "aux-new") {
		t.Fatal("日志归属修正不能改变业务载荷")
	}
	dir := router.BucketDir(scope)
	raw, err := os.ReadFile(filepath.Join(dir, "network.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		for _, identity := range []string{"run_id=912", "path_run_id=801", "step_id=5", "attempt=2", "phase=submit", "request_at="} {
			if !strings.Contains(line, identity) {
				t.Fatalf("传输行缺少运行身份 %s：%s", identity, line)
			}
		}
		if strings.Count(line, "runtime_request_seq=") != 1 {
			t.Fatalf("轨迹序号只能记录一次：%s", line)
		}
	}
	rel, _ := filepath.Rel(router.Root(), filepath.Join(dir, "step.log"))
	requests, summaries, available := service.ReadAttemptRequestsForTestWithRouter(router, 801, []model.RunStep{{ID: 5005, StepNo: 5}}, []model.RunStepAttempt{{StepID: 5005, AttemptNo: 2, LogPath: rel}})
	rows := requests["5:2"]
	if !available || len(rows) != 4 || summaries["5:2"].WriteCount != 2 {
		t.Fatalf("真实传输不得漏记或重复：%+v，汇总=%+v", rows, summaries)
	}
	if rows[0].RuntimeRequestSeq != 7 || rows[1].RuntimeRequestSeq != 7 || rows[0].RetryAttempt != 1 || rows[1].RetryAttempt != 2 || rows[0].RequestClass != "read" || rows[2].RequestClass != "write" || rows[3].RequestClass != "write" {
		t.Fatalf("读取、写入或 SID 刷新分类不正确：%+v", rows)
	}
	if rows[3].TargetMessage != original || rows[3].TargetCode != "POSTSCRIPT_ERROR" || rows[3].Result != "failure" {
		t.Fatalf("HTTP 200 的业务失败必须保留原话：%+v", rows[3])
	}
	curl, err := os.ReadFile(filepath.Join(dir, "curl.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if !row.DurationKnown || row.TraceID == "" || !strings.Contains(string(curl), "trace_id="+row.TraceID) {
			t.Fatalf("每条传输的真实耗时与 curl 关联必须齐全：%+v", row)
		}
	}
}
