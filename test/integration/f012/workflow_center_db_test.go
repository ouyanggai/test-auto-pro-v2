package f012

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// TestTargetHistoryRepositoryAgainstWorkflowCenter 验证新目标库 workflow-center 的各项直连只读查询功能正常。
func TestTargetHistoryRepositoryAgainstWorkflowCenter(t *testing.T) {
	cfg := config.LoadTargetBizDBConfig()
	if !cfg.Enabled() {
		t.Skip("未启用目标业务库只读配置，跳过测试")
	}

	targetConfig := config.LoadTargetConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo, err := planmysql.NewTargetHistoryRepository(ctx, cfg, targetConfig.CustomerCode, targetConfig.PlatformCode)
	if err != nil {
		t.Fatalf("连接新库失败: %v", err)
	}
	defer repo.Close()

	// 1. 验证历史实例候选查询
	candidates, total, err := repo.TargetHistoryCandidates(ctx, repository.TargetHistoryCandidateFilter{}, 1, 10)
	if err != nil {
		t.Fatalf("TargetHistoryCandidates 失败: %v", err)
	}
	t.Logf("成功从 %s 查询到历史实例总数: %d, 当前页条数: %d", cfg.Name, total, len(candidates))
	if len(candidates) == 0 {
		t.Fatalf("候选实例列表为空")
	}
	t.Logf("首条实例: ID=%s, 标题=%q, 流程名=%q, 发起人=%q",
		candidates[0].InstanceID, candidates[0].Name, candidates[0].FlowName, candidates[0].InitiatorName)

	// 2. 验证用户检索
	users, err := repo.SearchUsers(ctx, "张", 5)
	if err != nil {
		t.Fatalf("SearchUsers 失败: %v", err)
	}
	t.Logf("SearchUsers('张') 返回 %d 位用户", len(users))

	// 3. 验证停用人员查询
	disabledIDs, err := repo.ListDisabledUserIDs(ctx)
	if err != nil {
		t.Fatalf("ListDisabledUserIDs 失败: %v", err)
	}
	t.Logf("ListDisabledUserIDs 返回 %d 个停用用户ID", len(disabledIDs))

	// 4. 验证自动排除人员列表
	excluded, err := repo.ListAutoExcludedUsers(ctx, "")
	if err != nil {
		t.Fatalf("ListAutoExcludedUsers 失败: %v", err)
	}
	t.Logf("ListAutoExcludedUsers 返回 %d 位被排除人员", len(excluded))
}
