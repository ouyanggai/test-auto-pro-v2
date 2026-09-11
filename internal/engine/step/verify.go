package step

import (
	"context"
	"errors"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/verdict"
	"test-auto-pro-v2/internal/model"
)

// readInstanceFacts 重读目标事实：实例状态、当前节点与当前处理人待办。
// 读取属只读阶段，允许有界重试；失败时在快照里如实记录 ReadError 并返回错误，不伪造事实。
func (e *Executor) readInstanceFacts(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep) (InstanceFacts, error) {
	instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	dueNodeKey := ""
	if info, ok := runCtx.Nodes[step.NodeKey]; ok {
		dueNodeKey = strings.TrimSpace(info.TargetNodeID)
	}
	facts := InstanceFacts{StepNodeKey: dueNodeKey}
	if instanceID == "" {
		// 发起前实例不存在：这是确定事实，不是读取失败。
		return facts, nil
	}
	// 实例「已发」事实必须用流程发起人（计划账号）会话读取：已发列表是发起人视角的数据，
	// 节点处理人自己的账号看不到这条实例（2026-09-11 用户强调的硬约束）。
	// 当前处理人切换只作用于后面的待办与任务读取，绝不作用于实例列表。
	initiatorSession := session
	if !strings.EqualFold(strings.TrimSpace(session.Summary.Account), strings.TrimSpace(runCtx.PlanAccount)) {
		if planSession, planErr := e.sessions.Current(ctx, runCtx.PlanAccount); planErr == nil {
			initiatorSession = planSession
		}
	}
	snapshot, err := findSubmittedFlowSnapshot(ctx, e.target, initiatorSession, instanceID)
	if err != nil {
		facts.ReadError = target.UserFacingErrorMessage(target.WriteResponse{}, err)
		return facts, err
	}
	if !snapshot.Found {
		// 实例在已发列表不可见：发起成功前是常态；发起后不可见属于异常事实，由判定规则处理。
		return facts, nil
	}
	facts.Found = true
	facts.Status = snapshot.Status
	facts.FlowProxyID = strings.TrimSpace(snapshot.FlowProxyID)
	if len(snapshot.FormProxyIDs) > 0 {
		facts.FormProxyID = strings.TrimSpace(snapshot.FormProxyIDs[0])
	}
	facts.CurrentNodes = snapshot.CurrentNodes
	facts.BizRelevance = cloneBizRelevance(snapshot.BizRelevance)
	facts.CurrentHandlers = snapshot.Handlers
	if creatorReader, ok := e.target.(flowCreatorReader); ok && (step.Action == model.ActionSaveDraft || step.Action == model.ActionResubmit || step.Action == model.ActionWithdraw) {
		isCreator, creatorErr := creatorReader.IsFlowCreator(ctx, session, instanceID)
		if creatorErr != nil {
			facts.ReadError = target.UserFacingErrorMessage(target.WriteResponse{}, creatorErr)
			return facts, creatorErr
		}
		facts.CreatorRead, facts.IsInitiator = true, isCreator
	}
	_, dueNodes, _, _, err := e.target.FindDueFlow(ctx, session, instanceID)
	if err != nil {
		facts.ReadError = target.UserFacingErrorMessage(target.WriteResponse{}, err)
		return facts, err
	}
	facts.DueNodes = dueNodes
	// 当前处理人发现只允许在计划账号会话（放行前的准备/门禁阶段）使用；
	// 写后核验用当前处理人会话，必须只核对当前处理人本人的任务（会签场景其他人的待办还在）。
	allowDiscovery := strings.EqualFold(strings.TrimSpace(session.Summary.Account), strings.TrimSpace(runCtx.PlanAccount))
	taskSession, taskErr := e.readActionTaskFacts(ctx, runCtx, session, step, dueNodeKey, &facts, allowDiscovery)
	if taskErr == nil {
		session = taskSession
	} else {
		facts.ReadError = target.UserFacingErrorMessage(target.WriteResponse{}, taskErr)
		return facts, taskErr
	}
	// 这些动作的流程主事实本来不会推进，必须读取目标各自的业务记录，不能用“节点没变”代替成功验证。
	switch step.Action {
	case model.ActionStorageFormData:
		if reader, ok := e.target.(storageFormDataReader); ok {
			nodeID := firstNonEmpty(facts.CurrentTaskNodeID, dueNodeKey)
			storage, found, readErr := reader.ReadStorageFormData(ctx, session, instanceID, nodeID)
			if readErr != nil {
				facts.ReadError = target.UserFacingErrorMessage(target.WriteResponse{}, readErr)
				return facts, readErr
			}
			facts.ActionFactRead, facts.StorageRead, facts.StorageFound = true, true, found
			facts.StorageDataID, facts.StorageAuditDesc, facts.StorageUpdateDate = storage.DataID, storage.AuditDesc, storage.UpdateDate
		}
	case model.ActionUrge:
		if reader, ok := e.target.(urgeRecordReader); ok {
			count, readErr := reader.CountUrgeRecords(ctx, session, instanceID)
			if readErr != nil {
				facts.ReadError = target.UserFacingErrorMessage(target.WriteResponse{}, readErr)
				return facts, readErr
			}
			facts.ActionFactRead, facts.UrgeRecordsRead, facts.UrgeRecordCount = true, true, count
		}
	case model.ActionFollow, model.ActionUnfollow:
		if reader, ok := e.target.(trackingReader); ok {
			tracking, found, readErr := reader.ReadFlowTracking(ctx, session, instanceID)
			if readErr != nil {
				facts.ReadError = target.UserFacingErrorMessage(target.WriteResponse{}, readErr)
				return facts, readErr
			}
			if !found {
				readErr = errors.New("目标实例不存在，无法确认关注状态")
				facts.ReadError = readErr.Error()
				return facts, readErr
			}
			facts.ActionFactRead, facts.TrackingRead, facts.Tracking = true, true, tracking
		}
	}
	return facts, nil
}

