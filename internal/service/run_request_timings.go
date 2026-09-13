// F-030/T04：目标请求耗时明细只读解析。
// 唯一事实源是运行目录的 network.log（传输层真实计时 duration_s），
// 不用 step.log 的秒级阶段差值冒充接口耗时，不在 DTO 里返回 SID、密码或请求/响应正文。
package service

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
)

// RunRequestDTO 是一次真实目标接口请求的安全摘要（F-030/T04）。
type RunRequestDTO struct {
	// Phase 是该请求所属的执行阶段（plan/gate/prepare/verify 等），页面翻译为用户口径。
	Phase string `json:"phase,omitempty"`
	// RequestClass 是请求类型：read=读取、write=写入。
	RequestClass string `json:"requestClass"`
	// Endpoint 是目标接口路径；作为次级信息展示，不作为主标题。
	Endpoint string `json:"endpoint"`
	// DurationMs 是传输层真实耗时：开始发送到收到响应或传输失败，毫秒。
	DurationMs int64 `json:"durationMs"`
	// DurationKnown 表示 duration_s 字段存在且可解析；false 时前端显示「未知」而不是 0ms。
	DurationKnown bool `json:"durationKnown"`
	// StatusCode 是 HTTP 状态码；0 表示传输层失败未拿到响应。
	StatusCode int `json:"statusCode"`
	// Result 是目标业务结果：success / failure。
	Result string `json:"result"`
	// ResultSummary 是一句话业务结果摘要（F-034 T05）：来自结构化 WriteResponse.Message/Code、
	// 传输状态与尝试原因，如“目标已接受请求”“目标拒绝：手动条件分支,请选择”“连接未建立，未发送”。
	// 不返回完整请求/响应正文，不编造读取类响应内容。
	ResultSummary string `json:"resultSummary,omitempty"`
	// Blocking 表示该请求的失败属于前置条件阻塞（受控拒绝清单），不是写结果不确定。
	Blocking bool `json:"blocking,omitempty"`
	// RetryAttempt 是重试序号：1=首次，>1 为第 N 次重试。
	RetryAttempt int `json:"retryAttempt"`
	// TraceID 用于与 curl.log 原文互查，不是主标识。
	TraceID string `json:"traceId,omitempty"`
	// At 是请求发生时间（服务端记录）。
	At string `json:"at,omitempty"`
}

// runRequestSummaryDTO 是一次尝试的目标请求汇总指标。
type runRequestSummaryDTO struct {
	// TotalMs 是本次尝试全部目标请求的传输耗时合计（毫秒）。
	TotalMs int64 `json:"totalMs"`
	// WriteMs 是其中写请求的传输耗时合计；写请求单独展示，不与读请求混算。
	WriteMs int64 `json:"writeMs"`
	// Count 是请求总次数，WriteCount 是其中写请求次数。
	Count      int `json:"count"`
	WriteCount int `json:"writeCount"`
	// AllDurationsKnown 表示全部请求都有真实耗时；false 时汇总耗时显示「部分未知」。
	AllDurationsKnown bool `json:"allDurationsKnown"`
}

