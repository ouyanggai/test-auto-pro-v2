package f012_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// routeChangeNodeConfiguration 是旧路径已保存的人员与动作配置，换路后必须原样保留。
const routeChangeNodeConfiguration = `[{"key":"approve-1","action":"approve","scope":"task","nodeKey":"node-a","order":1}]`

// TestHistoryRouteChangeWritesTargetPathAndKeepsSourceConfiguration 验证换路在同一事务内只更新新路径数据，
// 旧路径的人员与动作配置保留不动。
func TestHistoryRouteChangeWritesTargetPathAndKeepsSourceConfiguration(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "换路原子提交")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	planID, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20801, "template-a", "new", 2)
	now := time.Date(2026, 9, 1, 13, 0, 0, 0, time.UTC)

	source := repository.HistoryPathConfigRecord{
		PathID: pathIDs[0], IdempotencyKey: historyIntegrationKey(20802),
		ConfigStatus: "configured", NodeStatus: "configured", DataStatus: model.HistoryDataStatusReady,
		SourceMode: model.HistorySourceModeOverride, RuntimeType: "formmaking",
		UserActions:       []byte(routeChangeNodeConfiguration),
		PersonStrategies:  []byte(`{"person-a":{"key":"person-a","strategy":"manual","seed":1,"selected":["candidate-a"]}}`),
		EffectiveFormData: []byte(`{"amount":12.30}`),
	}
	savedSource, err := store.SavePathConfig(ctx, source, 0, now)
	if err != nil {
		t.Fatalf("初始化旧路径配置失败：%v", err)
	}

	target := repository.HistoryPathConfigRecord{
		PathID: pathIDs[1], IdempotencyKey: historyIntegrationKey(20803),
		ConfigStatus: "pending", NodeStatus: "affected", DataStatus: model.HistoryDataStatusReady,
		SourceMode: model.HistorySourceModeOverride, RuntimeType: "formmaking",
		EffectiveFormData: []byte(`{"amount":88.00}`),
		BranchPatches:     []byte(`[{"path":"amount","before":12.30,"after":88.00,"reason":"人工修改条件字段","branchKey":"branch-b"}]`),
	}
	savedTarget, err := store.SavePathData(ctx, planID, pathIDs[0], pathIDs[1], target, 0, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("换路保存失败：%v", err)
	}
	if savedTarget.PathID != pathIDs[1] || savedTarget.DataRevision == 0 {
		t.Fatalf("换路没有写入新路径数据：%+v", savedTarget)
	}

	keptSource, found, err := store.GetPathConfig(ctx, pathIDs[0])
	if err != nil || !found {
		t.Fatalf("读取旧路径配置失败：err=%v found=%t", err, found)
	}
	if string(keptSource.UserActions) != routeChangeNodeConfiguration || keptSource.Revision != savedSource.Revision {
		t.Fatalf("换路修改了旧路径配置：actions=%s revision=%d", string(keptSource.UserActions), keptSource.Revision)
	}
}

// TestHistoryRouteChangeRollsBackOnRevisionConflict 验证修订号冲突时换路整体回滚，新路径数据不被部分写入。
func TestHistoryRouteChangeRollsBackOnRevisionConflict(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "换路修订号冲突")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	planID, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20811, "template-a", "new", 2)
	now := time.Date(2026, 9, 1, 14, 0, 0, 0, time.UTC)

	existing := repository.HistoryPathConfigRecord{
		PathID: pathIDs[1], IdempotencyKey: historyIntegrationKey(20812),
		ConfigStatus: "configured", NodeStatus: "configured", DataStatus: model.HistoryDataStatusReady,
		SourceMode: model.HistorySourceModeDefault, RuntimeType: "formmaking",
		EffectiveFormData: []byte(`{"amount":50.00}`),
	}
	stored, err := store.SavePathConfig(ctx, existing, 0, now)
	if err != nil {
		t.Fatalf("初始化新路径配置失败：%v", err)
	}

	stale := existing
	stale.IdempotencyKey = historyIntegrationKey(20813)
	stale.EffectiveFormData = []byte(`{"amount":99.00}`)
	if _, err := store.SavePathData(ctx, planID, pathIDs[0], pathIDs[1], stale, stored.Revision+7, now.Add(time.Minute)); err != repository.ErrHistoryPathConfigConflict {
		t.Fatalf("过期修订号必须冲突：%v", err)
	}
	after, found, err := store.GetPathConfig(ctx, pathIDs[1])
	if err != nil || !found {
		t.Fatalf("读取新路径配置失败：err=%v found=%t", err, found)
	}
	if string(after.EffectiveFormData) != `{"amount":50.00}` || after.Revision != stored.Revision {
		t.Fatalf("冲突换路发生了部分写入：data=%s revision=%d", string(after.EffectiveFormData), after.Revision)
	}
}

