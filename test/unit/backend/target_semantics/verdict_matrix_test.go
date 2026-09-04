package target_semantics_test

import (
	"testing"

	"test-auto-pro-v2/internal/engine/verdict"
)

// observationFor 按目标初判构造一次响应侧输入，端点与文案全部来自语义清单登记的真实证据。
func observationFor(initial verdict.Initial, reread verdict.Reread) verdict.Observation {
	observation := verdict.Observation{
		Action: "approve", Endpoint: "/flowInstanceApi/audit",
		Transport: verdict.TransportResponded, StatusCode: 200, Reread: reread,
	}
	switch initial {
	case verdict.InitialSuccessClaim:
		observation.Response = &verdict.Response{IsSuccess: true, Code: "RESP200", Message: "success"}
	case verdict.InitialAuthRejected:
		observation.Response = &verdict.Response{Code: "AUTH_401", Message: "当前登录用户会话过期或在其他设备登录，请重新登录"}
	case verdict.InitialPreRejected:
		observation.Response = &verdict.Response{Code: "ERROR_99999", Message: "该待办记录不存在"}
	case verdict.InitialOptimisticLock:
		observation.Response = &verdict.Response{Code: "ERROR_99999", Message: verdict.OptimisticLockMessage}
	default:
		observation.Response = &verdict.Response{Code: "RESP200", Message: "发生空指针异常"}
	}
	return observation
}

// TestTransportStepDecidesBeforeResponse 验证判定第一步：只看传输结果。
// 连接建立阶段失败可判确定失败且无副作用，其余非完整响应一律不确定，不做乐观归类。
func TestTransportStepDecidesBeforeResponse(t *testing.T) {
	cases := []struct {
		transport  verdict.Transport
		outcome    verdict.Outcome
		sideEffect verdict.SideEffect
	}{
		{verdict.TransportConnectRefused, verdict.OutcomeFailed, verdict.SideEffectNone},
		{verdict.TransportInterrupted, verdict.OutcomeUncertain, verdict.SideEffectPossible},
		{verdict.TransportUnclassified, verdict.OutcomeUncertain, verdict.SideEffectPossible},
	}
	for _, testCase := range cases {
		result := verdict.Evaluate(verdict.Observation{
			Action: "approve", Endpoint: "/flowInstanceApi/audit",
			Transport: testCase.transport, Reread: verdict.RereadUnreadable,
		})
		if result.Outcome != testCase.outcome || result.SideEffect != testCase.sideEffect {
			t.Fatalf("传输结果 %s 判定不正确：%+v", testCase.transport, result)
		}
		if result.Reason == "" || result.Basis == "" {
			t.Fatalf("传输结果 %s 缺少中文原因或依据：%+v", testCase.transport, result)
		}
	}
}

// TestResponseStepClassifiesFiveInitials 验证判定第二步的五类初判，并锁定两条硬约束：
// 只认 isSuccess 判成功，code=RESP200 不代表成功。
func TestResponseStepClassifiesFiveInitials(t *testing.T) {
	for _, initial := range []verdict.Initial{
		verdict.InitialSuccessClaim, verdict.InitialAuthRejected, verdict.InitialPreRejected,
		verdict.InitialOptimisticLock, verdict.InitialUnexplained,
	} {
		observation := observationFor(initial, verdict.RereadAdvanced)
		if result := verdict.Evaluate(observation); result.Initial != initial {
			t.Fatalf("初判不正确：期望 %s 实际 %s（%+v）", initial, result.Initial, result)
		}
	}
	// code=RESP200 但 isSuccess=false 必须落不可解释失败，绝不能因为 code 像成功码而判成功。
	resp200 := verdict.Evaluate(verdict.Observation{
		Action: "submit", Endpoint: "/web/flowInstanceApi/submit", Transport: verdict.TransportResponded,
		StatusCode: 200, Reread: verdict.RereadAdvanced,
		Response: &verdict.Response{IsSuccess: false, Code: "RESP200", Message: "发生空指针异常"},
	})
	if resp200.Initial != verdict.InitialUnexplained || resp200.Outcome != verdict.OutcomeUncertain {
		t.Fatalf("code=RESP200 的异常包被误判：%+v", resp200)
	}
}

// TestVerdictMatrixCoversAllTwentyCells 验证判定第三步的 20 格矩阵逐格结论。
// 「乐观锁冲突 + 明确未变」按语义清单第 1.6 节的源码确认结果是不确定，不是确定失败。
func TestVerdictMatrixCoversAllTwentyCells(t *testing.T) {
	rereads := []verdict.Reread{
		verdict.RereadAdvanced, verdict.RereadUnchanged, verdict.RereadUnreadable, verdict.RereadContradictory,
	}
	expected := map[verdict.Initial]map[verdict.Reread]verdict.Outcome{
		verdict.InitialSuccessClaim: {
			verdict.RereadAdvanced: verdict.OutcomeSucceeded, verdict.RereadUnchanged: verdict.OutcomeUncertain,
			verdict.RereadUnreadable: verdict.OutcomeUncertain, verdict.RereadContradictory: verdict.OutcomeUncertain,
		},
		verdict.InitialAuthRejected: {
			verdict.RereadAdvanced: verdict.OutcomeUncertain, verdict.RereadUnchanged: verdict.OutcomeFailed,
			verdict.RereadUnreadable: verdict.OutcomeUncertain, verdict.RereadContradictory: verdict.OutcomeUncertain,
		},
		verdict.InitialPreRejected: {
			verdict.RereadAdvanced: verdict.OutcomeUncertain, verdict.RereadUnchanged: verdict.OutcomeFailed,
			verdict.RereadUnreadable: verdict.OutcomeUncertain, verdict.RereadContradictory: verdict.OutcomeUncertain,
		},
		verdict.InitialOptimisticLock: {
			verdict.RereadAdvanced: verdict.OutcomeUncertain, verdict.RereadUnchanged: verdict.OutcomeUncertain,
			verdict.RereadUnreadable: verdict.OutcomeUncertain, verdict.RereadContradictory: verdict.OutcomeUncertain,
		},
		verdict.InitialUnexplained: {
			verdict.RereadAdvanced: verdict.OutcomeUncertain, verdict.RereadUnchanged: verdict.OutcomeUncertain,
			verdict.RereadUnreadable: verdict.OutcomeUncertain, verdict.RereadContradictory: verdict.OutcomeUncertain,
		},
	}
	cells := 0
	for initial, row := range expected {
		for _, reread := range rereads {
			result := verdict.Evaluate(observationFor(initial, reread))
			if result.Outcome != row[reread] {
				t.Fatalf("矩阵格 [%s][%s] 结论不正确：期望 %s 实际 %s（%s）",
					initial, reread, row[reread], result.Outcome, result.Reason)
			}
			if result.Outcome == verdict.OutcomeUncertain && result.SideEffect != verdict.SideEffectPossible {
				t.Fatalf("矩阵格 [%s][%s] 不确定时必须按可能有副作用处理：%+v", initial, reread, result)
			}
			if result.Reason == "" || result.Basis == "" {
				t.Fatalf("矩阵格 [%s][%s] 缺少中文原因或依据", initial, reread)
			}
			cells++
		}
	}
	if cells != 20 {
		t.Fatalf("矩阵格数不是 20：%d", cells)
	}
}
