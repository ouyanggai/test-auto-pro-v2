package executor_test

import (
	"encoding/json"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// TestSaveDraftBuildsSubmitProtocol 锁定保存草稿与普通提交共用目标 submit 端点，
// 但通过实例 ID 和 status=draft 保持草稿语义，不能再落入通用动作不支持分支。
func TestSaveDraftBuildsSubmitProtocol(t *testing.T) {
	runCtx := step.RunContext{
		PathRun:     model.PathRun{MainInstanceRef: "draft-instance-7"},
		PathName:    "合同审批",
		FlowProxyID: "flow-proxy-7",
		Nodes:       map[string]step.NodeInfo{"start": {TargetNodeID: "start-node"}},
	}
	formData := json.RawMessage(`{"title":"草稿标题","amount":12}`)
	request, endpoint, body, err := step.BuildRequestForTest(runCtx, model.CompiledActionStep{
		Action:  model.ActionSaveDraft,
		NodeKey: "start",
	}, target.Session{CompanyID: "company-7"}, formData, "")
	if err != nil {
		t.Fatalf("保存草稿请求构造失败：%v", err)
	}
	if endpoint != target.WriteEndpointSubmit {
		t.Fatalf("保存草稿必须使用 submit 端点，实际 %s", endpoint)
	}
	submit, ok := request.(*target.SubmitFlowInstanceRequest)
	if !ok {
		t.Fatalf("保存草稿请求类型错误：%T", request)
	}
	if submit.InstanceID != "draft-instance-7" || submit.Status != "draft" {
		t.Fatalf("保存草稿必须带已有实例和 draft 状态：%+v", submit)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["id"] != "draft-instance-7" || data["status"] != "draft" {
		t.Fatalf("目标草稿正文身份或状态错误：%v", body)
	}
	if body["formDataMongoVo"] == nil {
		t.Fatalf("保存草稿必须携带表单数据：%v", body)
	}
}

// TestSaveDraftCarriesFormData 锁定保存草稿需要读取并提交实例完整表单数据，
// 避免界面修改后只更新内存状态而目标仍显示旧值。
func TestSaveDraftCarriesFormData(t *testing.T) {
	if !step.ActionCarriesFormData(model.ActionSaveDraft) {
		t.Fatal("保存草稿必须纳入表单数据动作集合")
	}
}
