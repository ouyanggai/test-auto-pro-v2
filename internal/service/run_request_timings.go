// F-030/T04：目标请求耗时明细只读解析。
// 唯一事实源是运行目录的 network.log（传输层真实计时 duration_s），
// 不用 step.log 的秒级阶段差值冒充接口耗时，不在 DTO 里返回 SID、密码或请求/响应正文。
package service

import (
	"bufio"
	"encoding/base64"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/logging"
	"test-auto-pro-v2/internal/model"
)

// RunRequestDTO 是一次真实目标接口请求的安全摘要（F-030/T04）。
type RunRequestDTO struct {
	// Phase 是该请求所属的执行阶段（plan/gate/prepare/verify 等），页面翻译为用户口径。
	Phase string `json:"phase,omitempty"`
	// Method 是目标平台真实收到的 HTTP 方法。
	Method string `json:"method"`
	// RuntimeRequestSeq 是页面轨迹顺序号，0 表示该请求不属于浏览器转发。
	RuntimeRequestSeq int `json:"requestSeq,omitempty"`
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
	// BusinessResult 只取明确的目标包络结论；单凭 HTTP 200 不能宣称业务成功。
	BusinessResult string `json:"businessResult,omitempty"`
	// TargetMessage 是目标包络 message 原文，保留空格、换行和完整长度。
	TargetMessage string `json:"targetMessage,omitempty"`
	// TargetCode 是目标包络的原始业务码；HTTP 200 不能代替业务成功。
	TargetCode string `json:"targetCode,omitempty"`
	// ResultSummary 是一句话业务结果摘要（F-034 T05）：来自结构化 WriteResponse.Message/Code、
	// 传输状态与尝试原因，如"目标已接受请求""目标拒绝：手动条件分支,请选择""连接未建立，未发送"。
	// 不返回完整请求/响应正文，不编造读取类响应内容。
	ResultSummary string `json:"resultSummary,omitempty"`
	// Blocking 表示该请求的失败属于前置条件阻塞（受控拒绝清单），不是写结果不确定。
	Blocking bool `json:"blocking,omitempty"`
	// RetryAttempt 是发送序号：1=首次，>1 为受控重试后的第 N 次发送。
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

// readAttemptRequests 按路径、步骤序号与尝试号读取真实目标请求；不按时间猜测归属。
// 尝试行的 StepID 是数据库主键，日志 step_id 是步骤序号，必须先通过步骤事实转换。
// 第三个返回值说明日志是否完整可读；读取失败保留已经读取的事实，但不能据此断言未发送。
func (s *RunOrchestrationService) readAttemptRequests(pathRunID uint64, steps []model.RunStep, attempts []model.RunStepAttempt) (map[string][]RunRequestDTO, map[string]runRequestSummaryDTO, bool) {
	requestsByKey := map[string][]RunRequestDTO{}
	summaryByKey := map[string]runRequestSummaryDTO{}
	if s.router == nil || len(attempts) == 0 {
		return requestsByKey, summaryByKey, false
	}
	stepNoByID := map[uint64]int{}
	for _, step := range steps {
		stepNoByID[step.ID] = step.StepNo
	}
	attemptsByKey := map[string]*model.RunStepAttempt{}
	logPath := ""
	for index := range attempts {
		attempt := &attempts[index]
		if stepNo := stepNoByID[attempt.StepID]; stepNo > 0 {
			attemptsByKey[stepPhaseKey(stepNo, attempt.AttemptNo)] = attempt
		}
		if logPath == "" && attempt.LogPath != "" {
			logPath = attempt.LogPath
		}
	}
	if logPath == "" {
		return requestsByKey, summaryByKey, false
	}
	pathRunKey := strconv.FormatUint(pathRunID, 10)
	completeScope := true
	logFile := filepath.Join(s.router.Root(), filepath.Dir(filepath.FromSlash(logPath)), "network.log")
	available := readRequestLogFiles(logFile, func(line string) {
		fields := parseLogLine(line)
		if len(fields) == 0 && strings.TrimSpace(line) != "" {
			completeScope = false
		}
		if fields["path_run_id"] != pathRunKey {
			return
		}
		// network.log 也可能包含业务诊断行，它没有真实方法和端点，不能冒充一次物理请求。
		if fields["method"] == "" || fields["method"] == "-" || fields["endpoint"] == "" || fields["endpoint"] == "-" {
			return
		}
		stepNo, attemptNo := atoiOr(fields["step_id"], 0), atoiOr(fields["attempt"], 0)
		if stepNo <= 0 || attemptNo <= 0 {
			// 没有身份的请求不能并到邻近动作；该文件不能证明某个动作确实没有请求。
			completeScope = false
			return
		}
		key := stepPhaseKey(stepNo, attemptNo)
		attempt := attemptsByKey[key]
		if attempt == nil {
			return
		}
		dto := RunRequestDTO{
			Phase: fields["phase"], Method: fields["method"],
			RuntimeRequestSeq: atoiOr(fields["runtime_request_seq"], 0),
			RequestClass:      requestClassOf(fields), Endpoint: fields["endpoint"],
			Result: fields["result"], RetryAttempt: atoiOr(fields["retry_attempt"], 1),
			TraceID: fields["trace_id"], At: fields["request_at"],
			TargetCode: fields["code"],
		}
		if decoded, err := base64.StdEncoding.DecodeString(fields["target_message_b64"]); err == nil {
			dto.TargetMessage = string(decoded)
		}
		if dto.TargetCode == "-" {
			dto.TargetCode = ""
		}
		if dto.TraceID == "-" {
			dto.TraceID = ""
		}
		switch fields["outcome_kind"] {
		case "business_success":
			dto.BusinessResult = "success"
		case "business_failure":
			dto.BusinessResult = "failure"
		}
		// 只接受有限非负的实测值，损坏字段不能冒充 0 毫秒或溢出后的耗时。
		if seconds, err := strconv.ParseFloat(fields["duration_s"], 64); err == nil && seconds >= 0 && seconds < float64(math.MaxInt64)/1000 && !math.IsNaN(seconds) {
			dto.DurationMs, dto.DurationKnown = int64(seconds*1000), true
		}
		dto.StatusCode = atoiOr(fields["status_code"], 0)
		fillRequestSummary(&dto, attempt)
		requestsByKey[key] = append(requestsByKey[key], dto)
	})
	available = available && completeScope
	for key, list := range requestsByKey {
		// request_at 是传输开始时刻，响应返回顺序可能相反；时间相同时保留原日志顺序。
		sort.SliceStable(list, func(i, j int) bool {
			left, leftErr := time.Parse(time.RFC3339Nano, list[i].At)
			right, rightErr := time.Parse(time.RFC3339Nano, list[j].At)
			if leftErr != nil {
				return false
			}
			return rightErr != nil || left.Before(right)
		})
		summary := runRequestSummaryDTO{AllDurationsKnown: true}
		for _, item := range list {
			summary.Count++
			if item.RequestClass == "write" {
				summary.WriteCount++
			}
			if !item.DurationKnown {
				summary.AllDurationsKnown = false
				continue
			}
			summary.TotalMs += item.DurationMs
			if item.RequestClass == "write" {
				summary.WriteMs += item.DurationMs
			}
		}
		requestsByKey[key], summaryByKey[key] = list, summary
	}
	return requestsByKey, summaryByKey, available
}

// readRequestLogFiles 从保留的轮转文件到当前文件依次读取，避免日志轮转后丢掉前面动作的接口。
// 最老保留文件存在时，无法证明更早记录没有被清理，空状态必须标为证据不完整。
func readRequestLogFiles(path string, consume func(string)) bool {
	available := true
	for index := logging.DefaultMaxBackups; index >= 0; index-- {
		name := path
		if index > 0 {
			name += "." + strconv.Itoa(index)
		}
		file, err := os.Open(name)
		if err != nil {
			if index == 0 || !os.IsNotExist(err) {
				available = false
			}
			continue
		}
		if index == logging.DefaultMaxBackups {
			available = false
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			consume(scanner.Text())
		}
		if scanner.Err() != nil {
			available = false
		}
		_ = file.Close()
	}
	return available
}

// writeEndpointWhitelist 是写端点白名单，与 target 写动作常量（write_actions.go）一一对应。
// 报告中心的快速聚合与运行详情的请求归类共用这一份，避免两处口径漂移。
var writeEndpointWhitelist = []string{
	"flowInstanceApi/submit",
	"flowInstanceApi/audit",
	"flowInstanceApi/addSign",
	"flowInstanceApi/withdraw",
	"flowInstanceApi/retrieveProcess",
	"flowInstanceApi/transfer",
	"flowInstanceApi/forward",
	"urgeHandleRecord/sendUrgeMessage",
	"flowInstanceApi/rollBackThePreviousLevel",
	"flowInstanceApi/revocation",
	"flowInstanceApi/transpond",
	"flowInstanceApi/flowTracking",
	"flowInstanceApi/approverAppend",
	"flowInstanceApi/updateFlowProxy",
	"flowInstanceApi/storageFormData",
	"flowInstanceApi/reSubmit",
	target.RuntimeReSubmitEndpoint,
	target.RuntimeAuditEndpoint,
	"flowInstancePostscript/save",
	target.ContractReviewIncreaseEndpoint,
	target.ContractReviewModifyEndpoint,
	target.ContractReviewSaveRefFileEndpoint,
	target.RelationFileSaveBatchEndpoint,
	target.RelationFileDeleteEndpoint,
}

// isWriteEndpoint 判断端点是否属于写白名单：旧日志缺 request_class 字段时按端点兜底归类，
// 防止写请求被误计为读取（报告中心的接口读写统计与运行详情共用该判定）。
func isWriteEndpoint(endpoint string) bool {
	for _, candidate := range writeEndpointWhitelist {
		if strings.Contains(endpoint, candidate) {
			return true
		}
	}
	return false
}

// requestClassOf 从请求分类：优先日志里的 request_class，缺失时按写端点白名单兜底。
func requestClassOf(fields map[string]string) string {
	if fields["request_class"] == "write" {
		return "write"
	}
	if isWriteEndpoint(fields["endpoint"]) {
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

// ReadAttemptRequestsForTestWithRouter 暴露真实步骤映射、请求归属与证据可用状态，供根目录测试使用。
func ReadAttemptRequestsForTestWithRouter(router *logging.Router, pathRunID uint64, steps []model.RunStep, attempts []model.RunStepAttempt) (map[string][]RunRequestDTO, map[string]runRequestSummaryDTO, bool) {
	s := &RunOrchestrationService{router: router}
	return s.readAttemptRequests(pathRunID, steps, attempts)
}

// fillRequestSummary 填充一句话业务结果摘要与阻塞标记（F-034 T05）。
// 来源优先级：network.log 结构化业务消息 → 传输状态 → 尝试结论原因；没有可靠事实时留空，
// 由前端显示"暂无真实结果摘要"，不用阶段时间或猜测填充。摘要不包含 SID、密码或完整正文。
func fillRequestSummary(dto *RunRequestDTO, attempt *model.RunStepAttempt) {
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
	// 原始业务消息单独保留，摘要只加说明前缀，不截断、不改写目标原话。
	if message := dto.TargetMessage; message != "" {
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
	case dto.Result == "success" && dto.BusinessResult == "success":
		dto.ResultSummary = "目标已接受请求"
	case dto.Result == "failure":
		dto.ResultSummary = "目标返回失败，未提供错误说明"
	default:
		dto.ResultSummary = "已收到 HTTP 响应，未记录明确的目标业务结论"
	}
	// 写请求失败但日志没有 message 时，结合尝试结论补阻塞语义与原因摘要（初判 pre_rejected 是确定阻塞）。
	if dto.Result != "success" && dto.RequestClass == "write" && attempt != nil && attempt.Initial == "pre_rejected" {
		dto.Blocking = true
		if dto.ResultSummary == "目标返回失败，未提供错误说明" || dto.ResultSummary == "" {
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
