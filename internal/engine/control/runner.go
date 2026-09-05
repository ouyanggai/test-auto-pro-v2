// 运行 Runner：自动运行与「执行到下一节点 / 继续运行」的连续执行循环。
// 边界（纲领第 4.5 节）：暂停只在阶段 3 生效；已进入 submit 的步骤必须走完 verify 与 settle；
// 停止是终态。循环复用 F-016 的一步执行器与租约，不另造调度。
package control

import (
	"context"
	"fmt"

	"test-auto-pro-v2/internal/model"
)

// loopFailureReason 把循环内必须落库却失败的事实转成中文停止原因。
// 控制事实是真实写之前的审计记录：落不下去就不能继续往下发写请求，只能停在这里让人看到。
func loopFailureReason(what string, err error) string {
	return what + "落库失败，为避免出现没有审计记录的真实写，连续执行已停止：" + err.Error()
}

// startLoop 启动连续执行循环（自动运行 / 执行到下一节点 / 继续运行）。
// 循环在后台 goroutine 里跑：API 立即返回当前状态，前端按配置轮询。
func (s *Service) startLoop(ctx context.Context, pathRunID uint64, session *activeStep, command model.ControlCommand) {
	s.mu.Lock()
	if session == nil || session.loopRunning {
		s.mu.Unlock()
		return
	}
	session.loopRunning = true
	s.mu.Unlock()
	// 循环在后台跑，而控制 API 立即返回：HTTP 处理器一返回，请求上下文就被取消。
	// 必须切断取消信号只保留取值（日志作用域等）——否则第一步的目标调用立刻拿到 context.Canceled，
	// 传输档归类不出来即按不确定处理，"继续运行"会变成一次写都没发就把路径推进待对账。
	go s.runLoop(context.WithoutCancel(ctx), pathRunID, session, command)
}

// runLoop 是连续执行的主体。每轮：阶段 3 判定（停止请求/门禁/断点命中/下一节点边界/暂停）→ 执行一步。
// 执行复用 approveOneStep：与单步命令完全同一条路径，保证行为同源。
func (s *Service) runLoop(ctx context.Context, pathRunID uint64, session *activeStep, command model.ControlCommand) {
	defer func() {
		s.mu.Lock()
		session.loopRunning = false
		s.mu.Unlock()
	}()
	fromNode := ""
	if session.preview != nil {
		fromNode = session.preview.NodeKey
	}
	for {
		// 阶段 3 判定一：停止请求生效（本步尚未发出写请求，停在这里最安全）。
		s.mu.Lock()
		stopRequested := session.stopRequested
		s.mu.Unlock()
		if stopRequested {
			s.applyStop(ctx, pathRunID)
			return
		}
		s.mu.Lock()
		preview := session.preview
		s.mu.Unlock()
		if preview == nil {
			return
		}

		// 阶段 3 判定二：门禁不通过即停（不自动执行被门禁阻塞的步骤，留给用户查看与停止）。
		if preview.BlockReason != "" {
			s.mu.Lock()
			session.stopReason = "门禁不通过：" + preview.BlockReason
			s.mu.Unlock()
			return
		}

		// 阶段 3 判定三：断点命中（全部落事实与事件；是否停留按命令与命中情况）。
		facts := StepFacts{
			StepNo: preview.StepNo, NodeKey: preview.NodeKey, Action: string(preview.Action),
			IsWriteStep: preview.Endpoint != "", DeviationHit: session.deviationStalled,
		}
		hits := EvaluateBreakpointHits(facts, session.breakpoints)
		for _, hit := range hits {
			hitFact := model.RunControl{
				RunID: session.runCtx.Run.ID, PathRunID: pathRunID,
				Kind: model.ControlFactBreakpointHit, BreakpointType: hit.Breakpoint.Type,
				ObjectKind: "step", ObjectKey: fmt.Sprintf("%d", preview.StepNo),
				Reason: hit.Reason, Source: model.RunControlSourceUI, CreatedAt: s.now(),
			}
			if err := s.store.AppendRunControl(ctx, hitFact, s.now()); err != nil {
				s.mu.Lock()
				session.stopReason = loopFailureReason("断点命中事实", err)
				s.mu.Unlock()
				return
			}
			s.logFact(pathRunID, hitFact, preview.StepNo)
			s.appendEventOrWarn(ctx, pathRunID, model.RunEvent{
				RunID: session.runCtx.Run.ID, PathRunID: &pathRunID,
				Kind: "breakpoint_hit", Label: fmt.Sprintf("断点命中：%s（%s）", hit.Breakpoint.Label(), hit.Reason),
			}, preview.StepNo)
		}
		// 执行到下一节点的边界：语义节点变化，在阶段 3 暂停（先落断点事实再停）。
		if command == model.CommandNextNode && fromNode != "" && NextNodeBoundary(fromNode, preview.NodeKey) {
			s.mu.Lock()
			session.stopReason = fmt.Sprintf("已连续执行到下一个节点「%s」之前", preview.NodeName)
			s.mu.Unlock()
			return
		}
		if len(hits) > 0 {
			primary := PrimaryHit(hits)
			if primary.Breakpoint.Type == model.BreakpointPathDeviation {
				// 路径偏离强制停止：不产出放行类命令（后续步骤在偏离后的结构上不成立）。
				s.mu.Lock()
				session.deviationStalled = true
				session.stopReason = "路径偏离：" + primary.Reason + "；后续步骤不提供放行，只能停止或查看"
				s.mu.Unlock()
				s.appendEventOrWarn(ctx, pathRunID, model.RunEvent{
					RunID: session.runCtx.Run.ID, PathRunID: &pathRunID,
					Kind: "path_deviation_stopped", Label: "路径偏离断点强制停止，不提供放行",
				}, preview.StepNo)
				return
			}
			s.mu.Lock()
			session.stopReason = "命中" + primary.Breakpoint.Label() + "：" + primary.Reason
			s.mu.Unlock()
			return
		}

		// 每一步的放行事实（命令种类随循环命令），随后执行本步。
		approveFact := model.RunControl{
			RunID: session.runCtx.Run.ID, PathRunID: pathRunID,
			Kind: model.ControlFactApproved, Action: model.RunControlApprove,
			Command: command, Source: model.RunControlSourceUI, CreatedAt: s.now(),
		}
		if err := s.store.AppendRunControl(ctx, approveFact, s.now()); err != nil {
			s.mu.Lock()
			session.stopReason = loopFailureReason("放行事实", err)
			s.mu.Unlock()
			return
		}
		s.logFact(pathRunID, approveFact, preview.StepNo)
		result, err := s.approveOneStep(ctx, pathRunID, session, 1)
		if err != nil {
			s.mu.Lock()
			session.stopReason = "执行失败：" + err.Error()
			s.mu.Unlock()
			return
		}
		if result.Outcome.Verdict != "confirmed_success" || result.PathFinished {
			// 终局（失败/不确定/场景走完）：approveOneStep 已完成收尾或现场已作废。
			return
		}
		// 暂停请求在本步走完 verify 与 settle 后生效（纲领第 4.5 节）。
		s.mu.Lock()
		pauseRequested := session.pauseRequested
		s.mu.Unlock()
		if pauseRequested {
			pausedFact := model.RunControl{
				RunID: session.runCtx.Run.ID, PathRunID: pathRunID,
				Kind: model.ControlFactPaused, Source: model.RunControlSourceUI, CreatedAt: s.now(),
			}
			reason := "暂停请求已生效（本步已走完核验与落账）"
			if err := s.store.AppendRunControl(ctx, pausedFact, s.now()); err != nil {
				reason = loopFailureReason("暂停事实", err)
			} else {
				s.logFact(pathRunID, pausedFact, preview.StepNo)
			}
			s.mu.Lock()
			session.stopReason = reason
			s.mu.Unlock()
			return
		}
	}
}

