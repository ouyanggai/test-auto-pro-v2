package executorservice

import (
	"encoding/json"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/runtimeexec"
)

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
	cases := []struct {
		name string
		raw  string
		code string
	}{
		{"缺少方法", `{"url":"https://t.example.com/web/x"}`, "forward_invalid"},
		{"路径不在允许清单", `{"method":"GET","url":"https://target.example.com/other/path"}`, "path_not_allowed"},
		{"未知正文编码", `{"method":"POST","url":"https://t.example.com/web/x","bodyEncoding":"hex"}`, "forward_invalid"},
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

func TestTargetPathPrefixesMirror(t *testing.T) {
	joined := strings.Join(runtimeexec.TargetPathPrefixes, ",")
	if joined != "/web/,/flowInstanceApi,/newWorkflowCenter/,/api/" {
		t.Fatalf("目标路径前缀清单变化必须与 TS 侧同步：%s", joined)
	}
	if !runtimeexec.IsTargetPath("/web/flowInstanceApi/submit") {
		t.Fatalf("/web/ 前缀必须命中")
	}
	if !runtimeexec.IsTargetPath("/newWorkflowCenter/flowInstance/reSubmit") {
		t.Fatalf("新工作流前缀必须命中")
	}
	if runtimeexec.IsTargetPath("/static/app.js") {
		t.Fatalf("runtime 自身资源不允许被当成目标请求")
	}
}