// readAttemptRequests 读取路径运行目录的 network.log，把每条目标请求归入所属尝试。
// 归属规则：按 step.log 时间轴推导的尝试窗口（plan 开始 → settle 结束），
// network.log 行携带 path_run_id 与时间，落在窗口内即归属该尝试；窗口缺失（历史日志残缺）时不归属。
// 返回：归组键（step_id:attempt）→ 请求列表（按发生顺序），以及同键的汇总指标。
func (s *RunOrchestrationService) readAttemptRequests(pathRunID uint64, attempts []model.RunStepAttempt) (map[string][]RunRequestDTO, map[string]runRequestSummaryDTO) {
	requestsByKey := map[string][]RunRequestDTO{}
	summaryByKey := map[string]runRequestSummaryDTO{}
	if s.router == nil || len(attempts) == 0 {
		return requestsByKey, summaryByKey
	}
	logPath := ""
	for _, attempt := range attempts {
		if attempt.LogPath != "" {
			logPath = attempt.LogPath
			break
		}
	}
	if logPath == "" {
		return requestsByKey, summaryByKey
	}
	// 尝试窗口：与 parsePhaseTimings 同一份 step.log 时间轴（plan 首行 → settle 末行）。
	windows := s.attemptTimeWindows(pathRunID, attempts)
	if len(windows) == 0 {
		return requestsByKey, summaryByKey
	}
	// 尝试索引：结果摘要（F-034 T05）需要按归组键取到尝试结论（如 pre_rejected 阻塞标记）。
	attemptsByKey := map[string]*model.RunStepAttempt{}
	for index := range attempts {
		attemptsByKey[stepPhaseKey(int(attempts[index].StepID), attempts[index].AttemptNo)] = &attempts[index]
	}
	file, err := os.Open(filepath.Join(s.router.Root(), filepath.Dir(filepath.FromSlash(logPath)), "network.log"))
	if err != nil {
		return requestsByKey, summaryByKey
	}
	defer file.Close()
	pathRunKey := strconv.FormatUint(pathRunID, 10)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		fields := parseLogLine(scanner.Text())
		if fields["path_run_id"] != pathRunKey {
			continue
		}
		at, err := time.ParseInLocation("2006-01-02_15:04:05", fields["time"], time.Local)
		if err != nil {
			continue
		}
		// 只归属有执行事实的尝试：优先按日志行携带的 step_id/attempt 直接归属（F-030 评审 P1：
		// 执行器已把步骤号/尝试号/阶段注入目标请求上下文，传输层写入 network.log，
		// 不再依赖秒级时间窗口推断）；旧日志缺这两列时才回退到时间窗口兜底，
		// 避免临近步骤、重试或多次尝试时归错。
		var key string
		if fields["step_id"] != "" && fields["step_id"] != "-" && fields["attempt"] != "" && fields["attempt"] != "-" {
			stepNo := atoiOr(fields["step_id"], 0)
			for _, attempt := range attempts {
				// 步骤号与尝试号必须同时对上：只匹配尝试号会把其他步骤的请求错归属。
				if attempt.StepID == uint64(stepNo) && attempt.AttemptNo == atoiOr(fields["attempt"], 0) {
					key = stepPhaseKey(stepNo, attempt.AttemptNo)
					break
				}
			}
			// step_id 与尝试记录对不上时不硬归属：请求可能属于登录或结构读取，
			// 错归属比不归属更误导。
			if key == "" {
				continue
			}
		} else {
			key, _ = matchAttemptWindow(windows, at)
			if key == "" {
				continue
			}
		}
		dto := RunRequestDTO{
			Phase:        fields["phase"],
			RequestClass: requestClassOf(fields),
			Endpoint:     strings.TrimPrefix(fields["endpoint"], "/api/web/"),
			Result:       fields["result"],
			RetryAttempt: atoiOr(fields["retry_attempt"], 1),
			TraceID:      fields["trace_id"],
			At:           fields["time"],
		}
		if seconds, err := strconv.ParseFloat(fields["duration_s"], 64); err == nil && fields["duration_s"] != "" && fields["duration_s"] != "-" {
			dto.DurationMs = int64(seconds * 1000)
			dto.DurationKnown = true
		}
		dto.StatusCode = atoiOr(fields["status_code"], 0)
		fillRequestSummary(&dto, fields, attemptsByKey[key])
		requestsByKey[key] = append(requestsByKey[key], dto)
	}
	// 汇总指标与按发生时间排序。
	for key, list := range requestsByKey {
		sort.Slice(list, func(i, j int) bool { return list[i].At < list[j].At })
		summary := runRequestSummaryDTO{AllDurationsKnown: true}
		for _, item := range list {
			summary.Count++
			if !item.DurationKnown {
				summary.AllDurationsKnown = false
				continue
			}
			summary.TotalMs += item.DurationMs
			if item.RequestClass == "write" {
				summary.WriteCount++
				summary.WriteMs += item.DurationMs
			}
		}
		requestsByKey[key] = list
		summaryByKey[key] = summary
	}
	return requestsByKey, summaryByKey
}

// attemptTimeWindows 从 step.log 时间轴推导每个尝试的起止时刻：
// 开始 = 该尝试最早阶段行，结束 = 该尝试最晚阶段行（含 settle）。
func (s *RunOrchestrationService) attemptTimeWindows(pathRunID uint64, attempts []model.RunStepAttempt) map[string][2]time.Time {
	windows := map[string][2]time.Time{}
	logPath := ""
	for _, attempt := range attempts {
		if attempt.LogPath != "" {
			logPath = attempt.LogPath
			break
		}
	}
	if logPath == "" {
		return windows
	}
	file, err := os.Open(filepath.Join(s.router.Root(), filepath.Dir(filepath.FromSlash(logPath)), "step.log"))
	if err != nil {
		return windows
	}
	defer file.Close()
	pathRunKey := strconv.FormatUint(pathRunID, 10)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	type span struct{ start, end time.Time }
	spans := map[string]*span{}
	for scanner.Scan() {
		fields := parseLogLine(scanner.Text())
		if fields["path_run_id"] != pathRunKey || fields["step_id"] == "" || fields["attempt"] == "" {
			continue
		}
		at, err := time.ParseInLocation("2006-01-02_15:04:05", fields["time"], time.Local)
		if err != nil {
			continue
		}
		key := stepPhaseKey(atoiOr(fields["step_id"], 0), atoiOr(fields["attempt"], 0))
		entry, ok := spans[key]
		if !ok {
			entry = &span{start: at, end: at}
			spans[key] = entry
		}
		if at.Before(entry.start) {
			entry.start = at
		}
		if at.After(entry.end) {
			entry.end = at
		}
	}
	for key, span := range spans {
		windows[key] = [2]time.Time{span.start, span.end}
	}
	return windows
}

