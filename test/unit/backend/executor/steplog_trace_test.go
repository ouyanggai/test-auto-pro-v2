package executor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/logging"
)

// TestF016StepLogCarriesTraceKeysAfterSubmit 复核评审缺陷 5 的修复回归：
// step.log 在写请求发出后的阶段行必须携带 trace_id/curl_trace_id（纲领第 6.3 节链路贯穿），
// 写请求发出前的行没有链路 ID 就不得伪造；时间戳与同目录其他日志一致使用本地时钟。
func TestF016StepLogCarriesTraceKeysAfterSubmit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "step.log")
	writer := logging.NewWriter(path)
	scope := logging.Scope{PlanID: "1", RunID: "2", PathRunID: "3"}
	fixed := time.Date(2026, 9, 5, 8, 0, 0, 0, time.Local)
	log := step.NewStepLog(writer, scope, func() time.Time { return fixed })

	log.Phase("prepare", 1, 1, "会话就绪")
	if before, err := os.ReadFile(path); err != nil {
		t.Fatalf("读取 step.log 失败：%v", err)
	} else if strings.Contains(string(before), "trace_id=") {
		t.Fatalf("写请求发出前的行不得携带链路 ID：%s", before)
	}

	log.SetTraceID("t-1")
	if log.Phase("verify", 1, 1, "三值判定") == 0 {
		t.Fatal("阶段行应写入成功")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 step.log 失败：%v", err)
	}
	text := string(after)
	for _, key := range []string{"trace_id=t-1", "curl_trace_id=t-1", "phase=verify", "step_id=1", "attempt=1"} {
		if !strings.Contains(text, key) {
			t.Fatalf("submit 之后的阶段行缺少 %s：%s", key, text)
		}
	}
	// 本地时钟：固定时刻按本地时区格式化，不得出现 UTC 偏移后的时间。
	if !strings.Contains(text, fixed.Format("2006-01-02_15:04:05")) {
		t.Fatalf("step.log 应使用与同目录其他日志一致的本地时间：%s", text)
	}
}
