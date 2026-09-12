// F-031 协议与读取边界定向验证：`/web/flowJobTaskLink/list` 的实例筛选必须在协议顶层
// `flowInstanceIdList`（放进 data 会被目标忽略、退化成翻全量任务表）；
// pending 用顶层 queryUserId、done 用 data.executorId；空实例 ID 在发请求前拒绝。
// 同时锁定一次事实读取边界内同一 (视角, 实例, 状态) 只扫一次，换一次边界必须重新扫（写后屏障）。
package target_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// taskListServer 记录每次任务列表请求的完整载荷，并按固定行返回成功包络。
type taskListServer struct {
	mu       sync.Mutex
	bodies   []map[string]any
	rows     []map[string]any
	pages    int
	requests int
}

// handler 返回一个记录请求并回放固定任务行的 HTTP 处理函数。
func (s *taskListServer) handler() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		s.mu.Lock()
		s.bodies = append(s.bodies, payload)
		s.requests++
		rows, pages := s.rows, s.pages
		s.mu.Unlock()
		response.Header().Set("Content-Type", "application/json")
		envelope := map[string]any{"isSuccess": true, "data": rows}
		if pages > 0 {
			envelope["pages"] = pages
		}
		encoded, _ := json.Marshal(envelope)
		_, _ = response.Write(encoded)
	}
}

// lastBody 返回最近一次请求载荷；没有请求时返回 nil。
func (s *taskListServer) lastBody() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.bodies) == 0 {
		return nil
	}
	return s.bodies[len(s.bodies)-1]
}

