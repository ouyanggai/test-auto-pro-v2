package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

type ExecutionPathRepository struct {
	db *sql.DB
}

func NewExecutionPathRepository(db *sql.DB) *ExecutionPathRepository {
	return &ExecutionPathRepository{db: db}
}

func (r *ExecutionPathRepository) List(ctx context.Context, planID uint64) ([]model.ExecutionPath, error) {
	var exists int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_plans WHERE id = ?", planID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, repository.ErrPlanNotFound
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, plan_id, sequence_no, created_at, updated_at
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

func (r *ExecutionPathRepository) Create(ctx context.Context, planID uint64, createKey string, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ExecutionPath{}, false, err
	}
	defer tx.Rollback()
	source, nextSequenceNo, err := lockMutablePlan(ctx, tx, planID)
	if err != nil {
		return model.ExecutionPath{}, false, err
	}
	if existing, found, err := findExecutionPathByCreateKey(ctx, tx, createKey); err != nil {
		return model.ExecutionPath{}, false, err
	} else if found {
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
	if maxSequenceNo > sequenceNo {
		sequenceNo = maxSequenceNo
	}
	result, err := tx.ExecContext(ctx, `
INSERT INTO test_execution_paths (plan_id, sequence_no, create_key, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)`, planID, sequenceNo, createKey, now.UTC(), now.UTC())
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
	return model.ExecutionPath{ID: uint64(id), PlanID: planID, SequenceNo: sequenceNo, Choices: copyChoices(choices), CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, true, nil
}

func (r *ExecutionPathRepository) Update(ctx context.Context, planID, pathID uint64, choices []model.ExecutionPathChoice, now time.Time) (model.ExecutionPath, error) {
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
	if _, err := tx.ExecContext(ctx, "UPDATE test_execution_paths SET updated_at = ? WHERE id = ?", now.UTC(), pathID); err != nil {
		return model.ExecutionPath{}, err
	}
	if err := touchPlan(ctx, tx, planID, now); err != nil {
		return model.ExecutionPath{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.ExecutionPath{}, err
	}
	path.Choices = copyChoices(choices)
	path.UpdatedAt = now.UTC()
	return path, nil
}

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

func findExecutionPathByCreateKey(ctx context.Context, tx *sql.Tx, createKey string) (model.ExecutionPath, bool, error) {
	path, err := scanExecutionPath(tx.QueryRowContext(ctx, `
SELECT id, plan_id, sequence_no, created_at, updated_at
FROM test_execution_paths WHERE create_key = ?`, createKey))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ExecutionPath{}, false, nil
	}
	return path, err == nil, err
}

func findExecutionPath(ctx context.Context, tx *sql.Tx, planID, pathID uint64) (model.ExecutionPath, error) {
	path, err := scanExecutionPath(tx.QueryRowContext(ctx, `
SELECT id, plan_id, sequence_no, created_at, updated_at
FROM test_execution_paths WHERE id = ? AND plan_id = ?`, pathID, planID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ExecutionPath{}, repository.ErrExecutionPathNotFound
	}
	return path, err
}

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

func touchPlan(ctx context.Context, tx *sql.Tx, planID uint64, now time.Time) error {
	_, err := tx.ExecContext(ctx, "UPDATE test_plans SET updated_at = ? WHERE id = ?", now.UTC(), planID)
	return err
}

type executionPathScanner interface {
	Scan(...any) error
}

func scanExecutionPath(row executionPathScanner) (model.ExecutionPath, error) {
	var path model.ExecutionPath
	if err := row.Scan(&path.ID, &path.PlanID, &path.SequenceNo, &path.CreatedAt, &path.UpdatedAt); err != nil {
		return model.ExecutionPath{}, err
	}
	path.CreatedAt = path.CreatedAt.UTC()
	path.UpdatedAt = path.UpdatedAt.UTC()
	if path.ID == 0 || path.PlanID == 0 || path.SequenceNo < 1 {
		return model.ExecutionPath{}, repository.ErrExecutionPathDataInvalid
	}
	return path, nil
}

func copyChoices(choices []model.ExecutionPathChoice) []model.ExecutionPathChoice {
	return append([]model.ExecutionPathChoice(nil), choices...)
}
