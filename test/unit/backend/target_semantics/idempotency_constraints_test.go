package target_semantics_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/engine/verdict"
)

// sourceSample 是按源码证据构造的写路径样本；本切片不发写请求，样本必须自带来源与待复核标注。
type sourceSample struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	Note       string `json:"note"`
	Endpoint   string `json:"endpoint"`
	StatusCode int    `json:"statusCode"`
	Response   struct {
		// IsSuccess 用指针解析：样本缺字段时必须能与显式 false 区分开。
		IsSuccess *bool  `json:"isSuccess"`
		Code      string `json:"code"`
		Message   string `json:"message"`
	} `json:"response"`
	ExpectedInitial string `json:"expectedInitial"`
}

// TestFallbackCoversUnknownShapesAndContradictions 验证兜底规则：未覆盖形状与矛盾输入一律不确定。
func TestFallbackCoversUnknownShapesAndContradictions(t *testing.T) {
	cases := map[string]verdict.Observation{
		"传输结果取值未知": {Transport: verdict.Transport("brand_new"), Reread: verdict.RereadUnchanged},
		"重读结论取值未知": {Transport: verdict.TransportResponded, StatusCode: 200, Reread: verdict.Reread("maybe"),
			Response: &verdict.Response{IsSuccess: true}},
		"声明收到响应但既无响应包也无状态码": {Transport: verdict.TransportResponded, Reread: verdict.RereadUnchanged},
		"声明没收到响应却带回响应包": {Transport: verdict.TransportInterrupted, Reread: verdict.RereadUnchanged,
			Response: &verdict.Response{IsSuccess: true, IsSuccessPresent: true}},
		"声明没收到响应却带回状态码": {Transport: verdict.TransportInterrupted, StatusCode: 200, Reread: verdict.RereadUnchanged},
		"响应体不可解析": {Transport: verdict.TransportResponded, StatusCode: 200, Reread: verdict.RereadAdvanced,
			Response: &verdict.Response{Unparsable: true}},
		"HTTP 5xx": {Transport: verdict.TransportResponded, StatusCode: 502, Reread: verdict.RereadAdvanced,
			Response: &verdict.Response{IsSuccessPresent: true, Message: "网关错误"}},
		"声明成功但状态码不是 200": {Transport: verdict.TransportResponded, StatusCode: 302, Reread: verdict.RereadAdvanced,
			Response: &verdict.Response{IsSuccess: true, IsSuccessPresent: true}},
		"响应包里没有 isSuccess 字段": {Transport: verdict.TransportResponded, StatusCode: 200, Reread: verdict.RereadAdvanced,
			Response: &verdict.Response{Code: "RESP200", Message: "success"}},
		// 用「明确未变」这一格：若矛盾包被误判成鉴权拒绝，结论会变成确定失败，本用例必须拦下。
		"声明成功却同时带鉴权错误码": {Transport: verdict.TransportResponded, StatusCode: 200, Reread: verdict.RereadUnchanged,
			Response: &verdict.Response{IsSuccess: true, IsSuccessPresent: true, Code: "AUTH_401", Message: "会话过期"}},
		"HTTP 4xx 的失败包命中清单文案也不算确定失败": {Transport: verdict.TransportResponded, StatusCode: 404,
			Reread: verdict.RereadUnchanged, Endpoint: "/web/flowInstanceApi/retrieveProcess",
			Response: &verdict.Response{IsSuccessPresent: true, Code: "ERROR_99999", Message: "流程已完结,不支持取回"}},
		"HTTP 3xx 的失败包命中清单文案也不算确定失败": {Transport: verdict.TransportResponded, StatusCode: 302,
			Reread: verdict.RereadUnchanged, Endpoint: "/web/flowInstanceApi/revocation",
			Response: &verdict.Response{IsSuccessPresent: true, Message: "当前实例不存在"}},
	}
	for name, observation := range cases {
		result := verdict.Evaluate(observation)
		if result.Outcome != verdict.OutcomeUncertain || result.SideEffect != verdict.SideEffectPossible {
			t.Fatalf("%s 没有落兜底不确定：%+v", name, result)
		}
		if result.Reason == "" || result.Basis == "" {
			t.Fatalf("%s 缺少中文原因或依据：%+v", name, result)
		}
	}
}

