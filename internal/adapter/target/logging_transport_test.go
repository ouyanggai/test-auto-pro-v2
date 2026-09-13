package target

import "testing"

// F-034 评审 #3：传输层必须把目标业务包络的 message/code 安全摘要写入网络日志记录，
// 供运行详情“接口返回什么”直接展示目标原文；摘要压成单行且超长截断，不携带正文。
func TestInspectEnvelopeExtractsMessageAndCode(t *testing.T) {
	kind, _, _, message, code := inspectEnvelope(`{"isSuccess":false,"errorType":"custom_choose","message":"手动条件分支,请选择","code":"ERROR_99999","data":{}}`)
	if kind != "business_failure" {
		t.Fatalf("isSuccess=false 应为业务失败：%s", kind)
	}
	if message != "手动条件分支,请选择" || code != "ERROR_99999" {
		t.Fatalf("message/code 摘要应原样提取：message=%q code=%q", message, code)
	}
	kind, _, _, message, code = inspectEnvelope(`{"isSuccess":true,"message":"ok"}`)
	if kind != "business_success" || message != "ok" || code != "" {
		t.Fatalf("成功包络摘要不正确：kind=%s message=%q code=%q", kind, message, code)
	}
	// 多行超长文案压成单行并截断，保证单行日志格式。
	long := "a\\nb\\tc"
	kind, _, _, message, _ = inspectEnvelope(`{"isSuccess":false,"message":"` + long + `"}`)
	if kind != "business_failure" || message != "a b c" {
		t.Fatalf("多行摘要应压成单行：message=%q", message)
	}
	// 非包络正文不产生摘要。
	if _, _, _, message, code := inspectEnvelope("<html>502</html>"); message != "" || code != "" {
		t.Fatalf("非包络响应不应产生摘要：message=%q code=%q", message, code)
	}
}
