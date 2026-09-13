package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/engine/control"
	"test-auto-pro-v2/internal/engine/run"
	"test-auto-pro-v2/internal/engine/scenario"
	"test-auto-pro-v2/internal/engine/step"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
)

// readinessReader 是启动前复验运行准备结论的最小依赖（F-015 服务满足）。
type readinessReader interface {
	PlanReadiness(ctx context.Context, planID uint64, pathIDs []uint64) (model.PlanRunReadiness, error)
}

// runGraphReader 提供真实流程结构投影。
type runGraphReader interface {
	Get(ctx context.Context, planID uint64) (model.FlowGraph, error)
}

// runDetailGraphTimeout 限制详情轮询等待结构投影的最长时间。
// 目标模板树的正常读取已实测超过 500ms；预算过短会让轮询把仍在返回的正常请求误判为失败。
const runDetailGraphTimeout = 2 * time.Second

// RunOrchestrationErrorKind 是运行编排服务的错误种类，API 层映射为稳定状态码。
type RunOrchestrationErrorKind string

const (
	RunOrchestrationNotFound RunOrchestrationErrorKind = "not_found"
	RunOrchestrationConflict RunOrchestrationErrorKind = "conflict"
	RunOrchestrationStorage  RunOrchestrationErrorKind = "storage"
	RunOrchestrationInvalid  RunOrchestrationErrorKind = "invalid"
)

// mapPlanError 把计划存储错误映射为运行编排的稳定错误：不存在与存储故障严格分开。
func mapPlanError(err error) error {
	switch {
	case IsPlanErrorKind(err, PlanErrorNotFound):
		return &RunOrchestrationError{Kind: RunOrchestrationNotFound, Message: "计划不存在"}
	case IsPlanErrorKind(err, PlanErrorInvalidArgument):
		return &RunOrchestrationError{Kind: RunOrchestrationInvalid, Message: "计划 ID 不正确"}
	default:
		return &RunOrchestrationError{Kind: RunOrchestrationStorage, Message: "暂时无法读取计划，请重试"}
	}
}

// RunOrchestrationError 携带中文结论与错误种类。
type RunOrchestrationError struct {
	Kind    RunOrchestrationErrorKind
	Message string
}

// Error 返回中文结论。
func (e *RunOrchestrationError) Error() string {
	return e.Message
}

// RunOrchestrationService 是运行主线的应用服务：启动、详情、放行、停止、列表。
// 它负责装配执行上下文（计划、路径、结构、场景与数据）并复验运行准备结论；
// 目标交互与状态机分别交给 engine/step 与 engine/run、engine/control。
type RunOrchestrationService struct {
	plans     *PlanService
	paths     repository.ExecutionPathRepository
	graphs    runGraphReader
	configs   repository.HistoryPathConfigStore
	readiness readinessReader
	control   *control.Service
	store     repository.RunStore
	// runState 提供运行级状态推进与收尾（F-020 调度与聚合收尾需要）。
	runState  *run.Service
	router    *logging.Router
	runConfig config.RunConfig
	now       func() time.Time
	// pathNodes 提供路径配置快照的目标节点表（键=编译场景 nodeKey 的同一套键）。
	pathNodes *PathConfigService
}

// NewRunOrchestrationService 组装运行编排服务；router 用于读取 step.log 的阶段耗时。
func NewRunOrchestrationService(
	plans *PlanService,
	paths repository.ExecutionPathRepository,
	graphs runGraphReader,
	configs repository.HistoryPathConfigStore,
	readiness readinessReader,
	controlSvc *control.Service,
	store repository.RunStore,
	runState *run.Service,
	router *logging.Router,
	runConfig config.RunConfig,
	pathNodes *PathConfigService,
	now func() time.Time,
) *RunOrchestrationService {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &RunOrchestrationService{
		plans: plans, paths: paths, graphs: graphs, configs: configs,
		readiness: readiness, control: controlSvc, store: store, runState: runState,
		router: router, runConfig: runConfig, pathNodes: pathNodes, now: now,
	}
}

// StartRunInput 是启动一次运行的最小请求。
type StartRunInput struct {
	PlanID          uint64 `json:"planId"`
	ExecutionPathID uint64 `json:"executionPathId"`
}

// RunPreviewDTO 是下一步预览的公开形态：中文为主，不含会话等目标敏感信息。
type RunPreviewDTO struct {
	StepNo     int `json:"stepNo"`
	TotalSteps int `json:"totalSteps"`
	// ReleaseGroup 与 ReleaseRequired 供人工模式按动作组展示放行边界。
	ReleaseGroup    string `json:"releaseGroup,omitempty"`
	ReleaseRequired bool   `json:"releaseRequired,omitempty"`
	Action          string `json:"action"`
	ActionName      string `json:"actionName"`
	NodeKey         string `json:"nodeKey"`
	// NodeID 是当前步节点的图上标识：画布据此平移与高亮当前步（与 nodeKey 是两套键空间）。
	NodeID         string                     `json:"nodeId,omitempty"`
	NodeName       string                     `json:"nodeName"`
	ActorName      string                     `json:"actorName"`
	ExpectedEffect string                     `json:"expectedEffect"`
	Endpoint       string                     `json:"endpoint"`
	RequestPreview string                     `json:"requestPreview"`
	GateAllowed    bool                       `json:"gateAllowed"`
	GateReason     string                     `json:"gateReason,omitempty"`
	GateItems      []model.ActionPrecondition `json:"gateItems"`
	Facts          map[string]any             `json:"facts"`
	BlockReason    string                     `json:"blockReason,omitempty"`
}

// RunSummaryDTO 是运行列表条目：一行只对应一次计划运行（跨计划列表，2026-09-06）。
type RunSummaryDTO struct {
	RunID      uint64     `json:"runId"`
	RunNo      uint64     `json:"runNo"`
	ModeName   string     `json:"modeName"`
	StatusName string     `json:"statusName"`
	ResultName string     `json:"resultName,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	// PlanID 与 PlanName 标明这次运行属于哪个计划。
	PlanID   uint64 `json:"planId"`
	PlanName string `json:"planName,omitempty"`
	// 所属第一条路径运行的摘要（单路径运行即该路径本身的状态）。
	PathRunID         uint64 `json:"pathRunId"`
	PathRunStatusName string `json:"pathRunStatusName"`
	// 运行级摘要（F-020）：调度方式、路径总数与中文汇总（如「3 条路径：2 已完成、1 失败」）。
	ScheduleName string `json:"scheduleName,omitempty"`
	PathsSummary string `json:"pathsSummary,omitempty"`
	PathRunCount int    `json:"pathRunCount"`
}

// RunStepAttemptDTO 是一次尝试的公开事实。
type RunStepAttemptDTO struct {
	AttemptNo   int    `json:"attemptNo"`
	VerdictName string `json:"verdictName"`
	Reason      string `json:"reason"`
	Basis       string `json:"basis"`
	TraceID     string `json:"traceId"`
	DurationMs  int64  `json:"durationMs"`
	// LogPath 与 LogLine 让界面每一行都能落到 step.log 的具体行（记录到日志可达）。
	LogPath string `json:"logPath"`
	LogLine uint64 `json:"logLine"`
	// CurlBlock 是该次尝试在 curl.log 里的完整可重放命令与响应正文块，与日志文件同源。
	CurlBlock string `json:"curlBlock,omitempty"`
	// Requests 是本次尝试的真实目标请求明细（F-030/T04，来自 network.log 传输层计时）。
	Requests []RunRequestDTO `json:"requests,omitempty"`
	// RequestSummary 是请求汇总指标：总耗时、写耗时、次数（写单独统计）。
	RequestSummary *runRequestSummaryDTO `json:"requestSummary,omitempty"`
	// PhaseDurations 是七个阶段各自的耗时（毫秒），来自 step.log 的阶段时间轴。
	PhaseDurations     map[string]int64 `json:"phaseDurations,omitempty"`
	PhaseDurationsNote string           `json:"phaseDurationsNote,omitempty"`
	// IsReplay 保留为只读历史事实：用户侧重放已于 2026-09-06 移除，新运行的尝试恒为 false。
	IsReplay bool `json:"isReplay"`
}

// RunStepDTO 是一个已落账步骤的公开事实。
type RunStepDTO struct {
	StepNo     int    `json:"stepNo"`
	ActionName string `json:"actionName"`
	// Action 是稳定动作键（F-034 评审）：前端据此走集中中文映射，不从中文名反推动作类型。
	Action  string `json:"action,omitempty"`
	NodeKey string `json:"nodeKey"`
	// ReleaseGroup/ReleaseRequired 标明本步在动作组中的位置；执行事实本身不带动作组。
	ReleaseGroup    string `json:"releaseGroup,omitempty"`
	ReleaseRequired bool   `json:"releaseRequired,omitempty"`
	// NodeID 是该节点在图上的真实标识：画布与侧栏按它取运行状态与步骤（与 nodeKey 是两套键空间）。
	NodeID     string    `json:"nodeId,omitempty"`
	NodeName   string    `json:"nodeName"`
	ActorName  string    `json:"actorName"`
	StatusName string    `json:"statusName"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	DurationMs int64     `json:"durationMs"`
	// GateSnapshot 是放行时的门禁结论快照（逐项中文条件 JSON），侧栏据此还原当时的门禁判定。
	GateSnapshot string              `json:"gateSnapshot,omitempty"`
	Attempts     []RunStepAttemptDTO `json:"attempts"`
}

// RunNodeStateDTO 是画布节点的运行态（纲领九个中文状态）。
type RunNodeStateDTO struct {
	Status     string `json:"status"`
	StatusName string `json:"statusName"`
}

// RunNodePlanActionDTO 是某个节点上的一条已配置计划动作（来源：编译场景）。
// 字段全部是中文事实：编译场景本身就用中文写了前置条件、预期效果与失败处理，
// 界面直接展示，不在前端翻译第二遍，也不输出动作键、作用范围枚举与演员策略等内部值。
type RunNodePlanActionDTO struct {
	Sequence int `json:"sequence"`
	// ReleaseGroup 与 ReleaseRequired 标识同一次人工放行包含的物理步骤。
	ReleaseGroup    string `json:"releaseGroup,omitempty"`
	ReleaseRequired bool   `json:"releaseRequired,omitempty"`
	ActionName      string `json:"actionName"`
	// Action 是稳定动作键（F-034 评审）：前端据此走集中中文映射，不从中文名反推动作类型。
	Action         string `json:"action,omitempty"`
	SourceName     string `json:"sourceName"`
	ScopeName      string `json:"scopeName"`
	Precondition   string `json:"precondition,omitempty"`
	ExpectedEffect string `json:"expectedEffect,omitempty"`
	StopOnFailure  string `json:"stopOnFailure,omitempty"`
	RecoveryPolicy string `json:"recoveryPolicy,omitempty"`
	ReloadRequired bool   `json:"reloadRequired"`
	// ParameterCount 只给动作参数的项数：参数键是目标字段名，属内部标识，不上界面。
	ParameterCount int `json:"parameterCount"`
}

