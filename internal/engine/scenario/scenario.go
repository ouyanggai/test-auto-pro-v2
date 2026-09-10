package scenario

import (
	"fmt"
	"sort"
	"strings"

	"test-auto-pro-v2/internal/model"
)

const (
	// MaxActions 是单路径允许保存的独立动作记录上限；重复动作仍必须逐条保存。
	MaxActions = 100
)

// Input 是场景编译所需的语义输入；节点键必须是工具侧稳定键，不能携带目标临时 ID。
type Input struct {
	Actions      []model.ConfiguredAction
	Nodes        []model.FlowGraphNode
	NodeSequence []string
	FinalNodeKey string
	Catalog      []model.ActionCatalogItem
}

// Result 是编译后的独立动作和只读步骤；Steps 不代表任何已执行事实。
type Result struct {
	Actions []model.ConfiguredAction
	Steps   []model.CompiledActionStep
	Issues  []model.ActionConfigurationIssue
}

// CompileError 表示保存时发现的首个结构性动作问题。
type CompileError struct {
	Issues []model.ActionConfigurationIssue
}

// Error 返回首个阻断问题，完整问题列表仍保留在 Issues 中供 API 定位。
func (e *CompileError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "动作场景编译失败"
	}
	return e.Issues[0].Message
}

// NewCompiler 创建无状态场景编译器。
func NewCompiler() *Compiler { return &Compiler{} }

// Compiler 只读编译语义动作，不执行任何目标操作。
type Compiler struct{}

// Compile 按用户顺序生成固定尾动作、恢复链和系统导航步骤。
func Compile(input Input) (Result, error) { return NewCompiler().Compile(input) }

// Compile 校验结构性问题，并按实例状态与节点游标生成可持久化动作组。
func (c *Compiler) Compile(input Input) (Result, error) {
	actions, nodes, sequence, err := normalizeInput(input)
	if err == nil {
	} else {
		return Result{}, err
	}
	if finalNode := strings.TrimSpace(input.FinalNodeKey); finalNode != "" && nodeIndex(sequence, finalNode) < 0 {
		return Result{}, &CompileError{Issues: []model.ActionConfigurationIssue{{Code: "FINAL_NODE_UNKNOWN", Message: "最终导航节点不属于当前已核实路径", Blocking: true}}}
	}
	if len(actions) == 0 {
		return Result{Actions: []model.ConfiguredAction{}, Steps: []model.CompiledActionStep{}, Issues: []model.ActionConfigurationIssue{}}, nil
	}
	nodeTypes := make(map[string]string, len(nodes))
	for key, node := range nodes {
		nodeTypes[key] = node.Type
	}
	catalog := catalogIndex(input.Catalog)
	issues := make([]model.ActionConfigurationIssue, 0)
	for index, action := range actions {
		if issue := validateActionStructure(action, index, nodeTypes, catalog); issue == nil {
		} else if issue.Blocking {
			return Result{Actions: actions, Issues: []model.ActionConfigurationIssue{*issue}}, &CompileError{Issues: []model.ActionConfigurationIssue{*issue}}
		} else {
			issues = append(issues, *issue)
		}
		if issue := validateRollbackTargetIssue(action, index, nodeIndex(sequence, action.NodeKey), sequence, nodeTypes); issue == nil {
		} else {
			return Result{Actions: actions, Issues: []model.ActionConfigurationIssue{*issue}}, &CompileError{Issues: []model.ActionConfigurationIssue{*issue}}
		}
		if issue := validateStateReminder(action, index, actions); issue == nil {
		} else {
			issues = append(issues, *issue)
		}
	}
	compiler := &compiler{nodeTypes: nodeTypes, sequence: sequence, status: "new", completed: map[string]bool{}, cursor: -1}
	for _, action := range actions {
		if isFixedTailAction(action.Action) {
			continue
		}
		actionIndex := nodeIndex(sequence, action.NodeKey)
		if actionIndex >= 0 && compiler.cursor < actionIndex {
			compiler.advanceForward(actionIndex)
		}
		if actionIndex >= 0 {
			compiler.cursor = actionIndex
		} else {
			// 实例动作不绑定节点，按旧容器语义排在所有节点用户动作之后、当前节点固定尾动作之前。
			target := compiler.lastHumanIndex()
			if target > compiler.cursor {
				compiler.advanceForward(target)
			}
			if target >= 0 {
				compiler.cursor = target
			}
		}
		compiler.emitUserAction(action, actionIndex)
	}
	compiler.advanceToEnd()
	return Result{Actions: actions, Steps: compiler.steps, Issues: issues}, nil
}

// compiler 保存一次编译中的节点游标、实例状态和动作组游标。
type compiler struct {
	nodeTypes    map[string]string
	sequence     []string
	status       string
	completed    map[string]bool
	followed     bool
	cursor       int
	groupCounter int
	currentGroup string
	steps        []model.CompiledActionStep
}

// beginGroup 开启一个新动作组；固定尾动作与每个用户动作各自成组。
func (c *compiler) beginGroup() {
	c.groupCounter++
	c.currentGroup = fmt.Sprintf("release-group-%d", c.groupCounter)
}