// flowBizRelevanceReader 是真实目标客户端提供的扩展读取面，旧测试假件继续只实现基础实例读取。
type flowBizRelevanceReader interface {
	FindSubmittedFlowWithRelevance(context.Context, target.Session, string) (string, []string, string, []string, []target.BizRelevance, bool, error)
}

// submittedFlowFactsReader 是读取实例完整事实（含各当前节点待办处理人）的可选能力面。
type submittedFlowFactsReader interface {
	FindSubmittedFlowFacts(ctx context.Context, active target.Session, instanceID string) (target.SubmittedFlowFacts, error)
}

// findSubmittedFlowSnapshot 读取实例事实快照：优先取完整事实（含 currentAuditUserInfo 处理人），
// 旧客户端按既有签名降级，处理人信息留空，由调用方按无处理人事实处理。
func findSubmittedFlowSnapshot(ctx context.Context, client TargetClient, session target.Session, instanceID string) (target.SubmittedFlowFacts, error) {
	if reader, ok := client.(submittedFlowFactsReader); ok {
		return reader.FindSubmittedFlowFacts(ctx, session, instanceID)
	}
	if reader, ok := client.(flowBizRelevanceReader); ok {
		flowProxyID, currentNodes, status, formProxyIDs, bizRelevance, found, err := reader.FindSubmittedFlowWithRelevance(ctx, session, instanceID)
		if err != nil {
			return target.SubmittedFlowFacts{}, err
		}
		return target.SubmittedFlowFacts{
			FlowProxyID: flowProxyID, CurrentNodes: currentNodes, Status: status,
			FormProxyIDs: formProxyIDs, BizRelevance: bizRelevance, Found: found,
		}, nil
	}
	flowProxyID, currentNodes, status, formProxyIDs, found, err := client.FindSubmittedFlow(ctx, session, instanceID)
	if err != nil {
		return target.SubmittedFlowFacts{}, err
	}
	return target.SubmittedFlowFacts{
		FlowProxyID: flowProxyID, CurrentNodes: currentNodes, Status: status,
		FormProxyIDs: formProxyIDs, Found: found,
	}, nil
}

