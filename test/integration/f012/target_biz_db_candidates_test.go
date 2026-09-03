package f012_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// TestTargetBizDBCandidatesExcludeAutomationAndSearch 用真实目标业务库验证快速候选查询：
// 只读关键列、不限制发起人、剔除历史自动化实例、支持搜索并给出真实总数。
// 未配置 TARGET_DB_* 时跳过：该连接是可选加速路径，缺失时候选仍回落到目标只读 API。
func TestTargetBizDBCandidatesExcludeAutomationAndSearch(t *testing.T) {
	bizConfig := config.LoadTargetBizDBConfig()
	if !bizConfig.Enabled() {
		t.Skip("未配置目标业务库只读连接，跳过快速候选查询集成用例")
	}
	targetConfig := config.LoadTargetConfig()
	if strings.TrimSpace(targetConfig.CustomerCode) == "" {
		t.Skip("未配置目标客户编码，无法执行租户内候选查询")
	}
	flowName := strings.TrimSpace(os.Getenv("F012_BIZ_DB_FLOW_NAME"))
	if flowName == "" {
		flowName = "员工请假单（智慧斯能）"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := planmysql.NewTargetHistoryRepository(ctx, bizConfig, targetConfig.CustomerCode)
	if err != nil {
		t.Fatalf("打开目标业务库只读连接失败：%v", err)
	}
	defer store.Close()

	all, allTotal, err := store.TargetHistoryCandidates(ctx, repository.TargetHistoryCandidateFilter{FlowName: flowName}, 1, 20)
	if err != nil {
		t.Fatalf("读取候选失败：%v", err)
	}
	if allTotal == 0 || len(all) == 0 {
		t.Skipf("目标业务库中没有 %s 的实例，无法验证候选查询", flowName)
	}
	clean, cleanTotal, err := store.TargetHistoryCandidates(ctx, repository.TargetHistoryCandidateFilter{
		FlowName: flowName, ExcludeNameKeywords: []string{"自动"},
	}, 1, 20)
	if err != nil {
		t.Fatalf("剔除自动化实例后读取候选失败：%v", err)
	}
	if cleanTotal >= allTotal {
		t.Fatalf("剔除关键词没有生效：全部=%d 剔除后=%d", allTotal, cleanTotal)
	}
	initiators := map[string]bool{}
	for _, row := range clean {
		if strings.Contains(row.Name, "自动") {
			t.Fatalf("候选仍包含历史自动化实例：%s", row.Name)
		}
		if strings.TrimSpace(row.InstanceID) == "" || strings.TrimSpace(row.CreatedAt) == "" {
			t.Fatalf("候选缺少列表必需的关键列：%+v", row)
		}
		if row.FlowName != flowName {
			t.Fatalf("候选流程身份不一致：%+v", row)
		}
		initiators[row.InitiatorName] = true
	}
	if len(clean) > 1 && len(initiators) < 2 {
		t.Logf("当前页发起人只有 %d 位，无法证明跨发起人返回；已按不限制发起人查询", len(initiators))
	}
	for index := 1; index < len(clean); index++ {
		if clean[index-1].Status != "end" && clean[index].Status == "end" {
			t.Fatalf("已完成实例没有整体优先：第 %d 条=%s 第 %d 条=%s", index-1, clean[index-1].Status, index, clean[index].Status)
		}
	}

	keyword := strings.TrimSpace(clean[0].InitiatorName)
	if keyword == "" {
		keyword = strings.TrimSpace(clean[0].Name)
	}
	searched, searchTotal, err := store.TargetHistoryCandidates(ctx, repository.TargetHistoryCandidateFilter{
		FlowName: flowName, Query: keyword, ExcludeNameKeywords: []string{"自动"},
	}, 1, 20)
	if err != nil {
		t.Fatalf("按搜索词读取候选失败：%v", err)
	}
	if searchTotal == 0 || len(searched) == 0 {
		t.Fatalf("搜索 %q 没有命中任何候选", keyword)
	}
	if searchTotal > cleanTotal {
		t.Fatalf("搜索结果总数超过未搜索总数：搜索=%d 未搜索=%d", searchTotal, cleanTotal)
	}
	for _, row := range searched {
		if !strings.Contains(row.Name, keyword) && !strings.Contains(row.InitiatorName, keyword) && !strings.Contains(row.CompanyName, keyword) {
			t.Fatalf("搜索结果与搜索词无关：keyword=%q row=%+v", keyword, row)
		}
	}

	// 通配符必须被转义，不能被当作模式匹配全部实例。
	wildcard, wildcardTotal, err := store.TargetHistoryCandidates(ctx, repository.TargetHistoryCandidateFilter{
		FlowName: flowName, Query: "%", ExcludeNameKeywords: []string{"自动"},
	}, 1, 20)
	if err != nil {
		t.Fatalf("按通配符搜索候选失败：%v", err)
	}
	if wildcardTotal == cleanTotal && len(wildcard) == len(clean) && cleanTotal > 0 {
		t.Fatalf("搜索词中的通配符没有被转义：总数=%d", wildcardTotal)
	}
}