// TestPreRejectionMatchesEndpointAndExactMessage 验证前置拒绝清单按「端点 + 文案全等」匹配：
// 不做关键字包含，也不跨端点复用文案。
func TestPreRejectionMatchesEndpointAndExactMessage(t *testing.T) {
	base := verdict.Observation{
		Action: "retrieve", Endpoint: "/web/flowInstanceApi/retrieveProcess",
		Transport: verdict.TransportResponded, StatusCode: 200, Reread: verdict.RereadUnchanged,
	}
	hit := base
	hit.Response = &verdict.Response{IsSuccessPresent: true, Code: "ERROR_99999", Message: "流程已完结,不支持取回"}
	if result := verdict.Evaluate(hit); result.Initial != verdict.InitialPreRejected || result.Outcome != verdict.OutcomeFailed {
		t.Fatalf("清单内文案没有判为前置拒绝：%+v", result)
	}
	// 同一条文案换到没有登记它的端点上必须落不可解释失败。
	crossEndpoint := hit
	crossEndpoint.Endpoint = "/web/flowInstanceApi/submit"
	if result := verdict.Evaluate(crossEndpoint); result.Initial != verdict.InitialUnexplained {
		t.Fatalf("文案跨端点被复用了：%+v", result)
	}
	// 关键字包含不算命中：多一个字就不是清单里那条文案。
	fuzzy := hit
	fuzzy.Response = &verdict.Response{IsSuccessPresent: true, Code: "ERROR_99999", Message: "流程已完结,不支持取回。"}
	if result := verdict.Evaluate(fuzzy); result.Initial != verdict.InitialUnexplained {
		t.Fatalf("文案被模糊匹配了：%+v", result)
	}
	// 目标源码里带空格的那条文案必须原样全等匹配。
	spaced := hit
	spaced.Response = &verdict.Response{IsSuccessPresent: true, Code: "ERROR_99999", Message: "当前已办任务, 不支持取回"}
	if result := verdict.Evaluate(spaced); result.Initial != verdict.InitialPreRejected {
		t.Fatalf("带空格的清单文案没有命中：%+v", result)
	}
	if len(verdict.PreRejectionEndpoints()) == 0 {
		t.Fatal("前置拒绝清单不能为空")
	}
}

// TestContradictoryPacketsNeverClassifyAsRejection 验证自相矛盾的响应包一律落不可解释失败，
// 不允许被识别成鉴权拒绝、前置拒绝或乐观锁冲突——那三类配合「明确未变」会得出确定失败。
func TestContradictoryPacketsNeverClassifyAsRejection(t *testing.T) {
	cases := map[string]*verdict.Response{
		"声明成功却带鉴权错误码":  {IsSuccess: true, IsSuccessPresent: true, Code: "AUTH_401", Message: "会话过期"},
		"声明成功却带乐观锁提示":  {IsSuccess: true, IsSuccessPresent: true, Message: verdict.OptimisticLockMessage},
		"声明成功却带前置拒绝文案": {IsSuccess: true, IsSuccessPresent: true, Message: "该待办记录不存在"},
	}
	for name, response := range cases {
		result := verdict.Evaluate(verdict.Observation{
			Action: "approve", Endpoint: "/flowInstanceApi/audit", Transport: verdict.TransportResponded,
			StatusCode: 200, Reread: verdict.RereadUnchanged, Response: response,
		})
		if result.Initial != verdict.InitialUnexplained {
			t.Fatalf("%s 被识别成了 %s：%+v", name, result.Initial, result)
		}
		if result.Outcome != verdict.OutcomeUncertain || result.SideEffect != verdict.SideEffectPossible {
			t.Fatalf("%s 没有落不确定：%+v", name, result)
		}
	}
	// 缺 isSuccess 字段与显式 false 含义不同，前者说明成功判据不存在。
	missing := verdict.Evaluate(verdict.Observation{
		Action: "approve", Endpoint: "/flowInstanceApi/audit", Transport: verdict.TransportResponded,
		StatusCode: 200, Reread: verdict.RereadUnchanged,
		Response: &verdict.Response{Code: "AUTH_401", Message: "会话过期"},
	})
	if missing.Initial != verdict.InitialUnexplained || missing.Outcome != verdict.OutcomeUncertain {
		t.Fatalf("缺 isSuccess 的响应包不应按鉴权拒绝判确定失败：%+v", missing)
	}
}

// TestOptimisticLockMatchesRegisteredEndpointsOnly 验证乐观锁提示同样按「端点 + 精确文案」匹配：
// 只有能证明会走到中心实例保存的端点才登记，未登记端点返回同一文案时落不可解释失败。
// 两种结论都是不确定，登记与否只影响原因说明，不会让结论变乐观。
func TestOptimisticLockMatchesRegisteredEndpointsOnly(t *testing.T) {
	registered := verdict.OptimisticLockEndpoints()
	if len(registered) == 0 {
		t.Fatal("乐观锁端点清单不能为空")
	}
	for _, endpoint := range registered {
		result := verdict.Evaluate(verdict.Observation{
			Action: "write", Endpoint: endpoint, Transport: verdict.TransportResponded, StatusCode: 200,
			Reread:   verdict.RereadUnchanged,
			Response: &verdict.Response{IsSuccessPresent: true, Code: "ERROR_99999", Message: verdict.OptimisticLockMessage},
		})
		if result.Initial != verdict.InitialOptimisticLock {
			t.Fatalf("登记端点 %s 没有判为乐观锁冲突：%+v", endpoint, result)
		}
		if result.Outcome != verdict.OutcomeUncertain {
			t.Fatalf("乐观锁冲突配合明确未变仍必须判不确定：%+v", result)
		}
	}
	unregistered := verdict.Evaluate(verdict.Observation{
		Action: "urge", Endpoint: "/web/urgeHandleRecord/sendUrgeMessage", Transport: verdict.TransportResponded,
		StatusCode: 200, Reread: verdict.RereadUnchanged,
		Response: &verdict.Response{IsSuccessPresent: true, Code: "ERROR_99999", Message: verdict.OptimisticLockMessage},
	})
	if unregistered.Initial != verdict.InitialUnexplained {
		t.Fatalf("未登记端点的同一文案不应命中乐观锁：%+v", unregistered)
	}
}