// PathRunDetailDTO 是路径运行详情页的数据主体。
type PathRunDetailDTO struct {
	RunID             uint64     `json:"runId"`
	RunNo             uint64     `json:"runNo"`
	StartedAt         *time.Time `json:"startedAt,omitempty"`
	ModeName          string     `json:"modeName"`
	RunStatusName     string     `json:"runStatusName"`
	PathRunID         uint64     `json:"pathRunId"`
	PathRunStatus     string     `json:"pathRunStatus"`
	PathRunStatusName string     `json:"pathRunStatusName"`
	// 运行级信息（F-020）：调度方式、并发说明与全部路径运行摘要，供运行详情的路径切换区。
	RunScheduleName     string              `json:"runScheduleName,omitempty"`
	RunConcurrencyLabel string              `json:"runConcurrencyLabel,omitempty"`
	Paths               []RunPathSummaryDTO `json:"paths"`
	// Result 与 FinalTarget 是两件分开的事：路径结果只看步骤事实，最终目标事实如实描述目标现状。
	ResultName       string `json:"resultName,omitempty"`
	FailureClassName string `json:"failureClassName,omitempty"`
	// StopKind 是停止语义投影（F-034 T04）：blocked=目标在写入前明确拒绝（前置条件未满足，如手动分支
	// 未选择、未设置审批人），由后端按受控拒绝清单（尝试初判 pre_rejected）稳定派生；
	// 失败/结果待确认等其他语义不使用该字段，前端不得用错误文案猜测。
	StopKind     string `json:"stopKind,omitempty"`
	StopKindNote string `json:"stopKindNote,omitempty"`
	// StopStepNo 是触发本次阻塞/停止的步骤号（F-034 评审 #2）：前端只在展示该步骤详情时
	// 使用路径级阻塞信息，其他步骤用自身的执行状态，防止把第 3 步的阻塞显示到第 1 步。
	StopStepNo  int             `json:"stopStepNo,omitempty"`
	FinalTarget json.RawMessage `json:"finalTarget,omitempty"`
	PlanID      uint64          `json:"planId"`
	PlanName    string          `json:"planName"`
	PathID      uint64          `json:"pathId"`
	PathName    string          `json:"pathName"`
	// LogDir 是这次运行的路径运行日志目录（相对日志根），从页面身份（runId/pathRunId）直接算出，
	// 用户不必遍历全盘或按时间猜目录。
	LogDir string `json:"logDir,omitempty"`
	// InstanceID/InstanceName 是目标实例身份：实例名称只作业务信息，不参与日志目录寻址。
	// 名称读不到时 InstanceNameAvailable 为假并带原因，界面显示「实例名称不可用」，不用其他名字冒充。
	InstanceID            string                     `json:"instanceId,omitempty"`
	InstanceName          string                     `json:"instanceName,omitempty"`
	InstanceNameAvailable bool                       `json:"instanceNameAvailable"`
	InstanceNameNote      string                     `json:"instanceNameNote,omitempty"`
	Steps                 []RunStepDTO               `json:"steps"`
	CurrentPreview        *RunPreviewDTO             `json:"currentPreview,omitempty"`
	NodeStates            map[string]RunNodeStateDTO `json:"nodeStates"`
	// GraphError 原样携带结构读取失败的底层错误文案；为空表示结构读取正常。
	GraphError string `json:"graphError,omitempty"`
	// NodePlans 是按图节点 ID 索引的「本次运行在该节点上的已配置计划」：
	// 侧栏配置页签据此显示这个节点要做什么。数据只来自这条路径已保存的编译场景，
	// 不额外读目标平台——运行详情要看的是「本次运行执行的配置」，不是目标此刻的最新配置。
	NodePlans map[string][]RunNodePlanActionDTO `json:"nodePlans"`
	// PollIntervalMs 提示前端轮询间隔（来自配置），状态只在放行后变化。
	PollIntervalMs int64 `json:"pollIntervalMs"`
	// StaleAfterMs 是超过该时长仍无状态更新即视为疑似无响应的预算（来自配置）。
	StaleAfterMs int64 `json:"staleAfterMs"`

	// 控制现场（F-017）：生效断点、为什么停在这里、可用命令集合、条件写版本。
	// 这两个切片必须始终输出 JSON 数组而不是 null：Go 的 nil 切片会序列化成 null，
	// 前端模板按数组读取，null 会让整页渲染崩溃、永远停在加载态（实测运行 12 复现）。
	ControlVersion int64           `json:"controlVersion"`
	CurrentStepNo  int             `json:"currentStepNo"`
	Breakpoints    []BreakpointDTO `json:"breakpoints"`
	StopReason     string          `json:"stopReason,omitempty"`
	Commands       []CommandDTO    `json:"commands"`
	LoopRunning    bool            `json:"loopRunning"`
	StepInFlight   bool            `json:"stepInFlight"`
	StopRequested  bool            `json:"stopRequested"`
	PauseRequested bool            `json:"pauseRequested"`

	// SceneLost 表示这次运行的执行现场已经不在（服务重启或执行结果无法确认），
	// InterruptedNodeID/InterruptedNote 定位中断时正在执行、尚未落账的那一步，
	// 供节点面板把中断原因直接显示在那个节点上（否则该节点无任何步骤记录，点开一片空白）。
	SceneLost         bool   `json:"sceneLost"`
	SceneLostNote     string `json:"sceneLostNote,omitempty"`
	InterruptedNodeID string `json:"interruptedNodeId,omitempty"`
	InterruptedNote   string `json:"interruptedNote,omitempty"`
	// Retryable 表示服务端判定这条路径运行可以重试失败动作（F-028）：
	// 只有「步骤执行中确定失败（无目标副作用）」的运行可重试；结果待确认与启动阶段失败不在范围内。
	// 前端只按这个字段决定是否渲染重试按钮，不自行从状态推断。
	Retryable bool `json:"retryable"`

	// 模式切换（2026-09-06）：ModeSwitchPending 表示有待生效的切换（将在本步完成后生效），
	// PendingModeName 是目标模式的中文显示名。界面据此区分「请求已收到」与「已经生效」。
	ModeSwitchPending bool   `json:"modeSwitchPending"`
	PendingModeName   string `json:"pendingModeName,omitempty"`

	// PathChoices 是这条路径已保存的分支选择（分支节点 ID + 所选分支 ID），
	// 是画布遍历分析的直接输入，用于区分路径内/路径外节点（评审缺陷 8）。
	PathChoices []PathChoiceDTO `json:"pathChoices,omitempty"`
	// CurrentPhase/CurrentPhaseNote 是当前步实时阶段与中文补充，CurrentPhaseSince 是进入时刻；
	// 数据来自执行器的阶段上报，指示器据此推进（评审缺陷 7）。
	CurrentPhase      string    `json:"currentPhase,omitempty"`
	CurrentPhaseNote  string    `json:"currentPhaseNote,omitempty"`
	CurrentPhaseSince time.Time `json:"currentPhaseSince,omitempty"`
}

// BreakpointDTO 是断点的公开形态：类型、挂载对象种类与业务名称，不暴露内部键。
type BreakpointDTO struct {
	Type     string `json:"type"`
	TypeName string `json:"typeName"`
	NodeName string `json:"nodeName,omitempty"`
	// NodeKey 是节点断点的挂载键（配置令牌）：删除断点必须原样带回，否则删除静默无效（评审 P1）。
	NodeKey string `json:"nodeKey,omitempty"`
	StepNo  int    `json:"stepNo,omitempty"`
	Action  string `json:"action,omitempty"`
}

// PathChoiceDTO 是分支选择的公开形态：分支节点 ID 与所选分支 ID。
type PathChoiceDTO struct {
	RouteNodeID string `json:"routeNodeId"`
	BranchID    string `json:"branchId"`
}

// CommandDTO 是可用命令的公开形态（含中文停止条件说明）。
type CommandDTO struct {
	Command string `json:"command"`
	Label   string `json:"label"`
}

// compileRunSteps 用当前真实结构和新状态编译器从 user_actions 生成新运行步骤。
// 编译器只做结构校验；实时门禁仍由每一步的执行器在目标事实上复验。
func (s *RunOrchestrationService) compileRunSteps(ctx context.Context, planID, pathID uint64, path model.ExecutionPath, rawActions []byte) ([]model.CompiledActionStep, error) {
	graph, err := s.graphs.Get(ctx, planID)
	if err != nil {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationStorage, Message: "暂时无法读取真实流程结构，请重试"}
	}
	analysis, err := analyzer.NewExecutionPathAnalyzer().Analyze(graph, path.Choices)
	if err != nil {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "分支选择与当前真实结构不一致，请重新校验执行路径"}
	}
	nodes, sequence := semanticScenarioNodes(graph, analysis)
	compiled, compileErr := scenario.Compile(scenario.Input{
		Actions: decodeWorkspaceActions(rawActions), Nodes: nodes, NodeSequence: sequence, FinalNodeKey: lastString(sequence),
	})
	if compileErr == nil {
		return compiled.Steps, nil
	}
	return nil, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "动作配置存在结构问题，不能启动运行：" + compileErr.Error()}
}