// cloneBizRelevance 复制目标返回的业务关联，避免预览构造修改事实快照或后续落库基准。
func cloneBizRelevance(values []target.BizRelevance) []target.BizRelevance {
	if len(values) == 0 {
		return nil
	}
	result := make([]target.BizRelevance, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.OtherBiz) == "" {
			continue
		}
		result = append(result, target.BizRelevance{
			OtherBiz:   strings.TrimSpace(value.OtherBiz),
			OtherBizID: strings.TrimSpace(value.OtherBizID),
		})
	}
	return result
}

// readActionTaskFacts 读取任务级动作的目标事实：当前待办、已办任务、任务链和代理树。
// 这些事实只用于门禁和结果核验；任一关键事实缺失都保持禁用，不能靠动作配置猜测目标状态。
func (e *Executor) readActionTaskFacts(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep, nodeID string, facts *InstanceFacts, allowDiscovery bool) (target.Session, error) {
	instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	if instanceID == "" {
		return session, nil
	}
	_, hasTaskReader := e.target.(taskSnapshotReader)
	listReader, hasListReader := e.target.(taskSnapshotListReader)
	treeReader, hasTreeReader := e.target.(flowProxyTreeReader)
	auditReader, hasAuditReader := e.target.(auditRecordsReader)
	if step.Action == model.ActionUrge && hasListReader {
		pending, err := listReader.ListTaskSnapshots(ctx, session, instanceID, "pending")
		if err != nil {
			return session, err
		}
		facts.PendingTaskRead, facts.PendingTaskFound = true, len(pending) > 0
	}

	switch step.Action {
	case model.ActionStorageFormData, model.ActionApprove, model.ActionReject,
		model.ActionAddSign, model.ActionTransfer, model.ActionRollback:
		if !hasTaskReader {
			return session, nil
		}
		snapshot, candidateAssigneeID, candidateAssigneeName, taskSession, resolveErr := e.resolveTaskSnapshotForStep(ctx, runCtx, session, step, nodeID, "pending", facts, allowDiscovery)
		if resolveErr != nil {
			return session, resolveErr
		}
		session = taskSession
		facts.CurrentTaskRead = true
		facts.CurrentTaskFound = strings.TrimSpace(snapshot.JobTaskID) != ""
		facts.CurrentTaskLinkID = strings.TrimSpace(snapshot.LinkID)
		facts.CurrentTaskParentID = strings.TrimSpace(snapshot.ParentLinkID)
		facts.CurrentTaskBatchNo = strings.TrimSpace(snapshot.BatchNo)
		facts.CurrentTaskFlowProxy = strings.TrimSpace(snapshot.FlowProxyID)
		facts.CurrentTaskNodeID = strings.TrimSpace(snapshot.FlowNodeProxyID)
		facts.CurrentTaskAssigneeID = firstNonEmpty(strings.TrimSpace(snapshot.PendingUserID), candidateAssigneeID)
		facts.CurrentTaskAssigneeName = firstNonEmpty(strings.TrimSpace(snapshot.PendingUserName), candidateAssigneeName)
		switch step.Action {
		case model.ActionStorageFormData, model.ActionApprove, model.ActionReject:
			// 这三个动作都直接处理当前待办；门禁和写后核验必须使用同一条实时任务快照。
			// 会签中间态核验（2026-09-11 实测）：部分人同意后实例仍停在本节点，节点上还有
			// 其他人的 pending 任务。此时「待办是否清空」不能判定本步写是否生效——必须用
			// 按执行人过滤的已办（ListTaskSnapshots(done) 自带 executorId）核对「本当前处理人在
			// 本节点的已完成任务」是否出现；出现即本次审批已被目标记录。
			if facts.CurrentTaskRead && step.Action == model.ActionApprove && hasListReader {
				done, doneErr := listReader.ListTaskSnapshots(ctx, session, instanceID, "done")
				facts.CompletedTaskRead = true
				if doneErr == nil {
					for _, doneSnapshot := range done {
						if strings.TrimSpace(doneSnapshot.FlowNodeProxyID) == strings.TrimSpace(nodeID) {
							facts.CompletedTaskFound = true
							facts.CompletedTaskNodeID = strings.TrimSpace(doneSnapshot.FlowNodeProxyID)
							break
						}
					}
				}
			}
			return session, nil
		case model.ActionAddSign:
			personIDs := runCtx.ActionPersonIDs[ActionPersonIndex(step.NodeKey, step.Action)]
			proxyID := firstNonEmpty(snapshot.FlowProxyID, runCtx.FlowProxyID)
			if len(personIDs) == 0 || !hasTreeDocumentReader(e.target) || proxyID == "" {
				return session, nil
			}
			documentReader := e.target.(flowProxyDocumentReader)
			if _, err := documentReader.ReadFlowProxyDocument(ctx, session, proxyID); err != nil {
				return session, err
			}
			facts.EditableProxyRead = true
		case model.ActionTransfer:
			personIDs := runCtx.ActionPersonIDs[ActionPersonIndex(step.NodeKey, step.Action)]
			facts.ActorSwitchRead = facts.CurrentTaskFound && facts.CurrentTaskBatchNo != "" && len(personIDs) > 0
		case model.ActionRollback:
			facts.PreviousTaskExists = facts.CurrentTaskParentID != ""
			if !facts.PreviousTaskExists || !hasAuditReader {
				return session, nil
			}
			records, auditErr := auditReader.ListAuditRecords(ctx, session, instanceID)
			if auditErr != nil {
				return session, auditErr
			}
			previousNodeID, found, previousErr := previousNodeFromAuditRecords(records, facts.CurrentTaskParentID)
			if previousErr != nil {
				return session, previousErr
			}
			facts.PreviousTaskRead = true
			if !found || !hasTreeReader {
				return session, nil
			}
			proxyID := firstNonEmpty(snapshot.FlowProxyID, runCtx.FlowProxyID)
			if proxyID == "" {
				return session, nil
			}
			tree, err := treeReader.ReadProxyTree(ctx, session, proxyID)
			if err != nil {
				return session, err
			}
			facts.PreviousNodeType = nodeTypeInTree(tree, previousNodeID)
			facts.PreviousNodeIsStart = isStartNodeType(facts.PreviousNodeType)
		}

	case model.ActionRetrieve:
		if !hasTaskReader {
			return session, nil
		}
		snapshot, _, _, taskSession, resolveErr := e.resolveTaskSnapshotForStep(ctx, runCtx, session, step, nodeID, "done", facts, allowDiscovery)
		if resolveErr != nil {
			return session, resolveErr
		}
		session = taskSession
		facts.CompletedTaskRead = true
		facts.CompletedTaskFound = strings.TrimSpace(snapshot.JobTaskID) != ""
		facts.CompletedTaskLinkID = strings.TrimSpace(snapshot.LinkID)
		facts.CompletedTaskNodeID = strings.TrimSpace(snapshot.FlowNodeProxyID)
		facts.CompletedTaskParentID = strings.TrimSpace(snapshot.ParentLinkID)
		facts.CompletedTaskBatchNo = strings.TrimSpace(snapshot.BatchNo)
		facts.CompletedTaskAuditWay = strings.TrimSpace(snapshot.AuditWay)
		if !facts.CompletedTaskFound {
			return session, nil
		}
		// 目标任务列表没有“所有状态、所有人员”的有效读取方式：空 taskStatus 会被目标直接拒绝，
		// 而 pending/done 又只覆盖当前用户。后继任务是否合法必须交由 retrieveProcess 在实例锁内确认，
		// 以目标接口原文作为失败结果，不能用不完整列表臆测为“后继已处理”。
		if hasTreeReader {
			proxyID := firstNonEmpty(snapshot.FlowProxyID, runCtx.FlowProxyID)
			if proxyID != "" {
				tree, treeErr := treeReader.ReadProxyTree(ctx, session, proxyID)
				if treeErr != nil {
					return session, treeErr
				}
				facts.RetrieveNodeIsStart = isStartNodeType(nodeTypeInTree(tree, snapshot.FlowNodeProxyID))
				for _, currentNodeID := range facts.CurrentNodes {
					node := nodeInTree(tree, currentNodeID)
					if node == nil || node.AuditConfig == nil {
						continue
					}
					if isCountersignAuditWay(node.AuditConfig.Mode) {
						facts.CurrentTaskCountersign = true
					}
				}
			}
		}
		if !hasAuditReader {
			// 没有审核记录就无法排除重复取回和其他当前处理人已处理，不能误放行。
			facts.RetrieveAlreadyUsed = true
			facts.CurrentTaskHandledByOther = true
			return session, nil
		}
		records, auditErr := auditReader.ListAuditRecords(ctx, session, instanceID)
		if auditErr != nil {
			return session, auditErr
		}
		for _, record := range records {
			if record.FlowJobTaskID == facts.CompletedTaskLinkID && strings.EqualFold(record.AuditStatus, "retrieve") {
				facts.RetrieveAlreadyUsed = true
			}
			if !containsString(facts.DueNodes, record.FlowNodeProxyID) || record.ExecutorID == "" || record.ExecutorID == session.UserID {
				continue
			}
			if strings.EqualFold(record.AuditStatus, "pass") {
				facts.CurrentTaskHandledByOther = true
			}
		}
	}
	return session, nil
}

