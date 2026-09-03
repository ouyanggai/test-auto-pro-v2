package f012_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// TestHistoryReplayJobKeepsPerPathTerminalStatesForBothPayloads 验证一个任务内 FormMaking 与自定义表单两类载荷
// 各自独立落盘终态，单项失败不回滚已完成项，明细终态与路径配置在同一事务内更新。
func TestHistoryReplayJobKeepsPerPathTerminalStatesForBothPayloads(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "两类载荷回放任务事务")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	planID, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20701, "template-a", "new", 2)
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

	seedHistoryPathConfig(t, store, ctx, pathIDs[0], historyIntegrationKey(20702), now)
	seedHistoryPathConfig(t, store, ctx, pathIDs[1], historyIntegrationKey(20703), now)

	job, created, err := store.CreateReplay(ctx,
		model.HistoryReplayJob{ID: "job-both-payloads", PlanID: planID, IdempotencyKey: historyIntegrationKey(20704), CreatedAt: now, UpdatedAt: now},
		[]model.HistoryReplayItem{{PathID: pathIDs[0], UpdatedAt: now}, {PathID: pathIDs[1], UpdatedAt: now}})
	if err != nil || !created {
		t.Fatalf("创建回放任务失败：err=%v created=%t", err, created)
	}
	if job.Status != model.HistoryReplayStatusQueued || job.Total != 2 || job.Pending != 2 {
		t.Fatalf("新任务计数不正确：%+v", job)
	}

	claimed, err := store.ClaimReplayItems(ctx, job.ID, 2, "worker-a", now)
	if err != nil || len(claimed) != 2 {
		t.Fatalf("领取回放明细失败：err=%v claimed=%d", err, len(claimed))
	}
	formmaking, vueCustom := claimed[0], claimed[1]

	formmaking.Status = model.HistoryReplayItemStatusReady
	formmaking.DataStatus = model.HistoryDataStatusReady
	formmaking.RuntimeType = "formmaking"
	formmaking.RuntimeValidation = model.HistoryRuntimeValidation{Accepted: true}
	formmaking.EffectiveFormData = map[string]any{"amount": "12.30"}
	if err := store.CompleteReplayItem(ctx, job.ID, formmaking.ID, formmaking, now.Add(time.Minute)); err != nil {
		t.Fatalf("完成 FormMaking 明细失败：%v", err)
	}

	vueCustom.Status = model.HistoryReplayItemStatusNeedsInput
	vueCustom.DataStatus = model.HistoryDataStatusNeedsInput
	vueCustom.RuntimeType = "vue_custom"
	vueCustom.RuntimeValidation = model.HistoryRuntimeValidation{
		Accepted: false,
		Issues:   []model.HistoryDataIssue{{Code: "HISTORY_FIELD_MISSING", Path: "remark", Message: "目标自定义页面新增必填字段", Blocking: true}},
	}
	vueCustom.Issues = vueCustom.RuntimeValidation.Issues
	if err := store.CompleteReplayItem(ctx, job.ID, vueCustom.ID, vueCustom, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("完成自定义表单明细失败：%v", err)
	}

	page, err := store.ListReplayItems(ctx, planID, job.ID, 0, 10)
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("读取回放明细失败：err=%v items=%d", err, len(page.Items))
	}
	statuses := map[uint64]string{}
	for _, item := range page.Items {
		statuses[item.PathID] = item.Status + "/" + item.DataStatus
	}
	if statuses[pathIDs[0]] != "ready/ready" || statuses[pathIDs[1]] != "needs_input/needs_input" {
		t.Fatalf("两类载荷终态互相影响：%v", statuses)
	}

	aggregated, err := store.GetReplay(ctx, planID, job.ID)
	if err != nil || aggregated.Ready != 1 || aggregated.NeedsInput != 1 || aggregated.Pending != 0 || aggregated.Running != 0 {
		t.Fatalf("任务聚合计数不正确：err=%v job=%+v", err, aggregated)
	}

	for index, expected := range []struct{ runtimeType, dataStatus string }{{"formmaking", model.HistoryDataStatusReady}, {"vue_custom", model.HistoryDataStatusNeedsInput}} {
		config, found, configErr := store.GetPathConfig(ctx, pathIDs[index])
		if configErr != nil || !found {
			t.Fatalf("读取路径配置失败：err=%v found=%t", configErr, found)
		}
		if config.RuntimeType != expected.runtimeType || config.DataStatus != expected.dataStatus || config.DataRevision < 2 {
			t.Fatalf("明细终态没有在同一事务写入路径配置：%+v", config)
		}
	}
}

