package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/engine/verdict"
)

// f014AccountEnv 是只读集成测试使用的目标账号环境变量。
// 账号值按仓库约定只能来自环境变量或被 Git 忽略的本机配置，不写进仓库。
const f014AccountEnv = "TEST_AUTO_PRO_TARGET_ACCOUNT"

// f014Probe 是一次只读探针的真实观测结果，会写进只读 fixture 供文档与判定对照。
type f014Probe struct {
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
	Note       string `json:"note"`
}

// f014Envelope 只解析判定需要的三项，不关心目标返回的其余字段。
type f014Envelope struct {
	IsSuccess bool   `json:"isSuccess"`
	Success   bool   `json:"success"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// requireF014Target 读取真实目标配置与测试账号；缺配置直接失败，不静默跳过。
func requireF014Target(t *testing.T) (target.ClientConfig, string) {
	t.Helper()
	cfg := config.LoadTargetConfig()
	if missing := cfg.MissingRequired(); len(missing) > 0 {
		t.Fatalf("F-014 只读集成测试要求真实目标配置，缺失：%v", missing)
	}
	account := strings.TrimSpace(os.Getenv(f014AccountEnv))
	if account == "" {
		t.Fatalf("F-014 只读集成测试要求通过 %s 提供目标账号（值只放本机 .env.local，不进仓库）", f014AccountEnv)
	}
	return target.ClientConfig{
		BaseURL: cfg.APIGateway, LoginPassword: cfg.LoginPassword, LoginAESKey: cfg.LoginAESKey,
		LoginCode: cfg.LoginCode, PlatformCode: cfg.PlatformCode,
		TemplatePlatformCodes: cfg.TemplatePlatformCodes, CustomerCode: cfg.CustomerCode,
		Timeout: 30 * time.Second,
	}, account
}

// f014Post 按目标协议发一次只读 POST：路径拼在网关后，platformCode 与 sid 同时进查询串与请求体。
// 这里用裸 HTTP 而不是 Client 的封装方法，目的是拿到目标响应的原始形状供判定与 fixture 使用。
func f014Post(baseURL, path, sid string, body map[string]any, timeout time.Duration) (int, string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return 0, "", err
	}
	payload := map[string]any{}
	for key, value := range body {
		payload[key] = value
	}
	if sid != "" {
		payload["sid"] = sid
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(path, "/")
	query := parsed.Query()
	query.Set("platformCode", "200001")
	if sid != "" {
		query.Set("sid", sid)
	}
	parsed.RawQuery = query.Encode()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), bytes.NewReader(encoded))
	if err != nil {
		return 0, "", err
	}
	request.Header.Set("Content-Type", "application/json")
	if sid != "" {
		request.Header.Set("sid", sid)
	}
	response, err := (&http.Client{Timeout: timeout}).Do(request)
	if err != nil {
		return 0, "", err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return response.StatusCode, "", err
	}
	return response.StatusCode, string(content), nil
}

// writeF014Probe 把探针结果写进只读 fixture 目录，便于人工核对目标真实形状。
func writeF014Probe(t *testing.T, probe f014Probe) {
	t.Helper()
	dir := filepath.Join("..", "fixtures", "f014", "readonly")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("创建只读样本目录失败：%v", err)
	}
	encoded, err := json.MarshalIndent(probe, "", "  ")
	if err != nil {
		t.Fatalf("序列化只读样本失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, probe.Name+".json"), append(encoded, '\n'), 0o644); err != nil {
		t.Fatalf("写入只读样本失败：%v", err)
	}
}

// f014Initial 把一次真实响应交给判定包，返回响应侧初判。
func f014Initial(endpoint string, statusCode int, body string) (verdict.Initial, f014Envelope) {
	var envelope f014Envelope
	unparsable := json.Unmarshal([]byte(body), &envelope) != nil
	result := verdict.Evaluate(verdict.Observation{
		Action: "readonly_probe", Endpoint: endpoint, Transport: verdict.TransportResponded,
		StatusCode: statusCode, Reread: verdict.RereadUnreadable,
		Response: &verdict.Response{
			IsSuccess: envelope.IsSuccess || envelope.Success, Code: envelope.Code,
			Message: envelope.Message, Unparsable: unparsable,
		},
	})
	return result.Initial, envelope
}
