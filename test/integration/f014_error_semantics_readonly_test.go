package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/engine/verdict"
)

// TestF014ReadonlySuccessAndFailureShapes 连真实目标，用只读接口确认三种响应形状与判定包分类一致：
// 成功包、无效参数触发的业务失败包、失效会话触发的会话失效包。抓到的响应写入只读 fixture。
func TestF014ReadonlySuccessAndFailureShapes(t *testing.T) {
	clientConfig, client, session := requireF014Session(t)

	// 成功形状：取一个真实可见模板，读它的只读详情。
	templates, err := client.ListTemplates(context.Background(), session, "", 1, 5)
	if err != nil {
		t.Fatalf("读取模板列表失败：%v", err)
	}
	if len(templates.Items) == 0 {
		t.Fatal("真实目标没有返回任何可见流程模板，无法抓取成功包形状")
	}
	statusCode, body, err := f014Post(clientConfig.BaseURL, "/web/flowTemplateApi/findById", session.SID,
		map[string]any{"data": map[string]any{"id": templates.Items[0].ID}}, clientConfig.Timeout)
	if err != nil {
		t.Fatalf("读取模板详情失败：%v", err)
	}
	writeF014Probe(t, f014Probe{Name: "success-flow-template-find-by-id", Endpoint: "/web/flowTemplateApi/findById",
		StatusCode: statusCode, Body: body, Note: "真实目标只读抓取：成功包形状"})
	initial, envelope := f014Initial("/web/flowTemplateApi/findById", statusCode, body)
	if initial != verdict.InitialSuccessClaim {
		t.Fatalf("真实成功包没有判为成功声明：initial=%s status=%d body=%.200s", initial, statusCode, body)
	}
	if statusCode != 200 {
		t.Fatalf("目标成功响应不是 HTTP 200：%d", statusCode)
	}

	// 业务失败形状：模板列表缺分组 ID 时目标返回 ERROR_99999 加业务文案，属清单外文案。
	statusCode, body, err = f014Post(clientConfig.BaseURL, "/web/flowTemplateApi/list", session.SID,
		map[string]any{"data": map[string]any{}, "pages": 1, "size": 1}, clientConfig.Timeout)
	if err != nil {
		t.Fatalf("业务失败探针请求失败：%v", err)
	}
	writeF014Probe(t, f014Probe{Name: "business-failure-template-list-missing-group", Endpoint: "/web/flowTemplateApi/list",
		StatusCode: statusCode, Body: body, Note: "真实目标只读抓取：无效参数触发的业务失败包形状"})
	initial, envelope = f014Initial("/web/flowTemplateApi/list", statusCode, body)
	if statusCode != 200 {
		t.Fatalf("目标业务失败没有按 HTTP 200 返回，与语义清单第 1.2 节不一致：%d body=%.200s", statusCode, body)
	}
	if envelope.claimsSuccess() {
		t.Fatalf("缺分组 ID 的模板列表不应声明成功：%.200s", body)
	}
	if envelope.Code != "ERROR_99999" {
		t.Fatalf("业务失败的 code 与语义清单第 1.2 节不一致：%q", envelope.Code)
	}
	if initial != verdict.InitialUnexplained {
		t.Fatalf("清单外业务失败文案应落不可解释失败：initial=%s code=%s message=%s", initial, envelope.Code, envelope.Message)
	}

	// 会话失效形状：换一个必然无效的 sid。
	statusCode, body, err = f014Post(clientConfig.BaseURL, "/web/flowTemplateApi/list", "f014-invalid-sid",
		map[string]any{"data": map[string]any{}, "pages": 1, "size": 1}, clientConfig.Timeout)
	if err != nil {
		t.Fatalf("失效会话探针请求失败：%v", err)
	}
	writeF014Probe(t, f014Probe{Name: "session-expired-invalid-sid", Endpoint: "/web/flowTemplateApi/list",
		StatusCode: statusCode, Body: body, Note: "真实目标只读抓取：失效会话形状"})
	initial, envelope = f014Initial("/web/flowTemplateApi/list", statusCode, body)
	if initial != verdict.InitialAuthRejected && initial != verdict.InitialUnexplained {
		t.Fatalf("失效会话响应分类超出语义清单预期：initial=%s code=%s", initial, envelope.Code)
	}
	if initial == verdict.InitialUnexplained {
		// 目标用清单外形状拒绝失效会话时必须留下记录，供语义清单补登，不允许静默放过。
		t.Logf("注意：失效会话返回的形状不在会话失效三种形状内，code=%q message=%q，需按第 1.3 节补登",
			envelope.Code, envelope.Message)
	}
}

