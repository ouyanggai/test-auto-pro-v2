package integration_test

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// TestF042ExecutorMsRoundTrip 锁定执行器真实计时落库与回读（F-042 选项 B）：
// runtime 步骤写入真实毫秒（含真实 0）后按原值读回；非 runtime 步骤写 NULL，回读为未知。
// 关键不变量：真实 0 毫秒与「未知」必须可区分——0 存为已知值，只有未知才落 NULL，
// 报告才能正确显示「无执行器环节 / 未知 / 真实值」而不把真实 0 当缺失。
func TestF042ExecutorMsRoundTrip(t *testing.T) {
	store := openF020Store(t)
	ctx := context.Background()
	now := time.Now().UTC()

	_, pathRuns, err := store.runs.CreateRunWithPaths(ctx, repository.CreateRunInput{
		PlanID: 900, ExecutionPathIDs: []uint64{9001},
		Mode: model.RunModeAuto, Trigger: model.RunTriggerManual, MaxConcurrency: f020MaxConcurrency(1),
	})
	if err != nil {
		t.Fatalf("创建运行失败：%v", err)
	}
	pathRunID := pathRuns[0].ID

	// record 落一条步骤与尝试；executorMs/known 表达执行器真实计时的三种事实形态。
	record := func(stepNo int, action string, executorMs int64, known bool) {
		if _, err := store.runs.RecordStepAttempt(ctx, model.RunStep{
			PathRunID: pathRunID, StepNo: stepNo, Source: "user", Action: action, NodeKey: "node",
			Status: model.RunStepSucceeded, StartedAt: now, FinishedAt: now,
		}, model.RunStepAttempt{
			PathRunID: pathRunID, AttemptNo: 1, Verdict: "confirmed_success", TraceID: "t", LogPath: "logs/x", LogLine: 1,
			ExecutorMs: executorMs, ExecutorMsKnown: known,
		}, now); err != nil {
			t.Fatalf("落账步骤 %d 失败：%v", stepNo, err)
		}
	}

	record(1, "submit", 4321, true)   // runtime 真实耗时
	record(2, "approve", 0, true)     // runtime 真实 0 毫秒
	record(3, "save_draft", 0, false) // 非 runtime（Go 直发），执行器耗时未知

	attempts, err := store.runs.ListRunAttempts(ctx, pathRunID)
	if err != nil {
		t.Fatalf("读取尝试失败：%v", err)
	}
	if len(attempts) != 3 {
		t.Fatalf("应读回 3 条尝试，实际 %d", len(attempts))
	}
	// ListRunAttempts 按 id 升序返回，与落账顺序一致。
	if !attempts[0].ExecutorMsKnown || attempts[0].ExecutorMs != 4321 {
		t.Fatalf("runtime 真实耗时应读回 4321/已知，实际 %d/%v", attempts[0].ExecutorMs, attempts[0].ExecutorMsKnown)
	}
	if !attempts[1].ExecutorMsKnown || attempts[1].ExecutorMs != 0 {
		t.Fatalf("真实 0 毫秒应读回 0/已知（区别于未知），实际 %d/%v", attempts[1].ExecutorMs, attempts[1].ExecutorMsKnown)
	}
	if attempts[2].ExecutorMsKnown || attempts[2].ExecutorMs != 0 {
		t.Fatalf("非 runtime 步骤应读回未知（NULL），实际 %d/%v", attempts[2].ExecutorMs, attempts[2].ExecutorMsKnown)
	}
}
