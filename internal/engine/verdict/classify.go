package verdict

// classifyResponse 是判定第二步：把响应包收敛为五类初判。
// 顺序不可调换：先确认响应形状本身站得住脚，再看鉴权码与文案清单。
// 否则「isSuccess=true 却带鉴权码」这种矛盾包、以及 HTTP 3xx/4xx 的失败包，
// 都会被文案清单命中并进而得出「确定失败、无副作用」，违背冲突与新形状一律不确定的兜底规则。
func classifyResponse(observation Observation) Initial {
	// HTTP 401 本身就是完整的鉴权拒绝信号，语义清单第 1.3 节把它与两个错误码并列，
	// 这种响应通常没有业务包络，必须在形状校验之前先认掉。
	if observation.StatusCode == 401 {
		return InitialAuthRejected
	}
	response := observation.Response
	if response == nil || response.Unparsable {
		return InitialUnexplained
	}
	// 成功判据必须显式存在。响应包里没有 isSuccess 时不允许用 code 或文案补判，
	// 否则缺字段与 isSuccess=false 无法区分。
	if !response.IsSuccessPresent {
		return InitialUnexplained
	}
	// 除 401 外，只有 HTTP 200 的业务包络是语义清单第 1.2 节勘定过的形状。
	// 3xx、4xx、5xx 一律按新出现的形状处理，不进文案清单。
	if observation.StatusCode != 200 {
		return InitialUnexplained
	}
	if response.IsSuccess {
		// 声明成功却同时带着明确的失败标记，是响应包自相矛盾，不是成功。
		if carriesFailureMarker(observation.Endpoint, response) {
			return InitialUnexplained
		}
		return InitialSuccessClaim
	}
	if isAuthCode(response.Code) {
		return InitialAuthRejected
	}
	message := normalizeMessage(response.Message)
	if isOptimisticLock(observation.Endpoint, message) {
		return InitialOptimisticLock
	}
	if isPreRejection(observation.Endpoint, message) {
		return InitialPreRejected
	}
	return InitialUnexplained
}

// carriesFailureMarker 判断一个声明成功的响应包是否同时带着明确的失败标记：
// 鉴权错误码、乐观锁提示，或该端点登记过的前置拒绝文案。
func carriesFailureMarker(endpoint string, response *Response) bool {
	if isAuthCode(response.Code) {
		return true
	}
	message := normalizeMessage(response.Message)
	return isOptimisticLock(endpoint, message) || isPreRejection(endpoint, message)
}

// isAuthCode 判断响应码是否属于会话失效的两个目标错误码。
// AUTH_401 是现有只读路径漏认的那一个，本包必须认，依据见语义清单第 1.5 节。
func isAuthCode(code string) bool {
	normalized := normalizeMessage(code)
	for _, candidate := range AuthRejectedCodes {
		if normalized == candidate {
			return true
		}
	}
	return false
}

// combine 是判定第三步：把响应侧初判与事实重读结论组合成最终结论。
// 矩阵之外的任何组合一律落不确定，兜底规则不允许被后续切片放宽。
func combine(observation Observation, initial Initial) Verdict {
	switch initial {
	case InitialSuccessClaim:
		if observation.Reread == RereadAdvanced {
			return Verdict{
				Outcome: OutcomeSucceeded, SideEffect: SideEffectNone, Initial: initial,
				Reason: "目标声明成功，且重读确认流程已按期望前进",
				Basis:  "第 1.6 节矩阵：成功声明 + 已前进 = 确定成功",
			}
		}
		return uncertain(initial, successClaimReason(observation.Reread),
			"第 1.6 节矩阵：成功声明只有配合已前进才算成功，响应与事实冲突不是成功也不是失败")
	case InitialAuthRejected:
		if observation.Reread == RereadUnchanged {
			return Verdict{
				Outcome: OutcomeFailed, SideEffect: SideEffectNone, Initial: initial,
				Reason: "目标以会话失效拒绝本次请求，且重读确认流程侧事实明确没有变化",
				Basis:  "第 1.6 节矩阵：鉴权拒绝 + 明确未变 = 确定失败、无副作用",
			}
		}
		return uncertain(initial, rejectionReason("会话失效", observation.Reread),
			"第 1.6 节矩阵：拒绝与观察到的推进不是同一件事时只能判不确定")
	case InitialPreRejected:
		if observation.Reread == RereadUnchanged {
			return Verdict{
				Outcome: OutcomeFailed, SideEffect: SideEffectNone, Initial: initial,
				Reason: "命中前置拒绝清单，拒绝发生在任何写之前，且重读确认流程侧事实明确没有变化",
				Basis:  "第 1.6 节矩阵：前置拒绝 + 明确未变 = 确定失败、无副作用；清单见第 1.7 节",
			}
		}
		return uncertain(initial, rejectionReason("前置校验拒绝", observation.Reread),
			"第 1.6 节矩阵：前置拒绝只有配合明确未变才算确定失败")
	case InitialOptimisticLock:
		// 语义清单第 1.6 节已用源码确认：审批路径在乐观锁检查之前已经写过 Mongo 表单数据，
		// 关系库回滚不会撤销它，所以这一格由确定失败降级为不确定。
		return uncertain(initial,
			"目标检测到并发更新并回滚了关系库写入，但乐观锁检查之前已可能写入表单数据，无法证明目标什么都没写",
			"第 1.6 节矩阵与第 2.4 节：乐观锁冲突一律判不确定，含重读明确未变")
	case InitialUnexplained:
		return uncertain(initial,
			"目标返回的失败形状无法解释，可能是目标侧程序异常或清单外的业务拒绝",
			"第 1.6 节矩阵：不可解释失败在任何重读结论下都判不确定；重读只覆盖流程侧事实，证明不了跨存储没有部分生效")
	default:
		return uncertain(InitialNone, "响应侧初判无法归类", "第 1.6 节兜底规则")
	}
}

// successClaimReason 给出成功声明配上非「已前进」重读时的中文原因。
func successClaimReason(reread Reread) string {
	switch reread {
	case RereadUnchanged:
		return "目标声明成功，但重读显示流程侧事实明确没有变化，响应与事实冲突"
	case RereadUnreadable:
		return "目标声明成功，但重读失败，拿不到事实无法确认是否真的生效"
	default:
		return "目标声明成功，但重读结果自相矛盾，无法确认是否真的生效"
	}
}

// rejectionReason 给出拒绝类初判配上非「明确未变」重读时的中文原因。
func rejectionReason(kind string, reread Reread) string {
	switch reread {
	case RereadAdvanced:
		return "目标以" + kind + "拒绝本次请求，但重读显示流程已前进，两者不是同一件事，可能是上一次不确定写已生效"
	case RereadUnreadable:
		return "目标以" + kind + "拒绝本次请求，但重读失败，拿不到事实无法确认目标侧状态"
	default:
		return "目标以" + kind + "拒绝本次请求，但重读结果自相矛盾，无法确认目标侧状态"
	}
}
