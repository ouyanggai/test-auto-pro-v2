// T013 运行详情投影契约：三阶段划分、请求摘要不携带 SID/正文、失败口径如实。
package runtimeexec

import (
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/runtimeexec"
)

// TestBuildDetailProjectionThreeStages 三阶段投影：主写前的读取、主写、后置业务各自计数。
func TestBuildDetailProjectionThreeStages(t *testing.T) {
	trace := json.RawMessage(`[
		{"seq":1,"method":"POST","url":"https://t/api/web/flowTemplateApi/findById","state":"fulfilled","httpStatus":200},
		{"seq":2,"method":"POST","url":"https://t/api/web/flowInstanceApi/submit","state":"fulfilled","httpStatus":200},
		{"seq":3,"method":"POST","url":"https://t/api/web/file/api/relationFile/saveBatch","state":"fulfilled","httpStatus":200}
	]`)
	detail := runtimeexec.BuildDetailProjection(runtimeexec.ActionRunResult{Status: "success"}, trace, "exec-engine/v1")
	if len(detail.Stages) != 3 {
		t.Fatalf("必须投影为三阶段：%v", detail.Stages)
	}
	if detail.Stages[0].Name != "表单业务处理" || detail.Stages[1].Name != "流程动作" || detail.Stages[2].Name != "提交后业务处理" {
		t.Fatalf("阶段名与产品口径不一致：%v", detail.Stages)
	}
	if detail.Stages[0].Requests != 1 || detail.Stages[1].Requests != 1 || detail.Stages[2].Requests != 1 {
		t.Fatalf("阶段计数错误：%v", detail.Stages)
	}
}

// TestDetailProjectionSanitized 摘要不得携带 SID、查询串或完整正文。
func TestDetailProjectionSanitized(t *testing.T) {
	trace := json.RawMessage(`[
		{"seq":1,"method":"POST","url":"https://t/api/web/flowInstanceApi/submit?sid=SECRET","state":"fulfilled","httpStatus":200}
	]`)
	detail := runtimeexec.BuildDetailProjection(runtimeexec.ActionRunResult{Status: "success"}, trace, "v1")
	raw, _ := json.Marshal(detail)
	joined := string(raw)
	if strings.Contains(joined, "SECRET") || strings.Contains(joined, "sid=") {
		t.Fatalf("投影泄漏敏感信息：%s", joined)
	}
	for _, request := range detail.Requests {
		if strings.Contains(request.Path, "?") {
			t.Fatalf("请求摘要不得包含查询串：%s", request.Path)
		}
	}
}

// TestDetailProjectionKeepsBusinessSummary 运行详情必须保留写前/写后表单证据，不能只存在于瞬时返回值。
func TestDetailProjectionKeepsBusinessSummary(t *testing.T) {
	summary := json.RawMessage(`{"fieldCountBefore":3,"fieldCountAfter":3,"formBefore":{"a":1},"formAfter":{"a":1}}`)
	detail := runtimeexec.BuildDetailProjection(runtimeexec.ActionRunResult{Status: "success", BusinessSummary: summary}, nil, "v1")
	if string(detail.BusinessSummary) != string(summary) {
		t.Fatalf("运行详情丢失业务摘要：%s", detail.BusinessSummary)
	}
}

// TestDetailProjectionFailureStatuses 失败口径：主写被拒 → 流程动作 failed；后置失败 → 提交后阶段 failed。
func TestDetailProjectionFailureStatuses(t *testing.T) {
	rejected := runtimeexec.BuildDetailProjection(runtimeexec.ActionRunResult{Status: "target_rejected"}, json.RawMessage(`[
		{"seq":1,"method":"POST","url":"https://t/api/flowInstanceApi/audit","state":"fulfilled","httpStatus":200}
	]`), "v1")
	if rejected.Stages[1].State != runtimeexec.StageFailed {
		t.Fatalf("目标拒绝必须把流程动作标记 failed：%v", rejected.Stages[1])
	}
	uncertain := runtimeexec.BuildDetailProjection(runtimeexec.ActionRunResult{Status: "uncertain"}, json.RawMessage(`[
		{"seq":1,"method":"POST","url":"https://t/api/web/flowInstanceApi/submit","state":"forwarding"}
	]`), "v1")
	if uncertain.Stages[1].State != runtimeexec.StageUncertain {
		t.Fatalf("在途挂起必须把流程动作标记 uncertain：%v", uncertain.Stages[1])
	}
	business := runtimeexec.BuildDetailProjection(runtimeexec.ActionRunResult{Status: "business_failed"}, json.RawMessage(`[
		{"seq":1,"method":"POST","url":"https://t/api/web/flowInstanceApi/submit","state":"fulfilled","httpStatus":200},
		{"seq":2,"method":"POST","url":"https://t/api/web/file/saveBatch","state":"aborted"}
	]`), "v1")
	if business.Stages[2].State != runtimeexec.StageFailed {
		t.Fatalf("后置失败必须把提交后阶段标记 failed：%v", business.Stages[2])
	}
}