// addGroupedStep 为步骤分配顺序、动作组与首个放行标记。
func (c *compiler) addGroupedStep(step model.CompiledActionStep, releaseRequired bool) {
	if strings.TrimSpace(c.currentGroup) == "" {
		c.beginGroup()
		releaseRequired = true
	}
	step.Sequence = len(c.steps) + 1
	step.ReleaseGroup = c.currentGroup
	step.ReleaseRequired = releaseRequired
	c.steps = append(c.steps, step)
}

// advanceForward 补齐离开当前节点到目标节点之间的人工尾动作和系统导航。
func (c *compiler) advanceForward(target int) {
	if target < 0 {
		return
	}
	start := c.cursor
	if start < 0 {
		start = 0
	}
	for index := start; index < target; index++ {
		c.emitNodeCompletion(index)
	}
	c.cursor = target
}

// advanceToEnd 从当前游标收尾到路径末端，保证每个经过的人工节点都有固定同意写步骤。
func (c *compiler) advanceToEnd() {
	start := c.cursor
	if start < 0 {
		start = 0
	}
	for index := start; index < len(c.sequence); index++ {
		c.emitNodeCompletion(index)
	}
	if len(c.sequence) > 0 {
		c.cursor = len(c.sequence) - 1
	}
}

// emitNodeCompletion 在离开节点时补齐固定尾动作或系统节点只读导航。
func (c *compiler) emitNodeCompletion(index int) {
	if index < 0 || index >= len(c.sequence) {
		return
	}
	nodeKey := c.sequence[index]
	nodeType := strings.ToLower(strings.TrimSpace(c.nodeTypes[nodeKey]))
	switch {
	case nodeType == "start":
		c.emitFixedStartTail(nodeKey)
	case nodeType == "common" || nodeType == "synergy":
		c.emitFixedTaskTail(nodeKey)
	case isAutomaticNode(nodeType):
		c.addGroupedStep(navigationStep(nodeKey, nodeType), false)
	default:
	}
	c.cursor = index
}

// emitFixedStartTail 输出发起节点末尾的固定提交；已有草稿、驳回或撤回时改为重新提交。
func (c *compiler) emitFixedStartTail(nodeKey string) {
	c.beginGroup()
	action := model.ActionSubmit
	if c.status == "draft" || c.status == "rejected" || c.status == "withdraw" {
		action = model.ActionResubmit
	}
	c.addGroupedStep(model.CompiledActionStep{
		Source: model.ActionStepSourceSystemDefault, Action: action, Scope: model.ActionScopeInitiator, NodeKey: nodeKey,
		Precondition:   "发起节点用户动作已完成，按当前实例状态选择提交或重新提交",
		ExpectedEffect: expectedEffect(action), StopOnFailure: "提交门禁不满足时停止，不创建第二主实例",
		RecoveryPolicy: "重新读取实例状态、流程代理和当前路径", ReloadRequired: true,
	}, true)
	c.status = "run"
	c.completed = map[string]bool{}
}

// emitFixedTaskTail 输出审批/协同节点末尾的默认同意；它始终排在该节点所有用户动作之后。
func (c *compiler) emitFixedTaskTail(nodeKey string) {
	c.beginGroup()
	c.addGroupedStep(model.CompiledActionStep{
		Source: model.ActionStepSourceSystemDefault, Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: nodeKey,
		Precondition:   "当前人工节点所有用户动作已执行，且该节点仍有活动待办",
		ExpectedEffect: expectedEffect(model.ActionApprove), StopOnFailure: "当前待办门禁不满足时停止，不切换到其他演员",
		RecoveryPolicy: "重新读取实例状态、当前待办和真实路径", ReloadRequired: true,
	}, true)
	c.status = "run"
	c.completed[nodeKey] = true
}

// emitUserAction 输出一条用户动作，并在动作离开节点时补齐恢复链。
func (c *compiler) emitUserAction(action model.ConfiguredAction, actionIndex int) {
	c.beginGroup()
	prepRequired := false
	if action.Action == model.ActionRetrieve && c.completed[action.NodeKey] == false {
		c.addGroupedStep(recoveryApproveStep(action, "取回前没有可取回已办，先生成当前节点已办"), true)
		c.completed[action.NodeKey] = true
		prepRequired = true
	}
	c.addGroupedStep(userStep(action), prepRequired == false)
	switch action.Action {
	case model.ActionSaveDraft:
		c.status = "draft"
	case model.ActionSubmit, model.ActionResubmit:
		c.status = "run"
	case model.ActionReject:
		c.status = "rejected"
		c.emitRecoveryFromStart(actionIndex, action.Key)
	case model.ActionWithdraw:
		target := c.cursor
		if target < 0 {
			target = c.lastHumanIndex()
		}
		c.status = "withdraw"
		c.emitRecoveryFromStart(target, action.Key)
	case model.ActionRollback:
		c.emitRollbackRecovery(actionIndex, action.Key)
	case model.ActionRetrieve:
		c.completed[action.NodeKey] = false
	case model.ActionApprove:
		c.completed[action.NodeKey] = true
	case model.ActionFollow:
		c.followed = true
	case model.ActionUnfollow:
		c.followed = false
	}
	c.cursor = actionIndex
	if actionIndex < 0 {
		c.cursor = c.lastHumanIndex()
	}
}

