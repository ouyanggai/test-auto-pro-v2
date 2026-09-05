package integration

import (
	"context"
	"strings"
	"testing"
	"time"
)

// f018ToolCreatedInstance 是本工具真实发起、用于锁定实例可见性口径的实例。
// 它由 F-018 收口时的真实写产生（运行 13，2026-09-05，计划 11 路径 1121，账号骆蒙恩），
// 关键特征是：本工具的发起载荷不带任何业务关联（flowInstanceBizRelevanceList），
// 这正是暴露过滤问题的条件。若该实例被人工删除导致用例失败，
// 换一条同样由本工具发起的实例即可（path_runs.main_instance_ref 里能查到）。
const f018ToolCreatedInstance = "7bcf0c29f1054ba0bed5cf367ba2f2d2"

// TestF018ToolCreatedInstanceIsVisibleByExactLookup 锁定 F-016 遗留「实例可见性」问题的修复：
// 按实例 ID 精确复查事实时，必须能读到本工具自己发起的实例。
//
// 为什么必须有这条用例：读不到的后果不是少一行日志，而是把"确实已生效的发起"判成不确定，
// 让路径运行停进待对账，并让对账五维全部读不到、永远只能得出仍无法判定。
// F-016 首次真实写（实例 caf2046d…）与 F-018 收口前的运行 12（实例 6bd617f3…）都是这样被误判的。
func TestF018ToolCreatedInstanceIsVisibleByExactLookup(t *testing.T) {
	_, client, session := requireF014Session(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	proxyID, entries, status, _, found, err := client.FindSubmittedFlow(ctx, session, f018ToolCreatedInstance)
	if err != nil {
		t.Fatalf("按实例精确复查失败：%v", err)
	}
	if !found {
		t.Fatalf("本工具发起的实例 %s 必须能按 ID 精确复查到；读不到会让核验重读把已生效的发起判成不确定",
			f018ToolCreatedInstance)
	}
	if strings.TrimSpace(status) == "" || strings.TrimSpace(proxyID) == "" {
		t.Fatalf("精确复查必须带回状态与流程代理：status=%q proxy=%q", status, proxyID)
	}
	t.Logf("精确复查命中：状态=%s 流程代理=%s 当前节点入口=%v", status, proxyID, entries)
}

// TestF018CompanyRelevanceFilterHidesToolCreatedInstance 把根因固定成可执行证据：
// 同一条实例、同一个端点，只要在查询里加回「公司业务关联」过滤就读不到。
// 这条用例的作用是防止有人"顺手"把过滤加回 FindSubmittedFlow——那会把可见性问题原样带回来。
func TestF018CompanyRelevanceFilterHidesToolCreatedInstance(t *testing.T) {
	clientConfig, _, session := requireF014Session(t)
	baseData := map[string]any{
		"useScope":     "invest",
		"auditWayList": []string{},
		"statusList":   []string{"draft", "await_sent", "run", "withdraw", "termination", "abandon", "rejected", "end"},
	}
	withoutFilter := map[string]any{
		"data": baseData, "ids": []string{f018ToolCreatedInstance}, "pagination": true, "pages": 1, "size": 100,
	}
	status, body, err := f014Post(clientConfig.BaseURL, "/web/flowInstanceApi/list", session.SID, withoutFilter, clientConfig.Timeout)
	if err != nil || status != 200 {
		t.Fatalf("不带业务关联过滤的查询失败：status=%d err=%v", status, err)
	}
	if !strings.Contains(body, f018ToolCreatedInstance) {
		t.Fatalf("不带业务关联过滤时应当命中实例：%s", truncateBody(body))
	}

	filtered := map[string]any{}
	for key, value := range baseData {
		filtered[key] = value
	}
	filtered["flowInstanceBizRelevanceList"] = []map[string]any{{"otherBiz": "company", "otherBizId": ""}}
	withFilter := map[string]any{
		"data": filtered, "ids": []string{f018ToolCreatedInstance}, "pagination": true, "pages": 1, "size": 100,
	}
	status, body, err = f014Post(clientConfig.BaseURL, "/web/flowInstanceApi/list", session.SID, withFilter, clientConfig.Timeout)
	if err != nil || status != 200 {
		t.Fatalf("带业务关联过滤的查询失败：status=%d err=%v", status, err)
	}
	if strings.Contains(body, f018ToolCreatedInstance) {
		t.Fatal("目标行为已变化：带公司业务关联过滤时居然命中了本工具发起的实例，需要重新勘定语义清单第 19 条")
	}
	t.Logf("根因复现：加回公司业务关联过滤后返回空集（正文长度 %d）", len(body))
}
