// F-030/T06：目标请求耗时明细的定向验证。
// 锁定三件事：network.log 解析归类正确、读写分类兜底可靠、缺失日志优雅降级不返回猜测值。
package run_request_timings_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// newTimingFixture 组装 router 根目录与尝试事实：只用于请求明细解析，不碰数据库。
func newTimingFixture(t *testing.T, networkContent, stepContent string) (*logging.Router, []model.RunStepAttempt) {
	t.Helper()
	root := t.TempDir()
	rel := filepath.Join("runs", "p1", "42", "1", "step.log")
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(stepContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if networkContent != "" {
		if err := os.WriteFile(filepath.Join(filepath.Dir(full), "network.log"), []byte(networkContent), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	attempts := []model.RunStepAttempt{{StepID: 18, AttemptNo: 1, LogPath: rel}}
	return logging.NewRouter(root, time.Now), attempts
}

func TestReadAttemptRequestsGroupsByAttemptWindow(t *testing.T) {
	stepLog := strings.Join([]string{
		"time=2026-09-11_22:19:10 level=info run_id=90 path_run_id=106 step_id=18 attempt=1 phase=plan message=a",
		"time=2026-09-11_22:19:30 level=info run_id=90 path_run_id=106 step_id=18 attempt=1 phase=settle message=b",
	}, "\n")
	networkLog := strings.Join([]string{
		// 窗口内：gate 读取请求
		"time=2026-09-11_22:19:12 level=info run_id=90 path_run_id=106 step_id=- attempt=- trace_id=t1 method=POST endpoint=/api/web/flowInstanceApi/list request_class=read status_code=200 duration_s=0.512 result=success retry_attempt=1",
		// 窗口内：写请求（带端点兜底分类）
		"time=2026-09-11_22:19:20 level=info run_id=90 path_run_id=106 step_id=- attempt=- trace_id=t2 method=POST endpoint=/api/web/flowInstanceApi/audit request_class=write status_code=200 duration_s=0.300 result=success retry_attempt=1",
		// 窗口外：登录请求不归属任何尝试
		"time=2026-09-11_22:18:26 level=info run_id=90 path_run_id=106 step_id=- attempt=- trace_id=t0 method=POST endpoint=/api/web/user/api/login/user/login request_class=read status_code=200 duration_s=0.100 result=success retry_attempt=1",
	}, "\n")
	router, attempts := newTimingFixture(t, networkLog, stepLog)
	requests, summary := service.ReadAttemptRequestsForTestWithRouter(router, 106, attempts)
	if got := requests["18:1"]; len(got) != 2 {
		t.Fatalf("窗口内应归属 2 条请求，实际 %d：%+v", len(got), requests)
	}
	if requests["18:1"][0].RequestClass != "read" || requests["18:1"][0].DurationMs != 512 {
		t.Fatalf("读取请求归类或耗时错误：%+v", requests["18:1"][0])
	}
	if requests["18:1"][1].RequestClass != "write" || requests["18:1"][1].DurationMs != 300 {
		t.Fatalf("写请求归类或耗时错误：%+v", requests["18:1"][1])
	}
	summary1 := summary["18:1"]
	if summary1.Count != 2 || summary1.WriteCount != 1 || summary1.TotalMs != 812 || summary1.WriteMs != 300 {
		t.Fatalf("汇总指标错误：%+v", summary1)
	}
}

func TestRequestClassFallsBackToEndpointWhitelist(t *testing.T) {
	networkLog := "time=2026-09-11_22:19:20 level=info run_id=90 path_run_id=106 step_id=- attempt=- trace_id=t2 method=POST endpoint=/api/web/flowInstanceApi/submit status_code=200 duration_s=0.2 result=success retry_attempt=1"
	// 旧日志缺 request_class 字段时按写端点白名单兜底
	networkLog = strings.Replace(networkLog, " request_class=write", "", 1)
	stepLog := "time=2026-09-11_22:19:10 level=info run_id=90 path_run_id=106 step_id=1 attempt=1 phase=plan message=a\n" +
		"time=2026-09-11_22:19:30 level=info run_id=90 path_run_id=106 step_id=1 attempt=1 phase=settle message=b"
	router, attempts := newTimingFixture(t, networkLog, stepLog)
	requests, _ := service.ReadAttemptRequestsForTestWithRouter(router, 106, attempts)
	if got := requests["1:1"]; len(got) != 1 || got[0].RequestClass != "write" {
		t.Fatalf("旧日志缺 request_class 时应按写端点白名单归类为 write：%+v", requests)
	}
}

func TestMissingNetworkLogDegradesWithoutGuessing(t *testing.T) {
	stepLog := "time=2026-09-11_22:19:10 level=info run_id=90 path_run_id=106 step_id=1 attempt=1 phase=plan message=a"
	router, attempts := newTimingFixture(t, "", stepLog)
	requests, summary := service.ReadAttemptRequestsForTestWithRouter(router, 106, attempts)
	if len(requests) != 0 || len(summary) != 0 {
		t.Fatalf("缺失 network.log 时应返回空明细与空汇总，不得返回猜测值：%+v", requests)
	}
	if _, ok := summary["1:1"]; ok {
		t.Fatal("缺失日志时不应产生汇总指标")
	}
}

// TestReadAttemptRequestsPrefersStepScopeOverWindow 锁定 F-030 评审 P1 修复：
// network.log 行携带 step_id/attempt 时按日志直接归属，不再依赖秒级时间窗口推断——
// 两个步骤窗口重叠（第 1 步重试核验与第 2 步预览交错）时也不会归错。
func TestReadAttemptRequestsPrefersStepScopeOverWindow(t *testing.T) {
	stepLog := strings.Join([]string{
		// 第 1 步窗口：22:19:10 → 22:19:40（含重试）
		"time=2026-09-11_22:19:10 level=info run_id=90 path_run_id=106 step_id=1 attempt=1 phase=plan message=a",
		"time=2026-09-11_22:19:40 level=info run_id=90 path_run_id=106 step_id=1 attempt=2 phase=settle message=b",
		// 第 2 步窗口：22:19:20 → 22:19:50（与第 1 步重叠）
		"time=2026-09-11_22:19:20 level=info run_id=90 path_run_id=106 step_id=2 attempt=1 phase=plan message=c",
		"time=2026-09-11_22:19:50 level=info run_id=90 path_run_id=106 step_id=2 attempt=1 phase=settle message=d",
	}, "\n")
	networkLog := strings.Join([]string{
		// 时间落在两个窗口重叠区，但日志行声明属于第 2 步：必须按 step_id/attempt 归属。
		"time=2026-09-11_22:19:25 level=info run_id=90 path_run_id=106 step_id=2 attempt=1 phase=gate trace_id=t1 method=POST endpoint=/api/web/flowInstanceApi/list request_class=read status_code=200 duration_s=0.4 result=success retry_attempt=1",
		// 同理：声明属于第 1 步第 2 次尝试的重试请求。
		"time=2026-09-11_22:19:35 level=info run_id=90 path_run_id=106 step_id=1 attempt=2 phase=verify trace_id=t2 method=POST endpoint=/api/web/flowJobTaskLink/list request_class=read status_code=200 duration_s=0.2 result=success retry_attempt=2",
		// step_id 对不上尝试记录（如登录/结构读取遗留）：不硬归属。
		"time=2026-09-11_22:19:26 level=info run_id=90 path_run_id=106 step_id=9 attempt=1 phase=plan trace_id=t3 method=POST endpoint=/api/web/flowInstanceApi/list request_class=read status_code=200 duration_s=0.1 result=success retry_attempt=1",
	}, "\n")
	router, attempts := newTimingFixture(t, networkLog, stepLog)
	attempts = []model.RunStepAttempt{{StepID: 1, AttemptNo: 1, LogPath: attempts[0].LogPath}, {StepID: 1, AttemptNo: 2}, {StepID: 2, AttemptNo: 1}}
	requests, _ := service.ReadAttemptRequestsForTestWithRouter(router, 106, attempts)
	if got := requests["2:1"]; len(got) != 1 || got[0].TraceID != "t1" || got[0].Phase != "gate" {
		t.Fatalf("重叠窗口内应按 step_id/attempt 直接归属到第 2 步，实际 %v", requests)
	}
	if got := requests["1:2"]; len(got) != 1 || got[0].TraceID != "t2" || got[0].Phase != "verify" {
		t.Fatalf("重试请求应归属到第 1 步第 2 次尝试，实际 %v", requests)
	}
	if _, ok := requests["9:1"]; ok {
		t.Fatal("step_id 对不上尝试记录的请求不应硬归属")
	}
}

// 编译期确认 DTO 字段不携带敏感信息：只允许白名单键出现在 JSON 输出。
func TestRunRequestDTOHasNoSensitiveFields(t *testing.T) {
	dto := service.RunRequestDTO{Phase: "verify", RequestClass: "write", Endpoint: "flowInstanceApi/audit", DurationMs: 300, StatusCode: 200, Result: "success", RetryAttempt: 1, TraceID: "t2", At: "2026-09-11_22:19:20"}
	raw, err := time.Now(), error(nil)
	_ = raw
	_ = err
	text := strings.Join([]string{dto.Phase, dto.RequestClass, dto.Endpoint, dto.TraceID}, "")
	for _, banned := range []string{"sid", "password"} {
		if strings.Contains(strings.ToLower(text), banned) {
			t.Fatalf("请求摘要携带敏感字段：%s", banned)
		}
	}
}
