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
// isSuccess 用指针解析：缺字段与显式 false 含义不同，前者说明成功判据不存在。
// 这里刻意不读别名字段 success：语义清单第 1.2 节的硬约束是只认 isSuccess，
// 现有只读路径对别名的兼容不属于 F-014 范围，也不参与本切片的任何断言。
type f014Envelope struct {
	IsSuccess *bool  `json:"isSuccess"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// claimsSuccess 只按 isSuccess 判断目标是否声明成功。
func (e f014Envelope) claimsSuccess() bool {
	return e.IsSuccess != nil && *e.IsSuccess
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
		// 沿用配置里的目标超时（默认 120 秒）。真实目标登录本身要十几秒，
		// 这里写死一个更短的值会让只读用例在目标慢的时候偶发超时失败。
		Timeout: cfg.HTTPTimeout,
	}, account
}

// requireF014Session 为调用方单独登录一次并返回目标客户端与会话；失败直接让用例失败，不静默跳过。
// 这里刻意不共用会话：测试账号是共享账号，目标平台在同账号别处登录时会让旧会话失效，
// 共用一个长会话反而会让后面的用例偶发「会话已失效」。每个用例各自登录，一次抖动只影响一个用例。
func requireF014Session(t *testing.T) (target.ClientConfig, *target.Client, target.Session) {
	t.Helper()
	clientConfig, account := requireF014Target(t)
	client, err := target.NewClient(clientConfig)
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	// 登录本身要有界重试：目标存在分钟级抖动（纲领第 4.4.1 节实测），
	// 而登录是每个只读用例的第一步，不重试会让整批用例被一次抖动打断。
	// 只读阶段允许重试，与"submit 阶段禁止重试"的边界互不冲突。
	var session target.Session
	var loginErr error
	for attempt := 1; attempt <= 3; attempt++ {
		session, loginErr = client.Login(context.Background(), account)
		if loginErr == nil {
			break
		}
		t.Logf("真实目标登录第 %d 次失败，5 秒后重试：%v", attempt, loginErr)
		time.Sleep(5 * time.Second)
	}
	if loginErr != nil {
		t.Fatalf("真实目标登录连续 3 次失败：%v", loginErr)
	}
	if strings.TrimSpace(session.SID) == "" {
		t.Fatal("真实目标登录没有返回会话标识")
	}
	return clientConfig, client, session
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
			IsSuccess: envelope.claimsSuccess(), IsSuccessPresent: envelope.IsSuccess != nil,
			Code: envelope.Code, Message: envelope.Message, Unparsable: unparsable,
		},
	})
	return result.Initial, envelope
}
