package f012_test

import (
	"encoding/json"
	"testing"
	"time"

	"test-auto-pro-v2/internal/jsonvalues"
	"test-auto-pro-v2/internal/repository"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// historyNumberFixture 是一组会被 MySQL 原生 JSON 列改写的目标数字字面量：
// 目标 Java 用 fastjson 解析成 BigDecimal 并在 eq 中同时比较数值与小数位，
// 长数字还是业务单号，一旦被转成双精度就直接损坏。
const historyNumberFixture = `{"amount":12.30,"rate":0.10,"sci":1.28E2,"idNumber":1234567890123456789012,` +
	`"count":1,"deep":{"rows":[{"tax":0.010,"total":100.00}]}}`

// TestHistorySnapshotKeepsTargetNumberLiterals 验证不可变历史快照按目标字面量原样存取。
func TestHistorySnapshotKeepsTargetNumberLiterals(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "历史快照数字保真")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	planID, _ := insertHistoryPlanWithPaths(t, database.DB, 20601, "template-a", "new", 1)

	raw, err := jsonvalues.DecodeObject([]byte(historyNumberFixture))
	if err != nil {
		t.Fatalf("解析目标历史数据失败：%v", err)
	}
	saved, err := store.SaveSnapshot(ctx, repositoryHistorySnapshot(planID, "candidate-number", "formmaking", "digest-number", raw))
	if err != nil {
		t.Fatalf("保存历史快照失败：%v", err)
	}
	loaded, err := store.GetSnapshot(ctx, planID, saved.ID)
	if err != nil {
		t.Fatalf("读取历史快照失败：%v", err)
	}
	assertSameHistoryPayload(t, "历史快照 rawFormData", []byte(historyNumberFixture), loaded.RawFormData)
}

// TestHistoryPathConfigKeepsTargetNumberLiterals 验证路径有效表单数据与分支补丁不被数据库改写数字。
func TestHistoryPathConfigKeepsTargetNumberLiterals(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "路径配置数字保真")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	_, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20611, "template-a", "new", 1)
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

	patches := []byte(`[{"path":"amount","before":12.30,"after":20.00,"reason":"分支条件","branchKey":"branch-1"}]`)
	record := repository.HistoryPathConfigRecord{
		PathID: pathIDs[0], IdempotencyKey: historyIntegrationKey(20612),
		ConfigStatus: "pending", NodeStatus: "pending", DataStatus: "ready", SourceMode: "path", RuntimeType: "formmaking",
		EffectiveFormData: []byte(historyNumberFixture), BranchPatches: patches,
	}
	if _, err := store.SavePathConfig(ctx, record, 0, now); err != nil {
		t.Fatalf("保存路径配置失败：%v", err)
	}
	loaded, found, err := store.GetPathConfig(ctx, pathIDs[0])
	if err != nil || !found {
		t.Fatalf("读取路径配置失败：err=%v found=%t", err, found)
	}
	assertSameHistoryPayload(t, "路径有效表单数据", []byte(historyNumberFixture), decodeHistoryPayload(t, loaded.EffectiveFormData))
	assertSameHistoryPayload(t, "分支补丁", patches, decodeHistoryPayload(t, loaded.BranchPatches))
}

// TestHistoryPathConfigIdempotentRetryAcceptsSameBody 验证同一幂等键重放完全相同正文不被误判为换正文，
// 但小数位不同的正文仍必须被拒绝。
func TestHistoryPathConfigIdempotentRetryAcceptsSameBody(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "路径配置幂等重放")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	_, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20621, "template-a", "new", 1)
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

	key := historyIntegrationKey(20622)
	record := repository.HistoryPathConfigRecord{
		PathID: pathIDs[0], IdempotencyKey: key,
		ConfigStatus: "pending", NodeStatus: "pending", DataStatus: "ready", SourceMode: "path", RuntimeType: "formmaking",
		EffectiveFormData: []byte(historyNumberFixture),
	}
	first, err := store.SavePathConfig(ctx, record, 0, now)
	if err != nil {
		t.Fatalf("首次保存路径配置失败：%v", err)
	}
	retry, err := store.SavePathConfig(ctx, record, 0, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("同幂等键重放相同正文被拒绝：%v", err)
	}
	if retry.Revision != first.Revision || retry.DataRevision != first.DataRevision {
		t.Fatalf("幂等重放不应推进修订号：first=%d/%d retry=%d/%d", first.Revision, first.DataRevision, retry.Revision, retry.DataRevision)
	}
	changed := record
	changed.EffectiveFormData = []byte(`{"amount":12.3,"rate":0.10,"sci":1.28E2,"idNumber":1234567890123456789012,` +
		`"count":1,"deep":{"rows":[{"tax":0.010,"total":100.00}]}}`)
	if _, err := store.SavePathConfig(ctx, changed, 0, now.Add(2*time.Minute)); err != repository.ErrHistoryPathConfigIdempotency {
		t.Fatalf("小数位不同的正文必须按换正文拒绝：%v", err)
	}
}

// TestHistoryPathConfigRejectsInvalidPayload 验证载荷列改为文本后仓储仍拒绝非法 JSON。
func TestHistoryPathConfigRejectsInvalidPayload(t *testing.T) {
	database, _, ctx := openHistoryIntegrationDatabase(t, "路径配置载荷校验")
	store := planmysql.NewHistoryReplayRepository(database.DB)
	_, pathIDs := insertHistoryPlanWithPaths(t, database.DB, 20631, "template-a", "new", 1)
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

	record := repository.HistoryPathConfigRecord{
		PathID: pathIDs[0], IdempotencyKey: historyIntegrationKey(20632),
		ConfigStatus: "pending", NodeStatus: "pending", DataStatus: "empty", SourceMode: "path", RuntimeType: "unknown",
		EffectiveFormData: []byte(`{"amount":`),
	}
	if _, err := store.SavePathConfig(ctx, record, 0, now); err != repository.ErrHistoryPathConfigDataInvalid {
		t.Fatalf("非法 JSON 载荷必须被拒绝：%v", err)
	}
	if _, found, err := store.GetPathConfig(ctx, pathIDs[0]); err != nil || found {
		t.Fatalf("非法载荷不能落盘：err=%v found=%t", err, found)
	}
}

// decodeHistoryPayload 把数据库读回的载荷文本解析为可比较结构，保留数字字面量。
func decodeHistoryPayload(t *testing.T, payload []byte) any {
	t.Helper()
	var value any
	if err := jsonvalues.Decode(payload, &value); err != nil {
		t.Fatalf("解析载荷失败：%v payload=%s", err, string(payload))
	}
	return value
}

// assertSameHistoryPayload 按目标字面量比较期望与实际载荷；json.Number 的小数位差异会被判定为不同。
func assertSameHistoryPayload(t *testing.T, label string, expected []byte, actual any) {
	t.Helper()
	var want any
	if err := jsonvalues.Decode(expected, &want); err != nil {
		t.Fatalf("%s 期望值不是合法 JSON：%v", label, err)
	}
	wantText, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("%s 期望值序列化失败：%v", label, err)
	}
	actualText, err := json.Marshal(actual)
	if err != nil {
		t.Fatalf("%s 实际值序列化失败：%v", label, err)
	}
	if string(wantText) != string(actualText) {
		t.Fatalf("%s 丢失目标数字字面量：want=%s got=%s", label, string(wantText), string(actualText))
	}
}
