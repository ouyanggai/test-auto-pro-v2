package step

import (
	"context"
	"errors"
	"fmt"
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
	// F-030/T02：本次事实读取边界内，同一 (会话, 实例, 状态) 的任务列表只扫一次；
	// 写后核验是新的一次调用，memo 从零开始——写后屏障由作用域天然保证。
	ctx = target.AttachTaskSnapshotScope(ctx)
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
	// 实例名称只作业务信息：目标没返回名称时保持空串，由日志与详情按「不可用」如实展示。
	facts.InstanceName = strings.TrimSpace(snapshot.Name)
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
	// F-035：目标节点已到达但未生成处理人的分型事实——实例可读、当前节点已知、
	// 但 currentAuditUserInfo 与待办都为空。这只是"正在生成处理人"的证据，
	// 绝不能被解释为"当前待办已经处理"；有界轮询结束后仍缺失则按 assignment_missing 阻塞。
	if len(facts.CurrentNodes) > 0 && len(facts.CurrentHandlers) == 0 && len(facts.DueNodes) == 0 {
		facts.HandlerGenerationRead = true
		facts.HandlerMissing = true
	}
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
	// 这些动作的流程主事实本来不会推进，必须读取目标各自的业务记录，不能用"节点没变"代替成功验证。
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

// firstCurrentNodeID 返回实例事实里第一个非空的当前节点标识；没有时返回空串。
//
// 用途：实例级动作（转发/催办）不绑定配置节点，需要用实例的当前节点去匹配当前处理人，
// 拿不到节点标识就只能按空节点查询，必然查不到待办。这里只做只读取值，不推断、不猜测。
// 输入边界：facts 为空指针时返回空串（调用方通过值接收，无需额外判空）。
func firstCurrentNodeID(facts *InstanceFacts) string {
	if facts == nil {
		return ""
	}
	for _, nodeID := range facts.CurrentNodes {
		if trimmed := strings.TrimSpace(nodeID); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// readActionTaskFacts 读取任务级动作的目标事实：当前待办、已办任务、任务链和代理树。
// 这些事实只用于门禁和结果核验；任一关键事实缺失都保持禁用，不能靠动作配置猜测目标状态。
func (e *Executor) readActionTaskFacts(ctx context.Context, runCtx RunContext, session target.Session, step model.CompiledActionStep, nodeID string, facts *InstanceFacts, allowDiscovery bool) (target.Session, error) {
	instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	if instanceID == "" {
		return session, nil
	}
	// 实例级动作（转发/催办等）不绑定配置节点，传进来的 nodeID 是空串。它们作用于**实例的
	// 当前活动待办**，因此节点标识回退到实例事实里已读到的当前节点（facts.CurrentNodes）：
	// 否则按空节点匹配不到当前处理人，写前必然报「目标上已无当前处理人在本节点的待办任务」
	// （2026-09-21 运行 79 路径 1 第 43 步实测：前 40 步全部成功，只剩收尾转发失败）。
	// 只回退查询用的节点标识，不放宽任何待办/处理人要求——后续仍按真实待办裁决。
	if strings.TrimSpace(nodeID) == "" && step.Scope == model.ActionScopeInstance {
		nodeID = firstNonEmpty(facts.CurrentTaskNodeID, firstCurrentNodeID(facts))
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
	// ActionForward 是实例级动作，配置里没有节点键；它作用于实例的当前活动待办，
	// 因此必须和审批族一样解析出"当前待办属于哪个节点、由谁处理"，否则写前无法拿到
	// jobTaskId，每一步都报「目标上已无当前处理人在本节点的待办任务」而停在写前
	// （2026-09-21 运行 78 路径 1 第 43 步实测）。
	case model.ActionStorageFormData, model.ActionApprove, model.ActionReject,
		model.ActionAddSign, model.ActionTransfer, model.ActionRollback, model.ActionForward:
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
		// F-035：解析不到待办且目标也没有生成处理人时，把分型事实落进诊断，
		// 让门禁显示「目标节点未生成处理人」而不是误导性的「当前待办已经处理」。
		if !facts.CurrentTaskFound && facts.HandlerGenerationRead && facts.HandlerMissing {
			facts.AssigneeDiag = fmt.Sprintf("目标节点已到达但未生成 currentAuditUserInfo（实例 %s），目标尚未产生处理人（assignment_missing）", instanceID)
		}
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
				// 核验依据分型必须与实际读取过程同源：读取失败、确实没有、确实命中
				// 是三种互不相容的语义，绝不能都塌缩成布尔对（第三轮复核 P0-4）。
				facts.CompletedTaskBasis = completedTaskBasisOf(true, facts.CompletedTaskFound, doneErr != nil)
			} else {
				// 本步没有做这项核对（如处理人即计划账号本人时不切换会话）：
				// 显式登记为占位，防止下游把未取证的布尔当成证据。
				facts.CompletedTaskBasis = CompletedTaskBasisNotAttempted
			}
			return session, nil
		case model.ActionForward:
			// 转发不改主实例游标，也不产生本节点的已办：它只需要把"实例当前活动待办"
			// 的真实节点与处理人事实留给写前门禁取 jobTaskId，因此这里不做额外核验。
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
		// 取回的已办读取必须带「本节点的真实办结人」视角，不能只看计划账号自己名下的已办。
		//
		// 为什么必须先解析视角（2026-09-21 运行 98 路径 5 第 13 步实测）：done 读取在
		// executorId 为空时回落到当前会话用户（这里就是计划账号 骆蒙恩），于是只看得见
		// 计划账号自己那一行已办（发起人 e06f47e5…，preExecuteName=项志飞），
		// 而本步节点「省区经理」的已办属于另一位真实审批人，根本不在这次读取结果里。
		// 后果有两层：门禁拿到的是"别的节点的已办"当成本节点事实；写前的处理人切换也解析不出
		// 执行人（CompletedTaskExecutorID 为空），取回就以空登录账号发出，实例没有回到本节点。
		// 因此从任务关联行里先定位本节点最近一次办结的关联行与执行人，用它作为 done 读取视角。
		var snapshot target.TaskSnapshot
		if viewUserID, viewDiag := e.retrieveDoneViewUser(ctx, runCtx, session, instanceID, strings.TrimSpace(nodeID)); viewUserID != "" {
			viewSnapshot, _, _, taskSession, resolveErr := e.resolveTaskSnapshotForView(ctx, runCtx, step, nodeID, "done", viewUserID, session)
			if resolveErr != nil {
				return session, resolveErr
			}
			snapshot = viewSnapshot
			session = taskSession
			facts.CompletedTaskRead = true
			facts.CompletedTaskFound = strings.TrimSpace(snapshot.JobTaskID) != ""
			facts.CompletedTaskBasis = completedTaskBasisOf(true, facts.CompletedTaskFound, false)
			facts.CompletedTaskLinkID = strings.TrimSpace(snapshot.LinkID)
			facts.CompletedTaskNodeID = strings.TrimSpace(snapshot.FlowNodeProxyID)
			facts.CompletedTaskParentID = strings.TrimSpace(snapshot.ParentLinkID)
			facts.CompletedTaskBatchNo = strings.TrimSpace(snapshot.BatchNo)
			facts.CompletedTaskAuditWay = strings.TrimSpace(snapshot.AuditWay)
			if facts.CompletedTaskFound {
				// 该行的执行人就是本节点真实办结人：取回必须由他本人发出，同时作为写前切换身份。
				facts.CompletedTaskExecutorID = viewUserID
				facts.CurrentTaskAssigneeID = viewUserID
				if viewDiag != "" {
					facts.AssigneeDiag = viewDiag
				}
				if hasAuditReader {
					if auditFactsErr := e.applyRetrieveAuditEvidence(ctx, session, instanceID, strings.TrimSpace(nodeID), facts); auditFactsErr != nil {
						return session, auditFactsErr
					}
				}
			}
		} else {
			fallbackSnapshot, _, _, taskSession, resolveErr := e.resolveTaskSnapshotForStep(ctx, runCtx, session, step, nodeID, "done", facts, allowDiscovery)
			if resolveErr != nil {
				return session, resolveErr
			}
			snapshot = fallbackSnapshot
			session = taskSession
			facts.CompletedTaskRead = true
			facts.CompletedTaskFound = strings.TrimSpace(snapshot.JobTaskID) != ""
			// 取回走 done 的 executorId 语义，读取成功即已核对：命中与否都是有效证据。
			facts.CompletedTaskBasis = completedTaskBasisOf(true, facts.CompletedTaskFound, false)
			facts.CompletedTaskLinkID = strings.TrimSpace(snapshot.LinkID)
			facts.CompletedTaskNodeID = strings.TrimSpace(snapshot.FlowNodeProxyID)
			facts.CompletedTaskParentID = strings.TrimSpace(snapshot.ParentLinkID)
			facts.CompletedTaskBatchNo = strings.TrimSpace(snapshot.BatchNo)
			facts.CompletedTaskAuditWay = strings.TrimSpace(snapshot.AuditWay)
		}
		if !facts.CompletedTaskFound {
			// 已办行按节点过滤读空，不能就此断定"没有已办"：目标在本节点办结后会把这个任务
			// 关联行的 flowNodeProxyID 改写成"实例流转到的下一节点"（2026-09-21 运行 93 路径 3
			// 实测：同一条 jobTaskId 858ded51… 在 pending 时是发起人 a408500c…，办结后变成
			// 省区经理 b9fc434e…），因此"按本步节点等值过滤"必然漏掉本人刚产生的已办。
			// 审核记录按办结时点落库、节点字段不被改写，是本节点「确实已被办结」的权威正面分型，
			// 用它补这一件事；补不到就保持读空，由门禁按既有纪律阻塞，绝不用未知顶替事实。
			if !hasAuditReader {
				return session, nil
			}
			if auditFactsErr := e.applyRetrieveAuditEvidence(ctx, session, instanceID, strings.TrimSpace(nodeID), facts); auditFactsErr != nil {
				return session, auditFactsErr
			}
			return session, nil
		}
		// 目标任务列表没有"所有状态、所有人员"的有效读取方式：空 taskStatus 会被目标直接拒绝，
		// 而 pending/done 又只覆盖当前用户。后继任务是否合法必须交由 retrieveProcess 在实例锁内确认，
		// 以目标接口原文作为失败结果，不能用不完整列表臆测为"后继已处理"。
		if hasTreeReader {
			proxyID := firstNonEmpty(snapshot.FlowProxyID, runCtx.FlowProxyID)
			if proxyID != "" {
				tree, treeErr := treeReader.ReadProxyTree(ctx, session, proxyID)
				if treeErr != nil {
					return session, treeErr
				}
				// 判断"已办是否落在发起节点"必须用本步配置节点的目标 ID，不能用已办行上的
				// flowNodeProxyID：该字段在本节点办结后会被目标改写成流转到的下一节点
				// （实测同一条已办从发起人 a408500c… 变成省区经理 b9fc434e…），
				// 用它判类型会把"发起节点不支持取回"这条门禁判反。
				facts.RetrieveNodeIsStart = isStartNodeType(nodeTypeInTree(tree, strings.TrimSpace(nodeID)))
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
		if auditFactsErr := e.applyRetrieveAuditEvidence(ctx, session, instanceID, strings.TrimSpace(nodeID), facts); auditFactsErr != nil {
			return session, auditFactsErr
		}
	}
	return session, nil
}

// retrieveDoneViewUser 解析本步节点最近一次办结人的用户 ID，作为取回 done 读取的查询视角。
//
// 职责：读本实例的审核记录（/web/flowAuditRecord/list，按实例查询、不受 executorId 过滤），
// 找出 flowNodeProxyID 等于本步节点、且带执行人用户 ID 的最近一条记录，返回该执行人用户 ID。
//
// 为什么必须存在（2026-09-21 运行 98/100 路径 5 第 13 步实测）：done 读取在 executorId 为空时
// 回落当前会话用户（门禁阶段就是计划账号会话），于是只看得见计划账号自己那一行已办，
// 本步节点「省区经理」的已办属于另一位真实审批人，不在结果里。后果有两层：门禁把"别的节点的已办"
// 当成本节点事实；写前处理人切换解析不出执行人（CompletedTaskExecutorID 为空），
// 取回以空登录账号发出，实例没有回到本节点，后续步骤报「当前待办已经处理」而停止。
//
// 为什么不用已办列表找执行人：/web/flowJobTaskLink/list 在 done 视角下按 executorId 过滤，
// 用它去找"别人的已办"是自相矛盾的（列表本身已被这个视角限死）。审核记录按实例查询、按节点归属，
// 才是"这个节点是谁办结的"这一事实的正确来源。
//
// 输入边界：
//   - 只读，不发写请求；instanceID 或 nodeID 为空时返回空串，由调用方沿用既有读取路径；
//   - 目标不支持审核记录读取时返回空串，由调用方回落既有口径，不因此阻塞。
//
// 关键约束：
//   - 只接受节点严格等于本步节点、且执行人用户 ID 非空、状态为「通过」的记录；
//     命中多条时取最后一条（记录按时间返回，后写覆盖=最近一次办结），与取回"取最近一次已办"一致；
//   - 找不到时返回空串，绝不猜测执行人，也绝不用计划账号顶替（那正是本缺陷的成因）。
func (e *Executor) retrieveDoneViewUser(ctx context.Context, runCtx RunContext, session target.Session, instanceID, nodeID string) (string, string) {
	if nodeID == "" || instanceID == "" {
		return "", ""
	}
	auditReader, hasAuditReader := e.target.(auditRecordsReader)
	if !hasAuditReader {
		return "", ""
	}
	records, auditErr := auditReader.ListAuditRecords(ctx, session, instanceID)
	if auditErr != nil {
		// 读取失败不属于"没有视角"：如实记诊断，由调用方回落既有读取路径后按门禁裁决。
		return "", "取回已办视角解析：审核记录读取失败，回落既有读取：" + auditErr.Error()
	}
	viewUserID := ""
	for _, record := range records {
		if strings.TrimSpace(record.FlowNodeProxyID) != nodeID {
			continue
		}
		// 只认「通过」这类办结记录：驳回等其它状态不代表本节点被办结，不能作为取回对象。
		if !strings.EqualFold(record.AuditStatus, "pass") {
			continue
		}
		if executorID := strings.TrimSpace(record.ExecutorID); executorID != "" {
			viewUserID = executorID
		}
	}
	if viewUserID == "" {
		return "", ""
	}
	return viewUserID, "取回已办视角解析：按审核记录定位到本步节点的真实办结人，已办读取改用该执行人视角"
}

// resolveTaskSnapshotForView 以指定执行人视角读取任务快照，并返回真正完成读取的会话。
//
// 职责：调用带视角的任务列表读取（done 走 data.executorId），在本实例里取出节点匹配的唯一快照。
//
// 为什么必须存在：取回的已办行所属执行人往往不是计划账号，用计划账号会话读取只会得到
// 计划账号自己名下的行。目标原生支持按 data.executorId 限定查询视角，引擎据此读取本节点真实办结行。
//
// 输入边界：
//   - viewUserID 为空时退化为既有读取路径；读取失败原样上抛，由调用方按既有纪律阻塞。
//
// 关键约束：
//   - 节点匹配必须严格等值；命中多行时按"最近一次"取最后一行；
//   - 不因视角读取无结果就放宽节点过滤或猜测任务号。
func (e *Executor) resolveTaskSnapshotForView(ctx context.Context, runCtx RunContext, step model.CompiledActionStep, nodeID, status, viewUserID string, session target.Session) (target.TaskSnapshot, string, string, target.Session, error) {
	viewReader, hasViewReader := e.target.(taskSnapshotViewReader)
	instanceID := strings.TrimSpace(runCtx.PathRun.MainInstanceRef)
	if !hasViewReader || instanceID == "" || strings.TrimSpace(viewUserID) == "" {
		snapshot, err := e.readTaskSnapshot(ctx, runCtx, step, nodeID, session, status)
		return snapshot, "", "", session, err
	}
	snapshots, active, viewErr := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, strings.TrimSpace(session.Summary.Account), session,
		func(callContext context.Context, active target.Session) ([]target.TaskSnapshot, error) {
			return viewReader.ListTaskSnapshotsForUser(callContext, active, instanceID, status, viewUserID)
		})
	if viewErr != nil {
		return target.TaskSnapshot{}, "", "", session, viewErr
	}
	matched := target.TaskSnapshot{}
	found := false
	for _, snapshot := range snapshots {
		if strings.TrimSpace(snapshot.FlowNodeProxyID) != strings.TrimSpace(nodeID) {
			continue
		}
		matched = snapshot
		found = true
	}
	if !found {
		return target.TaskSnapshot{}, "", "", active, nil
	}
	return matched, viewUserID, "", active, nil
}

// applyRetrieveAuditEvidence 用审核记录补齐取回所需的已办事实。
//
// 职责：读取本实例的审核记录，判定本步骤节点是否确实已被办结、办结人是谁，
// 并把结论写进 facts，供取回门禁与取回写请求的身份切换使用。
//
// 为什么必须存在（2026-09-21 运行 90/93 路径 2、3 实测）：目标在本节点办结后，会把任务关联行的
// flowNodeProxyID 改写成"实例流转到的下一节点"。同一条 jobTaskId 858ded51… 在 pending 时是
// 发起人 a408500c…，03:10:19 办结后变成省区经理 b9fc434e…。于是"拿本步节点等值过滤已办行"
// 永远读不到刚产生的已办（实测 target_rows=1/matched_rows=1 命中却因节点不符被丢弃），
// 门禁把"确实已办结"误判成"没有已办"，把可取回的实例判成不可取，路径停在取回写前且重跑结果相同。
// 审核记录按办结时点落库、节点字段不被改写，是本节点「确实已被办结」的权威正面分型。
//
// 输入边界：
//   - 只读审核记录，不发任何写请求；
//   - nodeID 为空时不做补齐（无从判断本节点），保持既有事实。
//
// 关键约束：
//   - 只有确实读到本节点「通过」且带执行人用户 ID 的记录才把已办判为命中，
//     绝不因"存在别的节点已办"就放行；
//   - 已办批次号保持读空，不猜测批次；实例归属由审核记录的 executorId 决定；
//   - 读取失败必须原样上抛，让本步按既有纪律阻塞，绝不吞错后当作"没有已办"。
func (e *Executor) applyRetrieveAuditEvidence(ctx context.Context, session target.Session, instanceID, nodeID string, facts *InstanceFacts) error {
	if facts == nil || strings.TrimSpace(nodeID) == "" {
		return nil
	}
	auditReader, hasAuditReader := e.target.(auditRecordsReader)
	if !hasAuditReader {
		return nil
	}
	records, auditErr := auditReader.ListAuditRecords(ctx, session, instanceID)
	if auditErr != nil {
		return auditErr
	}
	retrieveNodeID := strings.TrimSpace(nodeID)
	completedNodePassFound := false
	completedNodePassLinkID := ""
	for _, record := range records {
		if record.FlowJobTaskID == facts.CompletedTaskLinkID && strings.EqualFold(record.AuditStatus, "retrieve") {
			facts.RetrieveAlreadyUsed = true
		}
		// 取回节点最近一次「通过」的审批人：取回写请求必须以该处理人身份发出，目标据 executorId 校验归属。
		// 审核记录按时间返回，同节点多条时后写覆盖=取最近一次通过的执行人（会签取其一即可，目标校验的是成员归属）。
		if strings.TrimSpace(record.FlowNodeProxyID) == retrieveNodeID && strings.EqualFold(record.AuditStatus, "pass") && strings.TrimSpace(record.ExecutorID) != "" {
			facts.CompletedTaskExecutorID = strings.TrimSpace(record.ExecutorID)
			completedNodePassFound = true
			completedNodePassLinkID = strings.TrimSpace(record.FlowJobTaskID)
			if name := strings.TrimSpace(record.ExecutorName); name != "" {
				facts.CurrentTaskAssigneeName = name
			}
		}
		// 当前取回人可能与计划账号不同；先按审核记录解析出的真实执行人判断归属，
		// 再用会话 UserID 作为普通场景兜底，否则“该节点的人同意以后再取回”会被误判成他人已处理。
		if !containsString(facts.DueNodes, record.FlowNodeProxyID) || record.ExecutorID == "" ||
			record.ExecutorID == facts.CompletedTaskExecutorID || record.ExecutorID == session.UserID {
			continue
		}
		if strings.EqualFold(record.AuditStatus, "pass") {
			facts.CurrentTaskHandledByOther = true
		}
	}
	// 已办行按节点过滤读空、但审核记录证明本节点确实已被办结时，采用审核记录作为正面分型。
	if !facts.CompletedTaskFound && completedNodePassFound {
		facts.CompletedTaskFound = true
		facts.CompletedTaskBasis = CompletedTaskBasisCheckedFound
		facts.CompletedTaskNodeID = retrieveNodeID
		if completedNodePassLinkID != "" {
			// 保留任务关联行 ID，供"是否已被取回"的重复判定使用（与审核记录同源可比）。
			facts.CompletedTaskLinkID = completedNodePassLinkID
		}
	}
	// 把取回执行人交给写路径的处理人切换逻辑：取回一律以该办结节点的真实审批人身份发出，
	// 与其它任务级动作同一条切换会话链路，绝不用计划账号顶替（用户裁决 + 目标 executorId 归属校验）。
	if facts.CompletedTaskExecutorID != "" {
		facts.CurrentTaskAssigneeID = facts.CompletedTaskExecutorID
		// 执行人中文名称优先由审核记录中的 executorName 提供；若未提供则从目录反查真实姓名，绝不把 32 位 ID 当作姓名。
		if strings.TrimSpace(facts.CurrentTaskAssigneeName) == "" {
			if resolver, ok := e.target.(userAccountResolver); ok {
				if accounts, _, err := readOnlyWithSessionRetry(ctx, e.policy, e.sessions, session.Summary.Account, session,
					func(callContext context.Context, active target.Session) (map[string]target.UserAccount, error) {
						return resolver.UserAccountsByID(callContext, active, []string{facts.CompletedTaskExecutorID})
					}); err == nil {
					if info, ok := accounts[facts.CompletedTaskExecutorID]; ok {
						facts.CurrentTaskAssigneeName = firstNonEmpty(strings.TrimSpace(info.Name), strings.TrimSpace(info.Account))
					}
				}
			}
		}
	}
	return nil
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
//   - 审批：实例当前节点已越过本步节点即「已前进」；本步待办仍在且实例未前进即「明确未变」。
//
// expectedNextNodeKey 是本步配置的下一跳目标节点标识（运行期从路径配置取得，不是写死的常量），
// 用于识别「实例已经推进到下一跳、但目标返回的本人待办快照仍是写前旧值」这一读写时序窗口：
// 该窗口下重读事实会同时给出「节点已前进」与「本人待办未消失」两条互相冲突的观察，
// 只看待办会把一次已经生效的写误判成"未变"，进而把整条路径判成结果待确认（见下方审批分支）。
func ClassifyReread(action string, stepNodeKey string, expectedNextNodeKey string, before, after InstanceFacts) verdict.Reread {
	if after.ReadError != "" {
		return verdict.RereadUnreadable
	}
	// 推进事实优先于待办快照：实例当前节点已经命中本步配置的下一跳，就是这次写已生效的直接证据。
	// 目标的任务列表读取与实例节点写入不共享同一事务，会签/多处理人节点上本人那条 pending
	// 记录常常比实例节点位置晚一拍消失；此时若只凭"本人待办还在"判明确未变，
	// 会把一次真实成功的写判成无法确认，并让整条运行终局停止。
	// 这条判据只用已经读到的目标事实：实例当前节点集合与运行期传入的下一跳标识都来自本次重读与路径配置，
	// 不新增任何猜测，也不放松"读不到事实"时的保守判定。
	if advancedByNodePosition(stepNodeKey, expectedNextNodeKey, after) {
		return verdict.RereadAdvanced
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
		// 依据必须来自 CompletedTaskBasis 的正面分型：只有「确实核对过并命中」才算证据，
		// 占位（未做核对）与读取失败都不能把未生效的写判成已生效（第三轮复核 P0-4）。
		actorDone := action == string(model.ActionApprove) &&
			after.CompletedTaskBasis == CompletedTaskBasisCheckedFound &&
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
			if action == string(model.ActionApprove) &&
				after.CompletedTaskBasis == CompletedTaskBasisCheckedFound &&
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

// advancedByNodePosition 判断重读到的事实是否已经证明实例越过了本步节点。
// 判定条件必须同时成立，缺一不可：
//   - 本步节点标识与下一跳标识都已知（任一为空说明路径配置没有给出可核对的锚点，不能据此推断）；
//   - 重读到的实例当前节点集合非空（空集说明本次没读到节点位置，不能当成"已前进"）；
//   - 本步节点已经不在实例当前节点集合里（还在原地就说明写确实没推进）；
//   - 实例当前节点集合命中了本步配置的下一跳（越到别的分支属于路径偏离，由独立的偏离判定处理，
//     这里绝不能把"走到别处"也认成"本步成功"，否则会把走错分支洗成成功）。
//
// 只用已经读到的目标事实与运行期路径配置，不写死任何节点标识。
func advancedByNodePosition(stepNodeKey, expectedNextNodeKey string, after InstanceFacts) bool {
	current := strings.TrimSpace(stepNodeKey)
	expectedNext := strings.TrimSpace(expectedNextNodeKey)
	if current == "" || expectedNext == "" || len(after.CurrentNodes) == 0 {
		return false
	}
	for _, node := range after.CurrentNodes {
		if strings.TrimSpace(node) == current {
			return false
		}
	}
	for _, node := range after.CurrentNodes {
		if strings.TrimSpace(node) == expectedNext {
			return true
		}
	}
	return false
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
	var rejection *target.BusinessRejection
	if response.IsSuccessPresent {
		observation.Response = &verdict.Response{
			IsSuccess:        response.IsSuccess,
			IsSuccessPresent: true,
			Code:             response.Code,
			Message:          response.Message,
		}
	} else if errors.As(writeErr, &rejection) && transport == verdict.TransportResponded {
		// 目标返回完整响应且是明确业务拒绝（isSuccess=false 带 code/message，如「下一节点已被处理,
		// 不支持取回」）：这是确定失败、无副作用的结构化事实，绝不能因为没解析出 WriteResponse
		// 就当「不可解析」判成待确认——待确认对用户没有意义，也无从确认。原样带上 code/message
		// 交给判定包按业务拒绝判确定失败，界面直接显示目标原话。
		observation.Response = &verdict.Response{
			IsSuccess:        false,
			IsSuccessPresent: true,
			Code:             strings.TrimSpace(rejection.Code),
			Message:          strings.TrimSpace(rejection.Message),
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
