package history_replay_test

import (
	"context"
	"sync"
	"testing"

	"test-auto-pro-v2/internal/model"
)

// modeCapturingConfigurator 记录一键配置收到的动作模式，锁定「任务级模式」端到端透传到配置器。
type modeCapturingConfigurator struct {
	mu    sync.Mutex
	modes []string
}

func (c *modeCapturingConfigurator) AutoConfigurePathActions(_ context.Context, _, _ uint64, actionMode string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.modes = append(c.modes, actionMode)
	return nil
}

func (c *modeCapturingConfigurator) captured() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.modes...)
}

// TestHistoryReplayThreadsActionModeToConfigurator 锁定：一键配置的动作模式（每一步都同意 / 每节点随机）
// 作为任务级选择，随任务持久化并原样透传给动作配置器；worker 异步执行也必须拿到用户选择的模式。
func TestHistoryReplayThreadsActionModeToConfigurator(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{"每一步都同意", model.HistoryReplayActionModeApproveOnly, model.HistoryReplayActionModeApproveOnly},
		{"每节点随机动作", model.HistoryReplayActionModeRandom, model.HistoryReplayActionModeRandom},
		{"空串回落随机", "", model.HistoryReplayActionModeRandom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replay, _, _ := replayServiceFixture(4, 4, "formmaking")
			configurator := &modeCapturingConfigurator{}
			replay.SetActionConfigurator(configurator)
			job, err := replay.Create(context.Background(), 91,
				model.HistoryReplayCreateInput{PathIDs: []uint64{101}, ActionMode: tc.input},
				"123e4567-e89b-12d3-a456-4266141747aa")
			if err != nil {
				t.Fatalf("创建回放任务失败：%v", err)
			}
			waitReplayCompletion(t, replay, 91, job.ID)
			modes := configurator.captured()
			if len(modes) != 1 || modes[0] != tc.want {
				t.Fatalf("配置器收到的动作模式不正确：期望 [%s]，实际 %v", tc.want, modes)
			}
		})
	}
}
