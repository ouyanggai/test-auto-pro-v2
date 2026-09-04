// Package verdict 把一次目标平台写请求的可观测事实映射为三值结论：确定成功、确定失败、不确定。
//
// 本包是 docs/TARGET_SEMANTICS.md 第 1.6 节判定规则的唯一实现，规则改动必须先改语义清单。
// 三条不可破坏的约束直接体现在实现里：
//  1. 禁止用响应包的 code 判成功，只认 isSuccess。code=RESP200 是目标侧程序异常的渲染结果。
//  2. 禁止把业务拒绝并入「暂时不可用」。本包不引用 internal/adapter/target 的任何错误收敛。
//  3. 写请求禁止携带 batchCode，由 ValidateWritePayload 锁定。
//
// 本包是纯判定：无 IO、无目标调用、无数据库、无并发状态，也不接入任何现有执行流程。
package verdict

// Outcome 是三值结论。取值集合固定，后续切片不得扩张。
type Outcome string

const (
	// OutcomeSucceeded 表示这次写确定生效。
	OutcomeSucceeded Outcome = "confirmed_success"
	// OutcomeFailed 表示这次写确定没有生效。
	OutcomeFailed Outcome = "confirmed_failure"
	// OutcomeUncertain 表示无法判定，必须按不确定处理，禁止乐观归类。
	OutcomeUncertain Outcome = "uncertain"
)

// SideEffect 说明确定失败时目标侧是否可能已经留下痕迹。
type SideEffect string

const (
	// SideEffectNone 表示可以证明目标侧没有任何写入。
	SideEffectNone SideEffect = "none"
	// SideEffectPossible 表示不能排除部分写入，只能按可能有副作用处理。
	SideEffectPossible SideEffect = "possible"
)

// Transport 是传输层结果，判定第一步只看它。
type Transport string

const (
	// TransportResponded 表示收到了完整响应。
	TransportResponded Transport = "responded"
	// TransportConnectRefused 表示连接建立阶段即失败且可明确识别，请求未到达目标进程。
	TransportConnectRefused Transport = "connect_refused"
	// TransportInterrupted 表示超时、连接中断、context 取消或进程崩溃。
	TransportInterrupted Transport = "interrupted"
	// TransportUnclassified 表示识别不出属于哪一类，按不确定处理。
	TransportUnclassified Transport = "unclassified"
)

// Reread 是 verify 阶段事实重读的四值结论。
type Reread string

const (
	// RereadAdvanced 表示重读确认流程已按配置期望前进。
	RereadAdvanced Reread = "advanced"
	// RereadUnchanged 表示重读确认流程侧事实明确没有变化。
	RereadUnchanged Reread = "unchanged"
	// RereadUnreadable 表示重读本身失败，拿不到事实。
	RereadUnreadable Reread = "unreadable"
	// RereadContradictory 表示重读结果自相矛盾。
	RereadContradictory Reread = "contradictory"
)

// Initial 是响应侧初判，判定第二步的输出。
type Initial string

const (
	// InitialSuccessClaim 表示目标声明成功。
	InitialSuccessClaim Initial = "success_claim"
	// InitialAuthRejected 表示会话失效或鉴权被拒。
	InitialAuthRejected Initial = "auth_rejected"
	// InitialPreRejected 表示命中前置拒绝清单，拒绝发生在任何写之前。
	InitialPreRejected Initial = "pre_rejected"
	// InitialOptimisticLock 表示命中乐观锁冲突提示。
	InitialOptimisticLock Initial = "optimistic_lock"
	// InitialUnexplained 表示不可解释失败，含 code=RESP200 异常包与清单外文案。
	InitialUnexplained Initial = "unexplained"
	// InitialNone 表示还没走到响应侧初判。
	InitialNone Initial = ""
)

// Response 是目标响应包里判定要用到的部分。
type Response struct {
	// IsSuccess 是目标业务包络的唯一成功判据。
	IsSuccess bool
	// Code 只用于识别会话失效，绝不用来判成败。
	Code string
	// Message 参与「端点 + 精确文案」全等匹配。
	Message string
	// Unparsable 表示响应体不可解析或超长，按不可解释失败处理。
	Unparsable bool
}

