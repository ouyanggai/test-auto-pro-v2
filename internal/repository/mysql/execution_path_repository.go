package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

type ExecutionPathRepository struct {
	db *sql.DB
}

// NewExecutionPathRepository 创建基于同一计划数据库连接池的路径仓储。
func NewExecutionPathRepository(db *sql.DB) *ExecutionPathRepository {
	return &ExecutionPathRepository{db: db}
}

// List 按稳定序号读取计划路径及其最小分支选择集合。
func (r *ExecutionPathRepository) List(ctx context.Context, planID uint64) ([]model.ExecutionPath, error) {
	var exists int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_plans WHERE id = ?", planID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, repository.ErrPlanNotFound
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, plan_id, sequence_no, name, created_at, updated_at
FROM test_execution_paths
WHERE plan_id = ?
ORDER BY sequence_no ASC`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := make([]model.ExecutionPath, 0)
	for rows.Next() {
		path, scanErr := scanExecutionPath(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range paths {
		choices, err := loadExecutionPathChoices(ctx, r.db, paths[index].ID)
		if err != nil {
			return nil, err
		}
		paths[index].Choices = choices
	}
	return paths, nil
}

// FindByCreateKey 在指定计划内读取已经成功创建的幂等记录，不向其他计划暴露路径。
func (r *ExecutionPathRepository) FindByCreateKey(ctx context.Context, planID uint64, createKey string) (model.ExecutionPath, bool, error) {
	path, err := scanExecutionPath(r.db.QueryRowContext(ctx, `
SELECT id, plan_id, sequence_no, name, created_at, updated_at
FROM test_execution_paths
WHERE plan_id = ? AND create_key = ?`, planID, createKey))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ExecutionPath{}, false, nil
	}
	if err != nil {
		return model.ExecutionPath{}, false, err
	}
	path.Choices, err = loadExecutionPathChoices(ctx, r.db, path.ID)
	if err != nil {
		return model.ExecutionPath{}, false, err
	}
	return path, true, nil
}

// Create 在计划行锁保护下执行幂等检查、来源上限、序号分配和路径写入。
func (r *ExecutionPathRepository) Create(ctx context.Context, planID uint64, createKey, name string, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ExecutionPath{}, false, err
	}
	defer tx.Rollback()
	// 计划行锁既保护来源数量上限，也串行化同一计划的稳定序号计数器。
	source, nextSequenceNo, err := lockMutablePlan(ctx, tx, planID)
	if err != nil {
		return model.ExecutionPath{}, false, err
	}
	if existing, found, err := findExecutionPathByCreateKey(ctx, tx, createKey); err != nil {
		return model.ExecutionPath{}, false, err
	} else if found {
		// 相同幂等键必须返回原记录，不能再次分配序号或重复写选择。
		if existing.PlanID != planID {
			return model.ExecutionPath{}, false, repository.ErrExecutionPathDataInvalid
		}
		existing.Choices, err = loadExecutionPathChoices(ctx, tx, existing.ID)
		if err != nil {
			return model.ExecutionPath{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return model.ExecutionPath{}, false, err
		}
		return existing, false, nil
	}
	if source != "new" {
		// 已发和待发计划只对应一个既有实例，数据库事务内再次限制最多一条路径。
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_execution_paths WHERE plan_id = ?", planID).Scan(&count); err != nil {
			return model.ExecutionPath{}, false, err
		}
		if count != 0 {
			return model.ExecutionPath{}, false, repository.ErrExecutionPathLimit
		}
	}
	var maxSequenceNo uint
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence_no), 0) + 1 FROM test_execution_paths WHERE plan_id = ?", planID).Scan(&maxSequenceNo); err != nil {
		return model.ExecutionPath{}, false, err
	}
	sequenceNo := nextSequenceNo
	// MAX+1 兼容计数器加入前已有路径；之后由持久计数器保证删除最高序号也不回退。
	if maxSequenceNo > sequenceNo {
		sequenceNo = maxSequenceNo
	}
	name = storedExecutionPathName(name, sequenceNo)
	result, err := tx.ExecContext(ctx, `
INSERT INTO test_execution_paths (plan_id, sequence_no, create_key, name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`, planID, sequenceNo, createKey, name, now.UTC(), now.UTC())
	if err != nil {
		return model.ExecutionPath{}, false, err
	}
	id, err := result.LastInsertId()
	if err != nil || id < 1 {
		return model.ExecutionPath{}, false, repository.ErrExecutionPathDataInvalid
	}
	if err := insertExecutionPathChoices(ctx, tx, uint64(id), choices); err != nil {
		return model.ExecutionPath{}, false, err
	}
	// 计数器与路径在同一计划行锁和事务内推进，硬删除最高序号后也不能复用历史编号。
	if _, err := tx.ExecContext(ctx, "UPDATE test_plans SET next_path_sequence_no = ? WHERE id = ?", sequenceNo+1, planID); err != nil {
		return model.ExecutionPath{}, false, err
	}
	if err := touchPlan(ctx, tx, planID, now); err != nil {
		return model.ExecutionPath{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.ExecutionPath{}, false, err
	}
	return model.ExecutionPath{ID: uint64(id), PlanID: planID, SequenceNo: sequenceNo, Name: name, Choices: copyChoices(choices), CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, true, nil
}

// Update 在计划锁和单一事务内替换选择并同步路径、计划更新时间。
func (r *ExecutionPathRepository) Update(ctx context.Context, planID, pathID uint64, name string, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ExecutionPath{}, err
	}
	defer tx.Rollback()
	if _, _, err := lockMutablePlan(ctx, tx, planID); err != nil {
		return model.ExecutionPath{}, err
	}
	path, err := findExecutionPath(ctx, tx, planID, pathID)
	if err != nil {
		return model.ExecutionPath{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM test_execution_path_choices WHERE path_id = ?", pathID); err != nil {
		return model.ExecutionPath{}, err
	}
	if err := insertExecutionPathChoices(ctx, tx, pathID, choices); err != nil {
		return model.ExecutionPath{}, err
	}
	name = storedExecutionPathName(name, path.SequenceNo)
	if _, err := tx.ExecContext(ctx, "UPDATE test_execution_paths SET name = ?, updated_at = ? WHERE id = ?", name, now.UTC(), pathID); err != nil {
		return model.ExecutionPath{}, err
	}
	if err := touchPlan(ctx, tx, planID, now); err != nil {
		return model.ExecutionPath{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.ExecutionPath{}, err
	}
	path.Choices = copyChoices(choices)
	path.Name = name
	path.UpdatedAt = now.UTC()
	return path, nil
}

// FindBatchByCreateKey 在目标图读取前返回同一计划已经提交成功的批量结果。
func (r *ExecutionPathRepository) FindBatchByCreateKey(ctx context.Context, planID uint64, createKey string) (model.ExecutionPathBatchResult, bool, error) {
	batchID, batchPlanID, result, found, err := findExecutionPathBatchByCreateKey(ctx, r.db, createKey)
	if err != nil || !found {
		return model.ExecutionPathBatchResult{}, found, err
	}
	if batchPlanID != planID {
		return model.ExecutionPathBatchResult{}, false, nil
	}
	if result.Paths == nil {
		result.Paths, err = loadExecutionPathBatchPaths(ctx, r.db, batchID)
		if err != nil {
			return model.ExecutionPathBatchResult{}, false, err
		}
	}
	return result, true, nil
}

// GenerateAll 在单一计划锁和事务中完成重复过滤、连续序号分配、批量写入与幂等结果登记。
func (r *ExecutionPathRepository) GenerateAll(ctx context.Context, planID uint64, createKey string, candidates [][]model.ExecutionPathChoice, now time.Time) (model.ExecutionPathBatchResult, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ExecutionPathBatchResult{}, false, err
	}
	defer tx.Rollback()
	source, nextSequenceNo, err := lockMutablePlan(ctx, tx, planID)
	if err != nil {
		return model.ExecutionPathBatchResult{}, false, err
	}
	if batchID, batchPlanID, existing, found, err := findExecutionPathBatchByCreateKey(ctx, tx, createKey); err != nil {
		return model.ExecutionPathBatchResult{}, false, err
	} else if found {
		if batchPlanID != planID {
			return model.ExecutionPathBatchResult{}, false, repository.ErrExecutionPathDataInvalid
		}
		if existing.Paths == nil {
			existing.Paths, err = loadExecutionPathBatchPaths(ctx, tx, batchID)
			if err != nil {
				return model.ExecutionPathBatchResult{}, false, err
			}
		}
		if err := tx.Commit(); err != nil {
			return model.ExecutionPathBatchResult{}, false, err
		}
		return existing, false, nil
	}
	if source != "new" {
		return model.ExecutionPathBatchResult{}, false, repository.ErrExecutionPathSource
	}
	storedPaths, err := listExecutionPathsWithChoices(ctx, tx, planID)
	if err != nil {
		return model.ExecutionPathBatchResult{}, false, err
	}
	existingSignatures := make(map[string]struct{}, len(storedPaths))
	for _, path := range storedPaths {
		existingSignatures[executionPathChoiceSignature(path.Choices)] = struct{}{}
	}
	toCreate := make([][]model.ExecutionPathChoice, 0, len(candidates))
	existingCount := 0
	seenCandidates := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		signature := executionPathChoiceSignature(candidate)
		if _, duplicated := seenCandidates[signature]; duplicated {
			return model.ExecutionPathBatchResult{}, false, repository.ErrExecutionPathDataInvalid
		}
		seenCandidates[signature] = struct{}{}
		if _, exists := existingSignatures[signature]; exists {
			existingCount++
			continue
		}
		toCreate = append(toCreate, candidate)
	}
	result := model.ExecutionPathBatchResult{
		TotalCount: len(candidates), ExistingCount: existingCount, CreatedCount: len(toCreate),
		Paths: make([]model.ExecutionPath, 0, len(toCreate)),
	}
	inserted, err := tx.ExecContext(ctx, `
INSERT INTO test_execution_path_batches (plan_id, create_key, total_count, existing_count, created_count, created_at)
VALUES (?, ?, ?, ?, ?, ?)`, planID, createKey, result.TotalCount, result.ExistingCount, result.CreatedCount, now.UTC())
	if err != nil {
		return model.ExecutionPathBatchResult{}, false, err
	}
	batchID, err := inserted.LastInsertId()
	if err != nil || batchID < 1 {
		return model.ExecutionPathBatchResult{}, false, repository.ErrExecutionPathDataInvalid
	}
	var maxSequenceNo uint
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence_no), 0) + 1 FROM test_execution_paths WHERE plan_id = ?", planID).Scan(&maxSequenceNo); err != nil {
		return model.ExecutionPathBatchResult{}, false, err
	}
	sequenceNo := nextSequenceNo
	if maxSequenceNo > sequenceNo {
		sequenceNo = maxSequenceNo
	}
	for index, choices := range toCreate {
		name := storedExecutionPathName("", sequenceNo)
		pathKey := batchExecutionPathCreateKey(createKey, index)
		insertedPath, err := tx.ExecContext(ctx, `
INSERT INTO test_execution_paths (plan_id, sequence_no, create_key, name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`, planID, sequenceNo, pathKey, name, now.UTC(), now.UTC())
		if err != nil {
			return model.ExecutionPathBatchResult{}, false, err
		}
		pathID, err := insertedPath.LastInsertId()
		if err != nil || pathID < 1 {
			return model.ExecutionPathBatchResult{}, false, repository.ErrExecutionPathDataInvalid
		}
		if err := insertExecutionPathChoices(ctx, tx, uint64(pathID), choices); err != nil {
			return model.ExecutionPathBatchResult{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO test_execution_path_batch_items (batch_id, item_order, path_id)
VALUES (?, ?, ?)`, batchID, index+1, pathID); err != nil {
			return model.ExecutionPathBatchResult{}, false, err
		}
		result.Paths = append(result.Paths, model.ExecutionPath{
			ID: uint64(pathID), PlanID: planID, SequenceNo: sequenceNo, Name: name,
			Choices: copyChoices(choices), CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
		})
		sequenceNo++
	}
	if len(toCreate) > 0 {
		// 整批使用连续序号，并与批次结果在同一事务提交，任何一条失败都不会留下部分路径或推进计数器。
		if _, err := tx.ExecContext(ctx, "UPDATE test_plans SET next_path_sequence_no = ? WHERE id = ?", sequenceNo, planID); err != nil {
			return model.ExecutionPathBatchResult{}, false, err
		}
		if err := touchPlan(ctx, tx, planID, now); err != nil {
			return model.ExecutionPathBatchResult{}, false, err
		}
	}
	snapshot, err := json.Marshal(result)
	if err != nil {
		return model.ExecutionPathBatchResult{}, false, repository.ErrExecutionPathDataInvalid
	}
	// 幂等结果快照与路径同事务提交，后续用户主动删除路径也不能改变原请求已经返回的批次事实。
	if _, err := tx.ExecContext(ctx, "UPDATE test_execution_path_batches SET result_json = ? WHERE id = ?", snapshot, batchID); err != nil {
		return model.ExecutionPathBatchResult{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.ExecutionPathBatchResult{}, false, err
	}
	return result, true, nil
}

// Delete 硬删除指定计划的路径并依赖外键级联删除选择，计数器保持不变。
func (r *ExecutionPathRepository) Delete(ctx context.Context, planID, pathID uint64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, err := lockMutablePlan(ctx, tx, planID); err != nil {
		return err
	}
	if _, err := findExecutionPath(ctx, tx, planID, pathID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM test_execution_paths WHERE id = ? AND plan_id = ?", pathID, planID); err != nil {
		return err
	}
	if err := touchPlan(ctx, tx, planID, now); err != nil {
		return err
	}
	return tx.Commit()
}

// lockMutablePlan 锁定计划并返回来源和下一稳定序号，非待配置计划立即拒绝。
func lockMutablePlan(ctx context.Context, tx *sql.Tx, planID uint64) (string, uint, error) {
	var source string
	var status model.PlanStatus
	var nextSequenceNo uint
	// FOR UPDATE 将同一计划的计数器分配串行化，不同计划仍可并发创建路径。
	err := tx.QueryRowContext(ctx, "SELECT flow_source, status, next_path_sequence_no FROM test_plans WHERE id = ? FOR UPDATE", planID).Scan(&source, &status, &nextSequenceNo)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, repository.ErrPlanNotFound
	}
	if err != nil {
		return "", 0, err
	}
	if status != model.PlanStatusPendingConfiguration {
		return "", 0, repository.ErrExecutionPathPlanLocked
	}
	if nextSequenceNo == 0 {
		return "", 0, repository.ErrExecutionPathDataInvalid
	}
	return source, nextSequenceNo, nil
}

