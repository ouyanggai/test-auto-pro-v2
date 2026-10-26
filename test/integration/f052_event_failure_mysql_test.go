package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// TestF052LongErrorFinishesRun 验证真实 MySQL 的标题边界与失败事务；完整错误、原有详情和运行事实必须同时保留。
func TestF052LongErrorFinishesRun(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	long := "执行失败：" + strings.Repeat("中文错误🙂", 90)
	cases := []struct{ label, detail, wantDetail string }{
		{"短标题", "原始详情", "原始详情"},
		{strings.Repeat("界", 255), "边界详情", "边界详情"},
		{long, "", long},
		{long, "已有详情：请求未发出", long + "\n\n已有详情：请求未发出"},
		{long, "原始全文：" + long, "原始全文：" + long},
	}
	for i, c := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			planID := insertF032Plan(t, store.db.DB, fmt.Sprintf("f052-%d", i))
			run, paths, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{PlanID: planID, ExecutionPathIDs: []uint64{157}, Mode: model.RunModeAuto, Trigger: model.RunTriggerManual})
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			if _, err = store.runs.AdvanceRunStatus(ctx, run.ID, model.RunStatusPending, model.RunStatusRunning, model.RunEvent{Label: "运行开始"}, now); err != nil {
				t.Fatal(err)
			}
			path := paths[0]
			if _, err = store.runs.AdvancePathRunStatus(ctx, path.ID, model.PathRunStatusWaiting, model.PathRunStatusRunning, model.RunEvent{Label: "开始执行"}, now); err != nil {
				t.Fatal(err)
			}
			if _, err = store.runs.ClaimPathRunLease(ctx, path.ID, "test-worker", time.Minute, now); err != nil {
				t.Fatal(err)
			}
			result := model.RunResultFailed
			finished, err := store.runs.FinishPathRun(ctx, path.ID, model.PathRunStatusFailed, &result, nil, model.RunEvent{Kind: "path_run_finished", Label: c.label, Detail: c.detail}, now.Add(time.Second))
			if err != nil {
				t.Fatalf("长错误不能阻断收尾：%v", err)
			}
			if finished.Status != model.PathRunStatusFailed || finished.LeaseOwner != "" || finished.LeaseExpiresAt != nil || finished.FinishedAt == nil {
				t.Fatalf("失败状态或租约未收尾：%+v", finished)
			}
			after, err := store.runs.GetRun(ctx, run.ID)
			if err != nil || after.Status != model.RunStatusFailed || after.FinishedAt == nil {
				t.Fatalf("聚合失败未提交：%+v %v", after, err)
			}
			var label, detail, planStatus string
			if err := store.db.DB.QueryRowContext(ctx, "SELECT label, COALESCE(detail, '') FROM run_events WHERE path_run_id = ? AND kind = 'path_run_finished'", path.ID).Scan(&label, &detail); err != nil {
				t.Fatal(err)
			}
			if !utf8.ValidString(label) || utf8.RuneCountInString(label) > 255 || detail != c.wantDetail {
				t.Fatalf("原文丢失或标题越界：字数=%d 详情相等=%v", utf8.RuneCountInString(label), detail == c.wantDetail)
			}
			if utf8.RuneCountInString(c.label) <= 255 && label != c.label {
				t.Fatal("短标题或255字边界不应改变")
			}
			if utf8.RuneCountInString(c.label) > 255 && label != string([]rune(c.label)[:254])+"…" {
				t.Fatal("长标题应保留可读摘要")
			}
			if err := store.db.DB.QueryRowContext(ctx, "SELECT status FROM test_plans WHERE id = ?", planID).Scan(&planStatus); err != nil {
				t.Fatal(err)
			}
			if planStatus != string(model.PlanStatusCompleted) {
				t.Fatalf("计划状态未同步：%s", planStatus)
			}
		})
	}
}
