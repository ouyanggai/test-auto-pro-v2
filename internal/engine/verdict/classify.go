package verdict

// classifyResponse 是判定第二步：把响应包收敛为五类初判。
// 只认 isSuccess 判成功，code 只用于识别会话失效，绝不参与成败判断。
func classifyResponse(observation Observation) Initial {
	response := observation.Response
	if isAuthRejected(observation.StatusCode, response) {
		return InitialAuthRejected
	}
	if response == nil || response.Unparsable {
		return InitialUnexplained
	}
	if observation.StatusCode >= 500 {
		return InitialUnexplained
	}
	if response.IsSuccess {
		if observation.StatusCode == 200 {
			return InitialSuccessClaim
		}
		// 声明成功但状态码不是 200，属于新出现的形状，按不可解释失败处理。
		return InitialUnexplained
	}
	message := normalizeMessage(response.Message)
	if message == OptimisticLockMessage {
		return InitialOptimisticLock
	}
	if isPreRejection(observation.Endpoint, message) {
		return InitialPreRejected
	}
	return InitialUnexplained
}

// isAuthRejected 识别会话失效的三种形状：HTTP 401、code=RESP401、code=AUTH_401。
// 目标侧 AUTH_401 是现有只读路径漏认的那一种，本包必须认，依据见语义清单第 1.5 节。
func isAuthRejected(statusCode int, response *Response) bool {
	if statusCode == 401 {
		return true
	}
	if response == nil {
		return false
	}
	for _, code := range AuthRejectedCodes {
		if normalizeMessage(response.Code) == code {
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