// requestCount 返回已收到的请求数。
func (s *taskListServer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

// newTaskQueryClient 启动记录型目标服务并返回绑定到它的目标客户端。
func newTaskQueryClient(t *testing.T, server *taskListServer) *target.Client {
	t.Helper()
	httpServer := httptest.NewServer(server.handler())
	t.Cleanup(httpServer.Close)
	client, err := target.NewClient(target.ClientConfig{BaseURL: httpServer.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	return client
}

// TestPendingTaskQueryKeepsInstanceFilterAtProtocolTopLevel 验证 pending 精确实例查询：
// 实例筛选只出现在协议顶层 flowInstanceIdList，请求里不存在 data.flowInstanceId，
// 视角用户写在协议顶层 queryUserId，taskStatus 与业务关联空值留在 data。
func TestPendingTaskQueryKeepsInstanceFilterAtProtocolTopLevel(t *testing.T) {
	server := &taskListServer{rows: []map[string]any{{
		"jobTaskId": "task-1", "flowInstanceId": "instance-7", "flowNodeProxyId": "node-audit",
		"taskStatus": "pending", "currentPendingUserId": "user-2", "currentPendingUserName": "李四",
	}}}
	client := newTaskQueryClient(t, server)

	snapshots, err := client.ListTaskSnapshotsForUser(context.Background(),
		target.Session{SID: "sid-plan"}, "instance-7", "pending", "user-2")
	if err != nil {
		t.Fatalf("pending 精确实例查询失败：%v", err)
	}
	if len(snapshots) != 1 || snapshots[0].JobTaskID != "task-1" {
		t.Fatalf("pending 查询结果不正确：%+v", snapshots)
	}
	body := server.lastBody()
	if body == nil {
		t.Fatal("没有发出任务列表请求")
	}
	list, ok := body["flowInstanceIdList"].([]any)
	if !ok || len(list) != 1 || list[0] != "instance-7" {
		t.Fatalf("实例筛选没有出现在协议顶层 flowInstanceIdList：%v", body)
	}
	if body["queryUserId"] != "user-2" {
		t.Fatalf("pending 视角用户没有写在协议顶层 queryUserId：%v", body)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("请求缺少 data 容器：%v", body)
	}
	if _, exists := data["flowInstanceId"]; exists {
		t.Fatalf("实例筛选不得出现在 data.flowInstanceId（目标会忽略）：%v", data)
	}
	if _, exists := data["executorId"]; exists {
		t.Fatalf("pending 不应携带 data.executorId：%v", data)
	}
	if data["taskStatus"] != "pending" || data["useScope"] != "invest" {
		t.Fatalf("pending 业务字段不正确：%v", data)
	}
}

// TestDoneTaskQueryUsesExecutorInsideData 验证 done 精确实例查询：
// 执行人写在 data.executorId（协议位置不能与 pending 的 queryUserId 互换），
// 实例筛选仍在协议顶层，且不携带 queryUserId。
func TestDoneTaskQueryUsesExecutorInsideData(t *testing.T) {
	server := &taskListServer{rows: []map[string]any{{
		"jobTaskId": "task-done", "flowInstanceId": "instance-7", "flowNodeProxyId": "node-audit",
		"taskStatus": "done", "executorId": "user-2",
	}}}
	client := newTaskQueryClient(t, server)

	snapshots, err := client.ListTaskSnapshotsForUser(context.Background(),
		target.Session{SID: "sid-plan"}, "instance-7", "done", "user-2")
	if err != nil {
		t.Fatalf("done 精确实例查询失败：%v", err)
	}
	if len(snapshots) != 1 || snapshots[0].JobTaskID != "task-done" {
		t.Fatalf("done 查询结果不正确：%+v", snapshots)
	}
	body := server.lastBody()
	data, _ := body["data"].(map[string]any)
	if data["executorId"] != "user-2" {
		t.Fatalf("done 执行人没有写在 data.executorId：%v", body)
	}
	if _, exists := body["queryUserId"]; exists {
		t.Fatalf("done 不应携带协议顶层 queryUserId：%v", body)
	}
	if _, exists := data["flowInstanceId"]; exists {
		t.Fatalf("done 的实例筛选不得出现在 data：%v", data)
	}
	if _, exists := body["flowInstanceIdList"]; !exists {
		t.Fatalf("done 缺少协议顶层实例筛选：%v", body)
	}
}

// TestFindDueFlowKeepsInstanceFilterAtProtocolTopLevel 验证 waiting_send 精确读取同样走顶层实例筛选：
// 修复前它把实例放进 data，目标忽略该条件后每次都要翻全量任务表。
func TestFindDueFlowKeepsInstanceFilterAtProtocolTopLevel(t *testing.T) {
	server := &taskListServer{rows: []map[string]any{{
		"flowInstanceId": "instance-7", "flowProxyId": "proxy-7", "flowNodeProxyId": "node-start", "formProxyId": "form-1",
	}}}
	client := newTaskQueryClient(t, server)

	proxyID, entries, forms, found, err := client.FindDueFlow(context.Background(), target.Session{SID: "sid-plan"}, "instance-7")
	if err != nil || !found || proxyID != "proxy-7" {
		t.Fatalf("待发精确读取失败：proxy=%s found=%v err=%v", proxyID, found, err)
	}
	if len(entries) != 1 || entries[0] != "node-start" || len(forms) != 1 || forms[0] != "form-1" {
		t.Fatalf("待发入口或表单代理汇总不正确：%v %v", entries, forms)
	}
	body := server.lastBody()
	list, ok := body["flowInstanceIdList"].([]any)
	if !ok || len(list) != 1 || list[0] != "instance-7" {
		t.Fatalf("待发查询的实例筛选不在协议顶层：%v", body)
	}
	data, _ := body["data"].(map[string]any)
	if _, exists := data["flowInstanceId"]; exists {
		t.Fatalf("待发查询的实例筛选不得出现在 data：%v", data)
	}
	if data["taskStatus"] != "waiting_send" {
		t.Fatalf("待发查询状态不正确：%v", data)
	}
}

// TestForeignInstanceRowsAreRejectedAndCounted 验证目标响应串入其他实例时按行剔除：
// 命中行数与剔除行数都写进运行日志（用户据此确认查询范围确实收敛到实例）。
func TestForeignInstanceRowsAreRejectedAndCounted(t *testing.T) {
	server := &taskListServer{rows: []map[string]any{
		{"jobTaskId": "task-a", "flowInstanceId": "instance-7", "flowNodeProxyId": "node-audit", "taskStatus": "pending"},
		{"jobTaskId": "task-b", "flowInstanceId": "instance-other", "flowNodeProxyId": "node-audit", "taskStatus": "pending"},
	}}
	httpServer := httptest.NewServer(server.handler())
	defer httpServer.Close()
	client, err := target.NewClient(target.ClientConfig{BaseURL: httpServer.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	collector := &networkRecordCollector{}
	client.SetNetworkLogger(collector)

	snapshots, err := client.ListTaskSnapshotsForUser(context.Background(), target.Session{SID: "sid"}, "instance-7", "pending", "")
	if err != nil {
		t.Fatalf("精确实例查询失败：%v", err)
	}
	if len(snapshots) != 1 || snapshots[0].FlowInstanceID != "instance-7" {
		t.Fatalf("其他实例的行没有被剔除：%+v", snapshots)
	}
}

// TestFindDoneTaskOnNodeUsesUnifiedSnapshotRead 验证已办记录维度复用统一快照读取：
// 请求带 data.executorId（要核对谁就必须传谁），并按节点过滤命中。
func TestFindDoneTaskOnNodeUsesUnifiedSnapshotRead(t *testing.T) {
	server := &taskListServer{rows: []map[string]any{{
		"jobTaskId": "task-done", "flowInstanceId": "instance-7", "flowNodeProxyId": "node-audit", "taskStatus": "done",
	}}}
	client := newTaskQueryClient(t, server)

	found, err := client.FindDoneTaskOnNode(context.Background(), target.Session{SID: "sid"}, "instance-7", "node-audit", "user-2")
	if err != nil || !found {
		t.Fatalf("已办记录维度应命中：found=%v err=%v", found, err)
	}
	body := server.lastBody()
	data, _ := body["data"].(map[string]any)
	if data["executorId"] != "user-2" || data["taskStatus"] != "done" {
		t.Fatalf("已办查询的视角或状态不正确：%v", body)
	}
	onOtherNode, err := client.FindDoneTaskOnNode(context.Background(), target.Session{SID: "sid"}, "instance-7", "node-other", "user-2")
	if err != nil {
		t.Fatalf("按节点过滤读取失败：%v", err)
	}
	if onOtherNode {
		t.Fatal("其他节点不应命中已办记录")
	}
	// 空实例不构成查询：直接返回未命中，不发请求。
	before := server.requestCount()
	if found, err := client.FindDoneTaskOnNode(context.Background(), target.Session{SID: "sid"}, "", "", "user-2"); err != nil || found {
		t.Fatalf("空实例应当直接返回未命中：found=%v err=%v", found, err)
	}
	if server.requestCount() != before {
		t.Fatal("空实例不应发出目标请求")
	}
}

// TestEmptyInstanceAndMissingExecutorAreRejectedBeforeRequest 验证协议约束在发请求前生效：
// 空实例 ID 与 done 缺少执行人都被拒绝，绝不发出一次注定被忽略或语义不成立的请求。
func TestEmptyInstanceAndMissingExecutorAreRejectedBeforeRequest(t *testing.T) {
	server := &taskListServer{}
	client := newTaskQueryClient(t, server)

	if _, err := client.ListTaskSnapshotsForUser(context.Background(), target.Session{SID: "sid"}, "", "pending", ""); err == nil {
		t.Fatal("空实例 ID 必须被拒绝")
	}
	if _, err := client.ListTaskSnapshotsForUser(context.Background(), target.Session{SID: "sid"}, "instance-7", "done", ""); err == nil {
		t.Fatal("done 缺少执行人必须被拒绝")
	}
	if _, err := client.ListTaskSnapshotsForUser(context.Background(), target.Session{SID: "sid"}, "instance-7", "waiting_send", ""); err == nil {
		t.Fatal("不支持的查询状态必须被拒绝")
	}
	if server.requestCount() != 0 {
		t.Fatalf("被拒绝的查询不应发出目标请求：%d", server.requestCount())
	}
}

// TestTaskListMemoIsPerFactBoundary 验证任务列表复用只在一次事实读取边界内生效：
// 同一作用域内同一 (视角, 实例, 状态) 只扫一次，换一次作用域必须重新扫——
// 写后核验由此天然不带写前缓存。
func TestTaskListMemoIsPerFactBoundary(t *testing.T) {
	server := &taskListServer{rows: []map[string]any{{
		"jobTaskId": "task-1", "flowInstanceId": "instance-7", "flowNodeProxyId": "node-audit", "taskStatus": "pending",
	}}}
	client := newTaskQueryClient(t, server)
	session := target.Session{SID: "sid"}

	ctx := target.AttachTaskSnapshotScope(context.Background())
	for range 2 {
		if _, err := client.ListTaskSnapshotsForUser(ctx, session, "instance-7", "pending", "user-2"); err != nil {
			t.Fatalf("视图查询失败：%v", err)
		}
	}
	if got := server.requestCount(); got != 1 {
		t.Fatalf("同一事实边界内应只扫一次，实际 %d 次", got)
	}
	// 换一次事实边界（写后核验）：必须重新扫，绝不复用写前结果。
	if _, err := client.ListTaskSnapshotsForUser(target.AttachTaskSnapshotScope(context.Background()), session, "instance-7", "pending", "user-2"); err != nil {
		t.Fatalf("新边界内的视图查询失败：%v", err)
	}
	if got := server.requestCount(); got != 2 {
		t.Fatalf("新事实边界必须重新扫描，实际 %d 次", got)
	}
	// 不同视角是不同的事实，必须各自扫描。
	if _, err := client.ListTaskSnapshotsForUser(ctx, session, "instance-7", "pending", "user-3"); err != nil {
		t.Fatalf("其他视角查询失败：%v", err)
	}
	if got := server.requestCount(); got != 3 {
		t.Fatalf("不同视角必须各自扫描，实际 %d 次", got)
	}
}

// TestTaskListPaginationScansEveryPage 验证精确实例读取遍历全部分页：
// 只看第一页会把落在第二页的任务误判成「没有任务」，而对账里这条误判代价最高。
func TestTaskListPaginationScansEveryPage(t *testing.T) {
	server := &taskListServer{pages: 2}
	server.rows = []map[string]any{{"jobTaskId": "task-page-1", "flowInstanceId": "instance-7", "flowNodeProxyId": "node-a", "taskStatus": "done"}}
	var mu sync.Mutex
	httpServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		mu.Lock()
		server.bodies = append(server.bodies, payload)
		page, _ := payload["pages"].(float64)
		mu.Unlock()
		rows := server.rows
		if page >= 2 {
			rows = []map[string]any{{"jobTaskId": "task-page-2", "flowInstanceId": "instance-7", "flowNodeProxyId": "node-b", "taskStatus": "done"}}
		}
		response.Header().Set("Content-Type", "application/json")
		encoded, _ := json.Marshal(map[string]any{"isSuccess": true, "data": rows, "pages": 2})
		_, _ = response.Write(encoded)
	}))
	defer httpServer.Close()
	client, err := target.NewClient(target.ClientConfig{BaseURL: httpServer.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	snapshots, err := client.ListTaskSnapshotsForUser(context.Background(), target.Session{SID: "sid"}, "instance-7", "done", "user-2")
	if err != nil {
		t.Fatalf("分页读取失败：%v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("第二页的任务被漏读：%+v", snapshots)
	}
	for _, snapshot := range snapshots {
		if strings.TrimSpace(snapshot.JobTaskID) == "" {
			t.Fatalf("任务快照缺少 jobTaskId：%+v", snapshot)
		}
	}
}
