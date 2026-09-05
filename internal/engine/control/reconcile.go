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
	// 只读收集：发起前实例不存在的基准（审批类动作用已有实例基准，由执行器事实提供）。
	facts, err := s.steps.ReconcileFacts(ctx, runCtx, preview.StepNo)
	if err != nil {
		return nil, err
	}
	input := reconcile.Collect(reconcile.FactInput{
		StepNodeKey:       preview.NodeKey,
		BeforeStatus:      facts.BeforeStatus,
		BeforeHadInstance: facts.BeforeHadInstance,
		NowFound:          facts.NowFound,
		NowStatus:         facts.NowStatus,
		NowCurrentNodes:   facts.NowCurrentNodes,
		NowDueNodes:       facts.NowDueNodes,
		NowReadError:      facts.NowReadError,
		FormChanged:       false,
	})
	result := reconcile.Reconcile(input)

	view := &ReconcileResultView{
		Verdict: string(result.Verdict), VerdictName: verdictName(result.Verdict),
		Action: string(result.Action), Headline: result.Headline, Reasons: result.Reasons,
		ReplaysMax: defaultReplayLimit,
	}
	s.mu.Lock()
	session.reconcile = view
	s.mu.Unlock()

	// 对账三列写回最近一次尝试行（纲领第 7.2 节：这三列归属 run_step_attempts），
	// 并追加中文事件与 recovery.log。
	lastStep, lastAttempt, err := s.store.LatestStepAttempt(ctx, pathRunID)
	if err == nil {
		if err := s.store.RecordReconcileOutcome(ctx, lastAttempt.ID, string(result.Verdict), string(result.Action), false, s.now()); err != nil {
			return view, err
		}
		_ = s.store.AppendRunEvent(ctx, model.RunEvent{
			RunID: lastStep.PathRunID, PathRunID: &pathRunID,
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
	if model.IsTerminalPathRunStatus(pathRun.Status) {
		return ErrRunAlreadyFinished
	}
	// 动作与结论必须一一对应，其他组合一律拒绝。
	if (action == reconcile.ActionAdvance && view.Verdict != string(reconcile.VerdictEffective)) ||
		(action == reconcile.ActionReplay && view.Verdict != string(reconcile.VerdictNotEffective)) ||
		(action == reconcile.ActionManualEnd && view.Verdict != string(reconcile.VerdictIndeterminate)) {
		return fmt.Errorf("%w：动作 %s 与对账结论 %s 不匹配", ErrCommandNotAllowed, string(action), view.Verdict)
	}
	// 执行前必须再次只读对账（不能沿用旧结论）。
	fresh, err := s.ReconcileNow(ctx, pathRunID)
	if err != nil {
		return err
	}
	if fresh.Verdict != view.Verdict {
		return fmt.Errorf("重新对账后结论已变化，请以最新结论为准")
	}

	switch action {
	case reconcile.ActionAdvance:
		// 确认并前进：只推进一次游标，回到运行中并给出下一步预览。
		s.mu.Lock()
		nextIndex := session.nextIndex + 1
		session.nextIndex = nextIndex
		session.version++
		s.mu.Unlock()
		preview, finished, previewErr := s.steps.BuildPreview(ctx, session.runCtx, nextIndex)
		if previewErr != nil {
			return previewErr
		}
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
		// 上限走默认配置（1 次），超限只能登记人工结论。
		if view.ReplaysUsed >= view.ReplaysMax {
			return fmt.Errorf("重放次数已达上限（%d 次），只能登记人工核对结论并结束", view.ReplaysMax)
		}
		s.mu.Lock()
		session.replaysUsed++
		session.version++
		s.mu.Unlock()
		// 重放前路径必须回到运行中（从待对账回到运行中由状态机允许的迁移表达：待对账是终态，
		// 因此重放走“新建一次尝试”的路径，状态保持待对账终态下的新尝试由调用方以新运行承接）。
		if _, err := s.approveOneStep(ctx, pathRunID, session, view.ReplaysUsed+2); err != nil {
			return err
		}
		_ = s.store.AppendRunEvent(ctx, model.RunEvent{
			RunID: pathRun.RunID, PathRunID: &pathRunID,
			Kind: "replayed", Label: fmt.Sprintf("对账判定未生效，已按新尝试（第 %d 次）重放本步", view.ReplaysUsed+1),
		}, s.now())
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