// TestHistoryRouteChangeRejectsForeignPathAndMismatchedRecord 验证换路不接受跨计划路径或与目标不一致的记录。
func TestHistoryRouteChangeRejectsForeignPathAndMismatchedRecord(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "换路路径归属校验")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	planID, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20821, "template-a", "new", 2)
	otherPlanID, otherPathIDs := insertHistoryPlanWithPaths(t, database.DB, 20831, "template-b", "new", 1)
	now := time.Date(2026, 9, 1, 15, 0, 0, 0, time.UTC)

	mismatched := repository.HistoryPathConfigRecord{
		PathID: pathIDs[0], IdempotencyKey: historyIntegrationKey(20822),
		ConfigStatus: "pending", NodeStatus: "pending", DataStatus: model.HistoryDataStatusReady,
		SourceMode: model.HistorySourceModeOverride, RuntimeType: "formmaking",
		EffectiveFormData: []byte(`{"amount":1.00}`),
	}
	if _, err := store.SavePathData(ctx, planID, pathIDs[0], pathIDs[1], mismatched, 0, now); err != repository.ErrExecutionPathNotFound {
		t.Fatalf("记录路径与目标路径不一致必须被拒绝：%v", err)
	}

	foreign := mismatched
	foreign.PathID = otherPathIDs[0]
	if _, err := store.SavePathData(ctx, planID, pathIDs[0], otherPathIDs[0], foreign, 0, now); err != repository.ErrExecutionPathNotFound {
		t.Fatalf("跨计划换路必须被拒绝：%v", err)
	}
	if _, found, err := store.GetPathConfig(ctx, otherPathIDs[0]); err != nil || found {
		t.Fatalf("跨计划换路不能落盘：err=%v found=%t planID=%d", err, found, otherPlanID)
	}
}

// TestHistoryRouteChangeSerializesConcurrentWrites 验证同一新路径的并发换路只有一个成功，另一个按修订号冲突返回。
func TestHistoryRouteChangeSerializesConcurrentWrites(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "换路并发串行化")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	planID, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20841, "template-a", "new", 2)
	now := time.Date(2026, 9, 1, 16, 0, 0, 0, time.UTC)

	base, err := store.SavePathConfig(ctx, repository.HistoryPathConfigRecord{
		PathID: pathIDs[1], IdempotencyKey: historyIntegrationKey(20842),
		ConfigStatus: "pending", NodeStatus: "pending", DataStatus: model.HistoryDataStatusEmpty,
		SourceMode: model.HistorySourceModeNone, RuntimeType: "unknown",
	}, 0, now)
	if err != nil {
		t.Fatalf("初始化新路径配置失败：%v", err)
	}

	results := make([]error, 2)
	var group sync.WaitGroup
	for index := range results {
		group.Add(1)
		go func(slot int) {
			defer group.Done()
			record := repository.HistoryPathConfigRecord{
				PathID: pathIDs[1], IdempotencyKey: historyIntegrationKey(20843 + slot),
				ConfigStatus: "pending", NodeStatus: "affected", DataStatus: model.HistoryDataStatusReady,
				SourceMode: model.HistorySourceModeOverride, RuntimeType: "formmaking",
				EffectiveFormData: []byte(`{"amount":` + []string{"11.00", "22.00"}[slot] + `}`),
			}
			_, results[slot] = store.SavePathData(context.Background(), planID, pathIDs[0], pathIDs[1], record, base.Revision, now.Add(time.Duration(slot)*time.Second))
		}(index)
	}
	group.Wait()

	succeeded, conflicted := 0, 0
	for _, err := range results {
		switch err {
		case nil:
			succeeded++
		case repository.ErrHistoryPathConfigConflict:
			conflicted++
		default:
			t.Fatalf("并发换路返回了意外错误：%v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("并发换路没有被串行化：成功=%d 冲突=%d", succeeded, conflicted)
	}
	final, found, err := store.GetPathConfig(ctx, pathIDs[1])
	if err != nil || !found || final.Revision != base.Revision+1 {
		t.Fatalf("并发换路后修订号不正确：err=%v found=%t revision=%d", err, found, final.Revision)
	}
}
