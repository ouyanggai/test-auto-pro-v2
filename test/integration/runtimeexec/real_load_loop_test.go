// F-036 L1 装载台：真实装载发起/详情原生页面（只读、无写），断言捕获链与模板精确选择。
// 环境门禁：TEST_AUTO_PRO_TARGET_ACCOUNT + RUNTIME_EXECUTOR_URL 未配置时跳过并打印原因。
// 证据落盘：docs/auto/evidence/<时间戳>-L1-<场景>.json（施工令 §5.4）。
package runtimeexec_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/config"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/runtimeexec"
	mysql "test-auto-pro-v2/internal/repository/mysql"
	"test-auto-pro-v2/internal/session"
)

// f036BoundTemplateID 是当前验证用计划绑定模板 ID（员工请假单·智慧斯能）：
// 新发起来源下计划目标对象 ID 就是 /web/flowTemplateApi/list 返回的模板 ID；
// 可用 TEST_AUTO_PRO_F036_TEMPLATE_ID 覆盖。
func f036BoundTemplateID(t *testing.T) string {
	t.Helper()
	if id := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_TEMPLATE_ID")); id != "" {
		return id
	}
	return "73eaa9b297b54364a3c320d3002a1b4f"
}

// f036LoadDotEnv 在进程环境缺失键时从 .env.local 补充（go test 不会自动加载本地环境文件）。
// go test 的工作目录是包目录，这里从当前目录向上找到仓库根的 .env.local；
// 只补进程内环境变量，不打印任何值。
func f036LoadDotEnv(t *testing.T) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 6; i++ {
		raw, err := os.ReadFile(dir + "/.env.local")
		if err == nil {
			f036ApplyDotEnv(raw)
			return
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == "" || parent == dir {
			return
		}
		dir = parent
	}
}

// f036ApplyDotEnv 解析 .env.local 文本并补充进程环境。
func f036ApplyDotEnv(raw []byte) {
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		equal := strings.Index(line, "=")
		if equal <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:equal])
		value := strings.Trim(strings.TrimSpace(line[equal+1:]), `"'`)
		if key == "" || value == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}

// realLoopDeps 是真实闭环验证台的公共接线：目标出口 + 会话管理 + runtime 客户端。
// 返回的 target 客户端供用例直接做只读目标读取（模板配置、写后核验）。
func realLoopDeps(t *testing.T) (*runtimeexec.Client, *target.Client, func(context.Context, string) (target.Session, error)) {
	t.Helper()
	f036LoadDotEnv(t)
	account := os.Getenv("TEST_AUTO_PRO_TARGET_ACCOUNT")
	baseURL := os.Getenv("RUNTIME_EXECUTOR_URL")
	if account == "" || baseURL == "" {
		t.Skip("未配置 TEST_AUTO_PRO_TARGET_ACCOUNT / RUNTIME_EXECUTOR_URL，跳过真实闭环验证")
	}
	cfg := config.LoadTargetConfig()
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("目标配置缺失：%v", missing)
	}
	client, err := target.NewClient(target.ClientConfig{
		BaseURL: cfg.APIGateway, LoginPassword: cfg.LoginPassword, LoginAESKey: cfg.LoginAESKey,
		LoginCode: cfg.LoginCode, PlatformCode: cfg.PlatformCode, TemplatePlatformCodes: cfg.TemplatePlatformCodes,
		CustomerCode: cfg.CustomerCode, Timeout: cfg.HTTPTimeout,
	})
	if err != nil {
		t.Fatalf("目标客户端构造失败：%v", err)
	}
	sessions := session.NewManager(client, cfg.SessionTTL)
	rc, err := runtimeexec.NewClient(baseURL, client, sessions) // 第二参数=TargetForwarder，*target.Client 满足
	if err != nil {
		t.Fatalf("runtime 客户端构造失败：%v", err)
	}
	return rc, client, sessions.Current
}

// f036WriteEndpoints 是七个动作的主写端点：装载台断言它们一次都不允许出现。
var f036WriteEndpoints = []string{
	"/web/flowInstanceApi/submit",
	"/flowInstanceApi/audit",
	"/web/flowInstanceApi/storageFormData",
	"/web/flowInstanceApi/reSubmit",
	"/web/flowInstanceApi/transpond",
}

