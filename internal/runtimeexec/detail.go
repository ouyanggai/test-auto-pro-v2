// T013 运行详情投影：把 runtime 动作结果投影为「表单业务处理 / 流程动作 / 提交后业务处理」
// 三阶段摘要与逐请求摘要。页面不展示 SID、密码和完整正文；完整正文只走现有日志入口（step.log / network.log）。
package runtimeexec

import (
	"encoding/json"
	"net/url"
	"strings"
)

// StageState 是单个阶段的中文可解释状态。
type StageState string

const (
	StagePending   StageState = "pending"
	StageRunning   StageState = "running"
	StageDone      StageState = "done"
	StageFailed    StageState = "failed"
	StageUncertain StageState = "uncertain"
)

// RuntimeStage 是一个业务阶段的事实摘要。
type RuntimeStage struct {
	Name     string     `json:"name"`
	State    StageState `json:"state"`
	Note     string     `json:"note,omitempty"`
	Requests int        `json:"requests"`
}

// RuntimeRequestSummary 是单笔请求的可展示摘要：仅路径、方法、状态与耗时，无正文无 SID。
type RuntimeRequestSummary struct {
	Seq        int    `json:"seq"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	State      string `json:"state"`
	HTTPStatus int    `json:"httpStatus,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

// RuntimeExecutionDetail 是一次 runtime 动作的运行详情投影。
type RuntimeExecutionDetail struct {
	Stages            []RuntimeStage          `json:"stages"`
	Requests          []RuntimeRequestSummary `json:"requests"`
	CapabilityVersion string                  `json:"capabilityVersion"`
	Status            string                  `json:"status"`
	Message           string                  `json:"message,omitempty"`
}

// runtimeDetailRequestEntry 是轨迹条目的最小解码形状（与 executor 侧 summarize 对齐）。
type runtimeDetailRequestEntry struct {
	Seq         int    `json:"seq"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	State       string `json:"state"`
	AbortReason string `json:"abortReason,omitempty"`
	HTTPStatus  int    `json:"httpStatus,omitempty"`
	DurationMs  int64  `json:"durationMs,omitempty"`
}

// BuildDetailProjection 从动作结果与请求轨迹构造运行详情投影。
// 三阶段划分（对照文档第 3 节动作链）：表单业务处理（装载至流程主写之前）、
// 流程动作（主写端点请求）、提交后业务处理（主写之后至链路收敛）。
func BuildDetailProjection(result ActionRunResult, requestTrace json.RawMessage, capabilityVersion string) RuntimeExecutionDetail {
	detail := RuntimeExecutionDetail{
		Status:            result.Status,
		Message:           result.Message,
		CapabilityVersion: capabilityVersion,
	}
	var entries []runtimeDetailRequestEntry
	if len(requestTrace) > 0 {
		_ = json.Unmarshal(requestTrace, &entries)
	}
	flowSeq := -1
	summaries := make([]RuntimeRequestSummary, 0, len(entries))
	for _, entry := range entries {
		summaries = append(summaries, RuntimeRequestSummary{
			Seq:        entry.Seq,
			Method:     entry.Method,
			Path:       sanitizePath(entry.URL),
			State:      entry.State,
			HTTPStatus: entry.HTTPStatus,
			DurationMs: entry.DurationMs,
		})
		if isFlowPath(entry.URL) && entry.State == "fulfilled" && (flowSeq == -1 || entry.Seq < flowSeq) {
			flowSeq = entry.Seq
		}
	}
	detail.Requests = summaries

	before, flow, after := 0, 0, 0
	afterFailed := false
	for _, entry := range entries {
		switch {
		case flowSeq >= 0 && entry.Seq < flowSeq:
			before++
		case entry.Seq == flowSeq:
			flow++
		case flowSeq >= 0 && entry.Seq > flowSeq:
			after++
			if entry.State == "aborted" || entry.State == "blocked" {
				afterFailed = true
			}
		default:
			before++
		}
	}
	flowState := StageState(StageDone)
	switch {
	case result.Status == "uncertain" || result.Status == "timeout":
		flowState = StageUncertain
	case result.Status == "target_rejected":
		flowState = StageFailed
	case flow == 0:
		flowState = StagePending
	}
	afterState := StageState(StageDone)
	switch {
	case after == 0:
		afterState = StagePending
	case afterFailed || result.Status == "business_failed":
		afterState = StageFailed
	}
	detail.Stages = []RuntimeStage{
		{Name: "表单业务处理", State: StageDone, Requests: before},
		{Name: "流程动作", State: flowState, Requests: flow},
		{Name: "提交后业务处理", State: afterState, Requests: after},
	}
	// 表单业务阶段失败口径：首笔请求就中止（装载后的首个目标请求失败）说明表单链路未走通。
	if before == 0 && flow == 0 && strings.EqualFold(result.Status, "failed") {
		detail.Stages[0].State = StageFailed
	}
	return detail
}

// sanitizePath 只保留路径部分并剥离查询参数（查询可能携带 sid）。
func sanitizePath(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return parsed.Path
}

// isFlowPath 判定是否主写端点（与完成检测口径一致）。
func isFlowPath(raw string) bool {
	return strings.Contains(raw, "/web/flowInstanceApi/submit") ||
		strings.Contains(raw, "/flowInstanceApi/audit") ||
		strings.Contains(raw, "/web/flowInstanceApi/reSubmit") ||
		strings.Contains(raw, "/web/flowInstanceApi/transpond")
}
