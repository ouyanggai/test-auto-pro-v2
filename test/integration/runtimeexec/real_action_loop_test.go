// F-036 L2 动作台：单个 runtime 动作的真实写验证（施工令 §5.2）。
// 由环境变量驱动，避免包内默认发起真实写：
//   TEST_AUTO_PRO_F036_ACTION   必填（save_draft/submit/storage_form_data/approve/reject/resubmit/forward），为空跳过
//   TEST_AUTO_PRO_F036_INSTANCE 审批族/重提/转发的实例 ID
//   TEST_AUTO_PRO_F036_JOB_TASK 审批族的任务 ID
//   TEST_AUTO_PRO_F036_NODE     审批族的节点代理 ID
//   TEST_AUTO_PRO_F036_RECEIVER 转发接收人用户 ID
// 断言三件事同时命中：组件链+方法调用事实（result.requestTrace 之外的动作事实）、
// 写端点出现在 RequestTrace、写后目标事实核验；证据落盘 docs/auto/evidence/。
package runtimeexec_test

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/runtimeexec"
)

// f036ExpectedWriteEndpoint 按施工令附录 A.3 给出动作的期望主写端点。
func f036ExpectedWriteEndpoint(action string) string {
	switch action {
	case "save_draft", "submit":
		return "/web/flowInstanceApi/submit"
	case "storage_form_data":
		return "/web/flowInstanceApi/storageFormData"
	case "approve", "reject":
		return "/flowInstanceApi/audit"
	case "resubmit":
		return "/web/flowInstanceApi/reSubmit"
	case "forward":
		return "/web/flowInstanceApi/transpond"
	default:
		return ""
	}
}

// f036ReadbackNewInstance 从 businessSummary.debugWrites 的写响应里提取实例 ID，
// 并用目标只读 API 复核实例存在。发起族动作的实例 ID 只能来自写响应（运行编排之外没有别的来源）。
func f036ReadbackNewInstance(ctx context.Context, t *testing.T, client *target.Client, session target.Session, res runtimeexec.ActionRunResult) (map[string]any, string) {
	t.Helper()
	var summary struct {
		DebugWrites []struct {
			IsPermissionCheck   bool   `json:"isPermissionCheck"`
			ResponseBodyExcerpt string `json:"responseBodyExcerpt"`
		} `json:"debugWrites"`
	}
	if len(res.BusinessSummary) > 0 {
		if err := json.Unmarshal(res.BusinessSummary, &summary); err != nil {
			t.Fatalf("businessSummary 解析失败：%v", err)
		}
	}
	instanceID := ""
	for _, write := range summary.DebugWrites {
		if write.IsPermissionCheck {
			continue
		}
		match := regexp.MustCompile(`"id":"([0-9a-f]{32})"`).FindStringSubmatch(write.ResponseBodyExcerpt)
		if len(match) == 2 {
			instanceID = match[1]
			break
		}
	}
	if instanceID == "" {
		return nil, ""
	}
	facts, err := client.FindSubmittedFlowFacts(ctx, session, instanceID)
	if err != nil {
		t.Fatalf("写后实例事实读取失败：%v", err)
	}
	return map[string]any{"found": facts.Found, "status": facts.Status, "name": facts.Name, "currentNodes": facts.CurrentNodes}, instanceID
}