// emitRecoveryFromStart 生成“重新提交 → 沿已选路径逐个人工节点同意 → 返回原节点”的恢复链。
func (c *compiler) emitRecoveryFromStart(target int, triggerKey string) {
	if target < 0 {
		target = c.lastHumanIndex()
	}
	if target < 0 {
		target = 0
	}
	start := c.firstHumanIndex()
	if start < 0 {
		start = 0
	}
	c.addGroupedStep(model.CompiledActionStep{
		Source: model.ActionStepSourceRecovery, SourceActionKey: triggerKey,
		Action: model.ActionResubmit, Scope: model.ActionScopeInitiator, NodeKey: c.sequence[start],
		Precondition: "实例已被驳回或撤回，等待发起人重新提交", ExpectedEffect: expectedEffect(model.ActionResubmit),
		StopOnFailure: "恢复门禁不满足时停止并定位触发动作", RecoveryPolicy: "不创建第二主实例，按目标事实重新读取", ReloadRequired: true,
	}, false)
	c.status = "run"
	c.completed = map[string]bool{}
	for index := start + 1; index < target; index++ {
		c.emitRecoveryPass(index, triggerKey)
	}
	c.cursor = target
}

// emitRollbackRecovery 生成从真实前驱人工节点返回到原节点的恢复链。
func (c *compiler) emitRollbackRecovery(target int, triggerKey string) {
	previous := previousHumanIndex(target, c.sequence, c.nodeTypes)
	if previous < 0 {
		return
	}
	c.addGroupedStep(recoveryApproveStep(model.ConfiguredAction{Key: triggerKey, NodeKey: c.sequence[previous]}, "回退到真实前驱节点后重新处理"), false)
	c.completed[c.sequence[previous]] = true
	for index := previous + 1; index < target; index++ {
		c.emitRecoveryPass(index, triggerKey)
	}
	c.cursor = target
}

// emitRecoveryPass 在恢复链经过人工节点时发出真实同意，经过系统节点时只读导航。
func (c *compiler) emitRecoveryPass(index int, triggerKey string) {
	if index < 0 || index >= len(c.sequence) {
		return
	}
	nodeKey := c.sequence[index]
	nodeType := strings.ToLower(strings.TrimSpace(c.nodeTypes[nodeKey]))
	switch {
	case nodeType == "common" || nodeType == "synergy":
		c.addGroupedStep(recoveryApproveStep(model.ConfiguredAction{Key: triggerKey, NodeKey: nodeKey}, "沿已选路径恢复到原节点"), false)
		c.completed[nodeKey] = true
	case isAutomaticNode(nodeType):
		c.addGroupedStep(navigationStep(nodeKey, nodeType), false)
	default:
	}
}

// firstHumanIndex 返回路径上第一个发起或人工节点；没有人工节点时返回 -1。
func (c *compiler) firstHumanIndex() int {
	for index, key := range c.sequence {
		if isHumanNode(c.nodeTypes[key]) {
			return index
		}
	}
	return -1
}

// lastHumanIndex 返回路径上最后一个发起或人工节点；没有人工节点时返回 -1。
func (c *compiler) lastHumanIndex() int {
	for index := len(c.sequence) - 1; index >= 0; index-- {
		if isHumanNode(c.nodeTypes[c.sequence[index]]) {
			return index
		}
	}
	return -1
}

// recoveryApproveStep 创建恢复链经过人工节点时的真实同意步骤。
func recoveryApproveStep(action model.ConfiguredAction, precondition string) model.CompiledActionStep {
	return model.CompiledActionStep{
		Source: model.ActionStepSourceRecovery, SourceActionKey: action.Key,
		Action: model.ActionApprove, Scope: model.ActionScopeTask, NodeKey: action.NodeKey,
		Precondition: precondition, ExpectedEffect: expectedEffect(model.ActionApprove),
		StopOnFailure:  "恢复门禁不满足时停止并定位触发动作",
		RecoveryPolicy: "不创建第二主实例，按目标事实重新读取", ReloadRequired: true,
	}
}

// isHumanNode 判断节点是否需要真实人工写操作：发起、审批和协同都属于人工节点。
func isHumanNode(nodeType string) bool {
	switch strings.ToLower(strings.TrimSpace(nodeType)) {
	case "start", "common", "synergy":
		return true
	default:
		return false
	}
}

// isFixedTailAction 判断动作是否由固定尾动作承载；旧配置中的显式记录也会归一化。
func isFixedTailAction(action model.ActionKey) bool {
	switch action {
	case model.ActionSubmit, model.ActionApprove, model.ActionResubmit:
		return true
	default:
		return false
	}
}

// previousHumanIndex 跳过自动节点，定位指定位置之前最近的人工或发起节点。
func previousHumanIndex(position int, sequence []string, nodeTypes map[string]string) int {
	if position > len(sequence) {
		position = len(sequence)
	}
	for index := position - 1; index >= 0; index-- {
		if isHumanNode(nodeTypes[sequence[index]]) {
			return index
		}
	}
	return -1
}

