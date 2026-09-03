package model

// ActionScope 描述动作作用于发起实例、当前任务或实例旁支的范围。
type ActionScope string

const (
	// ActionScopeInitiator 表示发起端实例生命周期动作。
	ActionScopeInitiator ActionScope = "initiator"
	// ActionScopeTask 表示当前审批或协同待办动作。
	ActionScopeTask ActionScope = "task"
	// ActionScopeCompletedTask 表示已办任务恢复动作。
	ActionScopeCompletedTask ActionScope = "completed_task"
	// ActionScopeInstance 表示实例级管理动作。
	ActionScopeInstance ActionScope = "instance"
)

// ActionStepSource 区分用户配置、系统恢复和系统导航步骤。
type ActionStepSource string

const (
	// ActionStepSourceUser 表示用户明确配置的动作。
	ActionStepSourceUser ActionStepSource = "user"
	// ActionStepSourceRecovery 表示为恢复同一主实例而插入的系统步骤。
	ActionStepSourceRecovery ActionStepSource = "system_recovery"
	// ActionStepSourceNavigation 表示引擎通过条件、并行或结束节点的导航步骤。
	ActionStepSourceNavigation ActionStepSource = "system_navigation"
)

// ActionKey 是不依赖目标临时 ID 的稳定动作语义键。
type ActionKey string

const (
	// ActionSaveDraft 表示发起端保存草稿。
	ActionSaveDraft ActionKey = "save_draft"
	// ActionSubmit 表示发起端提交实例。
	ActionSubmit ActionKey = "submit"
	// ActionResubmit 表示驳回或撤回后重新提交。
	ActionResubmit ActionKey = "resubmit"
	// ActionStorageFormData 表示审批端暂存当前表单检查点。
	ActionStorageFormData ActionKey = "storage_form_data"
	// ActionAddSign 表示当前人工待办加签。
	ActionAddSign ActionKey = "add_sign"
	// ActionTransfer 表示当前待办移交。
	ActionTransfer ActionKey = "transfer"
	// ActionApprove 表示当前待办同意。
	ActionApprove ActionKey = "approve"
	// ActionReject 表示当前待办不同意。
	ActionReject ActionKey = "reject"
	// ActionRollback 表示回退直接前一待办。
	ActionRollback ActionKey = "rollback_previous"
	// ActionRetrieve 表示恢复已完成任务。
	ActionRetrieve ActionKey = "retrieve"
	// ActionWithdraw 表示实例创建人撤回实例。
	ActionWithdraw ActionKey = "withdraw"
	// ActionUrge 表示催办当前待办接收人。
	ActionUrge ActionKey = "urge"
	// ActionForward 表示创建独立辅助转发流程。
	ActionForward ActionKey = "forward"
	// ActionFollow 表示关注主实例。
	ActionFollow ActionKey = "follow"
	// ActionUnfollow 表示取消关注主实例。
	ActionUnfollow ActionKey = "unfollow"
	// ActionSystemAutomatic 表示系统节点的只读自动语义，不是可配置的用户写动作。
	ActionSystemAutomatic ActionKey = "system_automatic"
)

// ActionCategory 描述动作目录中的业务分组。
type ActionCategory string

const (
	// ActionCategoryLifecycle 表示发起实例生命周期动作。
	ActionCategoryLifecycle ActionCategory = "lifecycle"
	// ActionCategoryCurrentTodo 表示当前待办处理动作。
	ActionCategoryCurrentTodo ActionCategory = "current_todo"
	// ActionCategoryDoneRecovery 表示已办任务恢复动作。
	ActionCategoryDoneRecovery ActionCategory = "done_recovery"
	// ActionCategoryInstanceManagement 表示实例级管理动作。
	ActionCategoryInstanceManagement ActionCategory = "instance_management"
	// ActionCategorySystemAutomatic 表示系统节点的只读自动语义。
	ActionCategorySystemAutomatic ActionCategory = "system_automatic"
)