// buildRunContext 从真实业务记录装配执行上下文：只读，不触碰目标写接口。
func (s *RunOrchestrationService) buildRunContext(ctx context.Context, planID, pathID uint64) (step.RunContext, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return step.RunContext{}, err
	}
	path, err := s.paths.Get(ctx, planID, pathID)
	if err != nil {
		return step.RunContext{}, err
	}
	config, found, err := s.configs.GetPathConfig(ctx, pathID)
	if err != nil {
		return step.RunContext{}, err
	}
	steps := []model.CompiledActionStep{}
	if found && len(config.UserActions) > 0 {
		// 新运行以 user_actions 为唯一动作顺序来源，用当前状态编译器重新生成动作组；
		// 旧 compiled_steps 只服务已开始的历史运行，避免旧隐藏步骤污染新运行。
		compiled, compileErr := s.compileRunSteps(ctx, planID, pathID, path, config.UserActions)
		if compileErr == nil {
			steps = compiled
		} else {
			return step.RunContext{}, compileErr
		}
	} else if found && len(config.CompiledSteps) > 0 {
		if err := json.Unmarshal(config.CompiledSteps, &steps); err != nil {
			return step.RunContext{}, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "编译场景读取失败，请重新保存动作编排"}
		}
	}
	nodes := map[string]step.NodeInfo{}
	nodeEditableFields := map[string][]string{}
	snapshot, snapshotErr := s.pathNodes.Get(ctx, planID, pathID)
	if snapshotErr == nil {
		for _, group := range snapshot.Groups {
			for _, node := range group.Nodes {
				nodes[node.Key] = step.NodeInfo{
					Name: node.Name, Type: node.TypeName, EditableFields: node.EditableFieldKeys,
				}
				nodeEditableFields[node.Key] = node.EditableFieldKeys
			}
		}
	}
	// 分支选择：把路径已保存的 choice（分支节点+分支ID）解析为该分支的目标节点 ID。
	// 目标提交校验手动条件分支时要求显式携带所选节点（custom_choose），这是语义清单第 4 条的落点。
	branchSelections := map[string]string{}
	submitBranchTarget := ""
	graph, graphErr := s.graphs.Get(ctx, planID)
	if graphErr != nil {
		// 分支选择解析依赖真实结构；结构读不到时不能静默跳过——
		// 否则提交载荷缺失分支参数，会在目标侧以“手动条件分支,请选择”失败。
		return step.RunContext{}, &RunOrchestrationError{Kind: RunOrchestrationStorage, Message: "暂时无法读取真实流程结构，请重试"}
	}
	// F-034 T04：真实图边与节点类型进入运行上下文，跨节点手动分支入口解析不再依赖“下一业务节点
	// 恰好是分支目标”的巧合；入口是空节点时同样能精确解析。
	graphEdges := map[string][]step.GraphEdgeInfo{}
	graphNodeTypes := map[string]string{}
	for _, graphNode := range graph.Nodes {
		graphNodeTypes[graphNode.ID] = graphNode.Type
	}
	for _, edge := range graph.Edges {
		graphEdges[edge.Source] = append(graphEdges[edge.Source], step.GraphEdgeInfo{Target: edge.Target, BranchID: edge.BranchID})
	}
	// 配置快照与编译场景用的 nodeKey 是不透明派生键，发给目标匹配不上任何数据；
	// 这里按同一派生规则把真实节点标识补回节点表，供待办读取、按节点写参数与对账对照使用。
	for _, graphNode := range graph.Nodes {
		key := analyzer.PathConfigNodeToken(graphNode.ID)
		info, exists := nodes[key]
		if !exists {
			continue
		}
		info.TargetNodeID = graphNode.ID
		// 节点审批方式随真实结构实时补齐：提交载荷据此决定 nextAuditorList 的人员指定项。
		info.AuditType = graphNode.AuditType
		// F-034：目标 isSkip 声明随节点表进入执行器，跳过/阻塞分型不再靠待办位置猜测。
		info.IsSkip = graphNode.IsSkip
		nodes[key] = info
	}
	for index, choice := range path.Choices {
		matched := ""
		for _, edge := range graph.Edges {
			if edge.Source != choice.RouteNodeID || edge.BranchID != choice.BranchID {
				continue
			}
			matched = edge.Target
			break
		}
		if matched == "" {
			return step.RunContext{}, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "分支选择与当前真实结构不一致，请重新校验执行路径"}
		}
		branchSelections[choice.RouteNodeID] = matched
		if index == 0 {
			submitBranchTarget = matched
		}
	}
	actionPersonIDs := map[string][]string{}
	if s.pathNodes != nil {
		for _, compiled := range steps {
			if compiled.Action != model.ActionAddSign && compiled.Action != model.ActionTransfer && compiled.Action != model.ActionForward {
				continue
			}
			nodeKey := compiled.NodeKey
			if compiled.Action == model.ActionForward {
				nodeKey = analyzer.PathConfigInstanceActionKey()
			}
			resolved, resolveErr := s.pathNodes.ResolveActionPersonIDs(ctx, planID, pathID, nodeKey, compiled.Action)
			if resolveErr != nil {
				return step.RunContext{}, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "动作人员策略无法按当前目标结构解析：" + resolveErr.Error()}
			}
			actionPersonIDs[step.ActionPersonIndex(compiled.NodeKey, compiled.Action)] = resolved
		}
	}
	nextNodeAuditors := map[string][]target.NextAuditor{}
	for index, compiled := range steps {
		if compiled.Action != model.ActionSubmit && compiled.Action != model.ActionResubmit && compiled.Action != model.ActionApprove {
			continue
		}
		nextNodeKey := step.FollowingActionNodeKey(steps, index)
		if _, ok := nodes[nextNodeKey]; !ok {
			continue
		}
		if _, resolved := nextNodeAuditors[nextNodeKey]; resolved {
			continue
		}
		if s.pathNodes == nil {
			return step.RunContext{}, &RunOrchestrationError{Kind: RunOrchestrationStorage, Message: "下一节点处理人解析服务暂不可用"}
		}
		// 只要有已保存的处理人策略就随提交/同意携带：真实运行实测，目标对自选类审批节点
		// 缺少 bizId 处理人时不建待办（实例停在节点上无任务），后续审批永远"无待办"。
		// 固定规则节点没有已保存策略，解析失败按"目标自行指派"跳过，不阻塞启动。
		resolved, resolveErr := s.pathNodes.ResolveNodeAuditors(ctx, planID, pathID, nextNodeKey)
		if resolveErr != nil {
			continue
		}
		if len(resolved) == 0 {
			continue
		}
		nextNodeAuditors[nextNodeKey] = resolved
	}
	return step.RunContext{
		Run:                      model.Run{PlanID: planID},
		PathRun:                  model.PathRun{ExecutionPathID: pathID},
		PlanName:                 plan.Name,
		PathName:                 path.Name,
		PlanAccount:              plan.Account,
		FlowProxyID:              plan.TargetObjectID,
		Source:                   plan.FlowSource,
		Nodes:                    nodes,
		BranchSelections:         branchSelections,
		GraphEdges:               graphEdges,
		GraphNodeTypes:           graphNodeTypes,
		SubmitBranchTargetNodeID: submitBranchTarget,
		Steps:                    steps,
		EffectiveFormData:        config.EffectiveFormData,
		NodeEditableFields:       nodeEditableFields,
		ActionPersonIDs:          actionPersonIDs,
		NextNodeAuditors:         nextNodeAuditors,
	}, nil
}

// StartRun 启动一次单步运行：复验运行准备、装配执行上下文、交控制服务停在第一步之前。
func (s *RunOrchestrationService) StartRun(ctx context.Context, input StartRunInput) (*PathRunDetailDTO, error) {
	readiness, err := s.readiness.PlanReadiness(ctx, input.PlanID, []uint64{input.ExecutionPathID})
	if err != nil {
		return nil, err
	}
	var pathReadiness *model.PathRunReadiness
	for i := range readiness.Paths {
		if readiness.Paths[i].PathID == input.ExecutionPathID {
			pathReadiness = &readiness.Paths[i]
			break
		}
	}
	if pathReadiness == nil {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationNotFound, Message: "执行路径不存在或不属于该计划"}
	}
	if !pathReadiness.Runnable {
		reasons := make([]string, 0, len(pathReadiness.Blocks))
		for _, block := range pathReadiness.Blocks {
			reasons = append(reasons, block.Name+"："+block.Reason)
		}
		return nil, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "运行前检查未通过，不能启动：" + strings.Join(reasons, "；")}
	}

	runCtx, err := s.buildRunContext(ctx, input.PlanID, input.ExecutionPathID)
	if err != nil {
		return nil, err
	}
	if len(runCtx.Steps) == 0 {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "编译场景为空，不能启动；请先完成动作编排"}
	}
	started, err := s.control.Start(ctx, runCtx)
	if err != nil {
		return nil, err
	}
	// RunContext 是值传递：真实运行身份以控制服务返回值为准。
	return s.RunDetailByPathRun(ctx, started.PathRun.ID)
}

// ApproveWithCommand 按命令放行（F-017）：命令携带步游标与控制版本，条件写幂等。
// 请求上下文注入运行作用域：写请求的 network.log/curl.log 因此落进运行目录。
// resolvePathRunID 把 API 层的运行寻址解析为路径运行 ID。
// 控制端点全部以运行 ID（+可选路径运行 ID）寻址；绝不把运行 ID 直接当路径运行 ID 使用——
// 两个自增序列一旦错位，放行或对账就会作用到另一条路径运行上（评审缺陷 6）。
// 多路径运行（F-020）必须携带路径运行身份；单路径运行兼容不带。
func (s *RunOrchestrationService) resolvePathRunID(ctx context.Context, runID uint64, pathRunID uint64) (uint64, error) {
	if pathRunID != 0 {
		pathRun, err := s.store.GetPathRun(ctx, pathRunID)
		if err != nil {
			return 0, err
		}
		if pathRun.RunID != runID {
			return 0, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "该路径运行不属于这次运行"}
		}
		return pathRun.ID, nil
	}
	pathRun, err := s.store.GetPathRunByRun(ctx, runID)
	if err != nil {
		return 0, err
	}
	siblings, err := s.store.ListPathRunsByRun(ctx, runID)
	if err != nil {
		return 0, err
	}
	if len(siblings) > 1 {
		return 0, &RunOrchestrationError{Kind: RunOrchestrationConflict,
			Message: "这次运行有多条路径，请先选择要操作的路径"}
	}
	return pathRun.ID, nil
}

// ApproveWithCommand 放行当前步。runID 是运行 ID，pathRunID 非零时按路径运行寻址（F-020）。
func (s *RunOrchestrationService) ApproveWithCommand(ctx context.Context, runID uint64, pathRunID uint64, command model.ControlCommand, cursor int, version int64) (*PathRunDetailDTO, error) {
	pathRunID, err := s.resolvePathRunID(ctx, runID, pathRunID)
	if err != nil {
		return nil, err
	}
	scoped, err := s.withRunScope(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	if err := s.control.ApproveWithCommandAsync(scoped, pathRunID, command, cursor, version); err != nil {
		return nil, err
	}
	return s.RunDetailByPathRun(ctx, pathRunID)
}

// StartRunWithMode 按模式与预置断点启动（F-017）。
func (s *RunOrchestrationService) StartRunWithMode(ctx context.Context, input StartRunInput, mode model.RunMode, breakpoints []control.Breakpoint) (*PathRunDetailDTO, error) {
	if err := s.validateReadiness(ctx, input.PlanID, input.ExecutionPathID); err != nil {
		return nil, err
	}
	runCtx, err := s.buildRunContext(ctx, input.PlanID, input.ExecutionPathID)
	if err != nil {
		return nil, err
	}
	if len(runCtx.Steps) == 0 {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "编译场景为空，不能启动；请先完成动作编排"}
	}
	started, err := s.control.StartWithMode(ctx, runCtx, mode, breakpoints)
	if err != nil {
		return nil, err
	}
	// RunContext 是值传递：真实运行身份以控制服务返回值为准。
	return s.RunDetailByPathRun(ctx, started.PathRun.ID)
}

