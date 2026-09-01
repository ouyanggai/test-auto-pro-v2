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

// ActionCatalogItem 是按目标事实投影的动作能力和中文禁用原因。
type ActionCatalogItem struct {
	Action         ActionKey            `json:"action"`
	Scope          ActionScope          `json:"scope"`
	Label          string               `json:"label"`
	Description    string               `json:"description"`
	Enabled        bool                 `json:"enabled"`
	DisabledReason string               `json:"disabledReason,omitempty"`
	Parameters     []string             `json:"parameters"`
	Preconditions  []ActionPrecondition `json:"preconditions"`
	ExpectedEffect string               `json:"expectedEffect"`
	RequiresReload bool                 `json:"requiresReload"`
	SystemOnly     bool                 `json:"systemOnly"`
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

// ActionConfigurationInput 是保存有序动作的最小回写体，不携带目标 task/proxy ID。
type ActionConfigurationInput struct {
	Revision uint64             `json:"revision"`
	Actions  []ConfiguredAction `json:"actions"`
}
