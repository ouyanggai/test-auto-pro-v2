package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/formruntimemaintenance"
	planmysql "test-auto-pro-v2/internal/repository/mysql"
)

// TestFormRuntimeMaintenanceMySQLPersistence 验证单活动任务、租约、fencing 与阶段在临时 MySQL 中持久化。
func TestFormRuntimeMaintenanceMySQLPersistence(t *testing.T) {
	cfg := config.LoadPlanDBConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("F-007 维护任务测试缺少本机计划数据库配置名：%v", missing)
	}
	cfg.Name = temporaryPlanDatabaseName(t)
	t.Cleanup(func() { dropTemporaryPlanDatabase(t, cfg) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := planmysql.OpenAndMigrate(ctx, cfg)
	if err != nil {
		t.Fatalf("F-007 维护任务临时数据库迁移失败：%v", err)
	}
	defer database.Close()
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	store := formruntimemaintenance.NewMySQLStore(database.DB, func() time.Time { return now })
	source := formruntimemaintenance.SourceState{Repository: "rsh-flow-components", Branch: "master", Head: "bff4ef8b938db5578c3f7eab1f482a4e9388917c", InspectedAt: now}
	job, err := store.Create(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, source); !errors.Is(err, formruntimemaintenance.ErrJobAlreadyActive) {
		t.Fatalf("MySQL 单活动任务约束未生效：%v", err)
	}
	claimed, err := store.ClaimNext(ctx, formruntimemaintenance.Claim{WorkerID: "worker-a", LeaseDuration: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateProgress(ctx, formruntimemaintenance.Progress{ID: job.ID, WorkerID: "worker-a", FencingToken: claimed.FencingToken, LeaseDuration: time.Minute, Stage: formruntimemaintenance.StageRestart, Candidate: "candidate-job-1", Previous: "bootstrap-123456789012"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	stolen, err := store.ClaimNext(ctx, formruntimemaintenance.Claim{WorkerID: "worker-b", LeaseDuration: time.Minute})
	if err != nil || stolen.FencingToken <= claimed.FencingToken || stolen.Stage != formruntimemaintenance.StageRestart {
		t.Fatalf("MySQL 过期任务接管不正确：job=%+v err=%v", stolen, err)
	}
	if err := store.Complete(ctx, formruntimemaintenance.Completion{ID: job.ID, WorkerID: "worker-a", FencingToken: claimed.FencingToken}); !errors.Is(err, formruntimemaintenance.ErrStaleLease) {
		t.Fatalf("旧 fencing token 仍能完成任务：%v", err)
	}
	if err := store.Complete(ctx, formruntimemaintenance.Completion{ID: job.ID, WorkerID: "worker-b", FencingToken: stolen.FencingToken}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, job.ID)
	if err != nil || reloaded.Status != formruntimemaintenance.JobSucceeded || reloaded.Stage != formruntimemaintenance.StageCompleted || reloaded.Candidate != "candidate-job-1" {
		t.Fatalf("维护任务持久化不正确：job=%+v err=%v", reloaded, err)
	}
}
