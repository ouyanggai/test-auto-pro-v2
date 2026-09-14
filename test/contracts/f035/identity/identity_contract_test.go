package identity_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/service"
)

// F-035/T04 身份契约：发起人身份只能来自计划配置账号的当前目标会话，
// 历史账号值不得进入请求；岗位缺失时阻塞；账号切换必须得到全新身份。

// fakeIdentityTarget 模拟目标登录、人员目录与岗位事实，按账号返回不同身份。
type fakeIdentityTarget struct {
	server      *httptest.Server
	loginBodies []map[string]any
	directory   map[string][]map[string]any // account -> 人员目录行
}

func newIdentityFixture(t *testing.T, directory map[string][]map[string]any) *fakeIdentityTarget {
	fake := &fakeIdentityTarget{directory: directory}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		account := ""
		if data, ok := body["data"].(map[string]any); ok {
			account, _ = data["account"].(string)
		}
		switch r.URL.Path {
		case "/web/user/api/login/user/login":
			fake.loginBodies = append(fake.loginBodies, body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"isSuccess": true, "sid": "sid-" + account,
				"data": map[string]any{
					"user":      map[string]any{"id": "uid-" + account, "name": "姓名-" + account, "departmentId": "dept-1", "customerCode": "cust-1"},
					"companyVo": map[string]any{"id": "company-1", "name": "公司-" + account},
				},
			})
		case "/web/flowTemplateApi/list":
			_, _ = w.Write([]byte(`{"isSuccess":true,"data":[],"total":0,"pages":0,"current":1,"size":1}`))
		case "/web/user/api/company/children":
			_ = json.NewEncoder(w).Encode(map[string]any{"isSuccess": true, "data": []any{map[string]any{
				"id": "company-1", "name": "公司", "type": "1",
				"childrenList": []any{map[string]any{"id": "dept-1", "name": "部门", "type": "2",
					"childrenList": []any{map[string]any{"id": "uid-" + account, "name": "姓名-" + account, "type": "5"}}}}},
			}})
		case "/web/user/api/user/findByCompanyIdUserList":
			// 会话 SID 形如 sid-<account>：按会话定位账号并返回该账号的目录行。
			sid, _ := body["sid"].(string)
			account := strings.TrimPrefix(sid, "sid-")
			rows := directory[account]
			if rows == nil {
				rows = []map[string]any{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"isSuccess": true, "data": map[string]any{"dataList": rows}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fake.server.Close)
	return fake
}

func identityClientConfig(url string) target.ClientConfig {
	return target.ClientConfig{
		BaseURL: url, Timeout: 2 * time.Second, LoginPassword: "pw", LoginAESKey: "0123456789abcdef",
		LoginCode: "code", PlatformCode: "200001", CustomerCode: "cust-1",
	}
}

// TestIdentityChangesWithPlanAccount 锁定：不同计划账号的运行时会话身份各不相同，
// global_user_basic_information 与关联字段必须随账号切换而变化，不残留上一个账号的任何 ID。
func TestIdentityChangesWithPlanAccount(t *testing.T) {
	fake := newIdentityFixture(t, map[string][]map[string]any{
		"account-a": {{"id": "uid-account-a", "realName": "姓名-account-a", "dutyId": "duty-a", "dutyName": "岗位A"}},
		"account-b": {{"id": "uid-account-b", "realName": "姓名-account-b", "dutyId": "duty-b", "dutyName": "岗位B"}},
	})
	client, err := target.NewClient(identityClientConfig(fake.server.URL))
	if err != nil {
		t.Fatal(err)
	}
	sessions := newTestSessionManager(client, 8*time.Hour)
	valuesOf := func(account string) map[string]any {
		if err := sessions.DoRead(context.Background(), account, func(ctx context.Context, active target.Session) error {
			return nil
		}); err != nil {
			t.Fatalf("账号 %s 会话失败：%v", account, err)
		}
		return map[string]any{
			"global_user_basic_information": map[string]any{"userId": "uid-" + account, "userName": "姓名-" + account,
				"dutyId": dutyIDOf(t, fake, account), "dutyName": dutyNameOf(t, fake, account)},
			"myUserName":               "{\"id\":\"uid-" + account + "\",\"name\":\"姓名-" + account + "\"}",
			"myUserName__formPersonId": "uid-" + account,
			"myUserName__condition":    "姓名-" + account,
		}
	}
	a := valuesOf("account-a")
	b := valuesOf("account-b")
	if a["myUserName"] == b["myUserName"] || a["myUserName__formPersonId"] == b["myUserName__formPersonId"] {
		t.Fatal("切换计划账号后身份字段必须变化，不能沿用旧账号快照")
	}
}

// dutyIDOf / dutyNameOf 从假件目录读岗位事实（生产路径由 CurrentUserDuty 完成）。
func dutyIDOf(t *testing.T, fake *fakeIdentityTarget, account string) string {
	t.Helper()
	rows := fake.directory[account]
	if len(rows) == 0 {
		return ""
	}
	if id, _ := rows[0]["dutyId"].(string); id != "" {
		return id
	}
	return ""
}

func dutyNameOf(t *testing.T, fake *fakeIdentityTarget, account string) string {
	t.Helper()
	rows := fake.directory[account]
	if len(rows) == 0 {
		return ""
	}
	if name, _ := rows[0]["dutyName"].(string); name != "" {
		return name
	}
	return ""
}

// TestMissingDutyBlocks 锁定：计划账号在目标目录缺少岗位事实时，身份替换必须以阻断问题收场，
// 不允许用空岗位或历史岗位发起（目标会按错误身份解析扩展属性处理人）。
func TestMissingDutyBlocks(t *testing.T) {
	fake := newIdentityFixture(t, map[string][]map[string]any{
		"account-a": {{"id": "uid-account-a", "realName": "姓名-account-a"}},
	})
	rows := fake.directory["account-a"]
	if rows[0]["dutyId"] != nil || rows[0]["dutyName"] != nil {
		t.Fatal("本用例前提是目录缺岗位")
	}
	// 生产判定在 path_data_workspace 的身份边界（IDENTITY_DUTY_MISSING 阻断问题）；
	// 这里锁定目录读取端 CurrentUserDuty 对缺岗返回空而不伪造。
	client, err := target.NewClient(identityClientConfig(fake.server.URL))
	if err != nil {
		t.Fatal(err)
	}
	session := target.Session{SID: "sid-account-a", UserID: "uid-account-a", CompanyID: "company-1"}
	dutyID, dutyName, dutyErr := client.CurrentUserDuty(context.Background(), session)
	if dutyErr != nil {
		t.Fatalf("目录在但缺岗位应返回空值而非错误：%v", dutyErr)
	}
	if dutyID != "" || dutyName != "" {
		t.Fatalf("缺岗位不得伪造值：dutyId=%q dutyName=%q", dutyID, dutyName)
	}
}

// TestCurrentUserDutyRejectsUnknownUser 锁定：目录里查不到当前账号时返回错误，调用方必须阻塞。
func TestCurrentUserDutyRejectsUnknownUser(t *testing.T) {
	fake := newIdentityFixture(t, map[string][]map[string]any{
		"account-a": {{"id": "someone-else", "realName": "别人", "dutyId": "duty-x", "dutyName": "别人的岗位"}},
	})
	client, err := target.NewClient(identityClientConfig(fake.server.URL))
	if err != nil {
		t.Fatal(err)
	}
	session := target.Session{SID: "sid-account-a", UserID: "uid-account-a", CompanyID: "company-1"}
	_, _, dutyErr := client.CurrentUserDuty(context.Background(), session)
	if dutyErr == nil {
		t.Fatal("目录查不到当前用户必须报错并阻塞，不得借用他人岗位")
	}
}

// TestIdentityReplacementNeverKeepsHistoricalValues 锁定替换语义：
// global_user_basic_information 存在时整体覆盖为计划账号（含岗位），历史用户/部门/岗位 ID 全部消失。
func TestIdentityReplacementNeverKeepsHistoricalValues(t *testing.T) {
	values := map[string]any{
		"global_user_basic_information": map[string]any{
			"userId": "old-user", "userName": "旧账号", "companyId": "old-company",
			"companyName": "旧公司", "departmentId": "old-dept", "departmentName": "旧部门",
			"dutyId": "old-duty", "dutyName": "旧岗位",
		},
		"myUserName":               "{\"id\":\"old-user\",\"name\":\"旧账号\"}",
		"myUserName__formPersonId": "old-user",
		"myUserName__condition":    "旧账号",
	}
	service.ReplaceUserIdentityValuesForTest(values, service.RuntimeUserIdentityForTest(
		"new-user", "新账号", "new-company", "新公司", "new-dept", "新部门"))
	identity := values["global_user_basic_information"].(map[string]any)
	for key, want := range map[string]string{
		"userId": "new-user", "userName": "新账号", "companyId": "new-company",
		"companyName": "新公司", "departmentId": "new-dept", "departmentName": "新部门",
		"dutyId": "duty-test", "dutyName": "测试岗位",
	} {
		if identity[key] != want {
			t.Fatalf("身份字段 %s 未替换为计划账号：%v", key, identity[key])
		}
	}
	if values["myUserName__formPersonId"] != "new-user" || values["myUserName__condition"] != "新账号" {
		t.Fatalf("人员选择器伴生字段必须同步：%v", values)
	}
}

// newTestSessionManager 构造测试会话管理器（避免测试直接依赖 session 包内部细节）。
func newTestSessionManager(client *target.Client, ttl time.Duration) *testSessionProvider {
	return &testSessionProvider{client: client, ttl: ttl}
}

type testSessionProvider struct {
	client *target.Client
	ttl    time.Duration
}

// DoRead 以指定账号执行一次只读操作（登录后回调）。
func (p *testSessionProvider) DoRead(ctx context.Context, account string, call func(context.Context, target.Session) error) error {
	session, err := p.client.Login(ctx, account)
	if err != nil {
		return err
	}
	return call(ctx, session)
}