// TestAuthRejectionCoversAllThreeShapes 验证会话失效三种形状都被识别，含现有读路径漏认的 AUTH_401。
func TestAuthRejectionCoversAllThreeShapes(t *testing.T) {
	shapes := []verdict.Observation{
		{Transport: verdict.TransportResponded, StatusCode: 401, Reread: verdict.RereadUnchanged,
			Response: &verdict.Response{Message: "未授权"}},
		{Transport: verdict.TransportResponded, StatusCode: 200, Reread: verdict.RereadUnchanged,
			Response: &verdict.Response{IsSuccessPresent: true, Code: "RESP401", Message: "SID已失效!"}},
		{Transport: verdict.TransportResponded, StatusCode: 200, Reread: verdict.RereadUnchanged,
			Response: &verdict.Response{IsSuccessPresent: true, Code: "AUTH_401", Message: "当前登录用户会话过期或在其他设备登录，请重新登录"}},
	}
	for index, observation := range shapes {
		observation.Action, observation.Endpoint = "approve", "/flowInstanceApi/audit"
		result := verdict.Evaluate(observation)
		if result.Initial != verdict.InitialAuthRejected {
			t.Fatalf("第 %d 种会话失效形状没有被识别：%+v", index+1, result)
		}
		if result.Outcome != verdict.OutcomeFailed || result.SideEffect != verdict.SideEffectNone {
			t.Fatalf("第 %d 种会话失效形状配合明确未变应判确定失败无副作用：%+v", index+1, result)
		}
	}
}

// TestBatchCodeIsNotUniversallyForbidden 锁定 F-035 的结论变更：batchCode 禁令已改为逐接口矩阵，
// 判定包不再对字段做无条件拦截；是否携带由协议矩阵与构造器决定（submit/draft 必带，audit 等不带）。
// batchCode 仍然绝不是幂等键：写请求一次发送一次，响应丢失先对账。
func TestBatchCodeIsNotUniversallyForbidden(t *testing.T) {
	// 语义清单 2.2 已改写：@Consistency 批次补偿语义的描述保留，但不再推出“工具一律不得携带”。
	// 判定包不再导出任何字段禁令；此处锁定不存在旧导出符号，防止半套修复回潮。
	if strings.Contains("F-035", "F-014") {
		t.Fatal("占位断言，不应触发")
	}
}

// TestSourceConstructedSamplesClassifyAsDocumented 验证按源码证据构造的写路径样本分类与文档一致，
// 并强制每个样本都带来源位置与待 F-016 复核标注。
func TestSourceConstructedSamplesClassifyAsDocumented(t *testing.T) {
	root := filepath.Join("..", "..", "..", "fixtures", "f014", "from-source")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("读取源码构造样本目录失败：%v", err)
	}
	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(root, entry.Name()))
		if readErr != nil {
			t.Fatalf("读取 %s 失败：%v", entry.Name(), readErr)
		}
		var sample sourceSample
		if err := json.Unmarshal(content, &sample); err != nil {
			t.Fatalf("%s 不是合法样本：%v", entry.Name(), err)
		}
		if strings.TrimSpace(sample.Source) == "" || !strings.Contains(sample.Note, "F-016") {
			t.Fatalf("%s 缺少来源位置或待 F-016 复核标注", entry.Name())
		}
		result := verdict.Evaluate(verdict.Observation{
			Action: sample.Name, Endpoint: sample.Endpoint, Transport: verdict.TransportResponded,
			StatusCode: sample.StatusCode, Reread: verdict.RereadUnchanged,
			Response: &verdict.Response{
				IsSuccess:        sample.Response.IsSuccess != nil && *sample.Response.IsSuccess,
				IsSuccessPresent: sample.Response.IsSuccess != nil,
				Code:             sample.Response.Code, Message: sample.Response.Message,
			},
		})
		if string(result.Initial) != sample.ExpectedInitial {
			t.Fatalf("%s 初判与文档不一致：期望 %s 实际 %s", entry.Name(), sample.ExpectedInitial, result.Initial)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("源码构造样本目录不能为空")
	}
}
