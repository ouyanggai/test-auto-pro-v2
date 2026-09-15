// T010A 动作路由契约：八个 Go 直连动作各有一个固定端点与固定载荷构造器，
// 载荷与目标页面实际发送逐字段同源（页面出处已在注释与对照文档登记）。
package runtimeexec

import (
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

// actionEndpointCases 是八个 Go 直连动作的最小合法请求与唯一固定端点。
// 载荷形状出处（docs/s/03-审批页面全量动作对照.md 第 4 节 + T010A 页面核对附录）：
//   - add_sign:  EnterpriseExamineOpinion/AddCounterSign → updateFlowProxy（整棵代理树）
//   - transfer:  Backlog.handleHandOver → approverAppend（transfer + batchNo + approverAppendVo）
//   - rollback_previous: Backlog/FlowDialog 回退入口 → rollBackThePreviousLevel
//   - retrieve:  Finished.retrieveProcess → retrieveProcess（data{jobTaskId,id}）
//   - withdraw:  Submitted 撤回入口 → revocation（data{id,withdrawDesc}）
//   - urge:      Submitted.urgeFlow → sendUrgeMessage（顶层 flowInstanceId + data:{}）
//   - follow/unfollow: mixin/Finished → flowTracking（data.id + 顶层 tracking）
func actionEndpointCases() map[string]target.ActionWriteRequest {
	trackTrue := true
	trackFalse := false
	return map[string]target.ActionWriteRequest{
		"add_sign":          {Action: "add_sign", InstanceID: "ins", NodeProxyID: "node", UserIDs: []string{"u1"}, FlowProxyTree: []byte(`{"id":"tree"}`)},
		"transfer":          {Action: "transfer", InstanceID: "ins", JobTaskID: "task", BatchNo: "b1", NodeProxyID: "node", AuditStatus: "transfer", UserIDs: []string{"u1"}},
		"rollback_previous": {Action: "rollback_previous", InstanceID: "ins", JobTaskID: "task"},
		"retrieve":          {Action: "retrieve", InstanceID: "ins", JobTaskID: "task"},
		"withdraw":          {Action: "withdraw", InstanceID: "ins"},
		"urge":              {Action: "urge", InstanceID: "ins"},
		"follow":            {Action: "follow", InstanceID: "ins", Tracking: &trackTrue},
		"unfollow":          {Action: "unfollow", InstanceID: "ins", Tracking: &trackFalse},
	}
}

// TestGoDirectActionsHaveSingleFixedEndpoint 每个直连动作构造出的端点唯一且与登记一致。
func TestGoDirectActionsHaveSingleFixedEndpoint(t *testing.T) {
	expect := map[string]string{
		"add_sign":          target.WriteEndpointUpdateFlowProxy,
		"transfer":          target.WriteEndpointApproverAppend,
		"rollback_previous": target.WriteEndpointRollBack,
		"retrieve":          target.WriteEndpointRetrieve,
		"withdraw":          target.WriteEndpointRevocation,
		"urge":              target.WriteEndpointUrge,
		"follow":            target.WriteEndpointFlowTracking,
		"unfollow":          target.WriteEndpointFlowTracking,
	}
	for action, request := range actionEndpointCases() {
		_, endpoint, err := target.BuildActionBody(request)
		if err != nil {
			t.Fatalf("动作 %s 载荷构造失败：%v", action, err)
		}
		if endpoint != expect[action] {
			t.Fatalf("动作 %s 端点漂移：期望 %s，实际 %s", action, expect[action], endpoint)
		}
	}
}

// TestFollowUnfollowShareEndpointWithDistinctTracking 关注与取消关注共用端点，仅 tracking 布尔相反。
func TestFollowUnfollowShareEndpointWithDistinctTracking(t *testing.T) {
	cases := actionEndpointCases()
	followBody, followEndpoint, err := target.BuildActionBody(cases["follow"])
	if err != nil {
		t.Fatal(err)
	}
	unfollowBody, unfollowEndpoint, err := target.BuildActionBody(cases["unfollow"])
	if err != nil {
		t.Fatal(err)
	}
	if followEndpoint != unfollowEndpoint {
		t.Fatalf("关注/取消关注端点必须一致")
	}
	if followBody["tracking"] != true || unfollowBody["tracking"] != false {
		t.Fatalf("tracking 布尔必须相反：%v / %v", followBody["tracking"], unfollowBody["tracking"])
	}
}

// TestTransferPayloadMatchesTargetPage 移交载荷与 Backlog.handleHandOver 逐字段同源：
// data{id,jobTaskId,batchNo,auditRecord{transfer,executeDesc}} + approverAppendVo{flowNodeProxyId,userIds}。
func TestTransferPayloadMatchesTargetPage(t *testing.T) {
	body, _, err := target.BuildActionBody(actionEndpointCases()["transfer"])
	if err != nil {
		t.Fatal(err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("移交载荷缺 data 容器：%v", body)
	}
	for _, key := range []string{"id", "jobTaskId", "batchNo"} {
		if strings.TrimSpace(data[key].(string)) == "" {
			t.Fatalf("移交 data.%s 不能为空", key)
		}
	}
	auditRecord := data["auditRecord"].(map[string]any)
	if auditRecord["auditStatus"] != "transfer" {
		t.Fatalf("移交 auditStatus 必须是 transfer：%v", auditRecord)
	}
	appendVo := body["approverAppendVo"].(map[string]any)
	if appendVo["flowNodeProxyId"] == "" {
		t.Fatalf("移交 approverAppendVo.flowNodeProxyId 不能为空")
	}
	if ids, ok := appendVo["userIds"].([]string); !ok || len(ids) == 0 {
		t.Fatalf("移交 approverAppendVo.userIds 不能为空")
	}
}

// TestUrgePayloadMatchesTargetPage 催办不走 data 容器：顶层 flowInstanceId + 固定空 data。
func TestUrgePayloadMatchesTargetPage(t *testing.T) {
	body, _, err := target.BuildActionBody(actionEndpointCases()["urge"])
	if err != nil {
		t.Fatal(err)
	}
	if body["flowInstanceId"] != "ins" {
		t.Fatalf("催办必须顶层携带 flowInstanceId：%v", body)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || len(data) != 0 {
		t.Fatalf("催办 data 必须是空对象：%v", body["data"])
	}
}