// TestDetailProjectionKeepsSequenceAndReadWriteClass 页面 POST 读取不能被当成写入，附言是真实写入。
func TestDetailProjectionKeepsSequenceAndReadWriteClass(t *testing.T) {
	trace := json.RawMessage(`[
		{"seq":7,"method":"POST","url":"https://t/api/web/formProxy/findById?sid=SECRET","state":"fulfilled","httpStatus":200,"requestAt":"2026-10-15T01:00:00.010Z","durationMs":9000},
		{"seq":8,"method":"POST","url":"https://t/api/web/flowInstancePostscript/save","state":"blocked","abortReason":"附言已登记，禁止重发"}
	]`)
	original := "  错误 原话\n" + strings.Repeat("原因", 150)
	detail := runtimeexec.BuildDetailProjection(runtimeexec.ActionRunResult{Status: "business_failed", Message: original}, trace, "v2")
	if detail.Message != original || len(detail.Requests) != 2 {
		t.Fatalf("原话和逐请求轨迹必须保留：%+v", detail)
	}
	read, write := detail.Requests[0], detail.Requests[1]
	if read.Seq != 7 || read.RequestClass != "read" || read.RequestAt != "2026-10-15T01:00:00.010Z" || write.Seq != 8 || write.RequestClass != "write" || write.AbortReason != "附言已登记，禁止重发" {
		t.Fatalf("序号、分类或拦截说明错误：%+v", detail.Requests)
	}
	raw, _ := json.Marshal(detail)
	if strings.Contains(string(raw), "durationMs") || strings.Contains(string(raw), "SECRET") {
		t.Fatalf("页面墙钟耗时不能冒充接口传输耗时：%s", raw)
	}
}

// TestNewWorkflowSubmitIsFlowWrite 新工作流正式提交必须进入“流程动作”阶段并按写请求分类。
func TestNewWorkflowSubmitIsFlowWrite(t *testing.T) {
	trace := json.RawMessage(`[{"seq":1,"method":"POST","url":"https://t/api` + target.RuntimeSubmitEndpoint + `","state":"fulfilled","httpStatus":200}]`)
	detail := runtimeexec.BuildDetailProjection(runtimeexec.ActionRunResult{Status: "success"}, trace, "v2")
	if detail.Stages[1].Requests != 1 || detail.Requests[0].RequestClass != "write" {
		t.Fatalf("新提交阶段或请求分类错误：%+v", detail)
	}
}

// TestContractBusinessFailureProjectsToFormStage 合同业务前置失败必须显示在表单业务阶段，流程动作保持未发。
func TestContractBusinessFailureProjectsToFormStage(t *testing.T) {
	trace := json.RawMessage(`[{"seq":1,"method":"POST","url":"https://t/api` + target.ContractReviewModifyEndpoint + `","state":"fulfilled","httpStatus":200}]`)
	detail := runtimeexec.BuildDetailProjection(runtimeexec.ActionRunResult{
		Status: "business_failed", Message: "ERROR_99999：合同不存在或发起未成功",
	}, trace, "v2")
	if detail.Stages[0].State != runtimeexec.StageFailed || detail.Stages[1].State != runtimeexec.StagePending {
		t.Fatalf("合同业务失败必须落在表单业务阶段：%v", detail.Stages)
	}
	if detail.Stages[0].Note == "" {
		t.Fatal("表单业务阶段必须保留目标失败原因")
	}
	if detail.Requests[0].RequestClass != "write" {
		t.Fatalf("合同修改请求必须分类为写：%v", detail.Requests[0])
	}
}