// Observation 是判定输入，五项全部是可观测事实，不含工具推断。
type Observation struct {
	// Action 是动作目录里的动作标识，只进原因说明，不参与匹配。
	Action string
	// Endpoint 是本次请求的目标端点，参与「端点 + 精确文案」匹配。
	Endpoint string
	// Transport 是传输层结果。
	Transport Transport
	// StatusCode 是 HTTP 状态码；请求未发出时为 0。
	StatusCode int
	// Response 为空表示没有收到可解析的响应包。
	Response *Response
	// Reread 是事实重读结论。
	Reread Reread
}

// Verdict 是判定结果，除结论外一并给出中文原因与依据，便于直接写进日志与界面提示。
type Verdict struct {
	Outcome    Outcome
	SideEffect SideEffect
	Initial    Initial
	Reason     string
	Basis      string
}

// Evaluate 按 docs/TARGET_SEMANTICS.md 第 1.6 节三步求值，首个命中即为结论。
// 任何未覆盖组合、任何两项输入互相矛盾、任何新出现的响应形状都落「不确定」，这是兜底规则。
func Evaluate(observation Observation) Verdict {
	if conflict := detectConflict(observation); conflict != "" {
		return uncertain(InitialNone, conflict, "第 1.6 节兜底规则：输入互相矛盾一律判不确定")
	}
	switch observation.Transport {
	case TransportConnectRefused:
		return Verdict{
			Outcome: OutcomeFailed, SideEffect: SideEffectNone, Initial: InitialNone,
			Reason: "连接建立阶段即失败，请求没有到达目标平台进程",
			Basis:  "第 1.6 节第一步：连接被拒或域名解析失败可判确定失败、无副作用",
		}
	case TransportInterrupted:
		return uncertain(InitialNone,
			"请求已发出但没有收到完整响应，写可能已经在目标侧发生",
			"第 1.6 节第一步：超时、中断、取消与进程崩溃一律判不确定")
	case TransportResponded:
		return combine(observation, classifyResponse(observation))
	default:
		return uncertain(InitialNone,
			"传输结果无法归类，不做乐观归类",
			"第 1.6 节第一步：识别不出属于哪一类时按不确定处理")
	}
}

// detectConflict 检查五项输入之间的硬性矛盾，返回中文说明；没有矛盾时返回空串。
func detectConflict(observation Observation) string {
	if !validTransport(observation.Transport) {
		return "传输结果取值不在约定集合内"
	}
	if !validReread(observation.Reread) {
		return "事实重读结论取值不在约定集合内"
	}
	if observation.Transport == TransportResponded && observation.Response == nil && observation.StatusCode == 0 {
		return "传输结果声明收到完整响应，但既没有响应包也没有状态码"
	}
	if observation.Transport != TransportResponded && observation.Response != nil {
		return "传输结果声明没有收到响应，却带回了响应包"
	}
	if observation.Transport != TransportResponded && observation.StatusCode > 0 {
		return "传输结果声明没有收到响应，却带回了 HTTP 状态码"
	}
	return ""
}

// validTransport 判断传输结果取值是否在约定集合内；未知取值按矛盾处理而不是静默兜底。
func validTransport(transport Transport) bool {
	switch transport {
	case TransportResponded, TransportConnectRefused, TransportInterrupted, TransportUnclassified:
		return true
	default:
		return false
	}
}

// validReread 判断事实重读结论取值是否在约定集合内。
func validReread(reread Reread) bool {
	switch reread {
	case RereadAdvanced, RereadUnchanged, RereadUnreadable, RereadContradictory:
		return true
	default:
		return false
	}
}

// uncertain 组装一条不确定结论；不确定一律按可能有副作用处理。
func uncertain(initial Initial, reason, basis string) Verdict {
	return Verdict{Outcome: OutcomeUncertain, SideEffect: SideEffectPossible, Initial: initial, Reason: reason, Basis: basis}
}