// SwitchMode 运行中切换自动/单步模式（2026-09-06）：只作用于这一条路径运行，
// 切换在安全步骤边界生效；请求携带控制版本，重复点击只产生一次控制事实。
func (s *RunOrchestrationService) SwitchMode(ctx context.Context, runID uint64, pathRunID uint64, mode model.RunMode, version int64) (*PathRunDetailDTO, error) {
	pathRunID, err := s.resolvePathRunID(ctx, runID, pathRunID)
	if err != nil {
		return nil, err
	}
	scoped, err := s.withRunScope(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	if _, err := s.control.SwitchMode(scoped, pathRunID, mode, version); err != nil {
		return nil, err
	}
	return s.RunDetailByPathRun(ctx, pathRunID)
}

// SetBreakpoint / RemoveBreakpoint / RequestPause / ListBreakpoints / ControlView 是控制面转发。
// 除 ControlView 外都以运行 ID 寻址，进入服务即解析为路径运行 ID。
func (s *RunOrchestrationService) SetBreakpoint(ctx context.Context, runID uint64, pathRunID uint64, bp control.Breakpoint) ([]control.Breakpoint, error) {
	pathRunID, err := s.resolvePathRunID(ctx, runID, pathRunID)
	if err != nil {
		return nil, err
	}
	return s.control.SetBreakpoint(ctx, pathRunID, bp)
}

func (s *RunOrchestrationService) RemoveBreakpoint(ctx context.Context, runID uint64, pathRunID uint64, bp control.Breakpoint) ([]control.Breakpoint, error) {
	pathRunID, err := s.resolvePathRunID(ctx, runID, pathRunID)
	if err != nil {
		return nil, err
	}
	return s.control.RemoveBreakpoint(ctx, pathRunID, bp)
}

func (s *RunOrchestrationService) RequestPause(ctx context.Context, runID uint64, pathRunID uint64) error {
	pathRunID, err := s.resolvePathRunID(ctx, runID, pathRunID)
	if err != nil {
		return err
	}
	return s.control.RequestPause(ctx, pathRunID)
}

func (s *RunOrchestrationService) ListBreakpoints(ctx context.Context, runID uint64, pathRunID uint64) ([]control.Breakpoint, error) {
	pathRunID, err := s.resolvePathRunID(ctx, runID, pathRunID)
	if err != nil {
		return nil, err
	}
	return s.control.ListBreakpoints(ctx, pathRunID)
}

func (s *RunOrchestrationService) ControlView(pathRunID uint64) *control.SessionView {
	return s.control.View(pathRunID)
}

// validateReadiness 抽出启动前的运行准备复验（模式启动与 F-016 启动共用）。
func (s *RunOrchestrationService) validateReadiness(ctx context.Context, planID, executionPathID uint64) error {
	readiness, err := s.readiness.PlanReadiness(ctx, planID, []uint64{executionPathID})
	if err != nil {
		return err
	}
	var pathReadiness *model.PathRunReadiness
	for i := range readiness.Paths {
		if readiness.Paths[i].PathID == executionPathID {
			pathReadiness = &readiness.Paths[i]
			break
		}
	}
	if pathReadiness == nil {
		return &RunOrchestrationError{Kind: RunOrchestrationNotFound, Message: "执行路径不存在或不属于该计划"}
	}
	if !pathReadiness.Runnable {
		reasons := make([]string, 0, len(pathReadiness.Blocks))
		for _, block := range pathReadiness.Blocks {
			reasons = append(reasons, block.Name+"："+block.Reason)
		}
		return &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "运行前检查未通过，不能启动：" + strings.Join(reasons, "；")}
	}
	return nil
}

// Stop 停止路径运行并返回最新详情。runID 是运行 ID；pathRunID 非零时按路径运行寻址（F-020）。
func (s *RunOrchestrationService) Stop(ctx context.Context, runID uint64, pathRunID uint64) (*PathRunDetailDTO, error) {
	pathRunID, err := s.resolvePathRunID(ctx, runID, pathRunID)
	if err != nil {
		return nil, err
	}
	scoped, err := s.withRunScope(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	if _, err := s.control.Stop(scoped, pathRunID); err != nil {
		return nil, err
	}
	return s.RunDetailByPathRun(ctx, pathRunID)
}

// RetryFailedStep 重试失败动作（F-028）。runID 是运行 ID；pathRunID 非零时按路径运行寻址。
// 全链路只做五件事：校验失败态与失败分类、按当前配置重编译场景并核对已执行前缀、
// 重开路径运行与运行聚合、把控制现场装填回失败步骤、返回最新详情。
// 重试本身不发任何写请求；装填后的执行完全复用既有七阶段管线（实时门禁、一次写、写后核验、租约），
// 每次尝试按事实表只追加的纪律独立落账，再次失败可以再次重试。
func (s *RunOrchestrationService) RetryFailedStep(ctx context.Context, runID uint64, pathRunID uint64) (*PathRunDetailDTO, error) {
	pathRunID, err := s.resolvePathRunID(ctx, runID, pathRunID)
	if err != nil {
		return nil, err
	}
	scoped, err := s.withRunScope(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	ctx = scoped
	pathRun, err := s.store.GetPathRun(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	if pathRun.Status != model.PathRunStatusFailed {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationConflict,
			Message: "只有失败的路径运行可以重试失败动作；当前状态：" + model.PathRunStatusName(pathRun.Status)}
	}
	// 写结果不确定是终局（2026-09-06 产品裁决）：按聚合列防御性再拦一次，
	// 防止历史数据把待对账结论错写进失败态后从这里漏出去重发真实写请求。
	if pathRun.FailureClass != nil && *pathRun.FailureClass == model.FailureClassWriteUncertain {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationConflict,
			Message: "执行结果无法确认的运行不能重试；为避免重复操作，请从计划重新发起运行"}
	}
	runRow, err := s.store.GetRun(ctx, pathRun.RunID)
	if err != nil {
		return nil, err
	}
	factRows, err := s.store.ListRunSteps(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	// 重试按当前 user_actions 重新编译场景；前缀校验确保失败后没改过配置，
	// 改过就拒绝——继续执行一份与原运行不同的场景比重新发起一次运行更危险。
	runCtx, err := s.buildRunContext(ctx, runRow.PlanID, pathRun.ExecutionPathID)
	if err != nil {
		return nil, err
	}
	plan, err := planFailedStepRetry(runCtx.Steps, pathRun.TotalSteps, factRows)
	if err != nil {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: err.Error()}
	}
	reopenedPathRun, reopenedRun, err := s.runState.ReopenForRetry(ctx, pathRunID, plan.StepNo)
	if err != nil {
		return nil, err
	}
	// RunContext 是值传递：装填现场必须携带重开后的真实身份（含首次发起落库的主实例引用），
	// 否则门禁与事实重读拿不到实例，重试步骤会在目标事实上失败。
	runCtx.Run = reopenedRun
	runCtx.PathRun = reopenedPathRun
	if _, err := s.control.ArmRetrySession(ctx, runCtx, plan.CursorIndex, plan.ExecutedStepNos, plan.ExecutedNodeKeys); err != nil {
		return nil, err
	}
	return s.RunDetailByPathRun(ctx, pathRunID)
}

// RecoveryLogWriter 暴露 recovery.log 写入函数供控制服务装配。
func (s *RunOrchestrationService) RecoveryLogWriter() func(pathRunID uint64, message string) {
	return func(pathRunID uint64, message string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		scope, scopeErr := s.runLogScope(ctx, pathRunID)
		if scopeErr != nil {
			return
		}
		// 同一运行目录内的日志统一用本地时间：step.log、network.log 与 recovery.log 按时间对照才不错位。
		line := logging.FormatLine(time.Now(), "info", append(scope.Fields(), logging.Field{Key: "message", Value: strings.ReplaceAll(message, " ", "_")}))
		s.router.Bucket(scope, "recovery.log").WriteLine(line)
	}
}

// runLogScope 按路径运行身份构造日志作用域（供恢复与控制日志复用）。
func (s *RunOrchestrationService) runLogScope(ctx context.Context, pathRunID uint64) (logging.Scope, error) {
	return s.runScopeByPathRun(ctx, pathRunID)
}

// runScopeByPathRun 读取路径运行、运行与计划、执行路径的真实身份并构造日志作用域。
// 显示名只能来自数据库记录，读不到就留空由目录清洗成占位段，绝不拿计划名冒充路径名。
func (s *RunOrchestrationService) runScopeByPathRun(ctx context.Context, pathRunID uint64) (logging.Scope, error) {
	pathRun, err := s.store.GetPathRun(ctx, pathRunID)
	if err != nil {
		return logging.Scope{}, err
	}
	run, err := s.store.GetRun(ctx, pathRun.RunID)
	if err != nil {
		return logging.Scope{}, err
	}
	plan, err := s.plans.Get(ctx, run.PlanID)
	if err != nil {
		return logging.Scope{}, err
	}
	pathName := ""
	if path, pathErr := s.paths.Get(ctx, run.PlanID, pathRun.ExecutionPathID); pathErr == nil {
		pathName = strings.TrimSpace(path.Name)
	}
	return runScopeOf(pathRun, run, plan, pathName), nil
}

// runScopeOf 组装运行日志作用域：页面身份（计划/路径/运行/路径运行）+ 目标实例业务信息。
// 目录键只用页面已有的 runId/pathRunId（见 logging.Router.BucketDir），显示名与实例名称只作可读标签；
// 实例名称由执行器读到后补进 meta.json，这里只带已落账的实例 ID，绝不猜测名称。
func runScopeOf(pathRun model.PathRun, run model.Run, plan model.Plan, pathName string) logging.Scope {
	return logging.Scope{
		PlanID:            strconv.FormatUint(run.PlanID, 10),
		PlanName:          plan.Name,
		ExecutionPathID:   strconv.FormatUint(pathRun.ExecutionPathID, 10),
		ExecutionPathName: pathName,
		RunID:             strconv.FormatUint(run.ID, 10),
		RunSeq:            strconv.FormatUint(run.RunNo, 10),
		PathRunID:         strconv.FormatUint(pathRun.ID, 10),
		InstanceID:        strings.TrimSpace(pathRun.MainInstanceRef),
	}
}

// ControlLogWriter 暴露 control.log 写入函数供控制服务装配（复用 F-013 的运行目录路由）。
func (s *RunOrchestrationService) ControlLogWriter() func(pathRunID uint64, fields []fmt.Stringer) {
	return func(pathRunID uint64, fields []fmt.Stringer) {
		s.controlLogWriter()(pathRunID, fields)
	}
}

// controlLogWriter 把控制事实写进运行目录的 control.log（复用 F-013 的运行目录路由）。
// 每次写入按路径运行身份现算作用域：控制事实频率低，查库代价可接受。
func (s *RunOrchestrationService) controlLogWriter() func(pathRunID uint64, fields []fmt.Stringer) {
	return func(pathRunID uint64, fields []fmt.Stringer) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		scope, scopeErr := s.runScopeByPathRun(ctx, pathRunID)
		if scopeErr != nil {
			return
		}
		// 同一运行目录内的日志统一用本地时间（与 step.log、network.log 一致，按时间对照不错位）。
		line := logging.FormatLine(time.Now(), "info", append(scope.Fields(), toLoggingFields(fields)...))
		s.router.Bucket(scope, "control.log").WriteLine(line)
	}
}