// saveF036Evidence 把一次验证事实原样落盘到仓库根 docs/auto/evidence/。
// go test 的工作目录是包目录，这里向上定位仓库根再写。
func saveF036Evidence(t *testing.T, name string, fact map[string]any) {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Logf("证据落盘失败（不影响断言）：%v", err)
		return
	}
	for i := 0; i < 6; i++ {
		if _, statErr := os.Stat(root + "/.git"); statErr == nil {
			break
		}
		parent := root[:strings.LastIndex(root, "/")]
		if parent == "" || parent == root {
			break
		}
		root = parent
	}
	dir := root + "/docs/auto/evidence"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("证据目录创建失败（不影响断言）：%v", err)
		return
	}
	file := fmt.Sprintf("%s/%s-%s.json", dir, time.Now().Format("20060102-150405"), name)
	raw, err := json.MarshalIndent(fact, "", "  ")
	if err != nil {
		t.Logf("证据序列化失败：%v", err)
		return
	}
	if err := os.WriteFile(file, raw, 0o644); err != nil {
		t.Logf("证据写入失败（不影响断言）：%v", err)
		return
	}
	t.Logf("证据已落盘：%s", file)
}

// f036LoadOnce 创建会话并执行一次装载，返回原始响应（含 diagnostics）与会话 ID。
func f036LoadOnce(ctx context.Context, t *testing.T, rc *runtimeexec.Client, current func(context.Context, string) (target.Session, error), account string, payload *runtimeexec.LoadPayload) (json.RawMessage, string) {
	t.Helper()
	handle, err := rc.CreateSession(ctx, runtimeexec.SessionSpec{AccountKey: account, RunID: "1", PathRunID: "1", StepID: "1"})
	if err != nil {
		t.Fatalf("创建会话失败：%v", err)
	}
	t.Cleanup(func() { _ = rc.Destroy(context.Background(), handle.SessionID) })
	closeWS, prewarmErr := rc.Prewarm(ctx, handle.SessionID, account)
	if prewarmErr != nil {
		t.Fatalf("预热转发通道失败：%v", prewarmErr) // 装载期读取必须经通道
	}
	defer closeWS()

	envelope, err := runtimeexec.EncodeLoadMessage(handle.SessionID, fmt.Sprintf("load-%d", time.Now().UnixNano()), "1", "1", "1", payload)
	if err != nil {
		t.Fatalf("装载信封编码失败：%v", err)
	}
	var message runtimeexec.Message
	if err := json.Unmarshal(envelope, &message); err != nil {
		t.Fatalf("信封解析失败：%v", err)
	}
	raw, err := rc.Load(ctx, handle.SessionID, message)
	if err != nil {
		// 失败时把会话 trace 原样打出来：转发失败/中止原因必须能被看到，否则无从定位。
		entries := f036SessionTrace(t, handle.SessionID)
		for _, entry := range entries {
			t.Logf("trace[%d] %s %s state=%s http=%d", entry.Seq, entry.Method, entry.URL, entry.State, entry.HTTPStatus)
		}
		t.Fatalf("装载失败：%v", err)
	}
	return raw, handle.SessionID
}

