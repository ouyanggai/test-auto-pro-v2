package history_replay_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/service"
)

// candidatePagingFixture 模拟真实账号下同一流程已有数千条实例、且实例行不携带 flowCode 的目标环境。
type candidatePagingFixture struct {
	t         *testing.T
	mu        sync.Mutex
	listCalls int
	flowName  string
	formName  string
	endRows   int
	runRows   int
}

// handler 只实现登录与实例列表，并真实应用 statusList 与分页参数。
func (f *candidatePagingFixture) handler(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/web/user/api/login/user/login":
		writeHistoryTargetJSON(response, map[string]any{
			"isSuccess": true, "sid": "sid-private",
			"data": map[string]any{
				"user":      map[string]any{"id": "user-current", "name": "当前用户", "customerCode": "customer", "departmentId": "department"},
				"companyVo": map[string]any{"id": "company-current", "name": "测试公司", "customerCode": "customer"},
			},
		})
	case "/web/flowInstanceApi/list":
		body := decodeHistoryTargetBody(f.t, request)
		f.mu.Lock()
		f.listCalls++
		f.mu.Unlock()
		data, _ := body["data"].(map[string]any)
		rows := f.rowsForStatuses(stringSlice(data["statusList"]))
		page, size := intValue(body["pages"], 1), intValue(body["size"], 100)
		start := (page - 1) * size
		if start > len(rows) {
			start = len(rows)
		}
		end := start + size
		if end > len(rows) {
			end = len(rows)
		}
		writeHistoryTargetJSON(response, map[string]any{
			"isSuccess": true, "total": len(rows), "pages": page, "current": page, "size": size,
			"data": rows[start:end],
		})
	default:
		http.NotFound(response, request)
	}
}

// rowsForStatuses 按目标状态枚举返回该分组的原始行；所有行都不携带 flowCode。
func (f *candidatePagingFixture) rowsForStatuses(statuses []string) []any {
	wanted := map[string]bool{}
	for _, status := range statuses {
		wanted[status] = true
	}
	rows := make([]any, 0, f.endRows+f.runRows)
	if wanted["end"] {
		for index := 0; index < f.endRows; index++ {
			rows = append(rows, f.row("end-"+strconv.Itoa(index), "end", "2026-08-31 10:00:0"+strconv.Itoa(index%10)))
		}
	}
	if wanted["run"] {
		for index := 0; index < f.runRows; index++ {
			rows = append(rows, f.row("run-"+strconv.Itoa(index), "run", "2026-09-01 09:00:0"+strconv.Itoa(index%10)))
		}
	}
	return rows
}

// row 构造一条与计划身份同名、但没有 flowCode 的目标实例行。
func (f *candidatePagingFixture) row(id, status, createdAt string) map[string]any {
	return map[string]any{
		"id": id, "flowProxyId": "proxy-" + id, "formProxyId": "form-proxy-" + id,
		"flowName": f.flowName, "formName": f.formName, "name": "业务数据 " + id,
		"status": status, "createDate": createdAt, "createrId": "user-current", "companyId": "company-current",
	}
}

// intValue 读取假网关请求中的分页整数，缺失时使用调用方默认值。
func intValue(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	}
	return fallback
}

// TestHistoryCandidatesFindInstancesWithoutFlowCodeAndBoundReads 锁定真实环境回归：
// 目标实例列表行不返回 flowCode，候选必须仍能命中，且账号历史达到数千条时读取次数保持有界。
func TestHistoryCandidatesFindInstancesWithoutFlowCodeAndBoundReads(t *testing.T) {
	fixture := &candidatePagingFixture{t: t, flowName: "员工请假单（测试公司）", formName: "员工请假单（测试公司）", endRows: 5, runRows: 2082}
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	defer server.Close()
	client, err := target.NewClient(target.ClientConfig{
		BaseURL: server.URL, LoginPassword: "password", LoginAESKey: "0123456789abcdef", LoginCode: "code",
		PlatformCode: "invest", CustomerCode: "customer", Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建目标测试客户端失败：%v", err)
	}
	reader := service.NewTargetReadServiceWithClient(client, time.Hour)
	page, err := reader.HistoryCandidates(context.Background(), "account-a", "653a5a1170da4f9faca9afab96d32649", fixture.formName, fixture.flowName, 1, 20)
	if err != nil {
		t.Fatalf("读取业务数据候选失败：%v", err)
	}
	if len(page.Items) != 20 || page.Total != fixture.endRows+fixture.runRows || !page.HasMore {
		t.Fatalf("候选分页没有反映目标真实总数：items=%d total=%d hasMore=%v", len(page.Items), page.Total, page.HasMore)
	}
	for index := 0; index < fixture.endRows; index++ {
		if page.Items[index].Status != "end" {
			t.Fatalf("已完成实例没有整体优先返回：第 %d 条状态=%s", index, page.Items[index].Status)
		}
	}
	if page.Items[fixture.endRows].Status != "run" {
		t.Fatalf("已完成实例之后没有接续其他状态：%+v", page.Items[fixture.endRows])
	}
	fixture.mu.Lock()
	firstPageCalls := fixture.listCalls
	fixture.mu.Unlock()
	if firstPageCalls > 4 {
		t.Fatalf("读取一页候选发起了过多目标列表请求：%d", firstPageCalls)
	}
	third, err := reader.HistoryCandidates(context.Background(), "account-a", "", fixture.formName, fixture.flowName, 3, 20)
	if err != nil {
		t.Fatalf("读取第三页候选失败：%v", err)
	}
	if len(third.Items) != 20 || third.Items[0].ID == page.Items[0].ID {
		t.Fatalf("候选分页没有按窗口推进：items=%d 首条=%+v", len(third.Items), third.Items[0])
	}
	key := service.HistoryCandidateKey("account-a", page.Items[0])
	if key == "" {
		t.Fatal("候选键不能为空")
	}
}

// TestHistoryCandidatesRejectConflictingFlowCode 验证目标确实写入了 flowCode 时仍按精确值排除其他流程实例。
func TestHistoryCandidatesRejectConflictingFlowCode(t *testing.T) {
	fixture := newTargetHistoryFixture(t, false)
	reader, server := newTargetHistoryReader(t, fixture)
	defer server.Close()
	page, err := reader.HistoryCandidates(context.Background(), "account-a", fixture.flowCode, fixture.formName, fixture.flowName, 1, 20)
	if err != nil {
		t.Fatalf("读取业务数据候选失败：%v", err)
	}
	for _, item := range page.Items {
		if item.FlowCode != "" && item.FlowCode != fixture.flowCode {
			t.Fatalf("候选包含了 flowCode 冲突的其他流程实例：%+v", item)
		}
		if item.FormName != fixture.formName {
			t.Fatalf("候选包含了其他表单实例：%+v", item)
		}
	}
	if len(page.Items) != 2 {
		t.Fatalf("目标原字段过滤结果不正确：%+v", page.Items)
	}
}
