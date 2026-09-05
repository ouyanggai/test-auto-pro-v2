// 待对账现场的重建：把停在待对账的路径运行按已落库的运行事实重新装配成控制现场。
// 边界：只重建、不推进。重建过程只读目标（plan/gate 阶段的重读），绝不发写请求；
// 重建出来的现场只提供对账与对账给出的唯一动作，不提供放行类命令。
package control

import (
	"context"
	"fmt"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
)

// HasSession 判断这条路径运行此刻是否还有内存现场。
// 调用方据此决定要不要按运行事实重建（重建要读目标，能省就省）。
func (s *Service) HasSession(pathRunID uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active[pathRunID] != nil
}

// Rehydrate 按已落库的运行事实重建待对账路径运行的控制现场。
//
// 为什么需要它：对账与三个恢复动作的输入（本步预览、步骤游标、写之前的基准事实、已用重放次数）
// 原本只存在于本进程内存里，进程一重启就全丢，停在待对账的路径运行再也没有出路。
// 现在这些事实都在库里：步骤游标来自 run_steps，基准事实来自 run_step_attempts.before_facts，
// 已用重放次数按本步 is_replay 的尝试数还原，断点由控制事实回放，模式取自 runs。
//
// runCtx 由调用方装配（计划、路径配置、编译场景、节点真实标识都在应用服务层），
// 本方法只负责补齐运行身份、游标与基准，并重新构建一次本步预览（重新过门禁）。
// 只接受待对账：其他状态要么现场还在，要么已经真正结束，重建没有意义也不安全。
func (s *Service) Rehydrate(ctx context.Context, runCtx step.RunContext) error {
	pathRunID := runCtx.PathRun.ID
	if pathRunID == 0 {
		return fmt.Errorf("重建控制现场缺少路径运行标识")
	}
	if s.HasSession(pathRunID) {
		return nil
	}
	pathRun, err := s.runs.GetPathRun(ctx, pathRunID)
	if err != nil {
		return err
	}
	if !model.CanRecoverPathRunStatus(pathRun.Status) {
		return fmt.Errorf("路径运行当前为 %s，不需要也不允许重建对账现场",
			model.PathRunStatusName(pathRun.Status))
	}
	runAggregate, err := s.runs.GetRun(ctx, pathRun.RunID)
	if err != nil {
		return err
	}
	runCtx.Run = runAggregate
	runCtx.PathRun = pathRun

	steps, err := s.store.ListRunSteps(ctx, pathRunID)
	if err != nil {
		return err
	}
	attempts, err := s.store.ListRunAttempts(ctx, pathRunID)
	if err != nil {
		return err
	}
	cursor, baseline, replaysUsed, err := rebuildCursor(runCtx, steps, attempts)
	if err != nil {
		return err
	}
	runCtx.LastBeforeFacts, runCtx.LastBeforeFactsKnown = baseline.facts, baseline.known

	// 重新构建本步预览：plan 与 gate 阶段都用此刻的真实事实重算一遍。
	// 这一步只读，但会登录目标，因此重建放在真正需要现场的入口（对账 / 恢复动作）上按需触发。
	preview, finished, err := s.steps.BuildPreview(ctx, runCtx, cursor)
	if err != nil {
		return err
	}
	if finished || preview == nil {
		return fmt.Errorf("编译场景里已找不到第 %d 个步骤，无法重建对账现场；请新建一次运行", cursor+1)
	}

	controls, err := s.store.ListRunControls(ctx, pathRunID)
	if err != nil {
		return err
	}
	session := &activeStep{
		runCtx:      runCtx,
		preview:     preview,
		nextIndex:   cursor,
		mode:        runAggregate.Mode,
		breakpoints: ReplayBreakpoints(controls),
		version:     1,
		replaysUsed: replaysUsed,
		recoveryLog: s.recoveryLog,
		// 重建出来的现场天生就停在待对账：可用命令必须是空集，不给任何放行入口。
		awaitingReconciliation: true,
		// 重建出来的现场明说自己是重建的：用户要能看懂"为什么这里只有对账"。
		stopReason:       rehydrateStopReason(baseline.known),
		executedStepNos:  map[int]bool{},
		executedNodeKeys: map[string]bool{},
	}
	for _, record := range steps {
		session.executedStepNos[record.StepNo] = true
		session.executedNodeKeys[record.NodeKey] = true
	}

	s.mu.Lock()
	if existing := s.active[pathRunID]; existing != nil {
		// 并发重建：先到的现场为准，避免两份现场各自持有不同游标。
		s.mu.Unlock()
		return nil
	}
	s.active[pathRunID] = session
	s.mu.Unlock()
	s.recoveryLog.LogFact(pathRunID, fmt.Sprintf(
		"现场已按运行事实重建：步骤=%d 已用重放=%d 写前基准=%s", preview.StepNo, replaysUsed, baselineName(baseline.known)))
	return nil
}