// applyStop 在循环内执行停止：停止是终态，已发生的事实全部保留。
// 状态迁移失败时绝不声称已停止：保留现场与中文原因，让用户能再次停止，
// 否则路径运行在库里还是运行中，而工具已经把现场清掉，谁都收不了尾。
func (s *Service) applyStop(ctx context.Context, pathRunID uint64) {
	if _, err := s.runs.Stop(ctx, pathRunID); err != nil {
		s.mu.Lock()
		if session := s.active[pathRunID]; session != nil {
			session.stopRequested = false
			session.stopReason = "停止未生效：" + err.Error() + "；已发生的事实全部保留，可再次停止"
		}
		s.mu.Unlock()
		return
	}
	stoppedFact := model.RunControl{
		PathRunID: pathRunID, Kind: model.ControlFactStopped,
		Source: model.RunControlSourceUI, CreatedAt: s.now(),
	}
	if err := s.store.AppendRunControl(ctx, stoppedFact, s.now()); err == nil {
		s.logFact(pathRunID, stoppedFact, 0)
	} else {
		s.warnFactFailure(pathRunID, "停止事实", err)
	}
	s.mu.Lock()
	session := s.active[pathRunID]
	if session != nil {
		session.finished = true
	}
	s.mu.Unlock()
	s.clear(pathRunID)
}

// appendEventOrWarn 追加运行事件；事件只服务于界面与回放，数据库事实表才是权威，
// 因此失败不中断执行，但必须留下痕迹而不是静默丢弃（纲领第 6.4 节：被吞掉的错误算缺陷）。
func (s *Service) appendEventOrWarn(ctx context.Context, pathRunID uint64, event model.RunEvent, stepNo int) {
	if err := s.store.AppendRunEvent(ctx, event, s.now()); err != nil {
		s.warnFactFailure(pathRunID, "运行事件（"+event.Kind+"）", err)
	}
}

// warnFactFailure 把非致命的落库失败写进 control.log，保证事后能查到它发生过。
func (s *Service) warnFactFailure(pathRunID uint64, what string, err error) {
	s.logFact(pathRunID, model.RunControl{
		Kind:   model.ControlFactKind("append_failed"),
		Reason: what + "落库失败：" + err.Error(),
	}, 0)
}
