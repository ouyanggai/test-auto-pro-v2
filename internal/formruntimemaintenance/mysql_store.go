package formruntimemaintenance

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

// MySQLStore 持久化维护任务，确保进程崩溃后仍可接管恢复。
type MySQLStore struct {
	database *sql.DB
	now      func() time.Time
}

// NewMySQLStore 创建 MySQL 任务仓储。
func NewMySQLStore(database *sql.DB, now func() time.Time) *MySQLStore {
	if now == nil {
		now = time.Now
	}
	return &MySQLStore{database: database, now: now}
}

// Create 依赖 active_guard 唯一键在数据库层保证单活动任务。
func (s *MySQLStore) Create(ctx context.Context, source SourceState) (Job, error) {
	changes, err := json.Marshal(source.ChangedFiles)
	if err != nil {
		return Job{}, err
	}
	now := s.now().UTC()
	result, err := s.database.ExecContext(ctx, `
		INSERT INTO test_form_runtime_sync_jobs (
			status, stage, active_guard, source_repository, source_branch, source_head,
			source_dirty, source_changed_files, source_inspected_at, created_at, updated_at
		) VALUES ('PENDING', 'QUEUED', 1, ?, ?, ?, ?, ?, ?, ?, ?)
	`, source.Repository, source.Branch, source.Head, source.Dirty, changes, source.InspectedAt.UTC(), now, now)
	if err != nil {
		var mysqlError *mysqlDriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return Job{}, ErrJobAlreadyActive
		}
		return Job{}, fmt.Errorf("创建表单运行时同步任务: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil || id <= 0 {
		return Job{}, fmt.Errorf("读取表单运行时同步任务编号: %w", err)
	}
	return s.Get(ctx, uint64(id))
}

// Get 返回指定任务。
func (s *MySQLStore) Get(ctx context.Context, id uint64) (Job, error) {
	return scanJob(s.database.QueryRowContext(ctx, jobSelect+` WHERE id = ?`, id))
}

// Latest 返回最新任务。
func (s *MySQLStore) Latest(ctx context.Context) (Job, error) {
	return scanJob(s.database.QueryRowContext(ctx, jobSelect+` ORDER BY id DESC LIMIT 1`))
}

