package target_test

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestInspectEnvelopeExtractsMessageAndCode 提取目标包络结果和业务码，不把 HTTP 成功当成业务成功。
func TestInspectEnvelopeExtractsMessageAndCode(t *testing.T) {
	kind, _, _, message, code := target.InspectEnvelopeForTest(`{"isSuccess":false,"errorType":"custom_choose","message":"手动条件分支,请选择","code":"ERROR_99999","data":{}}`)
	if kind != "business_failure" {
		t.Fatalf("isSuccess=false 应为业务失败：%s", kind)
	}
	if message != "手动条件分支,请选择" || code != "ERROR_99999" {
		t.Fatalf("message/code 摘要应原样提取：message=%q code=%q", message, code)
	}
	kind, _, _, message, code = target.InspectEnvelopeForTest(`{"isSuccess":true,"message":"ok"}`)
	if kind != "business_success" || message != "ok" || code != "" {
		t.Fatalf("成功包络摘要不正确：kind=%s message=%q code=%q", kind, message, code)
	}
	// 原话完整保留，单行日志编码由日志器处理。
	long := "a\\nb\\tc"
	kind, _, _, message, _ = target.InspectEnvelopeForTest(`{"isSuccess":false,"message":"` + long + `"}`)
	if kind != "business_failure" || message != "a\nb\tc" {
		t.Fatalf("多行原话必须原样保留：message=%q", message)
	}
	// 非包络正文不产生摘要。
	if _, _, _, message, code := target.InspectEnvelopeForTest("<html>502</html>"); message != "" || code != "" {
		t.Fatalf("非包络响应不应产生摘要：message=%q code=%q", message, code)
	}
}

// TestTargetMessageKeepsFullLengthAndWhitespace 锁定长中文、空白和换行不会被截断、清洗。
func TestTargetMessageKeepsFullLengthAndWhitespace(t *testing.T) {
	long := "  错误 原话\n\t" + strings.Repeat("中", 300) + "  "
	raw, err := json.Marshal(map[string]any{"isSuccess": false, "message": long})
	if err != nil {
		t.Fatal(err)
	}
	kind, _, _, message, _ := target.InspectEnvelopeForTest(string(raw))
	if kind != "business_failure" {
		t.Fatalf("包络分类不正确：%s", kind)
	}
	if message != long {
		t.Fatalf("原话必须完整保留：len=%d", len([]rune(message)))
	}
	if !utf8.ValidString(message) {
		t.Fatal("原话必须是合法 UTF-8")
	}
}
