package run_orchestration_test

import (
	"fmt"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/service"
)

// TestF016PhaseTimingsKeyedByStepAndAttempt 复核评审缺陷 5 的修复回归：
// 阶段时间轴必须按 step_id:attempt 归组。同一尝试里 submit 之后的行带 trace_id、
// 之前的行不带，若按 trace_id 归组会把一次尝试拆成两个键，界面永远凑不齐七阶段耗时。
func TestF016PhaseTimingsKeyedByStepAndAttempt(t *testing.T) {
	lines := []string{
		"time=2026-09-05_08:00:00 level=info step_id=1 attempt=1 phase=plan message=a",
		"time=2026-09-05_08:00:01 level=info step_id=1 attempt=1 phase=gate message=a",
		"time=2026-09-05_08:00:02 level=info step_id=1 attempt=1 phase=control message=a",
		"time=2026-09-05_08:00:03 level=info step_id=1 attempt=1 phase=prepare message=a",
		"time=2026-09-05_08:00:04 level=info step_id=1 attempt=1 phase=submit trace_id=t-1 curl_trace_id=t-1 message=a",
		"time=2026-09-05_08:00:06 level=info step_id=1 attempt=1 phase=verify trace_id=t-1 curl_trace_id=t-1 message=a",
		"time=2026-09-05_08:00:08 level=info step_id=1 attempt=1 phase=settle trace_id=t-1 curl_trace_id=t-1 message=a",
		"time=2026-09-05_08:01:00 level=info step_id=1 attempt=2 phase=plan message=a",
		"time=2026-09-05_08:01:02 level=info step_id=1 attempt=2 phase=settle message=a",
	}
	timings := service.ParsePhaseTimingsForTest(strings.NewReader(strings.Join(lines, "\n")))

	first, ok := timings["1:1"]
	if !ok {
		t.Fatalf("尝试 1 必须按 step_id:attempt 归组，实际键 %v", keysOf(timings))
	}
	// 阶段耗时 = 下一阶段开始 − 本阶段开始；settle 是末阶段，耗时为 0。
	for phase, want := range map[string]int64{"plan": 1000, "gate": 1000, "control": 1000, "prepare": 1000, "submit": 2000, "verify": 2000, "settle": 0} {
		if got := first[phase]; got != want {
			t.Fatalf("阶段 %s 耗时应为 %dms，实际 %dms", phase, want, got)
		}
	}
	second, ok := timings["1:2"]
	if !ok {
		t.Fatalf("重放尝试（attempt=2）必须独立归组，实际键 %v", keysOf(timings))
	}
	if second["plan"] != 2000 {
		t.Fatalf("重放尝试的 plan 耗时应为 2000ms，实际 %dms", second["plan"])
	}
}

// keysOf 收集耗时表的全部键，用于失败信息。
func keysOf(m map[string]map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, fmt.Sprintf("%q", key))
	}
	return keys
}