// ClaimNext 锁定活动行并领取 pending 或租约过期任务；fencing token 每次接管递增。
func (s *MySQLStore) ClaimNext(ctx context.Context, claim Claim) (Job, error) {
	if strings.TrimSpace(claim.WorkerID) == "" || claim.LeaseDuration <= 0 {
		return Job{}, ErrJobNotReady
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	now := s.now().UTC()
	var id uint64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM test_form_runtime_sync_jobs
		WHERE active_guard = 1
		  AND (status = 'PENDING' OR (status = 'RUNNING' AND (lease_expires_at IS NULL OR lease_expires_at <= ?)))
		ORDER BY id LIMIT 1 FOR UPDATE
	`, now).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrJobNotReady
	}
	if err != nil {
		return Job{}, err
	}
	expiresAt := now.Add(claim.LeaseDuration)
	if _, err := tx.ExecContext(ctx, `
		UPDATE test_form_runtime_sync_jobs
		SET status='RUNNING', stage=IF(stage='QUEUED','INSPECT',stage), lease_owner=?, lease_expires_at=?,
			fencing_token=fencing_token+1, attempt_count=attempt_count+1,
			started_at=COALESCE(started_at,?), updated_at=?
		WHERE id=?
	`, claim.WorkerID, expiresAt, now, now, id); err != nil {
		return Job{}, err
	}
	job, err := scanJob(tx.QueryRowContext(ctx, jobSelect+` WHERE id = ?`, id))
	if err != nil {
		return Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	return job, nil
}

// RenewLease 通过 owner、token 和未过期条件阻断旧 Worker 续租。
func (s *MySQLStore) RenewLease(ctx context.Context, renewal LeaseRenewal) error {
	now := s.now().UTC()
	result, err := s.database.ExecContext(ctx, `
		UPDATE test_form_runtime_sync_jobs SET lease_expires_at=?, updated_at=?
		WHERE id=? AND status='RUNNING' AND lease_owner=? AND fencing_token=? AND lease_expires_at>?
	`, now.Add(renewal.LeaseDuration), now, renewal.ID, renewal.WorkerID, renewal.FencingToken, now)
	return requireOneRow(result, err)
}

// UpdateProgress 在同一租约下持久化阶段和候选/previous 版本。
func (s *MySQLStore) UpdateProgress(ctx context.Context, progress Progress) error {
	now := s.now().UTC()
	result, err := s.database.ExecContext(ctx, `
		UPDATE test_form_runtime_sync_jobs
		SET stage=?, candidate_version=COALESCE(NULLIF(?,''),candidate_version),
			previous_version=COALESCE(NULLIF(?,''),previous_version), lease_expires_at=?, updated_at=?
		WHERE id=? AND status='RUNNING' AND lease_owner=? AND fencing_token=? AND lease_expires_at>?
	`, progress.Stage, progress.Candidate, progress.Previous, now.Add(progress.LeaseDuration), now,
		progress.ID, progress.WorkerID, progress.FencingToken, now)
	return requireOneRow(result, err)
}

// Complete 终结任务并释放唯一活动位，失败时保留发生阶段供审计。
func (s *MySQLStore) Complete(ctx context.Context, completion Completion) error {
	now := s.now().UTC()
	status := JobSucceeded
	stageExpression := "'COMPLETED'"
	if strings.TrimSpace(completion.FailureReason) != "" {
		status = JobFailed
		stageExpression = "stage"
	}
	recovery := completion.RecoveryStatus
	if recovery == "" {
		recovery = RecoveryNotRequired
	}
	query := `
		UPDATE test_form_runtime_sync_jobs
		SET status=?, stage=` + stageExpression + `, active_guard=NULL, failure_reason=NULLIF(?,''),
			recovery_status=?, recovery_message=NULLIF(?,''), lease_owner=NULL, lease_expires_at=NULL,
			updated_at=?, completed_at=?
		WHERE id=? AND status='RUNNING' AND lease_owner=? AND fencing_token=? AND lease_expires_at>?
	`
	result, err := s.database.ExecContext(ctx, query, status, completion.FailureReason, recovery, completion.RecoveryMessage,
		now, now, completion.ID, completion.WorkerID, completion.FencingToken, now)
	return requireOneRow(result, err)
}

// requireOneRow 将租约条件未命中统一映射为 stale lease。
func requireOneRow(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrStaleLease
	}
	return nil
}

const jobSelect = `
	SELECT id,status,stage,source_repository,source_branch,source_head,source_dirty,source_changed_files,source_inspected_at,
		candidate_version,previous_version,failure_reason,recovery_status,recovery_message,
		lease_owner,lease_expires_at,fencing_token,attempt_count,created_at,started_at,updated_at,completed_at
	FROM test_form_runtime_sync_jobs
`

type rowScanner interface {
	Scan(...any) error
}

// scanJob 解码任务行并处理可空部署字段。
func scanJob(row rowScanner) (Job, error) {
	var job Job
	var changed []byte
	var candidate, previous, failure, recovery, recoveryMessage, owner sql.NullString
	var leaseExpires, started, completed sql.NullTime
	err := row.Scan(
		&job.ID, &job.Status, &job.Stage, &job.Source.Repository, &job.Source.Branch, &job.Source.Head,
		&job.Source.Dirty, &changed, &job.Source.InspectedAt, &candidate, &previous, &failure, &recovery, &recoveryMessage,
		&owner, &leaseExpires, &job.FencingToken, &job.AttemptCount, &job.CreatedAt, &started, &job.UpdatedAt, &completed,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, err
	}
	if err := json.Unmarshal(changed, &job.Source.ChangedFiles); err != nil {
		return Job{}, err
	}
	job.Candidate = candidate.String
	job.Previous = previous.String
	job.FailureReason = failure.String
	job.RecoveryStatus = RecoveryStatus(recovery.String)
	job.RecoveryMessage = recoveryMessage.String
	job.LeaseOwner = owner.String
	if leaseExpires.Valid {
		job.LeaseExpiresAt = &leaseExpires.Time
	}
	if started.Valid {
		job.StartedAt = &started.Time
	}
	if completed.Valid {
		job.CompletedAt = &completed.Time
	}
	return job, nil
}