// TestF014ReadonlyTimeoutClassifiesAsUncertain 用极短超时触发传输层中断，确认判定落不确定。
// 只读请求也可能超时，这条锁定的是判定规则，不是目标行为。
func TestF014ReadonlyTimeoutClassifiesAsUncertain(t *testing.T) {
	clientConfig, _ := requireF014Target(t)
	_, _, err := f014Post(clientConfig.BaseURL, "/web/flowTemplateApi/list", "f014-timeout-probe",
		map[string]any{"data": map[string]any{}, "pages": 1, "size": 1}, time.Millisecond)
	if err == nil {
		t.Fatal("1 毫秒超时必须触发传输层失败")
	}
	result := verdict.Evaluate(verdict.Observation{
		Action: "readonly_probe", Endpoint: "/web/flowTemplateApi/list",
		Transport: verdict.TransportInterrupted, Reread: verdict.RereadUnreadable,
	})
	if result.Outcome != verdict.OutcomeUncertain || result.SideEffect != verdict.SideEffectPossible {
		t.Fatalf("超时没有判为不确定且可能有副作用：%+v", result)
	}
}

// TestF014ReadonlyContractRegression 确认目标改走无 Redis 快速查询链路后，
// 工具已在调用的两个只读详情端点响应契约未变，并探测 auditWay 是编码名还是数字 ordinal。
func TestF014ReadonlyContractRegression(t *testing.T) {
	clientConfig, client, session := requireF014Session(t)
	templates, err := client.ListTemplates(context.Background(), session, "", 1, 5)
	if err != nil {
		t.Fatalf("读取模板列表失败：%v", err)
	}
	if len(templates.Items) == 0 {
		t.Fatal("真实目标没有返回任何可见流程模板，无法做契约回归")
	}
	templateID := templates.Items[0].ID
	statusCode, body, err := f014Post(clientConfig.BaseURL, "/web/flowTemplateApi/findById", session.SID,
		map[string]any{"data": map[string]any{"id": templateID}}, clientConfig.Timeout)
	if err != nil {
		t.Fatalf("读取模板详情失败：%v", err)
	}
	writeF014Probe(t, f014Probe{Name: "contract-flow-template-find-by-id", Endpoint: "/web/flowTemplateApi/findById",
		StatusCode: statusCode, Body: body, Note: "真实目标只读抓取：无 Redis 快速查询链路后的模板详情契约"})
	initial, _ := f014Initial("/web/flowTemplateApi/findById", statusCode, body)
	if initial != verdict.InitialSuccessClaim {
		t.Fatalf("模板详情契约已变：initial=%s status=%d body=%.300s", initial, statusCode, body)
	}
	if !strings.Contains(body, "\"data\"") {
		t.Fatalf("模板详情缺少 data 节点，契约已变：%.300s", body)
	}
	// 树读取仍走工具现有封装，确认解析层对新链路的响应仍可用。
	if _, err := client.ReadTemplateTree(context.Background(), session, templateID); err != nil {
		t.Fatalf("模板节点树解析失败，契约已变：%v", err)
	}
	// auditWay 部署状态探测：编码名说明 20260828 迁移已执行，数字 ordinal 说明未执行。
	conclusion := "未在响应中出现 auditWay，无法据此判断迁移状态"
	if index := strings.Index(body, "\"auditWay\":"); index >= 0 {
		value := strings.TrimSpace(body[index+len("\"auditWay\":"):])
		switch {
		case strings.HasPrefix(value, "\""):
			conclusion = "auditWay 为字符串编码名，说明目标环境已执行 20260828 迁移"
		default:
			conclusion = "auditWay 为数字 ordinal，说明目标环境尚未执行 20260828 迁移"
		}
	}
	writeF014Probe(t, f014Probe{Name: "deployment-audit-way-migration", Endpoint: "/web/flowTemplateApi/findById",
		StatusCode: statusCode, Body: conclusion, Note: "真实目标只读探测：审批方式迁移部署状态的间接结论"})
	t.Logf("部署状态探测结论：%s", conclusion)
	// 流程代理详情端点用无效 ID 探形状，正向读取由 F-012 的只读用例覆盖，本切片不构造实例 ID。
	statusCode, body, err = f014Post(clientConfig.BaseURL, "/web/flowProxy/findById", session.SID,
		map[string]any{"data": map[string]any{"id": "f014-not-exist"}}, clientConfig.Timeout)
	if err != nil {
		t.Fatalf("流程代理详情探针失败：%v", err)
	}
	writeF014Probe(t, f014Probe{Name: "contract-flow-proxy-find-by-id", Endpoint: "/web/flowProxy/findById",
		StatusCode: statusCode, Body: body, Note: "真实目标只读抓取：不存在的代理 ID 触发的失败形状"})
	if statusCode != 200 {
		t.Fatalf("流程代理详情失败没有按 HTTP 200 返回，契约已变：%d", statusCode)
	}
}