// toLoggingFields 把控制日志字段转为 logging 字段。
func toLoggingFields(fields []fmt.Stringer) []logging.Field {
	result := make([]logging.Field, 0, len(fields))
	for _, field := range fields {
		if f, ok := field.(interface{ String() string }); ok {
			text := f.String()
			if index := strings.Index(text, "="); index > 0 {
				result = append(result, logging.Field{Key: text[:index], Value: text[index+1:]})
			}
		}
	}
	return result
}

// withRunScope 按路径运行的真实身份构造日志作用域并注入上下文。
// 放行、停止、重试与详情读取都从这里取同一份作用域，因此 network.log、curl.log、step.log
// 与恢复/控制日志始终落在同一个路径运行目录。
func (s *RunOrchestrationService) withRunScope(ctx context.Context, pathRunID uint64) (context.Context, error) {
	scope, err := s.runScopeByPathRun(ctx, pathRunID)
	if err != nil {
		return ctx, err
	}
	return logging.WithScope(ctx, scope), nil
}

// runPathsSummary 把一次运行的路径状态汇成一句中文：按状态分组计数，失败在前。
func runPathsSummary(pathRuns []model.PathRun) string {
	// 展示顺序：失败、结果待确认、已停止、已取消、运行中/核验中、等待运行、已完成（用户最该先看的在前）。
	order := []struct {
		status model.PathRunStatus
		label  string
	}{
		{model.PathRunStatusFailed, "失败"},
		{model.PathRunStatusAwaitingReconciliation, "结果待确认"},
		{model.PathRunStatusStopped, "已停止"},
		{model.PathRunStatusCancelled, "已取消"},
		{model.PathRunStatusRunning, "运行中"},
		{model.PathRunStatusVerifying, "确认结果中"},
		{model.PathRunStatusPaused, "暂停"},
		{model.PathRunStatusWaiting, "等待运行"},
		{model.PathRunStatusCompleted, "已完成"},
	}
	counts := map[model.PathRunStatus]int{}
	for _, pathRun := range pathRuns {
		counts[pathRun.Status]++
	}
	parts := make([]string, 0, len(order))
	for _, entry := range order {
		if counts[entry.status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[entry.status], entry.label))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("共 %d 条：%s", len(pathRuns), strings.Join(parts, "、"))
}

// RunDetail 按运行 ID 读取详情（一次运行只跑一条路径）。
func (s *RunOrchestrationService) RunDetail(ctx context.Context, runID uint64) (*PathRunDetailDTO, error) {
	pathRun, err := s.store.GetPathRunByRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return s.RunDetailByPathRun(ctx, pathRun.ID)
}

// RunDetailByRunAndPathRun 读取指定路径运行的详情（F-020 多路径运行按路径切换）。
// pathRunID 必须属于该运行；不属于时给中文冲突错误，绝不跨运行读取。
func (s *RunOrchestrationService) RunDetailByRunAndPathRun(ctx context.Context, runID, pathRunID uint64) (*PathRunDetailDTO, error) {
	pathRun, err := s.store.GetPathRun(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	if pathRun.RunID != runID {
		return nil, &RunOrchestrationError{Kind: RunOrchestrationConflict, Message: "该路径运行不属于这次运行"}
	}
	return s.RunDetailByPathRun(ctx, pathRunID)
}

// fillRunLogLocation 补齐运行详情的日志目录与目标实例身份（F-031/T05、T06）。
// 目录由本次运行的 runId/pathRunId 直接算出，与页面「运行记录 -> 路径运行」逐层对应；
// 实例名称从该目录的 meta.json 读取。历史运行（F-031 之前）的日志在旧目录里，
// 按已落账的 step.log 相对路径回查它的 meta.json；两处都读不到名称时如实标记「实例名称不可用」，
// 绝不用计划名、路径名或候选处理人名称补造。
// lastPreRejectedStepNo 返回按落账顺序最后一次前置拒绝尝试所属的步骤号；没有记录时返回 0。
func lastPreRejectedStepNo(attempts []model.RunStepAttempt) int {
	// ListRunAttempts 按 run_step_attempts.id ASC 返回；AttemptNo 只在单个步骤内递增，
	// 不能跨步骤比较，否则前一步的第 3 次尝试会错误覆盖后一步的第 1 次阻塞。
	for index := len(attempts) - 1; index >= 0; index-- {
		if attempts[index].Initial == "pre_rejected" {
			return int(attempts[index].StepID)
		}
	}
	return 0
}

// lastAttemptWasPreRejected 判断该路径运行的最后一次尝试是否命中前置拒绝初判（F-034 T04）。
// pre_rejected 表示目标在任何写之前明确拒绝（如手动分支未选择、未设置审批人），可安全投影为阻塞；
// 没有任何尝试记录或初判为空时返回 false，不凭失败分类猜测。
func lastAttemptWasPreRejected(attempts []model.RunStepAttempt) bool {
	if len(attempts) == 0 {
		return false
	}
	// 尝试列表按落账主键升序排列；最后一条才是路径的最后一次尝试。
	return attempts[len(attempts)-1].Initial == "pre_rejected"
}

// fillRunLogLocation 填充运行日志目录与实例身份。
func (s *RunOrchestrationService) fillRunLogLocation(detail *PathRunDetailDTO, run model.Run, pathRun model.PathRun, plan model.Plan, attempts []model.RunStepAttempt) {
	if detail == nil || s.router == nil {
		return
	}
	pathName := ""
	if path, pathErr := s.paths.Get(context.Background(), run.PlanID, pathRun.ExecutionPathID); pathErr == nil {
		pathName = strings.TrimSpace(path.Name)
	}
	scope := runScopeOf(pathRun, run, plan, pathName)
	meta, found := s.router.ReadMeta(scope)
	dir := s.router.BucketDir(scope)
	if !found {
		// 旧目录回查：只按已落账的 step.log 相对路径定位，不扫描盘上的历史目录。
		if legacyDir, ok := legacyLogDir(s.router, attempts); ok {
			dir = legacyDir
			meta, found = s.router.ReadMetaAt(legacyDir)
		}
	}
	if relative, err := filepath.Rel(s.router.Root(), dir); err == nil {
		detail.LogDir = filepath.ToSlash(relative)
	}
	if instanceID := strings.TrimSpace(meta.InstanceID); instanceID != "" {
		detail.InstanceID = instanceID
	} else {
		detail.InstanceID = strings.TrimSpace(pathRun.MainInstanceRef)
	}
	detail.InstanceName = strings.TrimSpace(meta.InstanceName)
	detail.InstanceNameAvailable = detail.InstanceName != ""
	detail.InstanceNameNote = strings.TrimSpace(meta.InstanceNameNote)
	if !detail.InstanceNameAvailable && detail.InstanceNameNote == "" {
		detail.InstanceNameNote = "目标实例名称不可用"
	}
}

// legacyLogDir 从已落账尝试的 step.log 相对路径反查旧日志目录（F-031 之前的目录方案）。
// 取第一条非空记录即可：同一次路径运行的所有日志都在同一个目录里。
func legacyLogDir(router *logging.Router, attempts []model.RunStepAttempt) (string, bool) {
	if router == nil {
		return "", false
	}
	for _, attempt := range attempts {
		if strings.TrimSpace(attempt.LogPath) == "" {
			continue
		}
		dir := filepath.Dir(filepath.Join(router.Root(), filepath.FromSlash(attempt.LogPath)))
		return dir, true
	}
	return "", false
}

// RunDetailByPathRun 按路径运行 ID 读取详情。
func (s *RunOrchestrationService) RunDetailByPathRun(ctx context.Context, pathRunID uint64) (*PathRunDetailDTO, error) {
	pathRun, err := s.store.GetPathRun(ctx, pathRunID)
	if err != nil {
		return nil, err
	}
	run, err := s.store.GetRun(ctx, pathRun.RunID)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, run, pathRun)
}