// validateActionStructure 校验动作结构、作用域、节点与不可伪造参数；实时门禁只做非阻断提醒。
func validateActionStructure(action model.ConfiguredAction, index int, nodes map[string]string, catalog map[catalogKey]model.ActionCatalogItem) *model.ActionConfigurationIssue {
	if action.Action == "" || action.Action == model.ActionSystemAutomatic {
		return blockingIssue(index, action, "ACTION_NOT_CONFIGURABLE", "系统自动语义只能只读展示，不能作为用户动作保存")
	}
	wantScope, known := actionScope(action.Action)
	if known == false {
		return blockingIssue(index, action, "ACTION_UNKNOWN", "动作不属于当前目标动作目录")
	}
	if action.Scope != wantScope {
		return blockingIssue(index, action, "ACTION_SCOPE_INVALID", "动作作用域与目标动作目录不一致")
	}
	if action.Scope == model.ActionScopeInstance && strings.TrimSpace(action.NodeKey) != "" {
		return blockingIssue(index, action, "ACTION_NODE_INVALID", "实例动作不能绑定语义节点")
	}
	if action.Scope != model.ActionScopeInstance && strings.TrimSpace(action.NodeKey) == "" {
		return blockingIssue(index, action, "ACTION_NODE_REQUIRED", "节点动作必须绑定语义节点键")
	}
	if action.NodeKey != "" {
		nodeType, exists := nodes[action.NodeKey]
		if exists == false {
			return blockingIssue(index, action, "UNKNOWN_NODE", "动作节点不属于当前已核实路径")
		}
		if (action.Scope == model.ActionScopeTask || action.Scope == model.ActionScopeCompletedTask) && nodeType != "common" && nodeType != "synergy" {
			return blockingIssue(index, action, "ACTION_NODE_TYPE_INVALID", "当前节点不是可处理的人工审批或协同节点")
		}
		if action.Scope == model.ActionScopeInitiator && nodeType != "start" {
			return blockingIssue(index, action, "ACTION_NODE_TYPE_INVALID", "发起生命周期动作只能绑定发起节点")
		}
	}
	if action.Action == model.ActionTransfer && action.ActorPolicy == "" {
		return blockingIssue(index, action, "ACTOR_POLICY_REQUIRED", "移交动作必须明确目标演员策略")
	}
	if parameter := forbiddenParameterPath(action.Parameters, ""); parameter != "" {
		return blockingIssue(index, action, "ACTION_PARAMETER_TARGET_ID", "动作参数不能携带目标实例、任务、代理或人员临时标识："+parameter)
	}
	if len(catalog) == 0 || isFixedTailAction(action.Action) {
		return nil
	}
	catalogItem, exists := catalogGate(catalog, action)
	if exists == false {
		return advisoryIssue(index, action, "ACTION_NOT_IN_CATALOG", "当前实时动作目录未返回该动作，运行时将按目标事实复验")
	}
	if catalogItem.Enabled == false {
		reason := strings.TrimSpace(catalogItem.DisabledReason)
		if reason == "" {
			reason = "当前动作门禁未通过"
		}
		return advisoryIssue(index, action, "ACTION_DISABLED", reason)
	}
	return nil
}

// validateRollbackTargetIssue 阻止把回退动作配置到没有可恢复前驱的节点。
func validateRollbackTargetIssue(action model.ConfiguredAction, index, position int, sequence []string, nodeTypes map[string]string) *model.ActionConfigurationIssue {
	if action.Action != model.ActionRollback || position < 0 || position >= len(sequence) {
		return nil
	}
	if position == 0 {
		return blockingIssue(index, action, "ROLLBACK_PREVIOUS_MISSING", "当前节点没有目标引擎解析出的直接前一待办，不能回退")
	}
	previous := previousHumanIndex(position, sequence, nodeTypes)
	if previous < 0 || strings.EqualFold(strings.TrimSpace(nodeTypes[sequence[previous]]), "start") {
		return blockingIssue(index, action, "ROLLBACK_PREVIOUS_START", "直接前一节点是发起节点，目标规则不允许回退")
	}
	return nil
}

// validateStateReminder 把顺序相关的运行时变化降级为非阻断提醒。
func validateStateReminder(action model.ConfiguredAction, index int, actions []model.ConfiguredAction) *model.ActionConfigurationIssue {
	if action.Action != model.ActionFollow && action.Action != model.ActionUnfollow {
		return nil
	}
	seenFollow := false
	for _, previous := range actions {
		if previous.Order >= action.Order {
			break
		}
		switch previous.Action {
		case model.ActionFollow:
			seenFollow = true
		case model.ActionUnfollow:
			seenFollow = false
		}
	}
	switch action.Action {
	case model.ActionUnfollow:
		if seenFollow == false {
			return advisoryIssue(index, action, "ACTION_UNFOLLOW_WITHOUT_FOLLOW", "当前顺序没有可引用的关注动作，运行时将按目标实时关注状态裁决")
		}
	case model.ActionFollow:
		if seenFollow {
			return advisoryIssue(index, action, "ACTION_FOLLOW_ALREADY_ACTIVE", "当前顺序重复关注，运行时将按目标实时关注状态裁决")
		}
	}
	return nil
}

// blockingIssue 构造结构性阻断问题。
func blockingIssue(index int, action model.ConfiguredAction, code, message string) *model.ActionConfigurationIssue {
	return &model.ActionConfigurationIssue{Index: index, ActionKey: action.Action, ActionID: action.Key, Code: code, Message: message, Blocking: true}
}

