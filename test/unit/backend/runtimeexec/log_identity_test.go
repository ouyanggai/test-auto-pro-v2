package runtimeexec

import (
	"context"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// TestRuntimeRequestUsesDatabaseRunIDAndStepScope 页面运行号不能替代主键，独立读泵必须拿到本次日志身份。
func TestRuntimeRequestUsesDatabaseRunIDAndStepScope(t *testing.T) {
	view := &recordingTarget{}
	runner := &fakeRuntimeRunner{status: "success"}
	executor := newExecutorForRouting(t, view, runner)
	runCtx := runtimeRunCtx()
	runCtx.Run = model.Run{ID: 912, RunNo: 3, PlanID: 50, CreatedAt: time.Now()}
	runCtx.PathRun = model.PathRun{ID: 801, ExecutionPathID: 77}
	runCtx.PlanName, runCtx.PathName = "中文计划", "中文路径"
	action := runtimeStep(model.ActionSubmit)
	action.Sequence = 5
	preview := &step.StepPreview{ActorAccount: "actor-acct", ActionName: "提交"}
	executor.RunViaRuntimeForTest(context.Background(), runCtx, action, target.Session{}, preview, step.ApprovedStep{Attempt: 2})
	request := runner.lastRequest()
	if request.RunID != "912" || request.PathRunID != "801" || request.StepID != "5" {
		t.Fatalf("运行身份必须使用数据库主键：%+v", request)
	}
	scope := request.LogScope
	if scope.RunID != "912" || scope.RunSeq != "3" || scope.PathRunID != "801" || scope.PlanID != "50" || scope.ExecutionPathID != "77" || scope.StepID != "5" || scope.Attempt != "2" || scope.Phase != "submit" || scope.PlanName != "中文计划" || scope.ExecutionPathName != "中文路径" {
		t.Fatalf("转发作用域缺失或串运行：%+v", scope)
	}
	if len(view.writeCalls()) != 0 {
		t.Fatal("日志修正不能新增直连目标写请求")
	}
}
