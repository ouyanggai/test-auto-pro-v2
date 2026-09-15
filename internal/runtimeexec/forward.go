// T006 Go 侧请求转发事实：runtime 拦截到的目标请求在此校验路径允许清单后交给 TargetClient。
// 路径前缀与 form-runtime/executor/src/target-route.ts 的 TARGET_PATH_PREFIXES 镜像维护，
// 两侧必须同步修改；这里同时是「未登记目标写请求不能绕过 Go」的服务端最后一道判定。
package runtimeexec

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// TargetPathPrefixes 是目标平台请求的路径前缀允许清单：命中任一前缀的页面请求必须转发 Go。
var TargetPathPrefixes = []string{"/web/", "/flowInstanceApi", "/api/"}

// ForwardedRequest 是 request_forward 消息的载荷形状（与 TS 侧 RequestForwardPayload 对齐）。
// Body 为原样正文；二进制（上传）按 bodyEncoding=base64 保真，内网日志不脱敏。
type ForwardedRequest struct {
	Method        string            `json:"method"`
	URL           string            `json:"url"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body,omitempty"`
	BodyEncoding  string            `json:"bodyEncoding,omitempty"`
	Action        string            `json:"action,omitempty"`
	WriteSequence int               `json:"writeSequence,omitempty"`
}

// ParseForwardedRequest 解析并校验转发载荷；解析失败或路径不在允许清单都返回 ProtocolError。
func ParseForwardedRequest(raw json.RawMessage) (*ForwardedRequest, error) {
	var request ForwardedRequest
	if len(raw) == 0 {
		return nil, newProtocolError("forward_invalid", "request_forward 载荷不能为空")
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, newProtocolError("forward_invalid", "request_forward 载荷解析失败：%v", err)
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return &request, nil
}

// Validate 校验转发载荷：方法、URL、路径前缀、正文编码与写序号；请求头小写归一。
// 纪律：路径不在允许清单的目标请求直接拒绝，绝不放行为直连目标平台。
func (r *ForwardedRequest) Validate() error {
	if strings.TrimSpace(r.Method) == "" {
		return newProtocolError("forward_invalid", "转发请求缺少 method")
	}
	parsed, err := url.Parse(strings.TrimSpace(r.URL))
	if err != nil || parsed.Host == "" {
		return newProtocolError("forward_invalid", "转发请求 URL 不合法：%s", r.URL)
	}
	if !IsTargetPath(parsed.Path) {
		return newProtocolError("path_not_allowed", "目标路径不在允许清单：%s", parsed.Path)
	}
	switch strings.ToLower(r.BodyEncoding) {
	case "", "utf8", "base64":
	default:
		return newProtocolError("forward_invalid", "转发正文编码只支持 utf8/base64：%s", r.BodyEncoding)
	}
	if r.WriteSequence < 0 || r.WriteSequence > 2 {
		// 写序号只允许 0（非复合动作）、1（转发主请求）、2（转发附言）。
		return newProtocolError("forward_invalid", "转发写序号只允许 0/1/2：%d", r.WriteSequence)
	}
	r.normalize()
	return nil
}

// IsTargetPath 判定路径是否包含目标段（构建产物可能带 /api 等烘焙前缀，
// 与 TS 侧 isTargetRequest 的包含判定同口径）。
func IsTargetPath(path string) bool {
	for _, prefix := range TargetPathPrefixes {
		if strings.Contains(path, prefix) {
			return true
		}
	}
	return false
}

// normalize 归一请求头键名与方法名，保证日志与转发形状稳定。
func (r *ForwardedRequest) normalize() {
	r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	if r.Headers == nil {
		r.Headers = map[string]string{}
		return
	}
	normalized := make(map[string]string, len(r.Headers))
	for name, value := range r.Headers {
		normalized[strings.ToLower(name)] = value
	}
	r.Headers = normalized
}

// String 返回转发请求的稳定摘要（方法 + URL + 写序号），供 step.log 与运行详情使用。
func (r *ForwardedRequest) String() string {
	return fmt.Sprintf("%s %s (writeSequence=%d)", r.Method, r.URL, r.WriteSequence)
}