// detail 聚合路径运行详情：运行事实、节点状态、当前预览、最终目标事实。
func (s *RunOrchestrationService) detail(ctx context.Context, run model.Run, pathRun model.PathRun) (*PathRunDetailDTO, error) {
	plan, err := s.plans.Get(ctx, run.PlanID)
	if err != nil {
		return nil, err
	}
	// 结构读取仅服务画布展示；失败时降级为空结构继续返回，但把原始错误原样透传给前端，
	// 由前端在错误信息区展示真实原因，不做任何包装或改写。
	graphCtx, cancelGraph := context.WithTimeout(ctx, runDetailGraphTimeout)
	graph, graphErr := s.graphs.Get(graphCtx, run.PlanID)
	cancelGraph()
	if graphErr != nil {
		graph = model.FlowGraph{PlanID: run.PlanID}
	}
	graphErrorText := ""
	if graphErr != nil {
		graphErrorText = graphErr.Error()
	}
	steps, err := s.store.ListRunSteps(ctx, pathRun.ID)
	if err != nil {
		return nil, err
	}
	attempts, err := s.store.ListRunAttempts(ctx, pathRun.ID)
	if err != nil {
		return nil, err
	}
	detail := &PathRunDetailDTO{
		RunID: run.ID, RunNo: run.RunNo,
		StartedAt:         run.StartedAt,
		ModeName:          model.RunModeName(run.Mode),
		RunStatusName:     s.runStatusNameOf(ctx, run),
		PathRunID:         pathRun.ID,
		PathRunStatus:     string(pathRun.Status),
		PathRunStatusName: model.PathRunStatusName(pathRun.Status),
		PlanID:            run.PlanID,
		PlanName:          plan.Name,
		PathID:            pathRun.ExecutionPathID,
		PathName:          pathNameOf(ctx, s.paths, run.PlanID, pathRun.ExecutionPathID, plan.Name),
		NodeStates:        map[string]RunNodeStateDTO{},
		GraphError:        graphErrorText,
		NodePlans:         map[string][]RunNodePlanActionDTO{},
		// 数组型字段一律以空数组起步：nil 切片会序列化成 JSON null，前端按数组读取会整页崩溃。
		Steps:               []RunStepDTO{},
		Breakpoints:         []BreakpointDTO{},
		Commands:            []CommandDTO{},
		Paths:               []RunPathSummaryDTO{},
		RunScheduleName:     runScheduleName(run),
		RunConcurrencyLabel: runConcurrencyLabel(run),
		PollIntervalMs:      s.runConfig.StatusPollInterval.Milliseconds(),
		StaleAfterMs:        s.runConfig.StepProgressStaleAfter.Milliseconds(),
	}
	if pathRun.Result != nil {
		detail.ResultName = resultName(*pathRun.Result)
	}
	if pathRun.FailureClass != nil {
		detail.FailureClassName = model.FailureClassName(*pathRun.FailureClass)
		// F-034 T04：只有确定失败且最后一次尝试的初判是 pre_rejected（目标在任何写之前明确拒绝）
		// 才投影为“阻塞”；写结果不确定、传输中断等仍保持“结果待确认”，其余失败保持“失败”。
		if *pathRun.FailureClass == model.FailureClassTargetRejected && detail.PathRunStatus == string(model.PathRunStatusFailed) && lastAttemptWasPreRejected(attempts) {
			detail.StopKind = "blocked"
			detail.StopKindNote = "目标在执行前明确拒绝了请求：前置条件未满足，没有产生任何写入"
			detail.StopStepNo = lastPreRejectedStepNo(attempts)
		}
	}
	if pathRun.FinalTargetSummary != "" {
		detail.FinalTarget = json.RawMessage(pathRun.FinalTargetSummary)
	}
	// 日志位置与目标实例身份：目录按页面身份（runId/pathRunId）直接算出，实例名称只作业务信息。
	s.fillRunLogLocation(detail, run, pathRun, plan, attempts)
	phaseTimings := s.readPhaseTimings(pathRun.ID, attempts)
	// F-030/T04：真实目标请求明细与汇总，一次读取按尝试归组，详情接口不逐请求扫描。
	requestsByKey, summaryByKey := s.readAttemptRequests(pathRun.ID, attempts)
	// 步骤与预览携带的是配置令牌键（编译场景的 nodeKey），画布与侧栏按图节点 ID 取值。
	// 这里统一翻译出图节点 ID，键空间对齐后九个运行态、当前步标记与侧栏才真实可用（评审 P1）。
	tokenToGraphID := tokenToGraphNodeID(graph)
	detail.Steps = buildStepDTOs(steps, attempts, phaseTimings, requestsByKey, summaryByKey, s.router, tokenToGraphID)
	if preview := s.control.CurrentPreview(pathRun.ID); preview != nil {
		detail.CurrentPreview = previewDTO(preview)
		detail.CurrentPreview.NodeID = tokenToGraphID[preview.NodeKey]
		detail.CurrentStepNo = preview.StepNo
	}
	// 已配置路线（编译场景节点序列与分支选择）：画布据此标注「等待运行」并区分路径内外；
	// 配置读取或编译失败时退化为不标注，绝不阻塞详情展示。
	configuredNodeKeys, pathChoices, compiledSteps := s.configuredRouteOf(ctx, run, pathRun.ExecutionPathID)
	detail.PathChoices = pathChoices
	// 用编译场景补齐已执行步骤的动作组边界；缺少配置快照时保持空值，绝不猜测。
	stepGroups := make(map[int]model.CompiledActionStep, len(compiledSteps))
	for _, compiled := range compiledSteps {
		stepGroups[compiled.Sequence] = compiled
	}
	for index := range detail.Steps {
		if compiled, exists := stepGroups[detail.Steps[index].StepNo]; exists {
			detail.Steps[index].ReleaseGroup = compiled.ReleaseGroup
			detail.Steps[index].ReleaseRequired = compiled.ReleaseRequired
		}
	}
	detail.NodeStates = buildNodeStates(graph, steps, pathRun, detail.CurrentPreview, configuredNodeKeys)
	detail.NodePlans = buildNodePlans(compiledSteps, tokenToGraphID)
	if view := s.control.View(pathRun.ID); view != nil {
		detail.ModeName = model.RunModeName(view.Mode)
		detail.ControlVersion = view.Version
		detail.StopReason = view.StopReason
		detail.LoopRunning = view.LoopRunning
		detail.StepInFlight = view.StepInFlight
		detail.StopRequested = view.StopRequested
		detail.PauseRequested = view.PauseRequested
		detail.CurrentPhase = view.CurrentPhase
		detail.CurrentPhaseNote = view.CurrentPhaseNote
		detail.CurrentPhaseSince = view.CurrentPhaseSince
		detail.ModeSwitchPending = view.PendingMode != nil
		if view.PendingMode != nil {
			detail.PendingModeName = model.RunModeName(*view.PendingMode)
		}
		for _, command := range view.Commands {
			detail.Commands = append(detail.Commands, CommandDTO{Command: string(command), Label: CommandLabel(command)})
		}
		nodeTable := nodeNamesOf(graph)
		for _, bp := range view.Breakpoints {
			dto := BreakpointDTO{Type: string(bp.Type), TypeName: breakpointTypeName(string(bp.Type)),
				NodeKey: bp.NodeKey, StepNo: bp.StepNo, Action: bp.Action}
			if bp.NodeKey != "" {
				dto.NodeName = nodeNameFromTable(nodeTable, bp.NodeKey)
			}
			detail.Breakpoints = append(detail.Breakpoints, dto)
		}
	}
	detail.NodeStates = buildNodeStates(graph, steps, pathRun, detail.CurrentPreview, configuredNodeKeys)
	// 执行现场已丢失的运行（服务重启或执行结果无法确认，路径运行停在结果待确认且没有内存现场）
	// 无法安全继续：如实告诉用户并引导从计划重新运行；界面不给任何对账、重放或登记入口。
	if s.control.View(pathRun.ID) == nil && pathRun.Status == model.PathRunStatusAwaitingReconciliation {
		detail.SceneLost = true
		detail.SceneLostNote = "本次运行已停止。请查看该步骤的错误信息：目标接口没有返回可确认的结果，或目标状态暂时无法读取。为避免重复操作，需要从计划重新发起运行。"
		// 中断步定位：中断时正在执行的那一步尚未落账，已落账步骤之后的第一条编译步骤就是它。
		// 把它映射到图节点，前端在对应节点面板直接显示中断原因，否则用户点开节点什么也看不到。
		if len(compiledSteps) > len(steps) {
			interrupted := compiledSteps[len(steps)]
			detail.InterruptedNodeID = tokenToGraphID[interrupted.NodeKey]
			detail.InterruptedNote = fmt.Sprintf("第 %d 步「%s」执行中断：目标接口没有返回可确认的结果，或目标状态暂时无法读取。需要从计划重新发起运行。",
				interrupted.Sequence, interrupted.Action)
		}
	}
	// F-028 失败动作重试：确定失败（写请求确认未生效或未发出）的步骤执行失败可以重试；
	// 启动阶段失败（没有任何已执行步骤）引导重新发起，结果待确认是终局。
	// 前提是执行现场不在（有现场说明运行仍在推进，谈不上重试）。
	detail.Retryable = pathRun.Status == model.PathRunStatusFailed &&
		(pathRun.FailureClass == nil || *pathRun.FailureClass != model.FailureClassWriteUncertain) &&
		len(steps) > 0 && s.control.View(pathRun.ID) == nil
	if err := s.fillRunPathSummaries(ctx, run, detail); err != nil {
		return nil, err
	}
	return detail, nil
}

// runConcurrencyLabel 返回运行的中文并发说明。
func runConcurrencyLabel(runRow model.Run) string {
	if runCapacityOf(runRow) <= 1 {
		return "逐条依次运行"
	}
	return "最多同时运行 " + strconv.Itoa(runCapacityOf(runRow)) + " 条路径"
}

// fillRunPathSummaries 填充运行级路径摘要：运行详情的路径切换区据此展示全部路径运行的独立状态。
func (s *RunOrchestrationService) fillRunPathSummaries(ctx context.Context, runRow model.Run, detail *PathRunDetailDTO) error {
	pathRuns, err := s.store.ListPathRunsByRun(ctx, runRow.ID)
	if err != nil {
		return err
	}
	detail.Paths = make([]RunPathSummaryDTO, 0, len(pathRuns))
	for _, pathRun := range pathRuns {
		summary := RunPathSummaryDTO{
			PathRunID:  pathRun.ID,
			PathID:     pathRun.ExecutionPathID,
			PathName:   pathNameOf(ctx, s.paths, runRow.PlanID, pathRun.ExecutionPathID, ""),
			StatusName: model.PathRunStatusName(pathRun.Status),
		}
		if pathRun.Result != nil {
			summary.ResultName = resultName(*pathRun.Result)
		}
		summary.MainInstanceRef = pathRun.MainInstanceRef
		detail.Paths = append(detail.Paths, summary)
	}
	return nil
}

// nodeNamesOf 把真实结构节点表转为键到名称的映射。
func nodeNamesOf(graph model.FlowGraph) map[string]string {
	names := map[string]string{}
	for _, node := range graph.Nodes {
		names[node.ID] = node.Name
	}
	return names
}

// nodeNameFromTable 查节点业务名称；查不到原样返回键。
func nodeNameFromTable(names map[string]string, nodeKey string) string {
	if name := names[nodeKey]; name != "" {
		return name
	}
	return nodeKey
}

// breakpointTypeName 返回断点类型的中文显示名。
func breakpointTypeName(t string) string {
	switch model.BreakpointType(t) {
	case model.BreakpointFirstWrite:
		return "首次写断点"
	case model.BreakpointStep:
		return "步骤断点"
	case model.BreakpointNode:
		return "节点断点"
	case model.BreakpointAction:
		return "动作断点"
	case model.BreakpointPathDeviation:
		return "路径偏离断点"
	default:
		return t
	}
}

// CommandLabel 返回命令的中文说明（含停止条件）。
func CommandLabel(command model.ControlCommand) string {
	return control.CommandLabel(command)
}

// pathNameOf 读取执行路径名称；读取失败时回退到计划级占位，不阻塞详情展示。
func pathNameOf(ctx context.Context, paths repository.ExecutionPathRepository, planID, pathID uint64, fallback string) string {
	path, err := paths.Get(ctx, planID, pathID)
	if err != nil || strings.TrimSpace(path.Name) == "" {
		return fallback
	}
	return path.Name
}

