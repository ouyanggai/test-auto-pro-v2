package history_replay_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

// bizCandidateStore 是目标业务库只读候选来源的假实现，记录服务端下推的查询条件。
type bizCandidateStore struct {
	filters []repository.TargetHistoryCandidateFilter
	rows    []repository.TargetHistoryCandidateRow
	total   int
}

// TargetHistoryCandidates 返回固定关键列，并按调用方请求的窗口切片。
func (s *bizCandidateStore) TargetHistoryCandidates(_ context.Context, filter repository.TargetHistoryCandidateFilter, page, pageSize int) ([]repository.TargetHistoryCandidateRow, int, error) {
	s.filters = append(s.filters, filter)
	start := (page - 1) * pageSize
	if start >= len(s.rows) {
		return []repository.TargetHistoryCandidateRow{}, s.total, nil
	}
	end := start + pageSize
	if end > len(s.rows) {
		end = len(s.rows)
	}
	return append([]repository.TargetHistoryCandidateRow(nil), s.rows[start:end]...), s.total, nil
}

// TestHistoryCandidatesUseBizDBSourceWithoutTargetAPI 验证配置业务库来源后：
// 候选只来自一次关键列查询、搜索词与自动化剔除条件下推到查询、且完全不需要目标只读 API 会话。
func TestHistoryCandidatesUseBizDBSourceWithoutTargetAPI(t *testing.T) {
	store := &bizCandidateStore{total: 54, rows: []repository.TargetHistoryCandidateRow{
		{InstanceID: "instance-end", Name: "员工请假单（智慧斯能）-王璐-事假-1天", FlowName: "员工请假单（智慧斯能）", FormName: "员工请假单（智慧斯能）",
			FlowCode: "flow-code", FlowProxyID: "proxy-end", FormProxyID: "form-proxy-end", Status: "end",
			InitiatorName: "王璐", CompanyName: "广东斯能投资有限责任公司", CreatedAt: "2026-02-03 20:30:03"},
		{InstanceID: "instance-run", Name: "员工请假单（智慧斯能）-张智皓-事假-1天", FlowName: "员工请假单（智慧斯能）", FormName: "员工请假单（智慧斯能）",
			FlowCode: "flow-code", FlowProxyID: "proxy-run", FormProxyID: "form-proxy-run", Status: "run",
			InitiatorName: "张智皓", CompanyName: "广东斯能投资有限责任公司", CreatedAt: "2026-01-22 11:39:09"},
	}}
	// 目标只读客户端故意留空：走业务库来源时不允许触发任何目标 API 会话。
	reader := service.NewTargetReadService(config.TargetConfig{})
	reader.SetHistoryCandidateStore(store)
	page, err := reader.HistoryCandidates(context.Background(), "account-a", "flow-code", "员工请假单（智慧斯能）", "员工请假单（智慧斯能）", "王璐", 1, 20)
	if err != nil {
		t.Fatalf("业务库候选读取失败：%v", err)
	}
	if len(page.Items) != 2 || page.Total != 54 || !page.HasMore {
		t.Fatalf("业务库候选没有返回关键列与真实总数：items=%d total=%d hasMore=%v", len(page.Items), page.Total, page.HasMore)
	}
	if page.Items[0].StatusName != "完结" || page.Items[0].Initiator != "王璐" || page.Items[0].CompanyName == "" {
		t.Fatalf("候选摘要字段没有从业务库关键列补齐：%+v", page.Items[0])
	}
	if len(store.filters) != 1 {
		t.Fatalf("一页候选应只发起一次业务库查询：%d", len(store.filters))
	}
	filter := store.filters[0]
	if filter.Query != "王璐" || filter.FlowName != "员工请假单（智慧斯能）" || filter.FormName != "员工请假单（智慧斯能）" || filter.FlowCode != "flow-code" {
		t.Fatalf("搜索词或身份条件没有下推到业务库查询：%+v", filter)
	}
	if len(filter.ExcludeNameKeywords) == 0 || !strings.Contains(strings.Join(filter.ExcludeNameKeywords, ","), "自动") {
		t.Fatalf("历史自动化实例没有在查询层被剔除：%+v", filter.ExcludeNameKeywords)
	}
	if key := service.HistoryCandidateKey("account-a", page.Items[0]); key == "" {
		t.Fatal("业务库候选必须仍能生成不透明候选键")
	}
}

// TestHistoryCandidatesRejectAutomationRowsOnAPIFallback 验证目标只读 API 回落路径同样剔除自动化实例。
func TestHistoryCandidatesRejectAutomationRowsOnAPIFallback(t *testing.T) {
	fixture := newTargetHistoryFixture(t, false)
	fixture.automationRow = true
	reader, server := newTargetHistoryReader(t, fixture)
	defer server.Close()
	page, err := reader.HistoryCandidates(context.Background(), "account-a", fixture.flowCode, fixture.formName, fixture.flowName, "", 1, 20)
	if err != nil {
		t.Fatalf("读取候选失败：%v", err)
	}
	for _, item := range page.Items {
		if strings.Contains(item.BusinessSummary, "自动") {
			t.Fatalf("目标只读 API 回落路径没有剔除历史自动化实例：%+v", item)
		}
	}
	if len(page.Items) != 2 {
		t.Fatalf("剔除自动化实例后候选数量不正确：%+v", page.Items)
	}
}
