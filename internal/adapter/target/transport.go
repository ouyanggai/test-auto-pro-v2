package target

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sync/atomic"
)

// TransportPhase 描述一次目标请求在传输层的失败阶段，是交给 internal/engine/verdict 的独立事实，
// 只依据传输层证据（net.OpError.Op、httptrace.WroteRequest）判定，不靠错误文案推断。
// 四个取值与纲领第 4.4.1 节 / F-014 第 1.6 节的四档传输结论一一对应。
type TransportPhase string

const (
	// TransportResponded 表示已收到完整 HTTP 响应，后续结论按响应内容与事实重读判定。
	TransportResponded TransportPhase = "responded"
	// TransportConnectFailed 表示连接阶段未完成（连接被拒、连接超时、DNS 解析失败）：
	// 请求没有到达目标进程，确定失败且无副作用。
	TransportConnectFailed TransportPhase = "connect_refused"
	// TransportInterrupted 表示请求已发出但响应丢失（响应超时、连接中断）：
	// 写是否已在目标生效无法确定，任何情况下不得自动重发。
	TransportInterrupted TransportPhase = "interrupted"
	// TransportUnclassified 表示传输层证据不足以归类，按不确定处理，不做乐观归类。
	TransportUnclassified TransportPhase = "unclassified"
)

// classify 依据传输层事实给失败分档。
// 判定顺序不可调换：只要请求体已完整写出（WroteRequest 已发生），请求就到达了目标 TCP 连接，
// 无论此前是否有过失败的连接尝试，响应丢失一律按不确定处理；
// 只有在请求从未写出时，连接阶段失败（ConnectDone 报错、错误链里存在 dial 阶段的 net.OpError，
// 或错误返回时连接仍在建立中——请求级超时打断拨号时错误链往往没有 dial OpError）才构成
// “连接阶段未完成”的确定失败；两者都无法证明时按无法归类（不确定）兜底。
func (p *transportProbe) classify(err error) TransportPhase {
	if err == nil {
		return TransportResponded
	}
	if p.wroteRequest.Load() {
		return TransportInterrupted
	}
	if p.connectFailed.Load() {
		return TransportConnectFailed
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return TransportConnectFailed
	}
	if p.connectStarted.Load() && !p.connectDone.Load() {
		return TransportConnectFailed
	}
	return TransportUnclassified
}

// TransportOf 从错误链中取出传输层失败阶段，供执行器构造判定包输入。
// err 为 nil 表示请求已完成并收到响应；错误链里没有本适配层的分档事实时，
// 一律按无法归类处理（verdict 侧按不确定兜底），绝不推断成“确定失败”。
func TransportOf(err error) TransportPhase {
	if err == nil {
		return TransportResponded
	}
	var targetErr *Error
	if errors.As(err, &targetErr) && targetErr.Transport != "" {
		return targetErr.Transport
	}
	return TransportUnclassified
}

// bypassProxy 显式返回“不使用代理”。内网目标请求绝不经过本机代理：
// 纲领第 4.4.1 节实测本机代理会截走发往内网目标的请求并返回空正文 502，
// 因此这里不依赖开发机是否正确设置 no_proxy，直接对一切目标请求绕过环境代理。
func bypassProxy(*http.Request) (*url.URL, error) {
	return nil, nil
}

// transportProbe 跟踪单次请求的传输层事实：连接是否开始/完成/失败、请求体是否已完整写出。
// 全部判据由 httptrace 回调在传输层置位，不依赖错误文案推断。
type transportProbe struct {
	connectFailed  atomic.Bool
	connectStarted atomic.Bool
	connectDone    atomic.Bool
	wroteRequest   atomic.Bool
}

// trace 返回挂接到请求上下文的客户端追踪。
// ConnectStart/ConnectDone 对每次连接尝试回调，连接超时、连接被拒、DNS 失败都会在这里报错；
// WroteRequest 只在请求体完整写到连接上之后触发。
func (p *transportProbe) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		ConnectStart: func(network, addr string) {
			p.connectStarted.Store(true)
		},
		ConnectDone: func(network, addr string, err error) {
			p.connectDone.Store(true)
			if err != nil {
				p.connectFailed.Store(true)
			}
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			p.wroteRequest.Store(true)
		},
	}
}