// matchAttemptWindow 找到时间点落入的尝试窗口；多尝试窗口重叠时取开始更晚（更内层）的那个。
func matchAttemptWindow(windows map[string][2]time.Time, at time.Time) (string, [2]time.Time) {
	bestKey := ""
	var best [2]time.Time
	for key, window := range windows {
		if at.Before(window[0]) || at.After(window[1]) {
			continue
		}
		if bestKey == "" || window[0].After(best[0]) {
			bestKey, best = key, window
		}
	}
	return bestKey, best
}

// requestClassOf 从请求分类：写端点白名单与执行事实一致（纲领：写请求只出白名单端点）。
func requestClassOf(fields map[string]string) string {
	if fields["request_class"] == "write" {
		return "write"
	}
	// 兜底：旧日志缺 request_class 字段时按写端点白名单归类，
	// 白名单与 target 写动作常量（write_actions.go）一一对应，防写请求被误计为读取。
	switch {
	case strings.Contains(fields["endpoint"], "flowInstanceApi/submit"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/audit"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/addSign"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/withdraw"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/retrieveProcess"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/transfer"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/forward"),
		strings.Contains(fields["endpoint"], "urgeHandleRecord/sendUrgeMessage"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/rollBackThePreviousLevel"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/revocation"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/transpond"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/flowTracking"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/approverAppend"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/updateFlowProxy"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/storageFormData"),
		strings.Contains(fields["endpoint"], "flowInstanceApi/reSubmit"):
		return "write"
	}
	return "read"
}

// atoiOr 解析整数，失败或空串返回默认值。
func atoiOr(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

// ReadAttemptRequestsForTestWithRouter 暴露请求明细解析，供 test 目录锁定窗口归属与耗时口径。
func ReadAttemptRequestsForTestWithRouter(router *logging.Router, pathRunID uint64, attempts []model.RunStepAttempt) (map[string][]RunRequestDTO, map[string]runRequestSummaryDTO) {
	s := &RunOrchestrationService{router: router}
	return s.readAttemptRequests(pathRunID, attempts)
}

// fillRequestSummary 填充一句话业务结果摘要与阻塞标记（F-034 T05）。
// 来源优先级：network.log 结构化业务消息 → 传输状态 → 尝试结论原因；没有可靠事实时留空，
// 由前端显示“暂无真实结果摘要”，不用阶段时间或猜测填充。摘要不包含 SID、密码或完整正文。
func fillRequestSummary(dto *RunRequestDTO, fields map[string]string, attempt *model.RunStepAttempt) {
	if dto == nil {
		return
	}
	// 写请求失败且尝试初判为前置拒绝（pre_rejected）时标记阻塞，摘要优先展示目标原文（F-034 评审 #3）。
	blocking := dto.Result != "success" && dto.RequestClass == "write" && attempt != nil && attempt.Initial == "pre_rejected"
	// 传输层失败：没有收到响应，不能声称目标有任何结果。
	if dto.StatusCode == 0 {
		dto.ResultSummary = "连接未建立或响应未收到，未得到目标结果"
		return
	}
	// network.log 携带的目标业务消息（message 列由传输层记录的响应包络提取，不含正文）。
	if message := strings.TrimSpace(fields["message"]); message != "" {
		if dto.Result == "failure" {
			if blocking {
				dto.ResultSummary = "目标拒绝（前置条件未满足）：" + message
			} else {
				dto.ResultSummary = "目标拒绝：" + message
			}
		} else {
			dto.ResultSummary = "目标返回：" + message
		}
		dto.Blocking = blocking
		return
	}
	switch {
	case dto.Result == "success":
		dto.ResultSummary = "目标已接受请求"
	case dto.RequestClass == "read":
		dto.ResultSummary = "读取成功"
	default:
		dto.ResultSummary = "请求完成"
	}
	// 写请求失败但日志没有 message 时，结合尝试结论补阻塞语义与原因摘要（初判 pre_rejected 是确定阻塞）。
	if dto.Result != "success" && dto.RequestClass == "write" && attempt != nil && attempt.Initial == "pre_rejected" {
		dto.Blocking = true
		if dto.ResultSummary == "请求完成" || dto.ResultSummary == "" {
			dto.ResultSummary = "目标拒绝（前置条件未满足）：" + firstNonEmptySummary(attempt.Reason)
		}
	}
}

// firstNonEmptySummary 返回第一个非空摘要；两处都空时返回安全占位，不返回内部英文键。
func firstNonEmptySummary(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "见尝试原因与日志"
}
