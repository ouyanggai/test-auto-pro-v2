package contracts_test

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"test-auto-pro-v2/internal/api"
)

func TestHealthContract(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	recorder := httptest.NewRecorder()

	api.NewHandler().ServeHTTP(recorder, request)
	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 %d", response.StatusCode, http.StatusOK)
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", contentType)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("读取响应失败：%v", err)
	}
	want := `{"status":"ok","service":"test-auto-pro","version":"dev"}`
	if string(body) != want {
		t.Fatalf("响应 = %q，期望 %q", body, want)
	}
}

// TestHealthContractSupportsGzip 验证客户端显式声明 gzip 时返回可解压的稳定健康响应。
func TestHealthContractSupportsGzip(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := httptest.NewRecorder()
	api.NewHandler().ServeHTTP(recorder, request)
	if recorder.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("未启用 gzip：%q", recorder.Header().Get("Content-Encoding"))
	}
	reader, err := gzip.NewReader(recorder.Result().Body)
	if err != nil {
		t.Fatalf("gzip 响应无法读取：%v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil || string(body) != `{"status":"ok","service":"test-auto-pro","version":"dev"}` {
		t.Fatalf("gzip 响应不正确：body=%q err=%v", body, err)
	}
}
