package contracts_test

import (
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