// f036SessionTrace 调执行服务 /status 读取会话捕获的目标请求清单（只读观测口）。
func f036SessionTrace(t *testing.T, sessionID string) []struct {
	Seq        int    `json:"seq"`
	Method     string `json:"method"`
	URL        string `json:"url"`
	State      string `json:"state"`
	AbortReason string `json:"abortReason"`
	HTTPStatus int    `json:"httpStatus"`
} {
	t.Helper()
	base := strings.TrimRight(os.Getenv("RUNTIME_EXECUTOR_URL"), "/")
	response, err := http.Get(base + "/runtime/session/" + sessionID + "/status")
	if err != nil {
		t.Fatalf("读取会话状态失败：%v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("读取会话状态正文失败：%v", err)
	}
	var parsed struct {
		Trace struct {
			Entries []struct {
				Seq        int    `json:"seq"`
				Method     string `json:"method"`
				URL        string `json:"url"`
				State      string `json:"state"`
				AbortReason string `json:"abortReason"`
				HTTPStatus int    `json:"httpStatus"`
			} `json:"entries"`
		} `json:"trace"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("会话状态解析失败：%v（原文：%s）", err, string(body))
	}
	return parsed.Trace.Entries
}

// TestF036RealLoadLoopInitiate 发起装载：捕获组件链非空、模板精确命中计划绑定 ID、只读无写。
func TestF036RealLoadLoopInitiate(t *testing.T) {
	rc, client, current := realLoopDeps(t)
	account := os.Getenv("TEST_AUTO_PRO_TARGET_ACCOUNT")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := rc.Health(ctx); err != nil {
		t.Fatalf("执行服务不可用：%v", err)
	}

	boundID := f036BoundTemplateID(t)
	s, err := current(ctx, account)
	if err != nil {
		t.Fatalf("取目标会话失败：%v", err)
	}
	// 从目标实时读模板配置：auditWay 与表单代理 ID 都是目标事实，不在用例里硬编码业务值。
	snapshot, err := client.ReadTemplateConfiguration(ctx, s, boundID)
	if err != nil {
		t.Fatalf("读取目标模板配置失败：%v", err)
	}
	if strings.TrimSpace(snapshot.AuditWay) == "" {
		t.Fatalf("目标模板 %s 未返回 auditWay，发起装载无法进行", boundID)
	}

	load, err := runtimeexec.BuildLoadPayload(runtimeexec.LoadInput{
		Mode: runtimeexec.LoadModeInitiate,
		Session: runtimeexec.SessionFacts{
			UserID: s.UserID, CompanyID: s.CompanyID, DepartmentID: s.DepartmentID,
			CustomerCode: s.CustomerCode, Account: s.Summary.Account, DisplayName: s.Summary.DisplayName,
		},
		ToolFlowName:   "T-AUTO-F036-L1",
		SelectFlowType: snapshot.AuditWay,
		FlowTemplateID: boundID,
		BaseURL:        "",
	}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("装载载荷构造失败：%v", err)
	}
	raw, sessionID := f036LoadOnce(ctx, t, rc, current, account, load)
	t.Logf("装载响应：%s", string(raw))
	saveF036Evidence(t, "L1-initiate", map[string]any{
		"scene": "initiate", "account": account, "boundTemplateId": boundID,
		"auditWay": snapshot.AuditWay, "response": json.RawMessage(raw),
	})

	var diag struct {
		Ready       bool   `json:"ready"`
		Mode        string `json:"mode"`
		Diagnostics struct {
			Mode               string   `json:"mode"`
			HostPath           []string `json:"hostPath"`
			TemplateID         string   `json:"templateId"`
			TemplateCandidates []string `json:"templateCandidates"`
			Warnings           []string `json:"warnings"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(raw, &diag); err != nil {
		t.Fatalf("装载响应解析失败：%v（原文：%s）", err, string(raw))
	}
	if !diag.Ready {
		t.Fatalf("装载未就绪：%s", string(raw))
	}
	if diag.Diagnostics.Mode != "initiate" {
		t.Fatalf("诊断 mode 应为 initiate，实际 %s", diag.Diagnostics.Mode)
	}
	if len(diag.Diagnostics.HostPath) == 0 {
		t.Fatalf("装载诊断 hostPath 为空：原生组件链未捕获（P1-1）")
	}
	joined := strings.Join(diag.Diagnostics.HostPath, ">")
	if !strings.Contains(joined, "InitFlowDialog") || !strings.Contains(joined, "FlowDialog") {
		t.Fatalf("发起装载组件链应含 InitFlowDialog>FlowDialog，实际 %v", diag.Diagnostics.HostPath)
	}
	if diag.Diagnostics.TemplateID != boundID {
		t.Fatalf("实际装载模板 %s != 计划绑定模板 %s（P1-2）", diag.Diagnostics.TemplateID, boundID)
	}
	if len(diag.Diagnostics.TemplateCandidates) == 0 {
		t.Fatalf("模板候选集合为空：选择判据必须来自 /web/flowTemplateApi/list 真实响应")
	}

	// trace 断言（断言 4）：发起必须出现模板列表/详情读取，且绝不出现主写端点。
	entries := f036SessionTrace(t, sessionID)
	hasList, hasFind := false, false
	for _, entry := range entries {
		if strings.Contains(entry.URL, "/web/flowTemplateApi/list") {
			hasList = true
		}
		if strings.Contains(entry.URL, "/web/flowTemplateApi/findById") {
			hasFind = true
		}
		for _, write := range f036WriteEndpoints {
			if strings.Contains(entry.URL, write) {
				t.Fatalf("装载期出现写端点 %s：装载必须是纯读", entry.URL)
			}
		}
	}
	if !hasList || !hasFind {
		t.Fatalf("发起 trace 缺模板读取（list=%v findById=%v）：%d 笔", hasList, hasFind, len(entries))
	}
}

// TestF036RealLoadLoopDetail 详情装载（view 模式，只读）：实例取自目标业务库最新一条
// 同模板实例；断言捕获链非空、语义字段与 mode 对应、trace 无写端点。
func TestF036RealLoadLoopDetail(t *testing.T) {
	rc, client, current := realLoopDeps(t)
	account := os.Getenv("TEST_AUTO_PRO_TARGET_ACCOUNT")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := rc.Health(ctx); err != nil {
		t.Fatalf("执行服务不可用：%v", err)
	}

	boundID := f036BoundTemplateID(t)
	s, err := current(ctx, account)
	if err != nil {
		t.Fatalf("取目标会话失败：%v", err)
	}
	snapshot, err := client.ReadTemplateConfiguration(ctx, s, boundID)
	if err != nil {
		t.Fatalf("读取目标模板配置失败：%v", err)
	}
	instanceID := strings.TrimSpace(os.Getenv("TEST_AUTO_PRO_F036_INSTANCE"))
	if instanceID == "" {
		// 从目标业务库只读连接找同模板最新实例（施工令 §0 允许项 5）。
		repo, repoErr := mysql.NewTargetHistoryRepository(ctx, config.LoadTargetBizDBConfig(), s.CustomerCode)
		if repoErr != nil {
			t.Fatalf("目标业务库只读连接不可用，且未设置 TEST_AUTO_PRO_F036_INSTANCE：%v", repoErr)
		}
		defer repo.Close()
		rows, _, listErr := repo.TargetHistoryCandidates(ctx, repository.TargetHistoryCandidateFilter{
			FlowName: snapshot.FlowName,
		}, 1, 5)
		if listErr != nil || len(rows) == 0 {
			t.Fatalf("目标业务库未找到模板 %s 的实例：%v", snapshot.FlowName, listErr)
		}
		instanceID = rows[0].InstanceID
	}

	load, err := runtimeexec.BuildLoadPayload(runtimeexec.LoadInput{
		Mode: runtimeexec.LoadModeView,
		Session: runtimeexec.SessionFacts{
			UserID: s.UserID, CompanyID: s.CompanyID, DepartmentID: s.DepartmentID,
			CustomerCode: s.CustomerCode, Account: s.Summary.Account, DisplayName: s.Summary.DisplayName,
		},
		ToolFlowName:   "T-AUTO-F036-L1",
		InstanceID:     instanceID,
		FormProxyID:    snapshot.Forms[0].ID,
		FlowProxyID:    boundID,
		SelectFlowType: snapshot.AuditWay,
		BaseURL:        "",
	}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("装载载荷构造失败：%v", err)
	}
	raw, sessionID := f036LoadOnce(ctx, t, rc, current, account, load)
	t.Logf("装载响应：%s", string(raw))
	saveF036Evidence(t, "L1-detail-view", map[string]any{
		"scene": "detail-view", "account": account, "instanceId": instanceID,
		"response": json.RawMessage(raw),
	})

	var diag struct {
		Ready       bool `json:"ready"`
		Diagnostics struct {
			Mode       string   `json:"mode"`
			HostPath   []string `json:"hostPath"`
			Warnings   []string `json:"warnings"`
			PageErrors []any    `json:"pageErrors"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(raw, &diag); err != nil {
		t.Fatalf("装载响应解析失败：%v（原文：%s）", err, string(raw))
	}
	if !diag.Ready {
		t.Fatalf("装载未就绪：%s", string(raw))
	}
	if len(diag.Diagnostics.HostPath) == 0 {
		t.Fatalf("详情装载 hostPath 为空：原生组件链未捕获")
	}
	joined := strings.Join(diag.Diagnostics.HostPath, ">")
	if !strings.Contains(joined, "EnterpriseExamineDialog") {
		t.Fatalf("详情装载组件链应含 EnterpriseExamineDialog，实际 %v", diag.Diagnostics.HostPath)
	}
	// 只读装载绝不允许写端点。
	for _, entry := range f036SessionTrace(t, sessionID) {
		for _, write := range f036WriteEndpoints {
			if strings.Contains(entry.URL, write) {
				t.Fatalf("详情装载期出现写端点 %s：装载必须是纯读", entry.URL)
			}
		}
	}
}
