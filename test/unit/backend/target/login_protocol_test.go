package target_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// loginBodyShape 解析登录请求体，用于核对顶层 sign 与 data.code 三段协议事实。
type loginBodyShape struct {
	Sign string `json:"sign"`
	Data struct {
		Account string `json:"account"`
		Code    string `json:"code"`
	} `json:"data"`
}

// newLoginTestClient 按最小可登录配置构造客户端。
func newLoginTestClient(t *testing.T, baseURL, loginCode string) *target.Client {
	t.Helper()
	client, err := target.NewClient(target.ClientConfig{
		BaseURL:       baseURL,
		Timeout:       2 * time.Second,
		LoginPassword: "password",
		LoginAESKey:   "0123456789abcdef",
		LoginCode:     loginCode,
	})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	return client
}

// TestLoginFetchesFreshFlagPerAttempt 锁定协议事实①②：每次登录前都新取一次验证码 flag，
// 登录请求携带的顶层 sign 必须等于本次 attempt 刚取到的新鲜 flag，绝不复用上一次的 flag。
func TestLoginFetchesFreshFlagPerAttempt(t *testing.T) {
	var mu sync.Mutex
	verifiCalls := 0
	loginCalls := 0
	var lastIssuedFlag string
	var seenSigns []string

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		switch request.URL.Path {
		case "/web/user/api/login/user/getVerifiCode":
			verifiCalls++
			lastIssuedFlag = "flag-" + string(rune('0'+verifiCalls))
			_, _ = response.Write([]byte(`{"isSuccess":true,"data":{"flag":"` + lastIssuedFlag + `"}}`))
		case "/web/user/api/login/user/login":
			loginCalls++
			body, _ := io.ReadAll(request.Body)
			var shape loginBodyShape
			_ = json.Unmarshal(body, &shape)
			seenSigns = append(seenSigns, shape.Sign)
			// 目标只接受本次新鲜 flag：sign 必须等于最近一次发放的 flag。
			if shape.Sign != lastIssuedFlag {
				_, _ = response.Write([]byte(`{"isSuccess":false,"code":"LOGIN_403","message":"用户名错误或密码错误"}`))
				return
			}
			_, _ = response.Write([]byte(`{"isSuccess":true,"sid":"sid-ok","data":{"user":{"id":"u1","name":"骆蒙恩"}}}`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	client := newLoginTestClient(t, server.URL, "1234")
	for i := 0; i < 2; i++ {
		if _, err := client.Login(context.Background(), "account-a"); err != nil {
			t.Fatalf("第 %d 次登录应成功：%v", i+1, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if verifiCalls != 2 || loginCalls != 2 {
		t.Fatalf("每次登录应各取一次 flag：getVerifiCode=%d login=%d", verifiCalls, loginCalls)
	}
	if len(seenSigns) != 2 || seenSigns[0] == seenSigns[1] {
		t.Fatalf("两次登录应携带各自新鲜且不同的 flag，实际：%v", seenSigns)
	}
	if seenSigns[0] != "flag-1" || seenSigns[1] != "flag-2" {
		t.Fatalf("登录 sign 未跟随本次新鲜 flag，实际：%v", seenSigns)
	}
}

// TestLoginVerifiCodeHTTPFailureStopsBeforeCredentials 锁定：验证码接口 4xx/5xx 失败时，
// 登录必须在凭证校验前终止且不发登录请求，错误按传输分类透传，不伪装成登录拒绝。
func TestLoginVerifiCodeHTTPFailureStopsBeforeCredentials(t *testing.T) {
	loginCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/web/user/api/login/user/getVerifiCode" {
			response.WriteHeader(http.StatusBadGateway)
			_, _ = response.Write([]byte(`{"isSuccess":false,"message":"gateway"}`))
			return
		}
		loginCalls++
		_, _ = response.Write([]byte(`{"isSuccess":true,"sid":"sid"}`))
	}))
	defer server.Close()

	client := newLoginTestClient(t, server.URL, "1234")
	_, err := client.Login(context.Background(), "account-a")
	if err == nil {
		t.Fatal("验证码接口失败时登录必须返回错误")
	}
	if target.IsKind(err, target.ErrorLoginRejected) {
		t.Fatalf("验证码接口失败不得伪装成登录拒绝：%T %v", err, err)
	}
	if !target.IsKind(err, target.ErrorUnavailable) {
		t.Fatalf("验证码接口 5xx 应分类为不可用：%T %v", err, err)
	}
	if loginCalls != 0 {
		t.Fatalf("验证码接口失败后不得发送登录请求，实际登录次数=%d", loginCalls)
	}
}

// TestLoginMissingFlagNeverSendsEmptySign 锁定：验证码接口成功但缺 flag（业务失败或空值）时，
// 登录不得携带空 sign 继续撞登录端点，必须在取 flag 阶段终止为响应无效。
func TestLoginMissingFlagNeverSendsEmptySign(t *testing.T) {
	loginCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/web/user/api/login/user/getVerifiCode" {
			_, _ = response.Write([]byte(`{"isSuccess":false,"code":"CAPTCHA_500","message":"验证码服务异常","data":{"flag":""}}`))
			return
		}
		loginCalls++
		_, _ = response.Write([]byte(`{"isSuccess":true,"sid":"sid"}`))
	}))
	defer server.Close()

	client := newLoginTestClient(t, server.URL, "1234")
	_, err := client.Login(context.Background(), "account-a")
	if !target.IsKind(err, target.ErrorResponseInvalid) {
		t.Fatalf("缺 flag 应返回响应无效：%T %v", err, err)
	}
	if loginCalls != 0 {
		t.Fatalf("缺 flag 时不得发送登录请求，实际登录次数=%d", loginCalls)
	}
}