// advisoryIssue 构造运行时复验用的非阻断提醒。
func advisoryIssue(index int, action model.ConfiguredAction, code, message string) *model.ActionConfigurationIssue {
	return &model.ActionConfigurationIssue{Index: index, ActionKey: action.Action, ActionID: action.Key, Code: code, Message: message, Blocking: false}
}
func normalizeInput(input Input) ([]model.ConfiguredAction, map[string]model.FlowGraphNode, []string, error) {
	if len(input.Actions) > MaxActions {
		return nil, nil, nil, &CompileError{Issues: []model.ActionConfigurationIssue{{Index: MaxActions, Code: "ACTION_LIMIT", Message: "动作数量不能超过 100 条", Blocking: true}}}
	}
	actions := make([]model.ConfiguredAction, len(input.Actions))
	copy(actions, input.Actions)
	seenKeys := make(map[string]bool, len(actions))
	seenOrders := make(map[int]bool, len(actions))
	for index := range actions {
		action := &actions[index]
		action.Key = strings.TrimSpace(action.Key)
		action.NodeKey = strings.TrimSpace(action.NodeKey)
		action.ActorPolicy = strings.TrimSpace(action.ActorPolicy)
		if action.Key == "" {
			return nil, nil, nil, &CompileError{Issues: []model.ActionConfigurationIssue{{Index: index, Code: "ACTION_KEY_REQUIRED", Message: "动作记录缺少稳定键", Blocking: true}}}
		}
		if seenKeys[action.Key] {
			return nil, nil, nil, &CompileError{Issues: []model.ActionConfigurationIssue{{Index: index, ActionID: action.Key, Code: "ACTION_KEY_DUPLICATE", Message: "动作稳定键不能重复，请重新生成该条记录", Blocking: true}}}
		}
		seenKeys[action.Key] = true
		if action.Order <= 0 {
			action.Order = index + 1
		}
		if seenOrders[action.Order] {
			return nil, nil, nil, &CompileError{Issues: []model.ActionConfigurationIssue{{Index: index, ActionKey: action.Action, ActionID: action.Key, Code: "ACTION_ORDER_DUPLICATE", Message: "动作顺序不能重复，请拖拽后重新保存", Blocking: true}}}
		}
		seenOrders[action.Order] = true
		action.Parameters = cloneParameters(action.Parameters)
	}
	sort.SliceStable(actions, func(i, j int) bool { return actions[i].Order < actions[j].Order })
	for index := range actions {
		actions[index].Order = index + 1
	}
	nodes := make(map[string]model.FlowGraphNode, len(input.Nodes))
	for _, node := range input.Nodes {
		key := strings.TrimSpace(node.ID)
		if key == "" || nodes[key].ID != "" {
			if key == "" {
				continue
			}
			return nil, nil, nil, &CompileError{Issues: []model.ActionConfigurationIssue{{Code: "NODE_DUPLICATE", Message: "当前路径节点键不唯一，不能编译动作场景", Blocking: true}}}
		}
		node.ID, node.Type = key, strings.TrimSpace(node.Type)
		nodes[key] = node
	}
	sequence := append([]string(nil), input.NodeSequence...)
	if len(sequence) == 0 {
		for _, node := range input.Nodes {
			sequence = append(sequence, strings.TrimSpace(node.ID))
		}
	}
	seenNodes := make(map[string]bool, len(sequence))
	for index, key := range sequence {
		sequence[index] = strings.TrimSpace(key)
		if sequence[index] == "" || seenNodes[sequence[index]] {
			return nil, nil, nil, &CompileError{Issues: []model.ActionConfigurationIssue{{Code: "NODE_SEQUENCE_INVALID", Message: "当前路径节点顺序不完整，不能编译动作场景", Blocking: true}}}
		}
		if _, exists := nodes[sequence[index]]; !exists {
			return nil, nil, nil, &CompileError{Issues: []model.ActionConfigurationIssue{{Index: index, Code: "NODE_SEQUENCE_UNKNOWN", Message: "当前路径节点顺序包含未核实节点", Blocking: true}}}
		}
		seenNodes[sequence[index]] = true
	}
	return actions, nodes, sequence, nil
}

