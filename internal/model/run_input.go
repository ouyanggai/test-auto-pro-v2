package model

import "time"

const (
	// RunInputPreflightReady 表示运行输入和目标请求体都已通过只读预检。
	RunInputPreflightReady = "ready"
	// RunInputPreflightBlocked 表示当前配置必须修正后才能进入未来真实运行。
	RunInputPreflightBlocked = "blocked"
)

// RunInputSnapshot 是从当前已确认配置复制出的不可变运行输入值对象，本切片不持久化运行记录。
type RunInputSnapshot struct {
	Version             string                       `json:"version"`
	PlanID              uint64                       `json:"planId"`
	PathID              uint64                       `json:"pathId"`
	SequenceNo          uint                         `json:"sequenceNo"`
	AccountRef          string                       `json:"accountRef"`
	FlowSource          string                       `json:"flowSource"`
	TargetObjectRef     string                       `json:"targetObjectRef"`
	RenderType          string                       `json:"renderType"`
	TemplateRuleVersion string                       `json:"templateRuleVersion"`
	FormTemplateVersion string                       `json:"formTemplateVersion"`
	ShapeDigest         string                       `json:"shapeDigest"`
	SnapshotDigest      string                       `json:"snapshotDigest"`
	ConfigVersion       uint                         `json:"configVersion"`
	ConfigRevision      uint64                       `json:"configRevision"`
	NodeRevision        uint64                       `json:"nodeRevision"`
	FormRevision        uint64                       `json:"formRevision"`
	PathChoices         []ExecutionPathChoice        `json:"pathChoices"`
	NodeFieldValues     map[string]map[string]string `json:"nodeFieldValues"`
	ActionValues        map[string]string            `json:"actionValues"`
	ConfirmedNodeKeys   []string                     `json:"confirmedNodeKeys"`
	FormValues          map[string]any               `json:"formValues"`
	CapturedAt          time.Time                    `json:"capturedAt"`
}

// RunInputPreflightIssue 说明运行输入或目标适配预检的阻断原因。
type RunInputPreflightIssue struct {
	Code      string `json:"code"`
	Source    string `json:"source"`
	FieldPath string `json:"fieldPath,omitempty"`
	Message   string `json:"message"`
	CanRetry  bool   `json:"canRetry"`
}

// TargetSubmissionPreview 只公开目标请求的方法、路径、顶层键和摘要，不返回可直接发送的请求体。
type TargetSubmissionPreview struct {
	Method        string   `json:"method"`
	Path          string   `json:"path"`
	PayloadKeys   []string `json:"payloadKeys"`
	PayloadDigest string   `json:"payloadDigest"`
	SuccessChecks []string `json:"successChecks"`
}

// RunInputPreflightResult 是只读运行前检查结果，不创建运行记录也不调用目标写接口。
type RunInputPreflightResult struct {
	Status   string                   `json:"status"`
	Snapshot RunInputSnapshot         `json:"snapshot"`
	Target   TargetSubmissionPreview  `json:"target"`
	Issues   []RunInputPreflightIssue `json:"issues"`
}