// TestF036RealActionLoop 单动作真实写验证入口：环境变量未配置时跳过并打印原因。
func TestF036RealActionLoop(t *testing.T) {
	rc, client, current := realLoopDeps(t)
	account := os.Getenv("TEST_AUTO_PRO_TARGET_ACCOUNT")
	action := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_ACTION"))
	if action == "" {
		t.Skip("未配置 TEST_AUTO_PRO_F036_ACTION，跳过真实写动作验证")
	}
	endpoint := f036ExpectedWriteEndpoint(action)
	if endpoint == "" {
		t.Fatalf("未知动作 %s", action)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	if err := rc.Health(ctx); err != nil {
		// 执行服务未启动属可跳过场景（与未配置环境变量同类）：真实闭环需要本地服务在跑。
		t.Skipf("执行服务不可用（pnpm --dir executor-service start 可启动），跳过真实闭环验证：%v", err)
	}

	s, err := current(ctx, account)
	if err != nil {
		t.Fatalf("取目标会话失败：%v", err)
	}
	boundID := f036BoundTemplateID(t)
	snapshot, err := client.ReadTemplateConfiguration(ctx, s, boundID)
	if err != nil {
		t.Fatalf("读取目标模板配置失败：%v", err)
	}
	if len(snapshot.Forms) != 1 {
		t.Fatalf("目标模板应挂唯一表单，实际 %d 个", len(snapshot.Forms))
	}
	formProxyID := snapshot.Forms[0].ID
	flowProxyID := boundID

	request := runtimeexec.RuntimeRunRequest{
		RunID: "1", PathRunID: "1", StepID: "1",
		Account:      account,
		Action:       action,
		ToolFlowName: "T-AUTO-F036-" + action + "-" + time.Now().Format("0102-150405"),
		Session: runtimeexec.SessionFacts{
			UserID: s.UserID, CompanyID: s.CompanyID, DepartmentID: s.DepartmentID,
			CustomerCode: s.CustomerCode, Account: s.Summary.Account, DisplayName: s.Summary.DisplayName,
		},
		Initiator:      runtimeexec.ActorIdentity{UserID: s.UserID, Name: s.Summary.DisplayName},
		Values:         json.RawMessage(`{}`),
		Permissions:    []string{},
		ChangedFields:  []string{},
		FormProxyID:    formProxyID,
		FlowProxyID:    flowProxyID,
		FlowTemplateID: boundID,
		SelectFlowType: snapshot.AuditWay,
		AuditDesc:      "自动验证意见",
		BaseURL:        "",
	}
	// 表单值来源：TEST_AUTO_PRO_F036_VALUES_FILE 指向 JSON 文件（发起提交需要表单数据通过原生校验）。
	if valuesFile := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_VALUES_FILE")); valuesFile != "" {
		raw, readErr := os.ReadFile(valuesFile)
		if readErr != nil {
			t.Fatalf("读取表单值文件失败：%v", readErr)
		}
		request.Values = json.RawMessage(raw)
		// 未显式给 CHANGED_FIELDS 时补丁全部键：新发起链的表单初始为空，
		// 只补覆盖字段会让必填校验拦住提交（原生用户路径是整表填写）。
		if fields := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_CHANGED_FIELDS")); fields != "" {
			request.ChangedFields = strings.Split(fields, ",")
		} else {
			var values map[string]any
			if err := json.Unmarshal(raw, &values); err != nil {
				t.Fatalf("表单值文件不是 JSON 对象：%v", err)
			}
			allFields := make([]string, 0, len(values))
			for key := range values {
				allFields = append(allFields, key)
			}
			request.ChangedFields = allFields
		}
	}
	session3, sessErr := current(ctx, account)
	if sessErr != nil {
		t.Fatalf("取目标会话失败：%v", sessErr)
	}
	instanceID := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_INSTANCE"))
	jobTaskID := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_JOB_TASK"))
	nodeProxyID := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_NODE"))
	receiverID := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_RECEIVER"))
	switch action {
	case "save_draft", "submit":
		// 新发起链：无实例事实，由目标在页面上自装载模板。
	case "storage_form_data", "approve", "reject":
		request.InstanceID = instanceID
		request.NodeProxyID = nodeProxyID
		if instanceID == "" {
			t.Fatalf("动作 %s 需要 TEST_AUTO_PRO_F036_INSTANCE", action)
		}
		// 未显式给任务号时从目标实时解析当前待办（与引擎写前刷新同一读取）。
		if jobTaskID == "" {
			snapshot, snapErr := client.FindTaskSnapshot(ctx, session3, instanceID, nodeProxyID, "pending")
			if snapErr != nil {
				t.Fatalf("解析当前待办任务失败：%v", snapErr)
			}
			jobTaskID = strings.TrimSpace(snapshot.JobTaskID)
			if jobTaskID == "" {
				t.Fatalf("目标上已无当前账号在该节点的待办任务（node=%s）", nodeProxyID)
			}
			if nodeProxyID == "" {
				request.NodeProxyID = snapshot.FlowNodeProxyID
			}
		}
		request.JobTaskID = jobTaskID
	case "resubmit":
		request.InstanceID = instanceID
		request.LoadMode = runtimeexec.LoadModeEdit
		if instanceID == "" {
			t.Fatalf("动作 resubmit 需要 TEST_AUTO_PRO_F036_INSTANCE")
		}
	case "forward":
		request.InstanceID = instanceID
		request.JobTaskID = jobTaskID
		request.NodeProxyID = nodeProxyID
		request.ReceiverID = receiverID
		if instanceID == "" || receiverID == "" {
			t.Fatalf("动作 forward 需要 TEST_AUTO_PRO_F036_INSTANCE / TEST_AUTO_PRO_F036_RECEIVER")
		}
	}

	// TEST_AUTO_PRO_F036_BRANCH_ENTRY：手动分支的已选入口节点 ID（分支节点=TEST_AUTO_PRO_F036_NODE）。
	// 配置后同意/提交若被目标以「手动条件分支,请选择」拒绝，执行器会按已保存选择继续一次（S6 闭环）。
	if branchEntry := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_BRANCH_ENTRY")); branchEntry != "" {
		branchNode := nodeProxyID
		if branchNode == "" {
			branchNode = branchEntry
		}
		// 只配置选择（Go 侧继续依据），不预注入人员：分支人员由原生继续链路按需携带。
		request.BranchSelections = []runtimeexec.BranchSelection{{NodeProxyID: branchNode, EntryNodeID: branchEntry}}
	}
	// TEST_AUTO_PRO_F036_NEXT_AUDITOR_NODES=id1,id2：为若干下游自选节点预置本人人员
	// （目标在审批写入时会校验后续链路的选人需求）。
	if nodes := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_NEXT_AUDITOR_NODES")); nodes != "" {
		for _, nodeID := range strings.Split(nodes, ",") {
			nodeID = strings.TrimSpace(nodeID)
			if nodeID == "" {
				continue
			}
			request.NextNodeAuditors = append(request.NextNodeAuditors, runtimeexec.NextAuditorEntry{
				NodeProxyID:     nodeID,
				BizID:           s.UserID,
				Name:            s.Summary.DisplayName,
				AuditDetailType: "personnel",
			})
		}
	}
	// TEST_AUTO_PRO_F036_SELF_AUDIT=1：把下一节点处理人设为计划账号本人（nodeProxyId 用
	// TEST_AUTO_PRO_F036_NODE），让产生的待办落在本账号名下，供审批族动作真实验证。
	if os.Getenv("TEST_AUTO_PRO_F036_SELF_AUDIT") == "1" && (action == "submit" || action == "resubmit") {
		request.NextNodeAuditors = []runtimeexec.NextAuditorEntry{{
			NodeProxyID:     nodeProxyID,
			BizID:           s.UserID,
			Name:            s.Summary.DisplayName,
			AuditDetailType: "personnel",
		}}
		if nodeProxyID == "" {
			t.Fatalf("SELF_AUDIT 需要 TEST_AUTO_PRO_F036_NODE（下一节点代理 ID）")
		}
	}
	exec := runtimeexec.NewRuntimeExecutor(rc)
	res, err := exec.RunAction(ctx, request)
	fact := map[string]any{
		"action": action, "account": account, "instanceId": instanceID,
		"jobTaskId": jobTaskID, "toolFlowName": request.ToolFlowName,
	}
	if err != nil {
		fact["error"] = err.Error()
		saveF036Evidence(t, "L2-"+action+"-error", fact)
		t.Fatalf("动作执行失败：%v", err)
	}
	fact["result"] = map[string]any{"status": res.Status, "message": res.Message}
	fact["requestTrace"] = json.RawMessage(res.RequestTrace)
	fact["writeTraceIDs"] = res.WriteTraceIDs
	if len(res.BusinessSummary) > 0 {
		fact["businessSummary"] = json.RawMessage(res.BusinessSummary)
	}

	// 断言 1：状态必须 success。
	if res.Status != "success" {
		saveF036Evidence(t, "L2-"+action+"-"+res.Status, fact)
		t.Fatalf("动作状态=%s 消息=%s", res.Status, res.Message)
	}
	// 断言 2：传输层写 trace 必须存在（页面写请求确实经 Go 转发发出）。
	if len(res.WriteTraceIDs) == 0 {
		saveF036Evidence(t, "L2-"+action+"-no-write-trace", fact)
		t.Fatalf("动作成功但没有任何写请求 trace（端点级证据缺失）")
	}
	// 断言 3：RequestTrace 里出现该动作的期望写端点。
	traceText := string(res.RequestTrace)
	if !strings.Contains(traceText, endpoint) {
		saveF036Evidence(t, "L2-"+action+"-missing-endpoint", fact)
		t.Fatalf("请求轨迹缺少期望写端点 %s：%s", endpoint, traceText)
	}

	// 断言 4：写后目标事实核验（只读）。
	session2, err := current(ctx, account)
	if err != nil {
		t.Fatalf("取核验会话失败：%v", err)
	}
	readback := map[string]any{}
	switch action {
	case "save_draft", "submit":
		// 新实例：从写响应里提取实例 ID，再用目标只读 API 复核实例存在（写后事实核验）。
		readback["instanceFact"], readback["extractedInstanceId"] = f036ReadbackNewInstance(ctx, t, client, session2, res)
		if readback["extractedInstanceId"] == "" {
			saveF036Evidence(t, "L2-"+action+"-readback-no-id", fact)
			t.Fatalf("写响应中未提取到实例 ID，无法完成写后核验")
		}
	case "storage_form_data":
		stored, found, err := client.ReadStorageFormData(ctx, session2, instanceID, nodeProxyID)
		if err != nil {
			t.Fatalf("暂存核验读取失败：%v", err)
		}
		readback["storageFound"] = found
		readback["storageAuditDesc"] = stored.AuditDesc
		if !found {
			saveF036Evidence(t, "L2-"+action+"-readback-miss", fact)
			t.Fatalf("暂存写后未读到暂存记录")
		}
	case "approve", "reject":
		done, err := client.FindDoneTaskOnNode(ctx, session2, instanceID, nodeProxyID, session2.UserID)
		if err != nil {
			t.Fatalf("审批核验读取失败：%v", err)
		}
		readback["taskDoneOnNode"] = done
		if !done {
			saveF036Evidence(t, "L2-"+action+"-readback-miss", fact)
			t.Fatalf("审批写后未见当前节点已办任务")
		}
	case "resubmit", "forward":
		facts, err := client.FindSubmittedFlowFacts(ctx, session2, instanceID)
		if err != nil {
			t.Fatalf("实例事实读取失败：%v", err)
		}
		readback["instanceFacts"] = facts
	}
	fact["readback"] = readback
	saveF036Evidence(t, "L2-"+action, fact)
}