// validateAction 校验动作键、作用域、节点类型和已知目录门禁，不接受系统动作作为用户配置。
func validateAction(action model.ConfiguredAction, index int, nodes map[string]string, catalog map[catalogKey]model.ActionCatalogItem) *model.ActionConfigurationIssue {
	if action.Action == "" || action.Action == model.ActionSystemAutomatic {
		return issue(index, action, "ACTION_NOT_CONFIGURABLE", "系统自动语义只能只读展示，不能作为用户动作保存")
	}
	wantScope, known := actionScope(action.Action)
	if !known {
		return issue(index, action, "ACTION_UNKNOWN", "动作不属于当前目标动作目录")
	}
	if action.Scope != wantScope {
		return issue(index, action, "ACTION_SCOPE_INVALID", "动作作用域与目标动作目录不一致")
	}
	if action.Scope == model.ActionScopeInstance && strings.TrimSpace(action.NodeKey) != "" {
		return issue(index, action, "ACTION_NODE_INVALID", "实例动作不能绑定语义节点")
	}
	if action.Scope != model.ActionScopeInstance && strings.TrimSpace(action.NodeKey) == "" {
		return issue(index, action, "ACTION_NODE_REQUIRED", "节点动作必须绑定语义节点键")
	}
	if action.NodeKey != "" {
		nodeType, exists := nodes[action.NodeKey]
		if !exists {
			return issue(index, action, "UNKNOWN_NODE", "动作节点不属于当前已核实路径")
		}
		if action.Scope == model.ActionScopeTask && nodeType != "common" && nodeType != "synergy" {
			return issue(index, action, "ACTION_NODE_TYPE_INVALID", "当前节点不是可处理的人工审批或协同节点")
		}
		if action.Scope == model.ActionScopeInitiator && nodeType != "start" {
			return issue(index, action, "ACTION_NODE_TYPE_INVALID", "发起生命周期动作只能绑定发起节点")
		}
	}
	if action.Action == model.ActionTransfer && action.ActorPolicy == "" {
		return issue(index, action, "ACTOR_POLICY_REQUIRED", "移交动作必须明确目标演员策略")
	}
	if parameter := forbiddenParameterPath(action.Parameters, ""); parameter != "" {
		return issue(index, action, "ACTION_PARAMETER_TARGET_ID", "动作参数不能携带目标实例、任务、代理或人员临时标识："+parameter)
	}
	if len(catalog) > 0 {
		catalogItem, exists := catalogGate(catalog, action)
		if !exists {
			return issue(index, action, "ACTION_NOT_IN_CATALOG", "动作未出现在当前实时动作目录，不能绕过门禁保存")
		}
		if !catalogItem.Enabled {
			reason := strings.TrimSpace(catalogItem.DisabledReason)
			if reason == "" {
				reason = "当前动作门禁未通过"
			}
			return issue(index, action, "ACTION_DISABLED", reason)
		}
	}
	return nil
}

// validateActionState 阻止目标状态必然不满足的顺序：没有来源的重新提交与关注状态矛盾的动作。
func validateActionState(action model.ConfiguredAction, index int, resubmitReady, followed bool) *model.ActionConfigurationIssue {
	switch action.Action {
	case model.ActionResubmit:
		if !resubmitReady {
			return issue(index, action, "ACTION_RESUBMIT_WITHOUT_SOURCE", "重新提交必须排在保存草稿、不同意或撤回之后，目标实例才会进入可重新提交状态")
		}
	case model.ActionUnfollow:
		if !followed {
			return issue(index, action, "ACTION_UNFOLLOW_WITHOUT_FOLLOW", "取消关注必须排在关注之后，当前用户尚未关注该实例")
		}
	case model.ActionFollow:
		if followed {
			return issue(index, action, "ACTION_FOLLOW_ALREADY_ACTIVE", "当前用户已经关注该实例，不能重复关注")
		}
	}
	return nil
}

// validateNodeOrder 阻止没有恢复步骤支撑的回跳或移交后隐式切换演员。
func validateNodeOrder(action model.ConfiguredAction, index, nodeIndex, lastNodeIndex int, transferred bool) *model.ActionConfigurationIssue {
	if transferred && action.Scope == model.ActionScopeTask && action.Action != model.ActionTransfer && action.ActorPolicy == "" {
		return issue(index, action, "ACTOR_CONTINUITY_UNKNOWN", "移交后后续动作必须显式指定可回读的演员策略")
	}
	if nodeIndex >= 0 && lastNodeIndex >= 0 && nodeIndex < lastNodeIndex && action.Action != model.ActionRollback && action.Action != model.ActionRetrieve && action.Action != model.ActionResubmit {
		return issue(index, action, "ACTION_PATH_BACKTRACK", "动作回到前序节点但没有回退或取回恢复步骤")
	}
	return nil
}

// validateRollbackTarget 阻止把回退动作配置到没有可恢复前驱的发起节点。
func validateRollbackTarget(action model.ConfiguredAction, index, position int, sequence []string, nodeTypes map[string]string) *model.ActionConfigurationIssue {
	if action.Action != model.ActionRollback || position < 0 || position >= len(sequence) {
		return nil
	}
	if position == 0 {
		return issue(index, action, "ROLLBACK_PREVIOUS_MISSING", "当前节点没有目标引擎解析出的直接前一待办，不能回退")
	}
	previous := previousActionNodeIndex(position, sequence, nodeTypes)
	if previous < 0 || strings.EqualFold(strings.TrimSpace(nodeTypes[sequence[previous]]), "start") {
		return issue(index, action, "ROLLBACK_PREVIOUS_START", "直接前一节点是发起节点，目标规则不允许回退")
	}
	return nil
}

// previousActionNodeIndex 跳过条件、并行和结束等自动节点，定位目标引擎的最近人工前驱。
func previousActionNodeIndex(position int, sequence []string, nodeTypes map[string]string) int {
	if position > len(sequence) {
		position = len(sequence)
	}
	for index := position - 1; index >= 0; index-- {
		switch strings.ToLower(strings.TrimSpace(nodeTypes[sequence[index]])) {
		case "common", "synergy", "start":
			return index
		}
	}
	return -1
}