// TestHistoryReplayJobAllowsSingleActiveJobPerPlan 验证同计划只允许一个活动任务，
// 且同一幂等键只能对应同一批路径。
func TestHistoryReplayJobAllowsSingleActiveJobPerPlan(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "同计划单活动任务")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	planID, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20711, "template-a", "new", 2)
	now := time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC)

	firstKey := historyIntegrationKey(20712)
	first, created, err := store.CreateReplay(ctx,
		model.HistoryReplayJob{ID: "job-active-first", PlanID: planID, IdempotencyKey: firstKey, CreatedAt: now, UpdatedAt: now},
		[]model.HistoryReplayItem{{PathID: pathIDs[0], UpdatedAt: now}})
	if err != nil || !created {
		t.Fatalf("创建首个任务失败：err=%v created=%t", err, created)
	}

	replayed, created, err := store.CreateReplay(ctx,
		model.HistoryReplayJob{ID: "job-active-replayed", PlanID: planID, IdempotencyKey: firstKey, CreatedAt: now, UpdatedAt: now},
		[]model.HistoryReplayItem{{PathID: pathIDs[0], UpdatedAt: now}})
	if err != nil || created || replayed.ID != first.ID {
		t.Fatalf("相同幂等键相同路径必须返回既有任务：err=%v created=%t id=%s", err, created, replayed.ID)
	}

	if _, _, err := store.CreateReplay(ctx,
		model.HistoryReplayJob{ID: "job-active-other-paths", PlanID: planID, IdempotencyKey: firstKey, CreatedAt: now, UpdatedAt: now},
		[]model.HistoryReplayItem{{PathID: pathIDs[1], UpdatedAt: now}}); err != repository.ErrHistoryReplayIdempotency {
		t.Fatalf("相同幂等键换路径必须被拒绝：%v", err)
	}

	if _, _, err := store.CreateReplay(ctx,
		model.HistoryReplayJob{ID: "job-active-second", PlanID: planID, IdempotencyKey: historyIntegrationKey(20713), CreatedAt: now, UpdatedAt: now},
		[]model.HistoryReplayItem{{PathID: pathIDs[1], UpdatedAt: now}}); err != repository.ErrHistoryReplayActive {
		t.Fatalf("同计划活动任务未被限制为一个：%v", err)
	}

	active, found, err := store.FindActiveReplay(ctx, planID)
	if err != nil || !found || active.ID != first.ID {
		t.Fatalf("活动任务读取不正确：err=%v found=%t id=%s", err, found, active.ID)
	}

	if _, err := store.UpdateReplayStatus(ctx, planID, first.ID, model.HistoryReplayStatusCancelled, now.Add(time.Minute)); err != nil {
		t.Fatalf("取消任务失败：%v", err)
	}
	if _, _, err := store.CreateReplay(ctx,
		model.HistoryReplayJob{ID: "job-active-after-cancel", PlanID: planID, IdempotencyKey: historyIntegrationKey(20714), CreatedAt: now, UpdatedAt: now},
		[]model.HistoryReplayItem{{PathID: pathIDs[1], UpdatedAt: now}}); err != nil {
		t.Fatalf("取消后应允许创建新任务：%v", err)
	}
}