// buildStepDTOs 把步骤与尝试事实组装为公开 DTO，并附上 step.log 解析出的阶段耗时。
func buildStepDTOs(steps []model.RunStep, attempts []model.RunStepAttempt, phaseTimings map[string]map[string]int64,
	requestsByKey map[string][]RunRequestDTO, summaryByKey map[string]runRequestSummaryDTO,
	router *logging.Router, tokenToGraphID map[string]string) []RunStepDTO {
	attemptsByStep := map[uint64][]model.RunStepAttempt{}
	for _, attempt := range attempts {
		attemptsByStep[attempt.StepID] = append(attemptsByStep[attempt.StepID], attempt)
	}
	dtos := make([]RunStepDTO, 0, len(steps))
	for _, stepRecord := range steps {
		dto := RunStepDTO{
			StepNo:       stepRecord.StepNo,
			ActionName:   actionNameOf(stepRecord.Action),
			Action:       stepRecord.Action,
			NodeKey:      stepRecord.NodeKey,
			NodeID:       tokenToGraphID[stepRecord.NodeKey],
			ActorName:    stepRecord.ActorSummary,
			StatusName:   stepStatusName(stepRecord.Status),
			StartedAt:    stepRecord.StartedAt,
			FinishedAt:   stepRecord.FinishedAt,
			DurationMs:   stepRecord.FinishedAt.Sub(stepRecord.StartedAt).Milliseconds(),
			GateSnapshot: stepRecord.GateSnapshot,
			Attempts:     []RunStepAttemptDTO{},
		}
		for _, attempt := range attemptsByStep[stepRecord.ID] {
			attemptDTO := RunStepAttemptDTO{
				AttemptNo:   attempt.AttemptNo,
				VerdictName: verdictName(attempt.Verdict),
				Reason:      attempt.Reason,
				Basis:       attempt.Basis,
				TraceID:     attempt.TraceID,
				DurationMs:  attempt.DurationMs,
				LogPath:     attempt.LogPath,
				LogLine:     attempt.LogLine,
				IsReplay:    attempt.IsReplay,
			}
			// 阶段时间轴按 step_id:attempt 归组（与 parsePhaseTimings 的键一致）。
			timings, ok := phaseTimings[stepPhaseKey(stepRecord.StepNo, attempt.AttemptNo)]
			if ok {
				attemptDTO.PhaseDurations = timings
			} else {
				attemptDTO.PhaseDurationsNote = "执行过程记录缺失，暂时无法显示各阶段耗时"
			}
			// 真实目标请求明细与汇总：历史运行没有 network.log 时保持空，前端按缺失降级。
			requestKey := stepPhaseKey(stepRecord.StepNo, attempt.AttemptNo)
			if list, ok := requestsByKey[requestKey]; ok {
				attemptDTO.Requests = list
			}
			if summary, ok := summaryByKey[requestKey]; ok && summary.Count > 0 {
				summaryCopy := summary
				attemptDTO.RequestSummary = &summaryCopy
			}
			attemptDTO.CurlBlock = curlBlockFor(router, attempt.TraceID, attempt.LogPath)
			dto.Attempts = append(dto.Attempts, attemptDTO)
		}
		dtos = append(dtos, dto)
	}
	return dtos
}

// configuredRouteOf 读取这条路径的已保存配置：编译场景的节点序列、分支选择与编译场景本身。
// 只读存储快照，不做真实结构校验（校验属启动流程）；读取或解析失败返回空，画布退化为不标注。
// 第三个返回值是编译场景原文，供节点计划归组复用同一次读取，不为侧栏再读一遍配置。
func (s *RunOrchestrationService) configuredRouteOf(ctx context.Context, run model.Run, executionPathID uint64) ([]string, []PathChoiceDTO, []model.CompiledActionStep) {
	path, err := s.paths.Get(ctx, run.PlanID, executionPathID)
	if err != nil {
		return nil, nil, nil
	}
	choices := make([]PathChoiceDTO, 0, len(path.Choices))
	for _, choice := range path.Choices {
		choices = append(choices, PathChoiceDTO{RouteNodeID: choice.RouteNodeID, BranchID: choice.BranchID})
	}
	config, found, err := s.configs.GetPathConfig(ctx, executionPathID)
	if err == nil {
	} else {
		return nil, choices, nil
	}
	if found == false {
		return nil, choices, nil
	}
	compiledSteps := []model.CompiledActionStep{}
	if len(config.UserActions) > 0 {
		// 运行详情与启动使用同一套状态编译器；编译失败时退回历史快照，避免旧运行画面直接空白。
		if compiled, compileErr := s.compileRunSteps(ctx, run.PlanID, executionPathID, path, config.UserActions); compileErr == nil {
			compiledSteps = compiled
		}
	}
	if len(compiledSteps) == 0 && len(config.CompiledSteps) > 0 {
		if err := json.Unmarshal(config.CompiledSteps, &compiledSteps); err == nil {
		} else {
			return nil, choices, nil
		}
	}
	if len(compiledSteps) == 0 {
		return nil, choices, nil
	}
	keys := make([]string, 0, len(compiledSteps))
	for _, compiled := range compiledSteps {
		keys = append(keys, compiled.NodeKey)
	}
	return keys, choices, compiledSteps
}

// buildNodePlans 把编译场景按图节点 ID 归组成节点计划。
// 令牌键翻译不出图节点 ID 时（结构变化等）保留原键兜底，绝不悄悄丢掉这个节点的计划。
func buildNodePlans(compiledSteps []model.CompiledActionStep, tokenToGraphID map[string]string) map[string][]RunNodePlanActionDTO {
	plans := map[string][]RunNodePlanActionDTO{}
	for _, compiled := range compiledSteps {
		key := tokenToGraphID[compiled.NodeKey]
		if key == "" {
			key = compiled.NodeKey
		}
		plans[key] = append(plans[key], RunNodePlanActionDTO{
			Sequence:        compiled.Sequence,
			ReleaseGroup:    compiled.ReleaseGroup,
			ReleaseRequired: compiled.ReleaseRequired,
			ActionName:      actionNameOf(string(compiled.Action)),
			Action:          string(compiled.Action),
			SourceName:      actionStepSourceName(compiled.Source),
			ScopeName:       actionScopeName(compiled.Scope),
			Precondition:    compiled.Precondition,
			ExpectedEffect:  compiled.ExpectedEffect,
			StopOnFailure:   compiled.StopOnFailure,
			RecoveryPolicy:  compiled.RecoveryPolicy,
			ReloadRequired:  compiled.ReloadRequired,
			ParameterCount:  len(compiled.Parameters),
		})
	}
	return plans
}

// actionStepSourceName 返回场景步骤来源的中文名：界面不显示内部枚举值。
func actionStepSourceName(source model.ActionStepSource) string {
	switch source {
	case model.ActionStepSourceUser:
		return "用户配置"
	case model.ActionStepSourceRecovery:
		return "系统恢复"
	case model.ActionStepSourceNavigation:
		return "系统导航"
	case model.ActionStepSourceSystemDefault:
		return "固定尾动作"
	default:
		return "来源未知"
	}
}

// actionScopeName 返回动作作用范围的中文名：界面不显示内部枚举值。
func actionScopeName(scope model.ActionScope) string {
	switch scope {
	case model.ActionScopeInitiator:
		return "发起实例"
	case model.ActionScopeTask:
		return "当前待办"
	case model.ActionScopeCompletedTask:
		return "已办任务"
	case model.ActionScopeInstance:
		return "实例管理"
	default:
		return "范围未知"
	}
}

// buildNodeStates 推导画布节点的九个中文运行态：
// 已落账步骤的节点已完成；失败/结果待确认的收尾节点单独标出；当前步节点运行中；
// 已配置路线上尚未到达的节点等待运行；路线外节点未开始。状态不只靠颜色，界面必须渲染中文。
func buildNodeStates(graph model.FlowGraph, steps []model.RunStep, pathRun model.PathRun, preview *RunPreviewDTO, configuredNodeKeys []string) map[string]RunNodeStateDTO {
	tokenToGraphID := tokenToGraphNodeID(graph)
	states := map[string]RunNodeStateDTO{}
	for _, node := range graph.Nodes {
		states[node.ID] = RunNodeStateDTO{Status: string(model.PathRunStatusNotStarted), StatusName: model.PathRunStatusName(model.PathRunStatusNotStarted)}
	}
	// graphNodeKey 把令牌键翻译回图节点 ID；翻译不出（结构变化等）保留原键兜底，绝不丢状态。
	graphNodeKey := func(key string) string {
		if mapped := tokenToGraphID[key]; mapped != "" {
			return mapped
		}
		return key
	}
	settled := map[string]bool{}
	for _, stepRecord := range steps {
		key := graphNodeKey(stepRecord.NodeKey)
		settled[key] = true
		states[key] = nodeState(model.PathRunStatusCompleted)
	}
	// 收尾节点：失败或结果待确认时把最后一步的节点标成对应状态。
	if pathRun.FailureClass != nil {
		last := graphNodeKey(lastNodeOf(steps))
		switch *pathRun.FailureClass {
		case model.FailureClassWriteUncertain:
			if last != "" {
				states[last] = nodeState(model.PathRunStatusAwaitingReconciliation)
			}
		case model.FailureClassGateBlocked, model.FailureClassActorUnresolved, model.FailureClassTargetRejected, model.FailureClassToolBug:
			if last != "" {
				states[last] = nodeState(model.PathRunStatusFailed)
			}
		}
	}
	if pathRun.Status == model.PathRunStatusStopped {
		if last := graphNodeKey(lastNodeOf(steps)); last != "" {
			states[last] = nodeState(model.PathRunStatusStopped)
		}
	}
	// 场景内尚未到达的节点：等待运行。依据是已配置路线的节点序列（评审缺陷 5 的修复点）——
	// 旧实现遍历已落账步骤并检查自身是否未落账，条件恒不成立，「等待运行」从未出现过。
	configured := map[string]bool{}
	for _, nodeKey := range configuredNodeKeys {
		configured[graphNodeKey(nodeKey)] = true
	}
	for nodeID, state := range states {
		if configured[nodeID] && !settled[nodeID] && state.Status == string(model.PathRunStatusNotStarted) {
			states[nodeID] = nodeState(model.PathRunStatusWaiting)
		}
	}
	if preview != nil && preview.NodeKey != "" && pathRun.Status == model.PathRunStatusRunning {
		states[graphNodeKey(preview.NodeKey)] = nodeState(model.PathRunStatusRunning)
	}
	return states
}

// tokenToGraphNodeID 从真实结构推导「配置令牌键 -> 图节点 ID」映射：
// 编译场景、步骤事实与预览都用令牌键，而画布与侧栏以图节点 ID 为准，键空间必须在这里对齐。
func tokenToGraphNodeID(graph model.FlowGraph) map[string]string {
	mapping := make(map[string]string, len(graph.Nodes))
	for _, node := range graph.Nodes {
		mapping[analyzer.PathConfigNodeToken(node.ID)] = node.ID
	}
	return mapping
}

// lastNodeOf 返回最后一步所在的节点键。
func lastNodeOf(steps []model.RunStep) string {
	if len(steps) == 0 {
		return ""
	}
	return steps[len(steps)-1].NodeKey
}

// nodeState 生成节点运行态。
func nodeState(status model.PathRunStatus) RunNodeStateDTO {
	return RunNodeStateDTO{Status: string(status), StatusName: model.PathRunStatusName(status)}
}

