// 请求详情只按步骤事实归属，覆盖数据库主键与步骤序号不同、日志缺失和目标错误保真。
package run_request_timings_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// newTimingFixture 使用与真实落账一致的不同主键和步序，不依赖 step.log 推断请求归属。
func newTimingFixture(t *testing.T, content *string) (*logging.Router, []model.RunStep, []model.RunStepAttempt) {
	t.Helper()
	root := t.TempDir()
	rel := filepath.Join("runs", "测试计划", "paths", "测试路径", "step.log")
	dir := filepath.Join(root, filepath.Dir(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if content != nil {
		if err := os.WriteFile(filepath.Join(dir, "network.log"), []byte(*content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	steps := []model.RunStep{{ID: 70018, StepNo: 18}, {ID: 70019, StepNo: 19}}
	attempts := []model.RunStepAttempt{{StepID: 70018, AttemptNo: 1, LogPath: rel}, {StepID: 70018, AttemptNo: 2, LogPath: rel}, {StepID: 70019, AttemptNo: 1, LogPath: rel}}
	return logging.NewRouter(root, time.Now), steps, attempts
}

// TestRequestIdentityUsesStepNumber 验证主键不等于步序时仍精确归属，不混入相邻动作或其他路径。
func TestRequestIdentityUsesStepNumber(t *testing.T) {
	content := strings.Join([]string{
		"path_run_id=106 step_id=18 attempt=1 phase=prepare runtime_request_seq=1 trace_id=t1 method=POST endpoint=/api/web/formProxy/findById request_class=read status_code=200 duration_s=0.512 result=success retry_attempt=1",
		"path_run_id=106 step_id=18 attempt=2 phase=submit runtime_request_seq=4 trace_id=t2 method=POST endpoint=/api/web/flowInstancePostscript/save request_class=write status_code=200 duration_s=0.300 result=success retry_attempt=2",
		"path_run_id=106 step_id=19 attempt=1 trace_id=t3 method=POST endpoint=/api/web/flowInstanceApi/audit request_class=write status_code=200 duration_s=0.100 result=success retry_attempt=1",
		"path_run_id=999 step_id=18 attempt=1 trace_id=other-path method=POST endpoint=/api/web/flowInstanceApi/audit request_class=write status_code=200 duration_s=0.100 result=success retry_attempt=1",
		"path_run_id=106 step_id=70018 attempt=1 trace_id=wrong-id method=POST endpoint=/api/web/flowInstanceApi/audit request_class=write status_code=200 duration_s=0.100 result=success retry_attempt=1",
	}, "\n")
	router, steps, attempts := newTimingFixture(t, &content)
	requests, summaries, available := service.ReadAttemptRequestsForTestWithRouter(router, 106, steps, attempts)
	if !available || len(requests) != 3 || len(requests["18:1"]) != 1 || len(requests["19:1"]) != 1 {
		t.Fatalf("应只归属三次真实尝试：%+v，完整=%v", requests, available)
	}
	read, write := requests["18:1"][0], requests["18:2"][0]
	if read.Method != "POST" || read.RequestClass != "read" || read.DurationMs != 512 || read.RuntimeRequestSeq != 1 || read.Phase != "prepare" {
		t.Fatalf("读取请求事实不正确：%+v", read)
	}
	if write.RequestClass != "write" || write.RetryAttempt != 2 || write.RuntimeRequestSeq != 4 || summaries["18:2"].WriteCount != 1 {
		t.Fatalf("附言写入或重试事实丢失：%+v", write)
	}
}

// TestRequestOrderUsesTransportStart 验证并发响应先后相反时按传输开始时刻排序。
func TestRequestOrderUsesTransportStart(t *testing.T) {
	content := strings.Join([]string{
		"path_run_id=106 step_id=18 attempt=1 request_at=2026-10-15T10:00:00.020+08:00 trace_id=second method=POST endpoint=/api/web/flowInstancePostscript/save request_class=write status_code=200 duration_s=0.010 result=success",
		"path_run_id=106 step_id=18 attempt=1 request_at=2026-10-15T10:00:00.010+08:00 trace_id=first method=POST endpoint=/api/web/flowInstanceApi/transpond request_class=write status_code=200 duration_s=0.100 result=success",
	}, "\n")
	router, steps, attempts := newTimingFixture(t, &content)
	requests, _, _ := service.ReadAttemptRequestsForTestWithRouter(router, 106, steps, attempts)
	rows := requests["18:1"]
	if len(rows) != 2 || rows[0].TraceID != "first" || rows[1].TraceID != "second" {
		t.Fatalf("请求顺序应按开始时刻而非完成时刻：%+v", rows)
	}
}

// TestRequestEvidenceDistinguishesEmptyMissingAndUnreadable 验证缺失、读取中断和空文件给出不同证据状态。
func TestRequestEvidenceDistinguishesEmptyMissingAndUnreadable(t *testing.T) {
	cases := []struct {
		name      string
		content   *string
		available bool
	}{
		{name: "日志不存在"},
		{name: "可读空日志", content: stringPointer(""), available: true},
		{name: "扫描中断", content: stringPointer(strings.Repeat("x", 1024*1024+1))},
		{name: "没有动作归属", content: stringPointer("path_run_id=106 step_id=- attempt=- trace_id=unscoped method=POST endpoint=/api/web/flowInstanceApi/list result=success")},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			router, steps, attempts := newTimingFixture(t, item.content)
			requests, _, available := service.ReadAttemptRequestsForTestWithRouter(router, 106, steps, attempts)
			if available != item.available || len(requests) != 0 {
				t.Fatalf("空状态不能冒充未发出请求：可用=%v，明细=%+v", available, requests)
			}
		})
	}
}

// stringPointer 为测试明确区分日志不存在与存在但为空，不用于生产兼容逻辑。
func stringPointer(value string) *string { return &value }

// TestTargetMessageIsKeptVerbatim 验证长中文、空格、制表符和换行从日志解码后完整保留。
func TestTargetMessageIsKeptVerbatim(t *testing.T) {
	message := "  目标错误 原话\n\t" + strings.Repeat("具体原因", 100) + "  "
	content := "path_run_id=106 step_id=18 attempt=1 trace_id=err method=POST endpoint=/api/web/flowInstanceApi/audit request_class=write status_code=200 duration_s=0.150 result=failure code=ERROR_99999 target_message_b64=" + base64.StdEncoding.EncodeToString([]byte(message))
	router, steps, attempts := newTimingFixture(t, &content)
	attempts[0].Initial = "pre_rejected"
	requests, _, _ := service.ReadAttemptRequestsForTestWithRouter(router, 106, steps, attempts)
	row := requests["18:1"][0]
	if row.TargetMessage != message || row.TargetCode != "ERROR_99999" || !row.Blocking || !strings.Contains(row.ResultSummary, message) {
		t.Fatalf("目标错误原话不可截断或压成单行：%+v", row)
	}
}

// TestUnknownDurationKeepsPhysicalWriteCount 验证损坏耗时不冒充零，写请求次数仍按真实记录计数。
func TestUnknownDurationKeepsPhysicalWriteCount(t *testing.T) {
	lines := []string{}
	for _, duration := range []string{"-", "NaN", "Inf", "-1", "1e100"} {
		lines = append(lines, "path_run_id=106 step_id=18 attempt=1 method=POST endpoint=/api/web/flowInstancePostscript/save request_class=write duration_s="+duration+" result=failure")
	}
	content := strings.Join(lines, "\n")
	router, steps, attempts := newTimingFixture(t, &content)
	requests, summaries, _ := service.ReadAttemptRequestsForTestWithRouter(router, 106, steps, attempts)
	for _, row := range requests["18:1"] {
		if row.DurationKnown {
			t.Fatalf("损坏耗时不得当作已知：%+v", row)
		}
	}
	if summary := summaries["18:1"]; summary.WriteCount != 5 || summary.AllDurationsKnown {
		t.Fatalf("未知耗时不应影响真实写请求计数：%+v", summary)
	}
}

// TestReadFailureNeverSaysSuccess 验证无业务 message 的失败读取不会被摘要包装为读取成功。
func TestReadFailureNeverSaysSuccess(t *testing.T) {
	content := "path_run_id=106 step_id=18 attempt=1 method=POST endpoint=/api/web/flowInstanceApi/list request_class=read status_code=500 result=failure"
	router, steps, attempts := newTimingFixture(t, &content)
	requests, _, _ := service.ReadAttemptRequestsForTestWithRouter(router, 106, steps, attempts)
	if row := requests["18:1"][0]; strings.Contains(row.ResultSummary, "成功") {
		t.Fatalf("失败不能显示成功：%+v", row)
	}
}

// TestRunRequestDTOOnlyCarriesRequestFacts 验证详情契约只带请求摘要，不新增会话或完整正文字段。
func TestRunRequestDTOOnlyCarriesRequestFacts(t *testing.T) {
	raw, err := json.Marshal(service.RunRequestDTO{Method: "POST", RuntimeRequestSeq: 2, TargetMessage: "拒绝", TargetCode: "ERROR", RequestClass: "write"})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"sid", "password", "body", "headers", "responseBody"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("公开摘要不能新增正文或身份字段：%s", key)
		}
	}
	if fields["method"] != "POST" || fields["requestSeq"] != float64(2) || fields["targetMessage"] != "拒绝" {
		t.Fatalf("请求身份和原话缺失：%s", raw)
	}
}

// TestRequestRotationRetainsEarlierActionRequests 验证轮转后仍能读取保留的前面动作记录，不能把清理当作未发出。
func TestRequestRotationRetainsEarlierActionRequests(t *testing.T) {
	content := "path_run_id=106 step_id=19 attempt=1 method=POST endpoint=/web/formProxy/findById result=success"
	router, steps, attempts := newTimingFixture(t, &content)
	dir := filepath.Dir(filepath.Join(router.Root(), attempts[0].LogPath))
	older := "path_run_id=106 step_id=18 attempt=1 method=POST endpoint=/web/flowInstanceApi/transpond request_class=write result=success"
	if err := os.WriteFile(filepath.Join(dir, "network.log.1"), []byte(older), 0o644); err != nil {
		t.Fatal(err)
	}
	requests, _, available := service.ReadAttemptRequestsForTestWithRouter(router, 106, steps, attempts)
	if !available || len(requests["18:1"]) != 1 || len(requests["19:1"]) != 1 {
		t.Fatalf("轮转文件不能漏掉早先动作：%+v", requests)
	}
	if err := os.WriteFile(filepath.Join(dir, "network.log.3"), []byte(older), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, available = service.ReadAttemptRequestsForTestWithRouter(router, 106, steps, attempts)
	if available {
		t.Fatal("最老备份存在时不能假定更早记录没有被清理")
	}
}
