// T013 运行详情投影契约：三阶段划分、请求摘要不携带 SID/正文、失败口径如实。
package runtimeexec

import (
	"encoding/json"
	"strings"
	"testing"

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
