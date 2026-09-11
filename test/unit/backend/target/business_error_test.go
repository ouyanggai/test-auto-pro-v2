package target_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestListTemplatesPreservesBusinessRejection 验证完整业务拒绝原样保留 code/message，且只请求一次。
func TestListTemplatesPreservesBusinessRejection(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"isSuccess":false,"code":"FLOW_409","message":"流程状态已发生变化"}`))
	}))
	defer server.Close()
	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	_, err = client.ListTemplates(context.Background(), target.Session{SID: "sid"}, "", 1, 20)
	var rejection *target.BusinessRejection
	if !errors.As(err, &rejection) {
		t.Fatalf("应返回业务拒绝，实际：%T %v", err, err)
	}
	if rejection.Code != "FLOW_409" || rejection.Message != "流程状态已发生变化" {
		t.Fatalf("业务拒绝未保留原始内容：%+v", rejection)
	}
	if target.IsRetryableReadError(err) {
		t.Fatal("完整业务拒绝不应进入只读网络重试")
	}
	if calls != 1 {
		t.Fatalf("业务拒绝请求次数 = %d，期望 1", calls)
	}
}

// TestLoginPreservesBusinessRejection 验证登录完整业务拒绝保留目标 code/message，且不伪装成网络错误。
func TestLoginPreservesBusinessRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"isSuccess":false,"code":"LOGIN_403","message":"账号或密码错误"}`))
	}))
	defer server.Close()
	client, err := target.NewClient(target.ClientConfig{
		BaseURL:       server.URL,
		Timeout:       2 * time.Second,
		LoginPassword: "password",
		LoginAESKey:   "0123456789abcdef",
	})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	_, err = client.Login(context.Background(), "account-a")
	if !target.IsKind(err, target.ErrorLoginRejected) {
		t.Fatalf("登录业务拒绝未映射为登录拒绝：%T %v", err, err)
	}
	var rejection *target.BusinessRejection
	if !errors.As(err, &rejection) || rejection.Code != "LOGIN_403" || rejection.Message != "账号或密码错误" {
		t.Fatalf("登录业务拒绝未保留原始内容：%T %+v", err, rejection)
	}
	if target.IsRetryableWriteConnectError(err) {
		t.Fatal("登录业务拒绝不应进入连接重试")
	}
}

// TestDirectoryPreservesBusinessRejection 验证人员目录读取不把完整业务拒绝改写成泛化错误。
func TestDirectoryPreservesBusinessRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"isSuccess":false,"code":"DIRECTORY_403","message":"目录无权限"}`))
	}))
	defer server.Close()
	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	_, err = client.FormIdentityContext(context.Background(), target.Session{SID: "sid", CompanyID: "company"})
	var rejection *target.BusinessRejection
	if !errors.As(err, &rejection) || rejection.Code != "DIRECTORY_403" || rejection.Message != "目录无权限" {
		t.Fatalf("目录业务拒绝未保留原始内容：%T %+v", err, rejection)
	}
}

// TestRetryClassificationUsesTransportPhase 验证连接未建立可重试，响应已收到或响应中断不可重试。
func TestRetryClassificationUsesTransportPhase(t *testing.T) {
	connectError := target.NewError(target.ErrorTimeout, nil)
	connectTyped := connectError.(*target.Error)
	connectTyped.Transport = target.TransportConnectFailed
	if !target.IsRetryableReadError(connectError) || !target.IsRetryableWriteConnectError(connectError) {
		t.Fatal("连接阶段超时应允许读重试和写连接重试")
	}
	responded := target.NewError(target.ErrorTimeout, nil)
	respondedTyped := responded.(*target.Error)
	respondedTyped.Transport = target.TransportResponded
	if target.IsRetryableReadError(responded) || target.IsRetryableWriteConnectError(responded) {
		t.Fatal("已收到响应的超时不应重试")
	}
	interrupted := target.NewError(target.ErrorUnavailable, nil)
	interruptedTyped := interrupted.(*target.Error)
	interruptedTyped.Transport = target.TransportInterrupted
	if target.IsRetryableWriteConnectError(interrupted) {
		t.Fatal("请求已发出但响应丢失不应重发写请求")
	}
}