// actionScope 返回稳定动作键对应的目标作用域。
func actionScope(action model.ActionKey) (model.ActionScope, bool) {
	switch action {
	case model.ActionSaveDraft, model.ActionSubmit, model.ActionResubmit:
		return model.ActionScopeInitiator, true
	case model.ActionStorageFormData, model.ActionAddSign, model.ActionTransfer, model.ActionApprove, model.ActionReject, model.ActionRollback:
		return model.ActionScopeTask, true
	case model.ActionRetrieve:
		return model.ActionScopeCompletedTask, true
	case model.ActionWithdraw, model.ActionUrge, model.ActionForward, model.ActionFollow, model.ActionUnfollow:
		return model.ActionScopeInstance, true
	default:
		return "", false
	}
}

// catalogKey 用语义节点键与动作键共同定位门禁项；实例动作与整路径回退目录使用空节点键。
type catalogKey struct {
	node   string
	action model.ActionKey
}

// catalogIndex 将可选目录复制为门禁索引，不改变目录顺序或持有调用方切片。
func catalogIndex(items []model.ActionCatalogItem) map[catalogKey]model.ActionCatalogItem {
	result := make(map[catalogKey]model.ActionCatalogItem, len(items))
	for _, item := range items {
		result[catalogKey{node: strings.TrimSpace(item.NodeKey), action: item.Action}] = item
	}
	return result
}

// catalogGate 优先按动作所在语义节点取门禁，缺少节点级目录时回退到无节点键目录。
func catalogGate(catalog map[catalogKey]model.ActionCatalogItem, action model.ConfiguredAction) (model.ActionCatalogItem, bool) {
	if item, exists := catalog[catalogKey{node: strings.TrimSpace(action.NodeKey), action: action.Action}]; exists {
		return item, true
	}
	item, exists := catalog[catalogKey{action: action.Action}]
	return item, exists
}

// userStep 将一条语义动作投影为只读用户步骤，永远不表示目标请求已发送。
func userStep(action model.ConfiguredAction) model.CompiledActionStep {
	return model.CompiledActionStep{
		Source: model.ActionStepSourceUser, SourceActionKey: action.Key, Action: action.Action,
		Scope: action.Scope, ActorPolicy: action.ActorPolicy, NodeKey: action.NodeKey,
		Parameters: cloneParameters(action.Parameters), Precondition: "保存时已按当前路径、人员和动作目录核对门禁",
		ExpectedEffect: expectedEffect(action.Action), StopOnFailure: "目标事实重读失败或门禁变化时停止场景",
		RecoveryPolicy: "失败即停止并重读目标实例、任务和语义节点", ReloadRequired: true,
	}
}

// recoveryStep 创建系统恢复步骤，并保留触发它的用户动作键供预览定位。
func recoveryStep(action model.ActionKey, source model.ConfiguredAction, precondition, effect string) model.CompiledActionStep {
	scope, _ := actionScope(action)
	return model.CompiledActionStep{
		Source: model.ActionStepSourceRecovery, SourceActionKey: source.Key, Action: action,
		Scope: scope, ActorPolicy: source.ActorPolicy, NodeKey: source.NodeKey,
		Precondition: precondition, ExpectedEffect: effect,
		StopOnFailure: "恢复门禁不满足时停止并定位触发动作", RecoveryPolicy: "不创建第二主实例，按目标事实重新读取", ReloadRequired: true,
	}
}

// navigationStep 投影条件、并行、空和结束等系统节点的只读导航语义。
func navigationStep(nodeKey, nodeType string) model.CompiledActionStep {
	return model.CompiledActionStep{
		Source: model.ActionStepSourceNavigation, Action: model.ActionSystemAutomatic, Scope: model.ActionScopeInstance,
		NodeKey: nodeKey, Precondition: "目标引擎按当前流程结构自动处理" + nodeType + "节点",
		ExpectedEffect: "沿目标引擎真实自动语义继续导航，不执行工具侧写请求",
		StopOnFailure:  "自动节点事实未知时停止场景", RecoveryPolicy: "重读流程代理和实际路径", ReloadRequired: true,
	}
}

// finalNavigationStep 在所有用户动作后加入主实例最终导航屏障。
func finalNavigationStep(nodeKey string) model.CompiledActionStep {
	return model.CompiledActionStep{
		Source: model.ActionStepSourceNavigation, Action: model.ActionApprove, Scope: model.ActionScopeTask,
		ActorPolicy: "system:navigation", NodeKey: nodeKey,
		Precondition: "所有配置用户动作已完成且当前节点可离开", ExpectedEffect: "按目标真实路径完成最终导航；最终节点结束主实例",
		StopOnFailure: "最终导航门禁不满足时停止，不宣称流程完成", RecoveryPolicy: "重读主实例状态、当前节点和待办", ReloadRequired: true,
	}
}

