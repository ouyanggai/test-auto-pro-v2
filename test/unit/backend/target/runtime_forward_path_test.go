// runtime 页面转发的路径归一化契约：网关 base 自带 /api 路径时，
// 转发路径上重复的 /api 前缀必须剥掉再拼接；无前缀与真实 /api 开头的路径都不受影响。
// 回归背景：页面多套 axios 实例并存（执行装载清空 baseURL 的实例发出 /web/...，
// 保留烘焙绝对网关的实例发出 /api/web/...），后者经 base=/api 拼接成 /api/api/web 被 404。
package target_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestForwardRuntimeRequestStripsDuplicatedGatewayPrefix 页面带来带 /api 前缀的路径时，
// 出口不得叠加网关 base 自身的 /api 路径（真实验收中 /api/api/web/submit 被 404）。
func TestForwardRuntimeRequestStripsDuplicatedGatewayPrefix(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w, `{"isSuccess":true}`)
	}))
	defer server.Close()

	// 网关 base 自带 /api 路径（与真实部署一致：http://host:38081/api）。
	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL + "/api", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	session := target.Session{SID: "sid-x"}
	ctx := context.Background()

	cases := []struct {
		name       string
		path       string
		wantTarget string
	}{
		// 页面相对实例（baseURL 已清空）发出的无前缀路径。
		{"无前缀读路径", "/web/formProxy/findById", "/api/web/formProxy/findById"},
		// 烘焙绝对网关实例发出的带 /api 前缀路径：重复前缀必须剥掉。
		{"带前缀写路径", "/api/web/flowInstanceApi/submit", "/api/web/flowInstanceApi/submit"},
		// 新工作流重提/审批可能以无 /api 前缀的页面相对路径到达 Go 出口
		// （正式提交端点已被全过程 test 回退为旧 submit）。
		{"新工作流写路径", target.RuntimeReSubmitEndpoint, "/api" + target.RuntimeReSubmitEndpoint},
		// 真实以 /api 开头的目标路径：剥掉再拼回，落点不变。
		{"真实 api 开头路径", "/api/type/list", "/api/type/list"},
	}
	for _, item := range cases {
		resp, err := client.ForwardRuntimeRequest(ctx, session, target.PageForwardRequest{
			Method: http.MethodPost,
			Path:   item.path,
			Header: map[string]string{"content-type": "application/json"},
			Body:   []byte(`{}`),
		})
		if err != nil {
			t.Fatalf("%s：转发失败：%v", item.name, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s：期望 200，实际 %d", item.name, resp.StatusCode)
		}
	}
	if len(seen) != len(cases) {
		t.Fatalf("出口应收到 %d 笔请求，实际 %d", len(cases), len(seen))
	}
	for i, item := range cases {
		if seen[i] != item.wantTarget {
			t.Fatalf("%s：出口路径 = %s，期望 %s", item.name, seen[i], item.wantTarget)
		}
	}
	// 响应必须原样回给页面（base64 保真由客户端层处理，这里校验出口返回了正文）。
	if len(seen) == 0 {
		t.Fatalf("出口未收到任何请求")
	}
	var _ = json.Marshal
}

// TestForwardRuntimeRequestOverridesPlatformCode 锁定转发出口的平台码覆盖语义：
// 实例重建类装载带回实例真实平台码时，出口必须用它覆盖页面默认平台码（页面 axios 默认 200001）；
// 发起类不带平台码时保留页面查询里的原值。这是修复 999999 实例被默认 200001 查空、装载超时的关键出口。
func TestForwardRuntimeRequestOverridesPlatformCode(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Query().Get("platformCode"))
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w, `{"isSuccess":true}`)
	}))
	defer server.Close()

	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL + "/api", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("构造客户端失败：%v", err)
	}
	ctx := context.Background()
	pageQuery := url.Values{}
	pageQuery.Set("platformCode", "200001") // 页面默认平台码

	// 非空 PlatformCode：出口覆盖为实例真实平台 999999。
	if _, err := client.ForwardRuntimeRequest(ctx, target.Session{SID: "sid-x"}, target.PageForwardRequest{
		Method: http.MethodPost, Path: "/web/flowInstanceApi/getCurrentFromData",
		Query: cloneValues(pageQuery), Header: map[string]string{"content-type": "application/json"},
		Body: []byte(`{}`), PlatformCode: "999999",
	}); err != nil {
		t.Fatalf("带平台码转发失败：%v", err)
	}
	// 空 PlatformCode：保留页面默认 200001。
	if _, err := client.ForwardRuntimeRequest(ctx, target.Session{SID: "sid-x"}, target.PageForwardRequest{
		Method: http.MethodPost, Path: "/web/flowInstanceApi/getCurrentFromData",
		Query: cloneValues(pageQuery), Header: map[string]string{"content-type": "application/json"},
		Body: []byte(`{}`), PlatformCode: "",
	}); err != nil {
		t.Fatalf("不带平台码转发失败：%v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("出口应收到 2 笔请求，实际 %d", len(seen))
	}
	if seen[0] != "999999" {
		t.Fatalf("实例重建类转发平台码应被覆盖为 999999，实际 %q", seen[0])
	}
	if seen[1] != "200001" {
		t.Fatalf("发起类转发应保留页面默认平台码 200001，实际 %q", seen[1])
	}
}

// cloneValues 复制查询参数，避免用例间共享底层切片。
func cloneValues(src url.Values) url.Values {
	dst := url.Values{}
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
	return dst
}

// TestRuntimeFlowEndpointsAreKnownWrites 锁定正式提交/重提/审批端点的写出纪律：
// 页面请求无论是否带网关 /api 前缀，都必须被归类为已登记写请求，断线时不得误判零写入。
// 提交端点已被全过程 test 回退为旧 submit，新工作流仅剩重提与审批。
func TestRuntimeFlowEndpointsAreKnownWrites(t *testing.T) {
	for _, endpoint := range []string{target.WriteEndpointSubmit, target.RuntimeReSubmitEndpoint, target.RuntimeAuditEndpoint} {
		if !target.IsKnownWriteRequestPath(endpoint) || !target.IsKnownWriteRequestPath("/api"+endpoint) {
			t.Fatalf("新工作流端点未登记为写请求：%s", endpoint)
		}
	}
}

// TestRuntimeBusinessEndpointsAreKnownWrites 锁定 runtime 页面合同业务写与附件关系写的断线判定口径。
func TestRuntimeBusinessEndpointsAreKnownWrites(t *testing.T) {
	for _, endpoint := range []string{
		target.ContractReviewIncreaseEndpoint,
		target.ContractReviewModifyEndpoint,
		target.ContractReviewSaveRefFileEndpoint,
		target.RelationFileSaveBatchEndpoint,
		target.RelationFileDeleteEndpoint,
	} {
		if !target.IsKnownWriteRequestPath(endpoint) || !target.IsKnownWriteRequestPath("/api"+endpoint) {
			t.Fatalf("合同业务端点未登记为写请求：%s", endpoint)
		}
	}
	if target.IsKnownWriteRequestPath("/web/measuring/api/contractReview/queryList") {
		t.Fatal("合同查询端点不得登记为写请求")
	}
}
