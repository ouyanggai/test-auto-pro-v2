// 对账服务：进入待对账后的自动只读对账与三个唯一恢复动作（纲领第 4.4 节）。
// 边界：对账只读；重放是一次新尝试（重走七阶段）不是重发；重复请求只返回第一次决定。
package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"test-auto-pro-v2/internal/engine/reconcile"
	"test-auto-pro-v2/internal/model"
)

// ReconcileResultView 是对账结论的公开形态（进 DTO 与内存现场）。
type ReconcileResultView struct {
	Verdict     string   `json:"verdict"`
	VerdictName string   `json:"verdictName"`
	Action      string   `json:"action"`
	Headline    string   `json:"headline"`
	Reasons     []string `json:"reasons"`
	ReplaysUsed int      `json:"replaysUsed"`
	ReplaysMax  int      `json:"replaysMax"`
	// ReplayExhausted 表示证据仍指向未生效、但重放次数已用完：
	// 此时唯一动作降级为登记人工核对结论，界面不得再渲染重放入口。
	ReplayExhausted bool `json:"replayExhausted"`
}

// reconciliation 配额：重放次数上限走配置，默认 1（本服务用固定默认，超限固定人工结论）。
const defaultReplayLimit = 1

// ReconcileNow 对一条待对账路径运行执行只读对账：收集事实→纯判定→落三列与事件→recovery.log。
// 只读：任何失败都把 reconcile_again 作为唯一动作，绝不触发写。
func (s *Service) ReconcileNow(ctx context.Context, pathRunID uint64) (*ReconcileResultView, error) {
	s.mu.Lock()
	session := s.active[pathRunID]
	s.mu.Unlock()
	if session == nil {
		return nil, ErrNoActiveStep
	}
	runCtx := session.runCtx
	preview := session.preview
	if preview == nil {
		return nil, ErrNoActiveStep
	}
	// 运行聚合身份从库里取：事件行的 run_id 必须是真实运行 ID，
	// 早期实现误填了路径运行 ID，事件按运行维度回放时会指向不存在的运行。
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	// 只读收集：发起前实例不存在的基准（审批类动作用已有实例基准，由执行器事实提供）。
	facts, err := s.steps.ReconcileFacts(ctx, runCtx, preview.StepNo)
	if err != nil {
		return nil, err
	}
	input := reconcile.Collect(reconcile.FactInput{
		// 对账要与目标返回的真实节点集合对照，传真实节点标识而不是工具侧不透明键。
		StepNodeKey:       preview.TargetNodeID,
		BeforeStatus:      facts.BeforeStatus,
		BeforeHadInstance: facts.BeforeHadInstance,
		NowFound:          facts.NowFound,
		NowStatus:         facts.NowStatus,
		NowCurrentNodes:   facts.NowCurrentNodes,
		NowDueNodes:       facts.NowDueNodes,
		NowReadError:      facts.NowReadError,
		// 已办记录与动作痕迹按真实读取结果填入：读到才算证据，读不到由判定器按缺失降级。
		// 「未生效」是唯一会导致重放（再写一次）的结论，因此这两维必须来自真实读取而不是默认值。
		DoneRecordsRead:  facts.DoneRecordsRead,
		DoneRecordFound:  facts.DoneRecordFound,
		ActionTraceRead:  facts.ActionTraceRead,
		ActionTraceFound: facts.ActionTraceFound,
		ActionTraceTotal: facts.ActionTraceTotal,
		FormChanged:      false,
	})
	result := reconcile.Reconcile(input)

	s.mu.Lock()
	// 已用重放次数必须取会话真实计数：每次对账都重新构造结论视图，
	// 若这里回填 0，重放上限的判断就永远是 0 >= 1 不成立，"默认最多重放一次"形同虚设。
	replaysUsed := session.replaysUsed
	s.mu.Unlock()
	view := &ReconcileResultView{
		Verdict: string(result.Verdict), VerdictName: verdictName(result.Verdict),
		Action: string(result.Action), Headline: result.Headline, Reasons: result.Reasons,
		ReplaysUsed: replaysUsed, ReplaysMax: defaultReplayLimit,
	}
	// 重放次数用完后不再提供重放（纲领第 4.4 节：用完仍不确定就不给第二次）。
	// 结论仍如实是"未生效"——证据没有变；变的只是可执行动作：只能登记人工核对结论并结束。
	// 这一步必须在服务端完成，界面才不会渲染一个点下去必然被拒的按钮。
	if result.Verdict == reconcile.VerdictNotEffective && replaysUsed >= defaultReplayLimit {
		view.Action = string(reconcile.ActionManualEnd)
		view.ReplayExhausted = true
		view.Headline = fmt.Sprintf("对账仍判未生效，但重放次数已用完（%d 次）：只能登记人工核对结论并结束本路径运行",
			defaultReplayLimit)
	}
	s.mu.Lock()
	session.reconcile = view
	s.mu.Unlock()

	// 对账三列写回最近一次尝试行（纲领第 7.2 节：这三列归属 run_step_attempts），
	// 并追加中文事件与 recovery.log。
	_, lastAttempt, err := s.store.LatestStepAttempt(ctx, pathRunID)
	if err == nil {
		// is_replay 传既有值而不是写死 false：这一列描述"这次尝试本身是不是重放"，
		// 由执行时落账决定；对账回写三列时把它按 false 覆盖会把重放尝试记成首次执行。
		// 落库的恢复动作是服务端此刻真正提供的那一个（含重放用完后的降级），
		// 不是纯判定的原始动作；否则记录会显示一个当时并不可执行的动作。
		if err := s.store.RecordReconcileOutcome(ctx, lastAttempt.ID,
			string(result.Verdict), view.Action, lastAttempt.IsReplay, s.now()); err != nil {
			return view, err
		}
		_ = s.store.AppendRunEvent(ctx, model.RunEvent{
			RunID: pathRun.RunID, PathRunID: &pathRunID,
			Kind: "reconciled", Label: fmt.Sprintf("对账结论：%s——%s", verdictName(result.Verdict), result.Headline),
		}, s.now())
		s.recoveryLog.LogFact(pathRunID, fmt.Sprintf("attempt=%d verdict=%s action=%s step=%d", lastAttempt.AttemptNo, result.Verdict, result.Action, preview.StepNo))
		for _, reason := range result.Reasons {
			s.recoveryLog.LogFact(pathRunID, "  依据："+reason)
		}
	}
	return view, nil
}