// expectedEffect 返回固定动作的只读预期，避免编译器生成目标平台未证明的写语义。
func expectedEffect(action model.ActionKey) string {
	switch action {
	case model.ActionSaveDraft:
		return "目标实例保持 draft，不生成待办"
	case model.ActionSubmit, model.ActionResubmit:
		return "目标实例进入 run 并按真实路径解析待办"
	case model.ActionStorageFormData:
		return "保存当前用户、批次和节点的表单检查点，不推进任务"
	case model.ActionAddSign:
		return "更新实例私有代理和追加人员后重读任务映射"
	case model.ActionTransfer:
		return "切换当前待办演员后停留当前节点并重读任务"
	case model.ActionApprove:
		return "按目标引擎推进当前人工待办"
	case model.ActionReject:
		return "按目标规则进入驳回状态并等待发起人重提"
	case model.ActionRollback:
		return "回退到目标真实直接前一待办"
	case model.ActionRetrieve:
		return "在后继未处理时恢复目标已完成任务"
	case model.ActionWithdraw:
		return "按目标规则把运行中主实例置为撤回"
	case model.ActionUrge:
		return "向当前待办接收人发送催办，不移动主实例游标"
	case model.ActionForward:
		return "创建系统默认转发辅助流程，主实例游标不变"
	case model.ActionFollow:
		return "保存当前用户关注状态，不移动主实例游标"
	case model.ActionUnfollow:
		return "清除当前用户关注状态，不移动主实例游标"
	default:
		return "按目标事实继续"
	}
}

// appendStep 为步骤分配持久化预览顺序。
func appendStep(steps []model.CompiledActionStep, step model.CompiledActionStep) []model.CompiledActionStep {
	step.Sequence = len(steps) + 1
	return append(steps, step)
}

// nextAction 读取用户配置的下一条独立动作记录。
func nextAction(actions []model.ConfiguredAction, index int) *model.ConfiguredAction {
	if index < 0 || index+1 >= len(actions) {
		return nil
	}
	return &actions[index+1]
}

// nodeIndex 返回语义节点在当前路径中的位置，实例动作以 -1 表示无节点绑定。
func nodeIndex(sequence []string, key string) int {
	key = strings.TrimSpace(key)
	if key == "" {
		return -1
	}
	for index, value := range sequence {
		if value == key {
			return index
		}
	}
	return -1
}

// boundedIndex 防止不完整节点顺序导致切片越界。
func boundedIndex(index, length int) int {
	if index < 0 {
		return 0
	}
	if index >= length {
		return length
	}
	return index + 1
}

// isAutomaticNode 判断只读系统导航节点类型。
func isAutomaticNode(nodeType string) bool {
	switch strings.ToLower(strings.TrimSpace(nodeType)) {
	case "condition", "manual", "parallel", "merge", "empty", "end", "timer", "subprocess", "callback":
		return true
	default:
		return false
	}
}

// cloneParameters 复制参数 map，阻止编译结果与浏览器输入共享顶层可变对象。
func cloneParameters(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = cloneValue(value)
	}
	return result
}

// cloneValue 递归复制动作参数中的 JSON 对象和数组。
func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, nested := range typed {
			result[key] = cloneValue(nested)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, nested := range typed {
			result[index] = cloneValue(nested)
		}
		return result
	default:
		return value
	}
}

// forbiddenParameterPath 递归查找目标临时标识，防止浏览器把任务、代理或人员 ID 固化进动作配置。
func forbiddenParameterPath(values map[string]any, prefix string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := values[key]
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if forbiddenParameterKey(key) {
			return path
		}
		switch nested := value.(type) {
		case map[string]any:
			if found := forbiddenParameterPath(nested, path); found != "" {
				return found
			}
		case []any:
			for index, item := range nested {
				if child, ok := item.(map[string]any); ok {
					if found := forbiddenParameterPath(child, fmt.Sprintf("%s[%d]", path, index)); found != "" {
						return found
					}
				}
			}
		}
	}
	return ""
}

// forbiddenParameterKey 判断动作参数名是否代表目标平台运行时临时身份或浏览器上下文。
func forbiddenParameterKey(key string) bool {
	segments := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(key)), func(character rune) bool {
		return (character < 'a' || character > 'z') && (character < '0' || character > '9')
	})
	for _, segment := range segments {
		switch segment {
		case "id", "instanceid", "flowinstanceid", "taskid", "jobtaskid", "jobid", "proxyid", "formproxyid", "flowproxyid", "flownodeproxyid", "currentnodeproxyid", "targetinstanceid", "receiverid", "userid", "userids", "candidateid", "candidatekey":
			return true
		}
	}
	return false
}

// issue 构造一条阻断动作保存的结构化问题。
func issue(index int, action model.ConfiguredAction, code, message string) *model.ActionConfigurationIssue {
	return &model.ActionConfigurationIssue{Index: index, ActionKey: action.Action, ActionID: action.Key, Code: code, Message: message, Blocking: true}
}

// invalid 返回带首个动作定位的编译错误。
func invalid(index int, action model.ConfiguredAction, code, message string) (Result, error) {
	item := issue(index, action, code, message)
	return Result{Actions: []model.ConfiguredAction{action}, Issues: []model.ActionConfigurationIssue{*item}}, &CompileError{Issues: []model.ActionConfigurationIssue{*item}}
}

// String 生成便于日志和测试的确定性结果摘要，不包含动作参数正文。
func (r Result) String() string {
	return fmt.Sprintf("actions=%d steps=%d issues=%d", len(r.Actions), len(r.Steps), len(r.Issues))
}