// ConfiguredAction 是用户排序保存的一条独立动作记录；重复动作通过多条记录表达。
type ConfiguredAction struct {
	Key         string         `json:"key"`
	Action      ActionKey      `json:"action"`
	Scope       ActionScope    `json:"scope"`
	NodeKey     string         `json:"nodeKey,omitempty"`
	Order       int            `json:"order"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	ActorPolicy string         `json:"actorPolicy,omitempty"`
	Note        string         `json:"note,omitempty"`
	Revision    uint64         `json:"revision"`
}

// ActionPrecondition 是动作目录给出的实时前置事实摘要。
type ActionPrecondition struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
	Present  bool   `json:"present"`
}

// ActionParameter 描述目标接口请求中的一个语义参数，不携带目标临时 ID 或表单正文。
type ActionParameter struct {
	Name        string `json:"name"`
	Value       string `json:"value,omitempty"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// ActionCatalogItem 是按目标事实投影的动作能力和中文禁用原因。
type ActionCatalogItem struct {
	Action   ActionKey      `json:"action"`
	Category ActionCategory `json:"category"`
	Scope    ActionScope    `json:"scope"`
	// NodeKey 只承载工具侧语义节点键，不应由浏览器传入目标代理或任务 ID。
	NodeKey            string               `json:"nodeKey,omitempty"`
	Label              string               `json:"label"`
	Description        string               `json:"description"`
	TargetOperation    string               `json:"targetOperation"`
	Enabled            bool                 `json:"enabled"`
	DisabledReason     string               `json:"disabledReason"`
	Parameters         []string             `json:"parameters"`
	ParameterDetails   []ActionParameter    `json:"parameterDetails"`
	Preconditions      []ActionPrecondition `json:"preconditions"`
	ExpectedEffect     string               `json:"expectedEffect"`
	RequiresReload     bool                 `json:"requiresReload"`
	ReloadRequirements []string             `json:"reloadRequirements"`
	SystemOnly         bool                 `json:"systemOnly"`
	SystemNodeType     string               `json:"systemNodeType,omitempty"`
}

// ActionContext 是动作门禁服务读取的目标实时上下文投影。
type ActionContext struct {
	FlowSource         string `json:"flowSource"`
	InstanceStatus     string `json:"instanceStatus"`
	InstanceEnded      bool   `json:"instanceEnded"`
	IsInitiator        bool   `json:"isInitiator"`
	CurrentNodeKey     string `json:"currentNodeKey"`
	CurrentNodeType    string `json:"currentNodeType"`
	HasCurrentTask     bool   `json:"hasCurrentTask"`
	CurrentTaskDone    bool   `json:"currentTaskDone"`
	HasEditableProxy   bool   `json:"hasEditableProxy"`
	ForwardedContext   bool   `json:"forwardedContext"`
	HasCompletedTask   bool   `json:"hasCompletedTask"`
	NextTaskProcessed  bool   `json:"nextTaskProcessed"`
	PreviousTaskExists bool   `json:"previousTaskExists"`
	CanSwitchActor     bool   `json:"canSwitchActor"`
	Followed           bool   `json:"followed"`
	// PreviousNodeType 来自目标流程代理的直接前驱节点；为空时不能安全声称可以回退。
	PreviousNodeType string `json:"previousNodeType"`
	// PreviousNodeIsStart 是目标回退实现对发起节点前驱的明确判定。
	PreviousNodeIsStart bool `json:"previousNodeIsStart"`
	// HasPendingRecipient 表示当前实例是否存在可催办的真实待办接收人。
	HasPendingRecipient bool `json:"hasPendingRecipient"`
	// InstanceVisible 表示当前账号上下文已重读到实例；未设置时由其他实例事实推断。
	InstanceVisible bool `json:"instanceVisible"`
	// RetrieveNodeIsStart 表示已办任务所在节点是发起节点；目标引擎明确禁止取回该节点。
	RetrieveNodeIsStart bool `json:"retrieveNodeIsStart"`
	// RetrieveAlreadyUsed 表示该已办任务已经产生取回记录，重复取回必须阻止。
	RetrieveAlreadyUsed bool `json:"retrieveAlreadyUsed"`
	// CurrentTaskHandledByOther 表示会签/并行当前节点已有其他演员处理，取回不能越过该事实。
	CurrentTaskHandledByOther bool `json:"currentTaskHandledByOther"`
	// CurrentTaskCountersign/CurrentTaskParallel 是取回门禁所需的目标节点事实。
	CurrentTaskCountersign bool `json:"currentTaskCountersign"`
	CurrentTaskParallel    bool `json:"currentTaskParallel"`
}

// CompiledActionStep 是未来执行器消费的只读步骤，当前切片不执行该步骤。
type CompiledActionStep struct {
	Sequence        int              `json:"sequence"`
	Source          ActionStepSource `json:"source"`
	SourceActionKey string           `json:"sourceActionKey,omitempty"`
	Action          ActionKey        `json:"action"`
	Scope           ActionScope      `json:"scope"`
	ActorPolicy     string           `json:"actorPolicy,omitempty"`
	NodeKey         string           `json:"nodeKey,omitempty"`
	Parameters      map[string]any   `json:"parameters,omitempty"`
	Precondition    string           `json:"precondition"`
	ExpectedEffect  string           `json:"expectedEffect"`
	StopOnFailure   string           `json:"stopOnFailure"`
	RecoveryPolicy  string           `json:"recoveryPolicy"`
	ReloadRequired  bool             `json:"reloadRequired"`
}

// ActionConfigurationInput 是保存当前节点人员与有序动作的最小回写体，不携带目标 task/proxy ID。
type ActionConfigurationInput struct {
	Revision uint64 `json:"revision"`
	// Persons 沿用节点人员策略的不透明候选键；服务端按当前真实节点再次校验后写入独立配置列。
	Persons []PathConfigPersonStrategyInput `json:"persons,omitempty"`
	Actions []ConfiguredAction              `json:"actions"`
}

// ActionConfigurationIssue 定位动作配置中第一个无法恢复的顺序或事实问题。
type ActionConfigurationIssue struct {
	Index     int       `json:"index"`
	ActionKey ActionKey `json:"action"`
	ActionID  string    `json:"actionId,omitempty"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Blocking  bool      `json:"blocking"`
}

// ActionConfigurationResult 是节点动作保存与只读场景预览的领域结果。
type ActionConfigurationResult struct {
	Path             PathConfigPath             `json:"path"`
	Revision         uint64                     `json:"revision"`
	NodeRevision     uint64                     `json:"nodeRevision"`
	ActionRevision   uint64                     `json:"actionRevision"`
	Status           string                     `json:"status"`
	Actions          []ConfiguredAction         `json:"actions"`
	CompiledScenario []CompiledActionStep       `json:"compiledScenario"`
	Issues           []ActionConfigurationIssue `json:"issues"`
}