// RecoveryAction 执行对账给出的唯一合法动作。
// advance：确认并前进（只推进一次游标）；replay：重放这一步（新尝试、重走七阶段、重过门禁）；
// manual_end：登记人工核对结论并结束路径。重复/过期请求返回当前真实状态，不发写请求。
func (s *Service) RecoveryAction(ctx context.Context, pathRunID uint64, action reconcile.RecoveryAction, manual model.RunManualConclusion) error {
	s.mu.Lock()
	session := s.active[pathRunID]
	s.mu.Unlock()
	if session == nil {
		return ErrNoActiveStep
	}
	view := session.reconcile
	if view == nil {
		return fmt.Errorf("尚未完成对账，请先执行只读对账")
	}
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return err
	}
	// 守卫只拒绝"真正结束"的路径运行。待对账同样属于停摆态，但它正是恢复动作的唯一入口：
	// 这里若按 IsTerminalPathRunStatus 一律拒绝，三个恢复动作全部不可达，对账层等于空转。
	if !model.CanRecoverPathRunStatus(pathRun.Status) {
		return fmt.Errorf("%w：路径运行当前为 %s，没有可执行的恢复动作",
			ErrRunAlreadyFinished, model.PathRunStatusName(pathRun.Status))
	}
	// 一个结论只对应一个动作：请求的动作必须与服务端上一次给出的唯一合法动作全等。
	// 直接比对动作而不是逐条比对结论，重放次数用完后的降级也就自然被覆盖，
	// 不会出现"结论是未生效、于是放过重放"却在配额上被拒的两套口径。
	if string(action) != view.Action {
		return fmt.Errorf("%w：当前唯一合法动作是 %s，不接受 %s",
			ErrCommandNotAllowed, view.Action, string(action))
	}
	// 执行前必须再次只读对账（不能沿用旧结论）。
	fresh, err := s.ReconcileNow(ctx, pathRunID)
	if err != nil {
		return err
	}
	if fresh.Verdict != view.Verdict || fresh.Action != view.Action {
		return fmt.Errorf("重新对账后结论已变化（%s → %s），请以最新结论为准",
			verdictName(reconcile.Verdict(view.Verdict)), verdictName(reconcile.Verdict(fresh.Verdict)))
	}

	switch action {
	case reconcile.ActionAdvance:
		// 确认并前进：先把路径运行从待对账带回运行中（否则后续放行与收尾都会被状态机拒绝），
		// 再只推进一次游标并给出下一步预览。
		if err := s.runs.BackFromReconciliation(ctx, pathRunID,
			"对账判定上一次写已生效，确认并前进到下一步"); err != nil {
			return err
		}
		s.mu.Lock()
		nextIndex := session.nextIndex + 1
		session.nextIndex = nextIndex
		session.version++
		s.mu.Unlock()
		preview, finished, previewErr := s.steps.BuildPreview(ctx, session.runCtx, nextIndex)
		if previewErr != nil {
			return previewErr
		}
		s.recoveryLog.LogFact(pathRunID, fmt.Sprintf("恢复动作=advance 游标推进到第 %d 个步骤（只推进一次）", nextIndex+1))
		s.mu.Lock()
		if finished {
			session.finished = true
			s.mu.Unlock()
			facts, reviewErr := s.steps.FinalReview(ctx, session.runCtx)
			if reviewErr != nil {
				return reviewErr
			}
			return s.finishCompleted(ctx, pathRunID, session.runCtx, facts)
		}
		session.preview = preview
		session.reconcile = nil
		s.mu.Unlock()
		return nil
	case reconcile.ActionReplay:
		// 重放这一步：新的一次尝试（attempt 递增），重新走七阶段、重新过门禁。
		// 上限走默认配置（1 次），超限只能登记人工结论；判断用最新对账视图，不用可能已过期的旧视图。
		if fresh.ReplaysUsed >= fresh.ReplaysMax {
			return fmt.Errorf("重放次数已达上限（%d 次），只能登记人工核对结论并结束", fresh.ReplaysMax)
		}
		// 尝试号取库里最近一次尝试 + 1：重放是同一步的下一次尝试，
		// 用会话里的重放计数推算在恢复后（现场重建）会算错。
		nextAttempt := 2
		if _, lastAttempt, err := s.store.LatestStepAttempt(ctx, pathRunID); err == nil {
			nextAttempt = lastAttempt.AttemptNo + 1
		}
		// 重放前必须先回到运行中：租约领取只认 waiting/running/verifying/paused，
		// 停在待对账上直接执行会在阶段 4 领取推进权时被状态冲突拒绝。
		if err := s.runs.BackFromReconciliation(ctx, pathRunID,
			fmt.Sprintf("对账判定上一次写未生效，重放本步（第 %d 次尝试）", nextAttempt)); err != nil {
			return err
		}
		s.mu.Lock()
		session.replaysUsed++
		session.version++
		currentIndex := session.nextIndex
		s.mu.Unlock()
		// 重放是一次完整的新尝试，不是重发：预览必须重建，让 plan/gate/prepare 用此刻的真实事实
		// 重新算一遍（门禁此刻不通过就停止，绝不拿旧快照硬发）。
		preview, finished, previewErr := s.steps.BuildPreview(ctx, session.runCtx, currentIndex)
		if previewErr != nil {
			return previewErr
		}
		if finished || preview == nil {
			return fmt.Errorf("重放失败：本步已不在编译场景内，请重新启动一次运行")
		}
		s.mu.Lock()
		session.preview = preview
		session.reconcile = nil
		s.mu.Unlock()
		_ = s.store.AppendRunEvent(ctx, model.RunEvent{
			RunID: pathRun.RunID, PathRunID: &pathRunID,
			Kind: "replayed", Label: fmt.Sprintf("对账判定未生效，已按新尝试（第 %d 次）重放本步", nextAttempt),
		}, s.now())
		s.recoveryLog.LogFact(pathRunID, fmt.Sprintf("恢复动作=replay attempt=%d 重放前已重新对账并重过门禁", nextAttempt))
		if _, err := s.approveOneStep(ctx, pathRunID, session, nextAttempt, true); err != nil {
			return err
		}
		return nil
	case reconcile.ActionManualEnd:
		// 登记人工核对结论并结束：人工事实 append-only 落库，路径进入终态。
		conclusion := model.RunManualConclusion{
			RunID: pathRun.RunID, PathRunID: pathRunID,
			StepNo:         session.preview.StepNo,
			InstanceStatus: manual.InstanceStatus, CurrentNode: manual.CurrentNode,
			Note: manual.Note, Reporter: manual.Reporter,
		}
		if err := s.store.AppendManualConclusion(ctx, conclusion, s.now()); err != nil {
			return err
		}
		_ = s.store.AppendRunEvent(ctx, model.RunEvent{
			RunID: pathRun.RunID, PathRunID: &pathRunID,
			Kind: "manual_conclusion", Label: "已登记人工核对结论，路径运行结束",
		}, s.now())
		// 路径运行按设计留在待对账（那本身就是它的结论），但运行聚合必须收尾：
		// 待对账时运行故意保持"运行中"是为了把出路留给对账；人工结论一登记就再没有后续动作了，
		// 运行还挂在"运行中"会让运行列表长期显示一个不会再动的陈旧状态（纲领第 12 节禁止陈旧文案）。
		// 收尾取"已停止"：场景没有走完，成功与失败都不成立，而已发生的事实全部保留。
		if _, err := s.store.AdvanceRunStatus(ctx, pathRun.RunID,
			model.RunStatusRunning, model.RunStatusStopped, model.RunEvent{
				Kind:  "run_finished",
				Label: "写结果不确定且已登记人工核对结论，运行结束（已发生的事实全部保留）",
			}, s.now()); err != nil {
			return err
		}
		s.recoveryLog.LogFact(pathRunID, fmt.Sprintf(
			"恢复动作=manual_end 人工登记：实例状态=%s 当前节点=%s 登记人=%s；路径运行留在待对账并结束",
			conclusion.InstanceStatus, conclusion.CurrentNode, conclusion.Reporter))
		s.clear(pathRunID)
		return nil
	default:
		return fmt.Errorf("未知的恢复动作：%s", string(action))
	}
}

// verdictName 对账结论中文名。
func verdictName(verdict reconcile.Verdict) string {
	switch verdict {
	case reconcile.VerdictEffective:
		return "已生效"
	case reconcile.VerdictNotEffective:
		return "未生效"
	default:
		return "仍无法判定"
	}
}

// 防止未使用导入（ManualConclusion 时间字段由仓储层填充）。
var _ = time.Now
var _ = errors.New
var _ = json.Marshal
