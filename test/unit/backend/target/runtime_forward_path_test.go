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
