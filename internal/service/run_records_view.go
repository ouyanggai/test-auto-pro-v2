package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// 本文件装配 2026-09-06 交付验收的运行记录层级：
// 运行列表一行只对应一次计划运行（跨计划）、二级「本次运行的执行路径」页与整次运行删除。
// 准确进度的分母是启动时冻结的总步骤数（path_runs.total_steps），分子是已落库且确定成功的步骤；
// 完成才是 100%，失败或停止保留实际进度，不按时间估算、不用假进度。

// RunPathsDTO 是二级路径页的数据主体：一次运行的身份信息与它全部路径运行的准确进度。
type RunPathsDTO struct {
	RunID            uint64               `json:"runId"`
	RunNo            uint64               `json:"runNo"`
	ModeName         string               `json:"modeName"`
	RunStatusName    string               `json:"runStatusName"`
	ResultName       string               `json:"resultName,omitempty"`
	ScheduleName     string               `json:"scheduleName,omitempty"`
	ConcurrencyLabel string               `json:"concurrencyLabel,omitempty"`
	PlanID           uint64               `json:"planId"`
	PlanName         string               `json:"planName"`
	StartedAt        *time.Time           `json:"startedAt,omitempty"`
	FinishedAt       *time.Time           `json:"finishedAt,omitempty"`
	Paths            []RunPathProgressDTO `json:"paths"`
}