// readPhaseTimings 从 step.log 的阶段时间轴计算每个尝试的七阶段耗时（毫秒）。
// 耗时来自日志行的 time=（服务端记录的事实），不做界面估算；
// key 为 step_id:attempt（评审缺陷 5 的修复点）；phase 耗时 = 下一阶段开始时间 − 本阶段开始时间。
func (s *RunOrchestrationService) readPhaseTimings(pathRunID uint64, attempts []model.RunStepAttempt) map[string]map[string]int64 {
	result := map[string]map[string]int64{}
	logPath := ""
	for _, attempt := range attempts {
		if attempt.LogPath != "" {
			logPath = attempt.LogPath
			break
		}
	}
	if logPath == "" || s.router == nil {
		return result
	}
	file, err := os.Open(filepath.Join(s.router.Root(), filepath.FromSlash(logPath)))
	if err != nil {
		return result
	}
	defer file.Close()
	return parsePhaseTimings(file)
}

// stepPhaseKey 生成阶段耗时表的归组键：step_id:attempt，与 step.log 行内两列一一对应。
func stepPhaseKey(stepNo, attemptNo int) string {
	return strconv.Itoa(stepNo) + ":" + strconv.Itoa(attemptNo)
}

// parsePhaseTimings 从 step.log 内容计算每个尝试的七阶段耗时（毫秒）。
// 归组键固定为 step_id:attempt：plan..prepare 各阶段行先于写请求、天生没有 trace_id，
// 若按 trace_id 归组会把同一次尝试的行拆到两个键里，界面将永远拿不到完整阶段耗时
// （评审缺陷 5）；写请求之后的行另带 trace_id/curl_trace_id，只用于跨日志互查，不参与归组。
func parsePhaseTimings(rd io.Reader) map[string]map[string]int64 {
	result := map[string]map[string]int64{}
	type phaseMoment struct {
		phase string
		at    time.Time
	}
	type timeline struct {
		steps map[string][]phaseMoment
	}
	timelines := map[string]*timeline{}
	scanner := bufio.NewScanner(rd)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		fields := parseLogLine(scanner.Text())
		stepID := fields["step_id"]
		phase := fields["phase"]
		at, err := time.ParseInLocation("2006-01-02_15:04:05", fields["time"], time.Local)
		if stepID == "" || phase == "" || err != nil {
			continue
		}
		key := stepID + ":" + fields["attempt"]
		entry, ok := timelines[key]
		if !ok {
			entry = &timeline{steps: map[string][]phaseMoment{}}
			timelines[key] = entry
		}
		entry.steps[stepID] = append(entry.steps[stepID], phaseMoment{phase: phase, at: at})
	}
	phaseOrder := []string{"plan", "gate", "control", "prepare", "submit", "verify", "settle"}
	for key, entry := range timelines {
		for _, moments := range entry.steps {
			sort.Slice(moments, func(i, j int) bool { return moments[i].at.Before(moments[j].at) })
			seen := map[string]bool{}
			durations := map[string]int64{}
			for index, moment := range moments {
				if seen[moment.phase] {
					continue
				}
				seen[moment.phase] = true
				if index+1 < len(moments) {
					durations[moment.phase] = moments[index+1].at.Sub(moment.at).Milliseconds()
				} else {
					durations[moment.phase] = 0
				}
			}
			ordered := map[string]int64{}
			for _, phase := range phaseOrder {
				if value, ok := durations[phase]; ok {
					ordered[phase] = value
				}
			}
			if len(ordered) > 0 {
				result[key] = ordered
			}
		}
	}
	return result
}

// curlBlockFor 从本次运行目录的 curl.log 提取指定 trace_id 的完整请求块（begin 到 end），与日志逐字同源。
// 运行目录由尝试记录里的 step.log 相对路径推导（step.log 与三个网络日志同目录）——
// 绝不扫描全部计划与运行目录：详情页轮询会随历史运行数量线性变慢（评审缺陷 11）。
func curlBlockFor(router *logging.Router, traceID, stepLogPath string) string {
	if router == nil || traceID == "" || stepLogPath == "" {
		return ""
	}
	runDir := filepath.Join(router.Root(), filepath.Dir(filepath.FromSlash(stepLogPath)))
	matches, err := filepath.Glob(filepath.Join(runDir, "curl.log*"))
	if err != nil {
		return ""
	}
	// 目录里至多两三个 curl 日志文件，逐个扫描直到命中 trace_id；找不到就返回空，界面给中文说明。
	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if block := extractCurlBlock(string(content), traceID); block != "" {
			return block
		}
	}
	return ""
}

// extractCurlBlock 在日志文本里定位指定 trace 的 curl 块。
func extractCurlBlock(content, traceID string) string {
	begin := "--- begin curl trace_id=" + traceID + " ---"
	end := "--- end curl trace_id=" + traceID + " ---"
	start := strings.Index(content, begin)
	if start < 0 {
		return ""
	}
	stop := strings.Index(content[start:], end)
	if stop < 0 {
		return ""
	}
	return content[start : start+stop+len(end)]
}

// parseLogLine 解析统一单行日志的 key=value 字段。
func parseLogLine(line string) map[string]string {
	fields := map[string]string{}
	for _, part := range strings.Fields(line) {
		if index := strings.Index(part, "="); index > 0 {
			fields[part[:index]] = part[index+1:]
		}
	}
	return fields
}

// actionNameOf 返回动作的中文名（落账事实里只有动作键）。
// F-034 T03：未知键不再原样透出英文稳定键，统一用安全中文占位，原始键只能去日志里查。
func actionNameOf(action string) string {
	if name := model.ActionChineseName(model.ActionKey(action)); name != "" {
		return name
	}
	return "未识别动作"
}

// stepStatusName 返回步骤事实状态的中文显示名。
func stepStatusName(status model.RunStepStatus) string {
	switch status {
	case model.RunStepSucceeded:
		return "执行成功"
	case model.RunStepFailed:
		return "执行失败"
	case model.RunStepUncertain:
		return "结果待确认"
	case model.RunStepSkipped:
		return "已跳过"
	default:
		return string(status)
	}
}

// verdictName 返回三值结论的中文显示名。
func verdictName(verdict string) string {
	switch verdict {
	case "confirmed_success":
		return "执行成功"
	case "confirmed_failure":
		return "执行失败"
	case "uncertain":
		return "结果待确认"
	case step.StepVerdictTargetSkipped:
		return "目标跳过"
	default:
		return verdict
	}
}

// resultName 返回路径结果的中文显示名。
func resultName(result model.RunResult) string {
	switch result {
	case model.RunResultSucceeded:
		return "成功"
	case model.RunResultFailed:
		return "失败"
	case model.RunResultAwaitingReconcile:
		return "结果待确认"
	default:
		return string(result)
	}
}

// previewDTO 把执行器的下一步预览转为公开形态。
func previewDTO(preview *step.StepPreview) *RunPreviewDTO {
	if preview == nil {
		return nil
	}
	facts := map[string]any{
		"instanceFound":  preview.Facts.Found,
		"instanceStatus": preview.Facts.Status,
		"currentNodes":   preview.Facts.CurrentNodes,
		"dueNodes":       preview.Facts.DueNodes,
		// 任务级步骤的待办来自实际处理人会话，必须单独告诉界面，不能只用计划账号视角的 dueNodes 判断“无待办”。
		"currentTaskFound":        preview.Facts.CurrentTaskFound,
		"currentTaskAssigneeName": preview.Facts.CurrentTaskAssigneeName,
	}
	if preview.Facts.ReadError != "" {
		facts["readError"] = preview.Facts.ReadError
	}
	return &RunPreviewDTO{
		StepNo: preview.StepNo, TotalSteps: preview.TotalSteps,
		ReleaseGroup: preview.ReleaseGroup, ReleaseRequired: preview.ReleaseRequired,
		// F-034 T03：预览名称直接透传会在导航/历史场景出现空名或原始动作键；
		// 这里统一经过集中中文映射，已知动作键始终返回中文名。
		Action: string(preview.Action), ActionName: actionNameOf(string(preview.Action)),
		NodeKey: preview.NodeKey, NodeName: preview.NodeName,
		ActorName: preview.ActorName, ExpectedEffect: preview.ExpectedEffect,
		Endpoint: preview.Endpoint, RequestPreview: preview.RequestPreview,
		GateAllowed: preview.GateAllowed, GateReason: preview.GateReason,
		GateItems: preview.GateItems, Facts: facts, BlockReason: preview.BlockReason,
	}
}

// BuildNodePlansForTest 暴露节点计划归组，供 test 目录下的定向用例锁定键空间翻译与中文口径。
func BuildNodePlansForTest(compiledSteps []model.CompiledActionStep, tokenToGraphID map[string]string) map[string][]RunNodePlanActionDTO {
	return buildNodePlans(compiledSteps, tokenToGraphID)
}

// BuildStepDTOsForTest 暴露步骤事实映射，供 test 目录锁定稳定动作键与中文名称同时返回。
func BuildStepDTOsForTest(steps []model.RunStep, attempts []model.RunStepAttempt) []RunStepDTO {
	return buildStepDTOs(steps, attempts, nil, nil, nil, nil, nil)
}

// BuildNodeStatesForTest 暴露画布节点运行态推导，供 test 目录下的定向用例锁定「等待运行」语义。
func BuildNodeStatesForTest(graph model.FlowGraph, steps []model.RunStep, pathRun model.PathRun, preview *RunPreviewDTO, configuredNodeKeys []string) map[string]RunNodeStateDTO {
	return buildNodeStates(graph, steps, pathRun, preview, configuredNodeKeys)
}

// RunDetailGraphTimeoutForTest 暴露详情结构读取预算，供 test 目录锁定正常目标读取不被过短预算截断。
func RunDetailGraphTimeoutForTest() time.Duration {
	return runDetailGraphTimeout
}

// ParsePhaseTimingsForTest 暴露 step.log 阶段时间轴解析，供 test 目录下的定向用例锁定归组键与耗时口径。
func ParsePhaseTimingsForTest(rd io.Reader) map[string]map[string]int64 {
	return parsePhaseTimings(rd)
}

// LastAttemptWasPreRejectedForTest 暴露阻塞投影判据，供 test 目录锁定 F-034 行为。
func LastAttemptWasPreRejectedForTest(attempts []model.RunStepAttempt) bool {
	return lastAttemptWasPreRejected(attempts)
}

// LastPreRejectedStepNoForTest 暴露最后一次前置拒绝的步骤定位，供 test 目录锁定跨步骤尝试顺序。
func LastPreRejectedStepNoForTest(attempts []model.RunStepAttempt) int {
	return lastPreRejectedStepNo(attempts)
}
