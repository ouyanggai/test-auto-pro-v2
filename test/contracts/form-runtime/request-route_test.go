// T006 Go 侧请求路由契约：转发载荷校验、路径允许清单与 TS 侧 target-route.ts 镜像锁定。
package formruntime

import (
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/runtimeexec"
)

// TestForwardedRequestValidation 转发载荷必须方法/URL/路径/编码齐全，路径不在清单直接拒绝。
func TestForwardedRequestValidation(t *testing.T) {
	valid := json.RawMessage(`{
		"method": "POST",
		"url": "https://target.example.com/web/flowInstanceApi/submit",
		"headers": {"Content-Type": "application/json"},
		"body": "e30=",
		"bodyEncoding": "base64",
		"action": "submit",
		"writeSequence": 1
	}`)
	request, err := runtimeexec.ParseForwardedRequest(valid)
	if err != nil {
		t.Fatalf("合法转发载荷被拒绝：%v", err)
	}
	if request.Headers["content-type"] != "application/json" {
		t.Fatalf("请求头必须小写归一：%v", request.Headers)
	}
	if request.Method != "POST" {
		t.Fatalf("方法必须大写归一：%s", request.Method)
	}

	cases := []struct {
		name string
		raw  string
		code string
	}{
		{"缺少方法", `{"url":"https://t.example.com/web/x"}`, "forward_invalid"},
		{"URL 不合法", `{"method":"GET","url":"::::"}`, "forward_invalid"},
		{"路径不在允许清单", `{"method":"GET","url":"https://target.example.com/other/path"}`, "path_not_allowed"},
		{"未知正文编码", `{"method":"POST","url":"https://t.example.com/web/x","bodyEncoding":"hex"}`, "forward_invalid"},
		{"写序号越界", `{"method":"POST","url":"https://t.example.com/web/x","writeSequence":3}`, "forward_invalid"},
	}
	for _, item := range cases {
		_, err := runtimeexec.ParseForwardedRequest(json.RawMessage(item.raw))
		if err == nil {
			t.Fatalf("%s：非法转发载荷通过了校验", item.name)
		}
		protocolErr, ok := err.(*runtimeexec.ProtocolError)
		if !ok || protocolErr.Code != item.code {
			t.Fatalf("%s：期望错误码 %s，实际 %v", item.name, item.code, err)
		}
	}
}

// TestTargetPathPrefixesMirror 目标路径前缀与 TS 侧 target-route.ts 的 TARGET_PATH_PREFIXES 镜像；
// 两侧必须同步修改，任何一侧单方面扩张都意味着绕过校验。
func TestTargetPathPrefixesMirror(t *testing.T) {
	joined := strings.Join(runtimeexec.TargetPathPrefixes, ",")
	if joined != "/web/,/flowInstanceApi" {
		t.Fatalf("目标路径前缀清单变化必须与 TS 侧同步：%s", joined)
	}
	if !runtimeexec.IsTargetPath("/web/flowInstanceApi/submit") {
		t.Fatalf("/web/ 前缀必须命中")
	}
	if !runtimeexec.IsTargetPath("/flowInstanceApi/audit") {
		t.Fatalf("/flowInstanceApi 前缀必须命中")
	}
	if runtimeexec.IsTargetPath("/static/app.js") {
		t.Fatalf("runtime 自身资源不允许被当成目标请求")
	}
}