// findExecutionPathByCreateKey 在当前事务内读取幂等键对应路径。
func findExecutionPathByCreateKey(ctx context.Context, tx *sql.Tx, createKey string) (model.ExecutionPath, bool, error) {
	path, err := scanExecutionPath(tx.QueryRowContext(ctx, `
SELECT id, plan_id, sequence_no, name, created_at, updated_at
FROM test_execution_paths WHERE create_key = ?`, createKey))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ExecutionPath{}, false, nil
	}
	return path, err == nil, err
}

// findExecutionPath 按计划归属精确读取路径，避免跨计划修改。
func findExecutionPath(ctx context.Context, tx *sql.Tx, planID, pathID uint64) (model.ExecutionPath, error) {
	path, err := scanExecutionPath(tx.QueryRowContext(ctx, `
SELECT id, plan_id, sequence_no, name, created_at, updated_at
FROM test_execution_paths WHERE id = ? AND plan_id = ?`, pathID, planID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ExecutionPath{}, repository.ErrExecutionPathNotFound
	}
	return path, err
}

// insertExecutionPathChoices 只写真实条件或手动路由选择，不保存任何派生节点。
func insertExecutionPathChoices(ctx context.Context, tx *sql.Tx, pathID uint64, choices []model.ExecutionPathChoice) error {
	for _, choice := range choices {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO test_execution_path_choices (path_id, route_node_id, branch_id)
VALUES (?, ?, ?)`, pathID, choice.RouteNodeID, choice.BranchID); err != nil {
			return err
		}
	}
	return nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// loadExecutionPathChoices 按路由标识稳定读取一条路径的选择集合。
func loadExecutionPathChoices(ctx context.Context, db queryer, pathID uint64) ([]model.ExecutionPathChoice, error) {
	rows, err := db.QueryContext(ctx, `
SELECT route_node_id, branch_id
FROM test_execution_path_choices
WHERE path_id = ?
ORDER BY route_node_id ASC`, pathID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	choices := make([]model.ExecutionPathChoice, 0)
	for rows.Next() {
		var choice model.ExecutionPathChoice
		if err := rows.Scan(&choice.RouteNodeID, &choice.BranchID); err != nil {
			return nil, err
		}
		choices = append(choices, choice)
	}
	return choices, rows.Err()
}

// listExecutionPathsWithChoices 在计划锁内读取重复过滤所需的全部已保存线路。
func listExecutionPathsWithChoices(ctx context.Context, db queryer, planID uint64) ([]model.ExecutionPath, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, plan_id, sequence_no, name, created_at, updated_at
FROM test_execution_paths WHERE plan_id = ? ORDER BY sequence_no ASC`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := make([]model.ExecutionPath, 0)
	for rows.Next() {
		path, err := scanExecutionPath(rows)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range paths {
		paths[index].Choices, err = loadExecutionPathChoices(ctx, db, paths[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

type executionPathBatchQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// findExecutionPathBatchByCreateKey 读取全局幂等批次元数据，调用方负责核对计划归属。
func findExecutionPathBatchByCreateKey(ctx context.Context, db executionPathBatchQueryer, createKey string) (uint64, uint64, model.ExecutionPathBatchResult, bool, error) {
	var batchID, planID uint64
	var result model.ExecutionPathBatchResult
	var snapshot sql.NullString
	err := db.QueryRowContext(ctx, `
SELECT id, plan_id, total_count, existing_count, created_count, result_json
FROM test_execution_path_batches WHERE create_key = ?`, createKey).Scan(
		&batchID, &planID, &result.TotalCount, &result.ExistingCount, &result.CreatedCount, &snapshot,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, model.ExecutionPathBatchResult{}, false, nil
	}
	if err != nil {
		return 0, 0, model.ExecutionPathBatchResult{}, false, err
	}
	if snapshot.Valid && strings.TrimSpace(snapshot.String) != "" {
		if err := json.Unmarshal([]byte(snapshot.String), &result); err != nil {
			return 0, 0, model.ExecutionPathBatchResult{}, false, repository.ErrExecutionPathDataInvalid
		}
	}
	return batchID, planID, result, true, nil
}

// loadExecutionPathBatchPaths 按批次内稳定顺序恢复已提交成功的创建结果。
func loadExecutionPathBatchPaths(ctx context.Context, db queryer, batchID uint64) ([]model.ExecutionPath, error) {
	rows, err := db.QueryContext(ctx, `
SELECT path.id, path.plan_id, path.sequence_no, path.name, path.created_at, path.updated_at
FROM test_execution_path_batch_items item
JOIN test_execution_paths path ON path.id = item.path_id
WHERE item.batch_id = ? ORDER BY item.item_order ASC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := make([]model.ExecutionPath, 0)
	for rows.Next() {
		path, err := scanExecutionPath(rows)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range paths {
		paths[index].Choices, err = loadExecutionPathChoices(ctx, db, paths[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// touchPlan 在路径事务内同步计划更新时间，保证列表排序反映配置变化。
func touchPlan(ctx context.Context, tx *sql.Tx, planID uint64, now time.Time) error {
	_, err := tx.ExecContext(ctx, "UPDATE test_plans SET updated_at = ? WHERE id = ?", now.UTC(), planID)
	return err
}

type executionPathScanner interface {
	Scan(...any) error
}

// scanExecutionPath 将数据库行转换为 UTC 路径模型并拒绝非法关键字段。
func scanExecutionPath(row executionPathScanner) (model.ExecutionPath, error) {
	var path model.ExecutionPath
	if err := row.Scan(&path.ID, &path.PlanID, &path.SequenceNo, &path.Name, &path.CreatedAt, &path.UpdatedAt); err != nil {
		return model.ExecutionPath{}, err
	}
	path.CreatedAt = path.CreatedAt.UTC()
	path.UpdatedAt = path.UpdatedAt.UTC()
	if path.ID == 0 || path.PlanID == 0 || path.SequenceNo < 1 {
		return model.ExecutionPath{}, repository.ErrExecutionPathDataInvalid
	}
	path.Name = storedExecutionPathName(path.Name, path.SequenceNo)
	return path, nil
}

// copyChoices 隔离仓储返回值与调用方切片，避免事务完成后被意外修改。
func copyChoices(choices []model.ExecutionPathChoice) []model.ExecutionPathChoice {
	return append([]model.ExecutionPathChoice(nil), choices...)
}

// storedExecutionPathName 为历史空名称和新分配序号统一生成稳定默认名称。
func storedExecutionPathName(name string, sequenceNo uint) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Sprintf("路径 %d", sequenceNo)
	}
	return name
}

// executionPathChoiceSignature 以路由标识排序生成仅用于同线路判等的无歧义签名。
func executionPathChoiceSignature(choices []model.ExecutionPathChoice) string {
	ordered := copyChoices(choices)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].RouteNodeID == ordered[j].RouteNodeID {
			return ordered[i].BranchID < ordered[j].BranchID
		}
		return ordered[i].RouteNodeID < ordered[j].RouteNodeID
	})
	var signature strings.Builder
	for _, choice := range ordered {
		signature.WriteString(fmt.Sprintf("%d:%s%d:%s;", len(choice.RouteNodeID), choice.RouteNodeID, len(choice.BranchID), choice.BranchID))
	}
	return signature.String()
}

// batchExecutionPathCreateKey 从批次键派生每行唯一键，避免把一个幂等键写入多条全局唯一记录。
func batchExecutionPathCreateKey(batchKey string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", batchKey, index)))
	// 使用 UUID 文本形状兼容现有 CHAR(36) 约束；批次记录才是对外幂等事实，派生键只保障路径行唯一。
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}
