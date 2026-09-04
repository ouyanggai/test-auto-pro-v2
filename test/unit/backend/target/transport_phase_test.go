package target_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// newF016Client 用配置化的超时构造目标客户端；超时预算全部来自配置值，不在用例里写死全局行为。
func newF016Client(t *testing.T, baseURL string, timeout time.Duration) *target.Client {
	t.Helper()
	client, err := target.NewClient(target.ClientConfig{
		BaseURL:       baseURL,
		LoginPassword: "test-password",
		LoginAESKey:   "0123456789abcdef0123456789abcdef",
		Timeout:       timeout,
	})
	if err != nil {
		t.Fatalf("构造目标客户端失败：%v", err)
	}
	return client
}

// callLogin 发一次真实的登录请求作为传输层探针：登录走唯一请求出口 call，失败阶段分类与业务请求一致。
func callLogin(t *testing.T, client *target.Client) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := client.Login(ctx, "f016-probe")
	return err
}

// TestF016TransportConnectRefusedIsDeterministicFailure 断言“连接被拒”归入连接阶段未完成：
// 请求没有到达目标进程，确定失败且无副作用，不得判成响应丢失（不确定）。
func TestF016TransportConnectRefusedIsDeterministicFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("无法分配本地端口：%v", err)
	}
	closedAddr := listener.Addr().String()
	listener.Close()

	client := newF016Client(t, "http://"+closedAddr, 5*time.Second)
	err = callLogin(t, client)
	if err == nil {
		t.Fatal("连接被拒必须返回错误")
	}
	if got := target.TransportOf(err); got != target.TransportConnectFailed {
		t.Fatalf("连接被拒应归入 connect_refused，实际 %s（err=%v）", got, err)
	}
	var targetErr *target.Error
	if !errors.As(err, &targetErr) {
		t.Fatalf("错误应为目标适配层错误：%v", err)
	}
	if targetErr.Transport != target.TransportConnectFailed {
		t.Fatalf("错误结构未携带独立传输阶段字段：%+v", targetErr)
	}
}

// TestF016TransportConnectTimeoutIsDeterministicFailure 断言“连接超时”同样归入连接阶段未完成：
// 超时只说明还没连上目标，不等于请求已发出，结论必须是确定失败且无副作用。
func TestF016TransportConnectTimeoutIsDeterministicFailure(t *testing.T) {
	// 192.0.2.1 是 RFC 5737 保留网段（TEST-NET-1），SYN 会被网关丢弃，必然以连接超时收场。
	client := newF016Client(t, "http://192.0.2.1:1", 600*time.Millisecond)
	err := callLogin(t, client)
	if err == nil {
		t.Fatal("连接超时必须返回错误")
	}
	if got := target.TransportOf(err); got != target.TransportConnectFailed {
		t.Fatalf("连接超时应归入 connect_refused，实际 %s（err=%v）", got, err)
	}
	if !target.IsKind(err, target.ErrorTimeout) {
		t.Fatalf("连接超时应保留超时错误分类：%v", err)
	}
}

// TestF016TransportResponseLostIsUncertain 断言“请求已发出但响应丢失”归入 interrupted：
// 写请求可能已在目标生效，结论是不确定，任何情况下不得自动判成确定失败。
func TestF016TransportResponseLostIsUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 进入 handler 时服务端已收到请求头。劫持连接后先等待片刻确保客户端写完请求
		//（WroteRequest 已发生），再不写任何响应直接断开：客户端读响应时收到 EOF，
		// 复现“请求已发出、响应丢失”。
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("劫持连接失败：%v", err)
			return
		}
		defer conn.Close()
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client := newF016Client(t, server.URL, 5*time.Second)
	err := callLogin(t, client)
	if err == nil {
		t.Fatal("响应丢失必须返回错误")
	}
	if got := target.TransportOf(err); got != target.TransportInterrupted {
		t.Fatalf("响应丢失应归入 interrupted，实际 %s（err=%v）", got, err)
	}
}

// TestF016TransportRespondedCarriesHttpStatus 断言“收到完整响应”归入 responded：
// 即使是 5xx 这类失败响应，传输阶段也是已响应，后续结论交给响应侧判定。
func TestF016TransportRespondedCarriesHttpStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream gone"))
	}))
	defer server.Close()

	client := newF016Client(t, server.URL, 5*time.Second)
	err := callLogin(t, client)
	if err == nil {
		t.Fatal("5xx 响应必须返回错误")
	}
	if got := target.TransportOf(err); got != target.TransportResponded {
		t.Fatalf("完整响应应归入 responded，实际 %s（err=%v）", got, err)
	}
	var targetErr *target.Error
	if !errors.As(err, &targetErr) {
		t.Fatalf("错误应为目标适配层错误：%v", err)
	}
	if targetErr.HTTPStatus != http.StatusBadGateway {
		t.Fatalf("完整响应应保留 HTTP 状态码：%+v", targetErr)
	}
}

// TestF016TransportUnclassifiedStaysUncertain 断言无法归类的传输失败按不确定兜底，
// 绝不乐观归档成“确定失败”；请求成功完成时为 responded。
func TestF016TransportUnclassifiedStaysUncertain(t *testing.T) {
	if got := target.TransportOf(nil); got != target.TransportResponded {
		t.Fatalf("无错误应视为 responded，实际 %s", got)
	}
	if got := target.TransportOf(errors.New("无法归类的传输错误")); got != target.TransportUnclassified {
		t.Fatalf("非适配层错误应按 unclassified 兜底，实际 %s", got)
	}
}

// TestF016ClientBypassesLocalProxy 断言目标客户端对请求显式绕过本机代理：
// 即使设置了 http_proxy 环境变量，目标请求的代理决策也必须是无代理，
// 不依赖开发机是否正确配置 no_proxy（纲领第 4.4.1 节实测：本机代理会截走内网目标请求）。
func TestF016ClientBypassesLocalProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:7897")
	t.Setenv("http_proxy", "http://127.0.0.1:7897")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7897")
	t.Setenv("https_proxy", "http://127.0.0.1:7897")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	client := newF016Client(t, server.URL, 5*time.Second)
	request, err := http.NewRequest(http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatalf("构造请求失败：%v", err)
	}
	proxyURL, err := client.ProxyFor(request)
	if err != nil {
		t.Fatalf("代理决策返回错误：%v", err)
	}
	if proxyURL != nil {
		t.Fatalf("目标请求必须显式绕过本机代理，实际决策为 %s", proxyURL)
	}
}