// TestLoginRejectionPreservesTargetVerifyError 锁定协议拒绝与密码错误不混淆：目标以验证码文字
// 错误（某些 profile 会校验 data.code）拒绝登录时，原始 code/message 必须原样保留，分类为登录拒绝。
func TestLoginRejectionPreservesTargetVerifyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/web/user/api/login/user/getVerifiCode" {
			_, _ = response.Write([]byte(`{"isSuccess":true,"data":{"flag":"fresh-flag"}}`))
			return
		}
		body, _ := io.ReadAll(request.Body)
		var shape loginBodyShape
		_ = json.Unmarshal(body, &shape)
		// 模拟校验 data.code 文字的 profile：code 不等于预期即以验证码错误拒绝。
		if shape.Data.Code != "9999" {
			_, _ = response.Write([]byte(`{"isSuccess":false,"code":"VERIFY_CODE_ERROR","message":"验证码错误"}`))
			return
		}
		_, _ = response.Write([]byte(`{"isSuccess":true,"sid":"sid"}`))
	}))
	defer server.Close()

	client := newLoginTestClient(t, server.URL, "0000")
	_, err := client.Login(context.Background(), "account-a")
	if !target.IsKind(err, target.ErrorLoginRejected) {
		t.Fatalf("验证码文字校验拒绝应分类为登录拒绝：%T %v", err, err)
	}
	var rejection *target.BusinessRejection
	if !errors.As(err, &rejection) || rejection.Code != "VERIFY_CODE_ERROR" || rejection.Message != "验证码错误" {
		t.Fatalf("协议拒绝必须原样保留目标 code/message，不得改写成密码错误：%T %+v", err, rejection)
	}
}

// TestLoginSessionExpiredStaysSessionError 锁定 RESP401/AUTH_401/SID 失效在登录响应中出现时，
// 仍按「会话失效」分类，不被混淆成登录/密码拒绝；目标原始 message 通过 Cause 保留。
func TestLoginSessionExpiredStaysSessionError(t *testing.T) {
	for _, code := range []string{"RESP401", "AUTH_401"} {
		code := code
		t.Run(code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.Header().Set("Content-Type", "application/json")
				if request.URL.Path == "/web/user/api/login/user/getVerifiCode" {
					_, _ = response.Write([]byte(`{"isSuccess":true,"data":{"flag":"fresh-flag"}}`))
					return
				}
				_, _ = response.Write([]byte(`{"isSuccess":false,"code":"` + code + `","message":"会话已失效"}`))
			}))
			defer server.Close()

			client := newLoginTestClient(t, server.URL, "1234")
			_, err := client.Login(context.Background(), "account-a")
			if !target.IsKind(err, target.ErrorSessionExpired) {
				t.Fatalf("登录响应会话失效应分类为会话错误，不得混淆成登录拒绝：%T %v", err, err)
			}
			if target.IsKind(err, target.ErrorLoginRejected) {
				t.Fatalf("会话失效不得误报为登录/密码拒绝：%T %v", err, err)
			}
			if msg := target.UserFacingErrorMessage(target.WriteResponse{}, err); msg != "会话已失效" {
				t.Fatalf("会话失效应保留目标原始 message，实际：%q", msg)
			}
		})
	}
}

// TestLoginMissingSIDIsInvalidResponse 锁定登录成功包缺 sid 时判为响应无效，而非放行空会话。
func TestLoginMissingSIDIsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/web/user/api/login/user/getVerifiCode" {
			_, _ = response.Write([]byte(`{"isSuccess":true,"data":{"flag":"fresh-flag"}}`))
			return
		}
		_, _ = response.Write([]byte(`{"isSuccess":true,"data":{"user":{"id":"u1"}}}`))
	}))
	defer server.Close()

	client := newLoginTestClient(t, server.URL, "1234")
	_, err := client.Login(context.Background(), "account-a")
	if !target.IsKind(err, target.ErrorResponseInvalid) {
		t.Fatalf("登录成功但缺 sid 应判响应无效：%T %v", err, err)
	}
}