// TestHistoryReplayJobCancelKeepsCheckpointsAndResumeContinuesUnfinished 验证取消保留已完成检查点，
// 恢复只继续未完成明细，过期租约由新 worker 接管而旧 worker 无法再写入。
func TestHistoryReplayJobCancelKeepsCheckpointsAndResumeContinuesUnfinished(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "回放取消与恢复")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	planID, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20721, "template-a", "new", 3)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	for index, pathID := range pathIDs {
		seedHistoryPathConfig(t, store, ctx, pathID, historyIntegrationKey(20722+index), now)
	}

	job, _, err := store.CreateReplay(ctx,
		model.HistoryReplayJob{ID: "job-resume", PlanID: planID, IdempotencyKey: historyIntegrationKey(20726), CreatedAt: now, UpdatedAt: now},
		[]model.HistoryReplayItem{{PathID: pathIDs[0], UpdatedAt: now}, {PathID: pathIDs[1], UpdatedAt: now}, {PathID: pathIDs[2], UpdatedAt: now}})
	if err != nil {
		t.Fatalf("创建回放任务失败：%v", err)
	}

	claimed, err := store.ClaimReplayItems(ctx, job.ID, 2, "worker-a", now)
	if err != nil || len(claimed) != 2 {
		t.Fatalf("领取明细失败：err=%v claimed=%d", err, len(claimed))
	}
	done := claimed[0]
	done.Status = model.HistoryReplayItemStatusReady
	done.DataStatus = model.HistoryDataStatusReady
	done.RuntimeType = "formmaking"
	done.RuntimeValidation = model.HistoryRuntimeValidation{Accepted: true}
	if err := store.CompleteReplayItem(ctx, job.ID, done.ID, done, now.Add(time.Minute)); err != nil {
		t.Fatalf("完成第一条明细失败：%v", err)
	}

	cancelled, err := store.UpdateReplayStatus(ctx, planID, job.ID, model.HistoryReplayStatusCancelled, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("取消任务失败：%v", err)
	}
	if cancelled.Ready != 1 || cancelled.Pending != 2 || cancelled.Running != 0 {
		t.Fatalf("取消没有保留检查点：%+v", cancelled)
	}

	stale := claimed[1]
	stale.Status = model.HistoryReplayItemStatusReady
	stale.DataStatus = model.HistoryDataStatusReady
	stale.RuntimeType = "formmaking"
	if err := store.CompleteReplayItem(ctx, job.ID, stale.ID, stale, now.Add(3*time.Minute)); err == nil {
		t.Fatal("取消后旧 worker 仍然写入了明细")
	}

	resumed, err := store.UpdateReplayStatus(ctx, planID, job.ID, model.HistoryReplayStatusQueued, now.Add(4*time.Minute))
	if err != nil || resumed.Status != model.HistoryReplayStatusQueued || resumed.Ready != 1 || resumed.Pending != 2 {
		t.Fatalf("恢复任务状态不正确：err=%v job=%+v", err, resumed)
	}

	remaining, err := store.ClaimReplayItems(ctx, job.ID, 10, "worker-b", now.Add(5*time.Minute))
	if err != nil || len(remaining) != 2 {
		t.Fatalf("恢复后只应继续未完成明细：err=%v remaining=%d", err, len(remaining))
	}
	for _, item := range remaining {
		if item.PathID == done.PathID {
			t.Fatalf("恢复重复处理了已完成路径：%+v", item)
		}
	}

	failed := remaining[0]
	failed.Status = model.HistoryReplayItemStatusFailed
	failed.DataStatus = model.HistoryDataStatusNeedsInput
	failed.Issues = []model.HistoryDataIssue{{Code: "HISTORY_TARGET_READ_FAILED", Message: "目标历史数据读取不完整", Blocking: true}}
	if err := store.CompleteReplayItem(ctx, job.ID, failed.ID, failed, now.Add(6*time.Minute)); err != nil {
		t.Fatalf("写入失败明细失败：%v", err)
	}
	after, err := store.RecountReplay(ctx, job.ID, now.Add(7*time.Minute))
	if err != nil || after.Ready != 1 || after.Failed != 1 {
		t.Fatalf("单项失败回滚了已完成项：err=%v job=%+v", err, after)
	}

	takeover, err := store.ClaimReplayItems(ctx, job.ID, 10, "worker-c", now.Add(30*time.Minute))
	if err != nil || len(takeover) != 1 {
		t.Fatalf("过期租约未被新 worker 接管：err=%v claimed=%d", err, len(takeover))
	}
	expired := remaining[1]
	expired.Status = model.HistoryReplayItemStatusReady
	expired.DataStatus = model.HistoryDataStatusReady
	expired.RuntimeType = "formmaking"
	if err := store.CompleteReplayItem(ctx, job.ID, expired.ID, expired, now.Add(31*time.Minute)); err == nil {
		t.Fatal("租约过期的旧 worker 仍然写入了明细")
	}
}

// seedHistoryPathConfig 先建立路径配置行，使回放完成时的同事务更新有可写目标。
func seedHistoryPathConfig(t *testing.T, store *planmysql.HistoryReplayRepository, ctx context.Context, pathID uint64, idempotencyKey string, now time.Time) {
	t.Helper()
	if _, err := store.SavePathConfig(ctx, repository.HistoryPathConfigRecord{
		PathID: pathID, IdempotencyKey: idempotencyKey,
		ConfigStatus: "pending", NodeStatus: "pending", DataStatus: model.HistoryDataStatusEmpty,
		SourceMode: model.HistorySourceModeNone, RuntimeType: "unknown",
	}, 0, now); err != nil {
		t.Fatalf("初始化路径配置失败：%v", err)
	}
}