// RunPathProgressDTO 是一次运行里一条执行路径的进度事实：状态、当前节点与准确进度。
type RunPathProgressDTO struct {
	PathRunID        uint64     `json:"pathRunId"`
	PathID           uint64     `json:"pathId"`
	PathName         string     `json:"pathName"`
	StatusName       string     `json:"statusName"`
	ResultName       string     `json:"resultName,omitempty"`
	FailureClassName string     `json:"failureClassName,omitempty"`
	CurrentNodeName  string     `json:"currentNodeName,omitempty"`
	DoneSteps        int        `json:"doneSteps"`
	TotalSteps       int        `json:"totalSteps"`
	ProgressPercent  int        `json:"progressPercent"`
	StartedAt        *time.Time `json:"startedAt,omitempty"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
	MainInstanceRef  string     `json:"mainInstanceRef,omitempty"`
}

// ListAllRuns 跨计划列出运行（最新在前）：一行只对应一次计划运行，
// 计划名称一次查询批量补齐，不逐行查库。
func (s *RunOrchestrationService) ListAllRuns(ctx context.Context, status string) ([]RunSummaryDTO, error) {
	runs, err := s.store.ListAllRunsFiltered(ctx, status, 0, 100)
	if err != nil {
		return nil, err
	}
	planNames := map[uint64]string{}
	if plans, listErr := s.plans.List(ctx, "", ""); listErr == nil {
		for _, plan := range plans {
			planNames[plan.ID] = plan.Name
		}
	}
	// 计划间串行排队中的运行：状态显示为「排队中」，与普通等待启动区分，让用户知道为什么没跑。
	queued := map[uint64]bool{}
	if queuedIDs, qErr := s.store.ListQueuedRunIDs(ctx); qErr == nil {
		for _, id := range queuedIDs {
			queued[id] = true
		}
	}
	items := make([]RunSummaryDTO, 0, len(runs))
	for _, run := range runs {
		statusName := model.RunStatusName(run.Status)
		if run.Status == model.RunStatusPending && queued[run.ID] {
			statusName = "排队中"
		}
		item := RunSummaryDTO{
			RunID:      run.ID,
			RunNo:      run.RunNo,
			ModeName:   model.RunModeName(run.Mode),
			StatusName: statusName,
			StartedAt:  run.StartedAt,
			FinishedAt: run.FinishedAt,
			PlanID:     run.PlanID,
			PlanName:   planNames[run.PlanID],
		}
		if run.Result != nil {
			item.ResultName = resultName(*run.Result)
		}
		pathRuns, err := s.store.ListPathRunsByRun(ctx, run.ID)
		if err == nil && len(pathRuns) > 0 {
			item.PathRunID = pathRuns[0].ID
			item.PathRunStatusName = model.PathRunStatusName(pathRuns[0].Status)
			item.PathRunCount = len(pathRuns)
			item.ScheduleName = runScheduleName(run)
			item.PathsSummary = runPathsSummary(pathRuns)
		}
		items = append(items, item)
	}
	return items, nil
}

// RunPaths 读取一次运行的二级路径页数据：每条路径显示名称、状态、当前节点、已完成/总步骤与准确进度。
// 节点名称来自真实结构投影；结构读取失败时如实降级为空名称，绝不阻塞运行事实展示。
func (s *RunOrchestrationService) RunPaths(ctx context.Context, runID uint64) (*RunPathsDTO, error) {
	run, err := s.store.GetRun(ctx, runID)
	if err != nil {
		if errors.Is(err, repository.ErrRunNotFound) {
			return nil, &RunOrchestrationError{Kind: RunOrchestrationNotFound, Message: "任务不存在"}
		}
		return nil, err
	}
	plan, err := s.plans.Get(ctx, run.PlanID)
	if err != nil {
		return nil, mapPlanError(err)
	}
	graph, graphErr := s.graphs.Get(ctx, run.PlanID)
	if graphErr != nil {
		graph = model.FlowGraph{PlanID: run.PlanID}
	}
	pathRuns, err := s.store.ListPathRunsByRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	tokenToGraphID := tokenToGraphNodeID(graph)
	nodeNames := nodeNamesOf(graph)
	dto := &RunPathsDTO{
		RunID:            run.ID,
		RunNo:            run.RunNo,
		ModeName:         model.RunModeName(run.Mode),
		RunStatusName:    s.runStatusNameOf(ctx, run),
		ScheduleName:     runScheduleName(run),
		ConcurrencyLabel: runConcurrencyLabel(run),
		PlanID:           run.PlanID,
		PlanName:         plan.Name,
		StartedAt:        run.StartedAt,
		FinishedAt:       run.FinishedAt,
		Paths:            make([]RunPathProgressDTO, 0, len(pathRuns)),
	}
	if run.Result != nil {
		dto.ResultName = resultName(*run.Result)
	}
	for _, pathRun := range pathRuns {
		summary := RunPathProgressDTO{
			PathRunID:       pathRun.ID,
			PathID:          pathRun.ExecutionPathID,
			PathName:        pathNameOf(ctx, s.paths, run.PlanID, pathRun.ExecutionPathID, plan.Name),
			StatusName:      model.PathRunStatusName(pathRun.Status),
			StartedAt:       pathRun.StartedAt,
			FinishedAt:      pathRun.FinishedAt,
			MainInstanceRef: pathRun.MainInstanceRef,
		}
		if pathRun.Result != nil {
			summary.ResultName = resultName(*pathRun.Result)
		}
		if pathRun.FailureClass != nil {
			summary.FailureClassName = model.FailureClassName(*pathRun.FailureClass)
		}
		steps, err := s.store.ListRunSteps(ctx, pathRun.ID)
		if err != nil {
			return nil, err
		}
		doneSteps := 0
		lastNodeKey := ""
		for _, stepRecord := range steps {
			// 目标自动跳过的步骤没有写动作，但确实已被流程越过：计入已完成进度。
			if stepRecord.Status == model.RunStepSucceeded || stepRecord.Status == model.RunStepSkipped {
				doneSteps++
			}
			lastNodeKey = stepRecord.NodeKey
		}
		summary.DoneSteps = doneSteps
		summary.TotalSteps = s.frozenTotalSteps(ctx, run, pathRun)
		if summary.TotalSteps > 0 {
			// 进度只由已落库步骤与冻结分母得出：完成才是 100%，失败或停止保留实际进度。
			summary.ProgressPercent = doneSteps * 100 / summary.TotalSteps
		}
		summary.CurrentNodeName = currentNodeNameOf(pathRun, previewNodeName(s.control.CurrentPreview(pathRun.ID)), lastNodeKey, tokenToGraphID, nodeNames)
		dto.Paths = append(dto.Paths, summary)
	}
	return dto, nil
}

// frozenTotalSteps 返回本次运行冻结的总步骤数；历史运行（迁移 031 之前）没有冻结值，
// 退化为按当前已保存编译场景长度估算，并在无法取得时如实给 0（界面显示「—」），绝不编造。
func (s *RunOrchestrationService) frozenTotalSteps(ctx context.Context, run model.Run, pathRun model.PathRun) int {
	if pathRun.TotalSteps != nil {
		return *pathRun.TotalSteps
	}
	config, found, err := s.configs.GetPathConfig(ctx, pathRun.ExecutionPathID)
	if err != nil || !found || len(config.CompiledSteps) == 0 {
		return 0
	}
	compiledSteps := []model.CompiledActionStep{}
	if err := json.Unmarshal(config.CompiledSteps, &compiledSteps); err != nil {
		return 0
	}
	return len(compiledSteps)
}

// previewNodeName 取当前步预览里的节点业务名称；没有预览时为空。
func previewNodeName(preview *step.StepPreview) string {
	if preview == nil {
		return ""
	}
	return preview.NodeName
}

// currentNodeNameOf 推导路径当前所在节点的业务名称：
// 运行中/核验中优先用实时预览；否则用最后一步落账节点（令牌键翻译为图节点 ID 再取中文名）。
func currentNodeNameOf(pathRun model.PathRun, previewName string, lastNodeKey string, tokenToGraphID map[string]string, nodeNames map[string]string) string {
	if pathRun.Status == model.PathRunStatusRunning || pathRun.Status == model.PathRunStatusVerifying {
		if previewName != "" {
			return previewName
		}
	}
	if lastNodeKey == "" {
		return ""
	}
	if graphID := tokenToGraphID[lastNodeKey]; graphID != "" {
		return nodeNames[graphID]
	}
	return ""
}

// DeleteRun 删除整次工具侧运行及其路径、步骤、尝试、事件与控制记录；
// 绝不删除目标平台实例或业务数据。运行中的记录必须先停止再删除（仓储层同事务守卫）。
func (s *RunOrchestrationService) DeleteRun(ctx context.Context, runID uint64) error {
	return s.store.DeleteRun(ctx, runID, s.now())
}

// runStatusNameOf 返回运行状态的中文显示名：计划间串行队列中的等待运行显示为「排队中」，
// 与普通等待启动区分，让用户在路径页与详情页都能一眼看出为什么还没跑。
func (s *RunOrchestrationService) runStatusNameOf(ctx context.Context, run model.Run) string {
	if run.Status != model.RunStatusPending {
		return model.RunStatusName(run.Status)
	}
	if queued, err := s.store.ListQueuedRunIDs(ctx); err == nil {
		for _, id := range queued {
			if id == run.ID {
				return "排队中"
			}
		}
	}
	return model.RunStatusName(run.Status)
}
