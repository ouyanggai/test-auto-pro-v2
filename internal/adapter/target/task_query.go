package target

import (
	"context"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/logging"
)

// 本文件是 `/web/flowJobTaskLink/list` 唯一的请求载荷出口（F-031/T01）。
// 目标协议把「实例筛选」放在请求顶层 flowInstanceIdList（参考前端 GroupApproveManage/Backlog/index.vue
// 的同端点用法，参考 Java 仓储也只绑定 f.flowInstanceIdList）；放进 data.flowInstanceId 目标端完全不理会：
// 请求照样返回 200，实例条件却被静默忽略，工具随即退化成翻全量任务表（实测单次扫描翻到第 11 页 × 74 条）。
// 因此本端点禁止调用方自行拼接同名字段：waiting_send、pending、done 三类查询都由这里构造并校验。
// 视角参数的位置同样不能互换：pending 用协议顶层 queryUserId，done 用 data.executorId。

const (
	// taskStatusWaitingSend 是待发任务的真实状态名（发起人视角，不传查询视角用户）。
	taskStatusWaitingSend = "waiting_send"
	// taskStatusPending 是当前待办的真实状态名。
	taskStatusPending = "pending"
	// taskStatusDone 是已办任务的真实状态名。
	taskStatusDone = "done"
)

// taskSnapshotQuery 是一次精确实例任务查询的协议输入。
type taskSnapshotQuery struct {
	// InstanceID 是目标实例 ID：实例筛选只能来自它，落到协议的顶层 flowInstanceIdList。
	InstanceID string
	// TaskStatus 只接受 waiting_send、pending、done 三个真实状态名。
	TaskStatus string
	// ViewUserID 是 pending 的查询视角用户（协议顶层 queryUserId）：
	// 为空时目标按 SID 解析当前用户；非空时查该用户的待办。它不是候选处理人名单。
	ViewUserID string
	// ExecutorID 是 done 的实际执行人（data.executorId）：核对谁的已办就必须传谁，
	// 为空说明视角语义不成立，直接拒绝，绝不退化成「随便查一个视角」。
	ExecutorID string
}

// buildTaskSnapshotBody 生成 `/web/flowJobTaskLink/list` 的完整请求载荷并校验协议约束。
// page 从 1 计；size 是目标分页大小。缺少实例 ID、未知状态或 done 缺少执行人都在发请求前拒绝。
func buildTaskSnapshotBody(query taskSnapshotQuery, page, size int) (map[string]any, error) {
	instanceID := strings.TrimSpace(query.InstanceID)
	if instanceID == "" {
		return nil, invalidResponse("task lookup missing flow instance id")
	}
	status := strings.TrimSpace(query.TaskStatus)
	data := map[string]any{
		"taskStatus":                   status,
		"auditWayList":                 []string{},
		"useScope":                     "invest",
		"flowInstanceBizRelevance":     map[string]any{},
		"flowInstanceBizRelevanceList": []any{},
	}
	body := map[string]any{
		"data": data, "pagination": true, "pages": page, "size": size,
		// 实例过滤是协议顶层字段：data.flowInstanceId 会被目标忽略，绝不能写在那里。
		"flowInstanceIdList": []string{instanceID},
	}
	switch status {
	case taskStatusWaitingSend:
		// 待发任务由发起人会话读取，协议不带 queryUserId，也不带 executorId。
	case taskStatusPending:
		if viewUserID := strings.TrimSpace(query.ViewUserID); viewUserID != "" {
			body["queryUserId"] = viewUserID
		}
	case taskStatusDone:
		executorID := strings.TrimSpace(query.ExecutorID)
		if executorID == "" {
			return nil, invalidResponse("done task lookup missing executor id")
		}
		data["executorId"] = executorID
	default:
		return nil, invalidResponse("unsupported task status")
	}
	return body, nil
}

// taskQueryStats 是一次精确实例任务查询的请求与命中事实。
// HTTP 200 只证明包络被接受，不证明实例过滤生效；请求数、目标返回行数与命中行数
// 才是「查询已收敛到实例范围」的证据，必须写进运行日志供人工核对。
type taskQueryStats struct {
	// Requests 是实际发出的目标分页请求数。
	Requests int
	// Pages 是目标返回的总页数（目标未返回时为 0）。
	Pages int
	// Rows 是目标返回的全部行数，含被实例校验剔除的无关行。
	Rows int
	// ForeignRows 是被实例校验剔除的无关实例行数：非零说明目标响应可能串入其他实例。
	ForeignRows int
	// Matched 是通过实例校验并保留的行数。
	Matched int
}

// reportTaskQuery 把一次精确查询的范围事实写进当前作用域的日志；未接日志器时静默跳过。
// 只写字段，不写请求正文，也不回填任何被剔除的业务内容。
func (c *Client) reportTaskQuery(ctx context.Context, query taskSnapshotQuery, stats taskQueryStats) {
	logger, ok := c.networkLogger.(interface {
		Info(scope logging.Scope, message string, extra ...logging.Field)
	})
	if !ok {
		return
	}
	view := strings.TrimSpace(query.ViewUserID)
	if view == "" {
		view = strings.TrimSpace(query.ExecutorID)
	}
	logger.Info(logging.ScopeFrom(ctx), "精确实例任务查询",
		logging.Field{Key: "query_instance_id", Value: strings.TrimSpace(query.InstanceID)},
		logging.Field{Key: "query_task_status", Value: strings.TrimSpace(query.TaskStatus)},
		logging.Field{Key: "query_view_user", Value: view},
		logging.Field{Key: "request_count", Value: formatCount(stats.Requests)},
		logging.Field{Key: "target_rows", Value: formatCount(stats.Rows)},
		logging.Field{Key: "foreign_rows", Value: formatCount(stats.ForeignRows)},
		logging.Field{Key: "matched_rows", Value: formatCount(stats.Matched)},
	)
}

// formatCount 把非负计数写成十进制文本，负值按 0 处理（计数不可能为负）。
func formatCount(value int) string {
	if value <= 0 {
		return "0"
	}
	return strconv.Itoa(value)
}