// previousNodeFromAuditRecords 用审核记录里的 flowJobTaskId 定位前一任务的真实节点。
// 目标源码明确该字段保存的是任务关联行 ID，正好与当前任务的 pid 对应；同一任务允许有多条记录，
// 但它们必须指向同一个节点，否则目标事实自相矛盾，不能选择其中一条继续回退。
func previousNodeFromAuditRecords(records []target.AuditRecordSnapshot, previousLinkID string) (string, bool, error) {
	previousLinkID = strings.TrimSpace(previousLinkID)
	if previousLinkID == "" {
		return "", false, nil
	}
	nodeID := ""
	found := false
	for _, record := range records {
		if strings.TrimSpace(record.FlowJobTaskID) != previousLinkID {
			continue
		}
		found = true
		candidate := strings.TrimSpace(record.FlowNodeProxyID)
		if candidate == "" {
			continue
		}
		if nodeID != "" && nodeID != candidate {
			return "", false, errors.New("前一任务的审核记录指向多个流程节点")
		}
		nodeID = candidate
	}
	if found && nodeID == "" {
		return "", false, errors.New("前一任务的审核记录缺少流程节点")
	}
	return nodeID, found, nil
}

// nodeInTree 按目标节点 ID 深度优先查找代理树节点。
func nodeInTree(tree *target.FlowNodeTemplate, nodeID string) *target.FlowNodeTemplate {
	if tree == nil {
		return nil
	}
	if strings.TrimSpace(tree.ID) == strings.TrimSpace(nodeID) {
		return tree
	}
	if node := nodeInTree(tree.Child, nodeID); node != nil {
		return node
	}
	for _, branch := range tree.ConditionNodes {
		if node := nodeInTree(branch.Child, nodeID); node != nil {
			return node
		}
	}
	for _, branch := range tree.ParallelNodes {
		if node := nodeInTree(branch.Child, nodeID); node != nil {
			return node
		}
	}
	return nil
}

