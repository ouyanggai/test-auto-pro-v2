package run_events_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/engine/run"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// eventStore 仅模拟只读步骤仍处于运行中的事实，未实现的方法不参与测试。
type eventStore struct {
	repository.RunStore
	events   []model.RunEvent
	eventErr error
}

// AdvancePathRunStatus 模拟导航步骤没有 verifying 阶段，不能强行改变运行状态。
func (s *eventStore) AdvancePathRunStatus(context.Context, uint64, model.PathRunStatus, model.PathRunStatus, model.RunEvent, time.Time) (model.PathRun, error) {
	return model.PathRun{}, repository.ErrRunStatusConflict
}

// GetPathRun 返回目标已确认的当前运行状态。
func (s *eventStore) GetPathRun(context.Context, uint64) (model.PathRun, error) {
	return model.PathRun{ID: 801, RunID: 912, Status: model.PathRunStatusRunning}, nil
}

// AppendRunEvent 捕获追加事件，不写数据库或触碰目标平台。
func (s *eventStore) AppendRunEvent(_ context.Context, event model.RunEvent, _ time.Time) error {
	s.events = append(s.events, event)
	return s.eventErr
}

// TestReadonlyStepsStillProduceFlowEvents 验证导航与自动跳过仍有可观察的节点结果，但不改变状态。
func TestReadonlyStepsStillProduceFlowEvents(t *testing.T) {
	for _, eventErr := range []error{nil, errors.New("模拟事件存储故障")} {
		store := &eventStore{eventErr: eventErr}
		service := run.NewService(store, "test-worker", time.Minute, time.Now)
		if err := service.BackToRunning(context.Background(), 801, "第 5 步\n节点「条件分支」\n导航步骤只读校验通过"); err != nil {
			t.Fatalf("观察记录不能改变只读步骤结论：%v", err)
		}
		if len(store.events) != 1 || store.events[0].Kind != "path_run_step_settled" || store.events[0].RunID != 912 || *store.events[0].PathRunID != 801 {
			t.Fatalf("节点事件缺失或串运行：%+v", store.events)
		}
	}
}

// TestFlowEventKeepsContextAndOriginalMessage 验证事件显示局部结果在整条流程中的位置，保留长错误原话。
func TestFlowEventKeepsContextAndOriginalMessage(t *testing.T) {
	original := "  目标 错误\n\t" + strings.Repeat("原始原因", 100) + "  "
	preview := &step.StepPreview{StepNo: 5, NodeName: "审批节点", ActionName: "转发", ActorName: "张三", ReceiverName: "李四", Endpoint: "/web/flowInstanceApi/transpond"}
	text := step.SettleEventDetailForTest(preview, original)
	for _, part := range []string{"第 5 步", "节点「审批节点」", "转发", "执行人：张三", "接收人：李四", original} {
		if !strings.Contains(text, part) {
			t.Fatalf("事件缺少上下文或原话 %q：%q", part, text)
		}
	}
	if strings.Contains(text, "/web/") {
		t.Fatal("整条流程事件不重复列局部接口明细")
	}
}
