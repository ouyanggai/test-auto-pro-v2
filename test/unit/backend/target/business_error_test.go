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