// nodeTypeInTree 返回目标节点类型；未找到时保持空值，交由门禁阻止未证实动作。
func nodeTypeInTree(tree *target.FlowNodeTemplate, nodeID string) string {
	if node := nodeInTree(tree, nodeID); node != nil {
		return strings.TrimSpace(node.Type)
	}
	return ""
}

// isStartNodeType 判断目标代理树中的发起节点类型。
func isStartNodeType(value string) bool {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", "_")) {
	case "start", "begin", "initiator", "发起", "开始":
		return true
	default:
		return false
	}
}

// isCountersignAuditWay 判断目标审批方式是否为会签；未知枚举不猜测为会签。
func isCountersignAuditWay(value string) bool {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", "_")) {
	case "countersign", "counter_sign", "会签":
		return true
	default:
		return false
	}
}

// hasTreeDocumentReader 判断目标是否支持读取完整流程代理文档。
func hasTreeDocumentReader(client TargetClient) bool {
	_, ok := client.(flowProxyDocumentReader)
	return ok
}

// firstNonEmpty 返回首个非空标识，避免用旧代理覆盖任务现场返回的新代理。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

// containsString 判断目标节点集合是否包含指定真实节点标识。
func containsString(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

// storageFormDataReader 是暂存动作写后读取检查点的最小能力面。
type storageFormDataReader interface {
	ReadStorageFormData(context.Context, target.Session, string, string) (target.StorageFormData, bool, error)
}

// urgeRecordReader 是催办动作写后读取催办记录的最小能力面。
type urgeRecordReader interface {
	CountUrgeRecords(context.Context, target.Session, string) (int, error)
}

// trackingReader 是关注动作写后读取当前用户关注状态的最小能力面。
type trackingReader interface {
	ReadFlowTracking(context.Context, target.Session, string) (bool, bool, error)
}

// taskSnapshotListReader 是门禁读取完整任务链所需的可选能力面。
type taskSnapshotListReader interface {
	ListTaskSnapshots(context.Context, target.Session, string, string) ([]target.TaskSnapshot, error)
}

// flowProxyTreeReader 是回退/取回门禁读取真实节点类型所需的可选能力面。
type flowProxyTreeReader interface {
	ReadProxyTree(context.Context, target.Session, string) (*target.FlowNodeTemplate, error)
}

// auditRecordsReader 是取回门禁读取重复取回和会签处理事实所需的可选能力面。
type auditRecordsReader interface {
	ListAuditRecords(context.Context, target.Session, string) ([]target.AuditRecordSnapshot, error)
}

// flowCreatorReader 是已有实例发起人归属核验的可选能力面。
type flowCreatorReader interface {
	IsFlowCreator(context.Context, target.Session, string) (bool, error)
}

// ClassifyReread 把前后两次事实对照为判定包的重读四值（纲领第 7.4 节：只依据事实）。
// 判定口径：
//   - 发起：之前实例不存在，之后存在且处于运行/待发即「已前进」；仍不存在即「明确未变」；
//     落成草稿或终态与发起语义矛盾即「自相矛盾」；
//   - 审批：本步待办仍在即「明确未变」；待办消失且实例未出现撤回类回退状态即「已前进」。
func ClassifyReread(action string, stepNodeKey string, before, after InstanceFacts) verdict.Reread {
	if after.ReadError != "" {
		return verdict.RereadUnreadable
	}
	if action == string(model.ActionStorageFormData) {
		if !after.ActionFactRead {
			return verdict.RereadUnreadable
		}
		if after.StorageFound && (!before.StorageFound || after.StorageDataID != before.StorageDataID || after.StorageAuditDesc != before.StorageAuditDesc || after.StorageUpdateDate != before.StorageUpdateDate) {
			return verdict.RereadAdvanced
		}
		return verdict.RereadUnchanged
	}
	if action == string(model.ActionUrge) {
		if !after.ActionFactRead {
			return verdict.RereadUnreadable
		}
		if after.UrgeRecordCount > before.UrgeRecordCount {
			return verdict.RereadAdvanced
		}
		return verdict.RereadUnchanged
	}
	if action == string(model.ActionFollow) || action == string(model.ActionUnfollow) {
		if !after.ActionFactRead {
			return verdict.RereadUnreadable
		}
		wantTracking := action == string(model.ActionFollow)
		if after.Tracking == wantTracking {
			return verdict.RereadAdvanced
		}
		return verdict.RereadContradictory
	}
	if action == string(model.ActionSubmit) || action == string(model.ActionResubmit) || action == string(model.ActionSaveDraft) {
		if !after.Found {
			return verdict.RereadUnchanged
		}
		switch after.Status {
		case "run", "await_sent":
			if action == string(model.ActionSaveDraft) {
				return verdict.RereadContradictory
			}
			return verdict.RereadAdvanced
		case "draft":
			if action == string(model.ActionSaveDraft) {
				return verdict.RereadAdvanced
			}
			// 普通提交或重新提交落成草稿，与动作语义矛盾。
			return verdict.RereadContradictory
		default:
			// 其余状态（撤回/终止/放弃/驳回/结束）都不是发起动作应有的事实。
			return verdict.RereadContradictory
		}
	}
	// 审批、不同意和暂存都以当前账号的任务链接为事实。实例节点列表是全局入口，
	// 不能在任务已消失时替代当前账号任务的核验。
	if (action == string(model.ActionApprove) || action == string(model.ActionReject) || action == string(model.ActionStorageFormData)) && after.CurrentTaskRead {
		// 会签中间态（2026-09-11 实测）：当前处理人同意后节点上仍有其他人的 pending 任务，
		// 按「节点待办是否清空」对照会误判「写未生效」。此时以按执行人过滤的已办为准——
		// 当前处理人在本节点的已完成任务已出现，说明本次审批已被目标记录，实例只是在等其他人。
		actorDone := action == string(model.ActionApprove) && after.CompletedTaskRead && after.CompletedTaskFound &&
			strings.TrimSpace(after.CompletedTaskNodeID) == strings.TrimSpace(stepNodeKey)
		if after.CurrentTaskFound {
			if actorDone {
				return verdict.RereadAdvanced
			}
			return verdict.RereadUnchanged
		}
		if after.Found && action == string(model.ActionReject) && strings.EqualFold(after.Status, "rejected") {
			return verdict.RereadAdvanced
		}
		if after.Found && action == string(model.ActionApprove) && (strings.EqualFold(after.Status, "withdraw") || strings.EqualFold(after.Status, "rejected")) {
			return verdict.RereadContradictory
		}
		return verdict.RereadAdvanced
	}
	// 审批：本步节点的待办仍在，说明写未生效——除非本当前处理人的已办已出现（会签中间态）。
	for _, node := range after.DueNodes {
		if node == stepNodeKey {
			if action == string(model.ActionApprove) && after.CompletedTaskRead && after.CompletedTaskFound &&
				strings.TrimSpace(after.CompletedTaskNodeID) == strings.TrimSpace(stepNodeKey) {
				return verdict.RereadAdvanced
			}
			return verdict.RereadUnchanged
		}
	}
	if after.Found {
		switch after.Status {
		case "withdraw", "termination", "abandon", "rejected":
			// 不同意成功实例即 rejected、撤回成功实例即 withdraw——这是动作的预期效果而非矛盾；
			// 只有同意类动作遇到撤回类状态才是「事实与动作矛盾」（评审 P1）。
			if action == string(model.ActionApprove) {
				return verdict.RereadContradictory
			}
			if action == string(model.ActionReject) && after.Status == "rejected" {
				return verdict.RereadAdvanced
			}
			if action == string(model.ActionWithdraw) && after.Status == "withdraw" {
				return verdict.RereadAdvanced
			}
			return verdict.RereadContradictory
		}
	}
	// 待办已消失：无论实例推进到下一节点还是直接结束，写都已生效。
	return verdict.RereadAdvanced
}

// ActionFactVerified 判断不推进主流程的动作是否已经由专用结果接口确认了预期变化。
// 仅仅成功读到接口不代表写入生效；暂存、催办和关注必须先通过各自事实对照才能成功。
func ActionFactVerified(action model.ActionKey, reread verdict.Reread) bool {
	switch action {
	case model.ActionStorageFormData, model.ActionUrge, model.ActionFollow, model.ActionUnfollow:
		return reread == verdict.RereadAdvanced
	default:
		return false
	}
}

// buildObservation 组装判定包的五项输入：动作与端点、传输结论、HTTP 状态码、响应包、重读结论。
// 全部来自传输层事实与目标重读事实，不靠错误文案推断（F-014 第 1.6 节）。
func buildObservation(endpoint string, writeErr error, response target.WriteResponse, reread verdict.Reread) verdict.Observation {
	transport := targetTransportOf(writeErr)
	observation := verdict.Observation{
		Endpoint:   endpoint,
		Transport:  transport,
		StatusCode: response.StatusCode,
		Reread:     reread,
	}
	// 会话失效拒绝是适配层识别过的结构化事实（响应已收到、按勘定清单匹配）：
	// 传给判定包按鉴权拒绝初判，配合重读「明确未变」可得到确定失败，而不是永远不确定。
	var targetErr *target.Error
	if errors.As(writeErr, &targetErr) && targetErr.Kind == target.ErrorSessionExpired &&
		transport == verdict.TransportResponded {
		observation.SessionRejected = true
	}
	if response.IsSuccessPresent {
		observation.Response = &verdict.Response{
			IsSuccess:        response.IsSuccess,
			IsSuccessPresent: true,
			Code:             response.Code,
			Message:          response.Message,
		}
	} else if transport == verdict.TransportResponded {
		// 收到完整响应但成功判据不可解析：按不可解析响应交给判定包，不允许乐观归档。
		observation.Response = &verdict.Response{Unparsable: true}
	}
	return observation
}

// targetTransportOf 把适配层的传输阶段事实转换为判定包的传输枚举；两侧取值一一对应。
// 目标业务拒绝（isSuccess=false 的完整响应）在传输层属于 responded，判定按响应侧初判走。
func targetTransportOf(err error) verdict.Transport {
	var rejection *target.BusinessRejection
	if errors.As(err, &rejection) {
		return verdict.TransportResponded
	}
	switch target.TransportOf(err) {
	case target.TransportResponded:
		return verdict.TransportResponded
	case target.TransportConnectFailed:
		return verdict.TransportConnectRefused
	case target.TransportInterrupted:
		return verdict.TransportInterrupted
	default:
		return verdict.TransportUnclassified
	}
}

// ensureCompanyRelevance 保证发起/存草稿的实例至少带一条 company 业务关联。
// 根因：目标平台「已发流程」页面固定按 company 关联过滤，原生发起时 FlowDialog 会
// 提交 {otherBiz:"company", otherBizId:当前公司}；工具此前只在重读事实带出关联时才写，
// 新流程发起前关联为空 → 实例创建成功但在已发列表里永远搜不到（实测缺陷）。
// 已有关联时原样保留，只补缺失的 company 项。
func ensureCompanyRelevance(values []target.BizRelevance, companyID string) []target.BizRelevance {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return cloneBizRelevance(values)
	}
	result := cloneBizRelevance(values)
	for _, value := range result {
		if strings.EqualFold(strings.TrimSpace(value.OtherBiz), "company") {
			return result
		}
	}
	return append(result, target.BizRelevance{OtherBiz: "company", OtherBizID: companyID})
}