// rehydrateStopReason 给出重建现场的中文停摆原因。
func rehydrateStopReason(baselineKnown bool) string {
	reason := "服务重启后现场已按运行事实重建：这条路径运行停在待对账，只提供只读对账与对账给出的唯一动作。"
	if !baselineKnown {
		reason += "写之前的目标事实基准没有落库（该次尝试早于基准落库版本，或崩溃在落账之前），因此对账只会得出仍无法判定。"
	}
	return reason
}

// baselineName 给出基准事实的中文有无说明，进 recovery.log。
func baselineName(known bool) string {
	if known {
		return "已还原"
	}
	return "缺失（对账按证据缺失降级）"
}

// baseline 是写之前的目标事实基准及其是否真的取到。
type baseline struct {
	facts step.InstanceFacts
	known bool
}

// rebuildCursor 从已落账的步骤与尝试事实还原步骤游标、写前基准与本步已用重放次数。
//
// 判据（run_steps 与 run_step_attempts 在同一事务插入，因此每个步骤行必有尝试行）：
//   - 最近一步的最近一次尝试判定为「不确定」→ 这一步就是停在待对账的那一步，游标指向它；
//   - 否则说明是崩溃恢复（进程在落账之前就没了，那一步根本没有行）→ 在飞的是下一步，游标指向下一步；
//   - 一行都没有 → 从第一步开始。
//
// 已用重放次数只数本步的重放尝试：重放配额是"重放这一步最多几次"，不是整条路径的总数。
func rebuildCursor(runCtx step.RunContext, steps []model.RunStep, attempts []model.RunStepAttempt) (int, baseline, int, error) {
	if len(steps) == 0 {
		return 0, baseline{}, 0, nil
	}
	last := steps[0]
	for _, record := range steps {
		if record.StepNo >= last.StepNo {
			last = record
		}
	}
	lastAttempt := model.RunStepAttempt{}
	replaysUsed := 0
	for _, attempt := range attempts {
		if attempt.StepID != last.ID {
			continue
		}
		if attempt.AttemptNo >= lastAttempt.AttemptNo {
			lastAttempt = attempt
		}
		if attempt.IsReplay {
			replaysUsed++
		}
	}
	targetStepNo := last.StepNo
	uncertain := lastAttempt.Verdict == "uncertain"
	if !uncertain {
		// 崩溃恢复：最近一步已经有结论了，在飞的是它的下一步，那一步没有任何落账事实。
		targetStepNo = last.StepNo + 1
		lastAttempt = model.RunStepAttempt{}
		replaysUsed = 0
	}
	for index, compiled := range runCtx.Steps {
		if compiled.Sequence != targetStepNo {
			continue
		}
		facts, known := step.DecodeInstanceFacts(lastAttempt.BeforeFacts)
		return index, baseline{facts: facts, known: known}, replaysUsed, nil
	}
	return 0, baseline{}, 0, fmt.Errorf("编译场景里没有第 %d 步，无法重建对账现场；请新建一次运行", targetStepNo)
}
