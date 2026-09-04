package history_replay_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/analyzer"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/repository"
	"test-auto-pro-v2/internal/service"
)

// fakeCompanyDirectory 用固定映射模拟目标公司主数据，并记录查询次数以锁定"一致数据零写入"边界。
type fakeCompanyDirectory struct {
	byID        map[string]string
	byName      map[string][]string
	errByName   error
	callsByID   int
	callsByName int
}

// CompanyNameByID 返回公司 ID 对应的名称；缺失时 found=false。
func (f *fakeCompanyDirectory) CompanyNameByID(_ context.Context, companyID string) (string, bool, error) {
	f.callsByID++
	name, found := f.byID[companyID]
	return name, found, nil
}

// CompanyIDByName 返回公司名称对应的全部 ID，可注入读取失败以验证非阻断问题。
func (f *fakeCompanyDirectory) CompanyIDByName(_ context.Context, name string) ([]string, error) {
	f.callsByName++
	if f.errByName != nil {
		return nil, f.errByName
	}
	return f.byName[name], nil
}

// companySyncTemplate 还原请款单真实布局的关键约束：公司远程下拉、同前缀隐藏名称字段、
// 非公司数据源的远程下拉，以及与下拉无关的金额字段。
func companySyncTemplate() string {
	return `{"config":{"dataSource":[
		{"key":"gbftfvts","url":"/api/web/user/api/company/getSingleCompanyVosWithinProjectCompany?platformCode=200001"},
		{"key":"abp9bz1m","url":"/api/web/companyExpenseBaseData/list?platformCode=200001"}
	]},"list":[
		{"type":"select","model":"applicationFundsVo_payCompanyId","options":{"remote":true,"remoteDataSource":"gbftfvts"}},
		{"type":"input","model":"applicationFundsVo_payCompanyName","options":{"hidden":true}},
		{"type":"select","model":"companyNumber","options":{"remote":true,"remoteDataSource":"abp9bz1m"}},
		{"type":"select","model":"dummyCompanyId","options":{"remote":true,"remoteDataSource":"abp9bz1m"}},
		{"type":"number","model":"applicationFundsVo_payMoney"}
	]}`
}

// newCompanySyncService 组装带已保存数据与公司目录的最小数据工作区服务。
func newCompanySyncService(t *testing.T, planID, pathID uint64, stored string, directory *fakeCompanyDirectory) (*service.PathConfigService, *workspaceHistoryStore) {
	t.Helper()
	plan := model.Plan{ID: planID, Account: "account-a", FlowSource: "new", TargetObjectID: "flow-pay", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: pathID, PlanID: planID, SequenceNo: 1, Name: "付款单位路径"}
	store := newWorkspaceHistoryStore()
	store.snapshots[900] = model.HistorySnapshot{
		ID: 900, PlanID: planID, CandidateKey: "candidate-pay", FlowCode: "flow-pay", FormName: "请款单", FlowName: "请款单流程",
		RuntimeType: string(target.FormRenderTypeFormMaking), TemplateSummary: map[string]any{}, RawFormData: map[string]any{"applicationFundsVo_payMoney": 1999},
	}
	store.defaults[planID] = repository.HistoryDefaultRecord{PlanID: planID, SnapshotID: 900, Revision: 1}
	store.configs[pathID] = repository.HistoryPathConfigRecord{
		PathID: pathID, Revision: 5, DataRevision: 5, SourceMode: model.HistorySourceModeDefault,
		// DataStatus 必须是已就绪的保存态，否则读取边界会回到快照初始化分支，覆盖掉已保存正文。
		DataStatus:        model.HistoryDataStatusReady,
		EffectiveFormData: []byte(stored),
	}
	reader := workspaceTargetReader{snapshot: target.PathConfigurationSnapshot{
		Tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{ID: "end", Type: "end"}}, EntryNodeIDs: []string{"start"},
		FlowCode: "flow-pay", FlowName: "请款单流程", RenderType: target.FormRenderTypeFormMaking,
		Forms: []target.FormRuntimeTemplate{{Name: "请款单", TemplateData: companySyncTemplate()}},
	}}
	configService := service.NewPathConfigService(service.NewPlanService(&historyPlanRepository{plan: plan}), reader,
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(),
		&workspacePathRepository{paths: []model.ExecutionPath{path}})
	configService.SetHistoryWorkspaceStores(store, store)
	if directory != nil {
		configService.SetCompanyDirectory(directory)
	}
	return configService, store
}

// companySyncStoredData 是分支补丁只改名称字段后落盘的典型数据：名称已指向新公司，ID 仍是历史公司。
func companySyncStoredData() string {
	return `{
		"applicationFundsVo_payCompanyId":"old-company-id",
		"applicationFundsVo_payCompanyName":"临猗县斯能电力有限公司",
		"applicationFundsVo_payMoney":1999,
		"expenseCompanyId":"consistent-company-id",
		"expenseCompanyName":"广东斯能投资有限责任公司",
		"dummyCompanyId":"dummy-old-id",
		"dummyCompanyName":"不应被同步的公司"
	}`
}

// TestGetDataSyncsLinkedCompanySelectID 锁定读取边界：名称与 ID 指向不同公司时按目标公司表回填真实 ID 与虚拟显示值。
func TestGetDataSyncsLinkedCompanySelectID(t *testing.T) {
	directory := &fakeCompanyDirectory{
		byID:   map[string]string{"old-company-id": "广西润兴电力有限公司", "consistent-company-id": "广东斯能投资有限责任公司", "dummy-old-id": "目录外的公司"},
		byName: map[string][]string{"临猗县斯能电力有限公司": {"new-company-id"}},
	}
	configService, _ := newCompanySyncService(t, 651, 661, companySyncStoredData(), directory)
	configuration, err := configService.GetData(context.Background(), 651, 661)
	if err != nil {
		t.Fatalf("读取数据工作区失败：%v", err)
	}
	values := configuration.EffectiveFormData
	if values["applicationFundsVo_payCompanyId"] != "new-company-id" {
		t.Fatalf("付款单位 ID 没有同步为真实公司：%v", values["applicationFundsVo_payCompanyId"])
	}
	if values["applicationFundsVo_payCompanyId__virtualName"] != "临猗县斯能电力有限公司" {
		t.Fatalf("付款单位虚拟显示值没有同步：%v", values["applicationFundsVo_payCompanyId__virtualName"])
	}
	// 金额走数字保真解码，类型可能是 json.Number，这里只锁定数值不被同步逻辑改变。
	if values["applicationFundsVo_payCompanyName"] != "临猗县斯能电力有限公司" || fmt.Sprintf("%v", values["applicationFundsVo_payMoney"]) != "1999" {
		t.Fatalf("分支补丁字段被同步逻辑破坏：%+v", values)
	}
	// 已一致的配对必须零改动：名称与 ID 指向同一家公司时不能触发任何回写。
	if values["expenseCompanyId"] != "consistent-company-id" {
		t.Fatalf("一致的公司配对被误改：%v", values["expenseCompanyId"])
	}
	// 非公司数据源的远程下拉即使名称与 ID 不一致也绝不能按公司主数据同步。
	if values["dummyCompanyId"] != "dummy-old-id" {
		t.Fatalf("非公司数据源的下拉被误同步：%v", values["dummyCompanyId"])
	}
	for _, issue := range configuration.Issues {
		if issue.Code == "COMPANY_LINK_UNRESOLVED" {
			t.Fatalf("同步成功时不应产生问题：%+v", issue)
		}
	}
	if !strings.Contains(string(mustJSON(t, values)), "new-company-id") {
		t.Fatalf("同步结果没有进入返回数据：%+v", values)
	}
}

// TestGetDataKeepsDataWhenDirectoryMissing 锁定未注入目录时保持历史行为，不产生任何问题。
func TestGetDataKeepsDataWhenDirectoryMissing(t *testing.T) {
	configService, _ := newCompanySyncService(t, 652, 662, companySyncStoredData(), nil)
	configuration, err := configService.GetData(context.Background(), 652, 662)
	if err != nil {
		t.Fatalf("读取数据工作区失败：%v", err)
	}
	if configuration.EffectiveFormData["applicationFundsVo_payCompanyId"] != "old-company-id" {
		t.Fatalf("未注入目录时数据被改动：%v", configuration.EffectiveFormData["applicationFundsVo_payCompanyId"])
	}
	for _, issue := range configuration.Issues {
		if issue.Code == "COMPANY_LINK_UNRESOLVED" {
			t.Fatalf("未注入目录时不应产生同步问题：%+v", issue)
		}
	}
}

// TestGetDataReportsUnresolvedCompanyLink 锁定同名多家公司时保留原值并给出阻断问题：
// 保留即意味着控件继续显示并提交历史公司，这种矛盾状态必须阻断并提示用户处理。
func TestGetDataReportsUnresolvedCompanyLink(t *testing.T) {
	directory := &fakeCompanyDirectory{
		byID:   map[string]string{"old-company-id": "广西润兴电力有限公司"},
		byName: map[string][]string{"临猗县斯能电力有限公司": {"company-a", "company-b"}},
	}
	configService, _ := newCompanySyncService(t, 653, 663, companySyncStoredData(), directory)
	configuration, err := configService.GetData(context.Background(), 653, 663)
	if err != nil {
		t.Fatalf("读取数据工作区失败：%v", err)
	}
	if configuration.EffectiveFormData["applicationFundsVo_payCompanyId"] != "old-company-id" {
		t.Fatalf("无法唯一解析时 ID 被猜测改写：%v", configuration.EffectiveFormData["applicationFundsVo_payCompanyId"])
	}
	found := false
	for _, issue := range configuration.Issues {
		if issue.Code == "COMPANY_LINK_UNRESOLVED" {
			found = true
			if !issue.Blocking {
				t.Fatalf("无法完成补丁映射必须是阻断问题：%+v", issue)
			}
			if issue.Path != "applicationFundsVo_payCompanyId" {
				t.Fatalf("问题没有定位到下拉字段：%+v", issue)
			}
		}
	}
	if !found {
		t.Fatalf("同名多家公司时没有记录阻断问题：%+v", configuration.Issues)
	}
}

// TestGetDataReportsCompanyDirectoryFailure 锁定目录读取失败时保留原值并给出阻断问题。
func TestGetDataReportsCompanyDirectoryFailure(t *testing.T) {
	directory := &fakeCompanyDirectory{
		byID:      map[string]string{"old-company-id": "广西润兴电力有限公司"},
		errByName: errors.New("目标公司目录暂不可用"),
	}
	configService, _ := newCompanySyncService(t, 654, 664, companySyncStoredData(), directory)
	configuration, err := configService.GetData(context.Background(), 654, 664)
	if err != nil {
		t.Fatalf("目录读取失败不应影响工作区读取：%v", err)
	}
	if configuration.EffectiveFormData["applicationFundsVo_payCompanyId"] != "old-company-id" {
		t.Fatalf("目录读取失败时 ID 被改写：%v", configuration.EffectiveFormData["applicationFundsVo_payCompanyId"])
	}
	found := false
	for _, issue := range configuration.Issues {
		if issue.Code == "COMPANY_LINK_UNRESOLVED" && issue.Blocking {
			found = true
		}
	}
	if !found {
		t.Fatalf("目录读取失败时没有记录阻断问题：%+v", configuration.Issues)
	}
}

// TestSaveDataSyncsLinkedCompanySelectID 锁定保存边界：浏览器捕获值或接口直存都必须落成名称与 ID 一致的数据。
func TestSaveDataSyncsLinkedCompanySelectID(t *testing.T) {
	directory := &fakeCompanyDirectory{
		byID:   map[string]string{"old-company-id": "广西润兴电力有限公司"},
		byName: map[string][]string{"临猗县斯能电力有限公司": {"new-company-id"}},
	}
	configService, store := newCompanySyncService(t, 655, 665, companySyncStoredData(), directory)
	result, err := configService.SaveData(context.Background(), 655, 665, "7c1f6a2e-93b1-4c4f-9d5a-5f1e2a3b4c5d",
		model.PathConfigurationDataInput{Revision: 5, Values: map[string]any{
			"applicationFundsVo_payCompanyId":   "old-company-id",
			"applicationFundsVo_payCompanyName": "临猗县斯能电力有限公司",
			"applicationFundsVo_payMoney":       1999,
		}, RuntimeValidation: model.HistoryRuntimeValidation{Accepted: true}})
	if err != nil {
		t.Fatalf("保存数据工作区失败：%v", err)
	}
	if result.EffectiveFormData["applicationFundsVo_payCompanyId"] != "new-company-id" {
		t.Fatalf("保存结果没有同步真实公司 ID：%v", result.EffectiveFormData["applicationFundsVo_payCompanyId"])
	}
	if result.EffectiveFormData["applicationFundsVo_payCompanyId__virtualName"] != "临猗县斯能电力有限公司" {
		t.Fatalf("保存结果没有补齐虚拟显示值：%v", result.EffectiveFormData["applicationFundsVo_payCompanyId__virtualName"])
	}
	var saved map[string]any
	if err := json.Unmarshal(store.configs[665].EffectiveFormData, &saved); err != nil {
		t.Fatalf("解析保存正文失败：%v", err)
	}
	if saved["applicationFundsVo_payCompanyId"] != "new-company-id" {
		t.Fatalf("落盘数据仍携带历史公司 ID：%v", saved["applicationFundsVo_payCompanyId"])
	}
}

// mustJSON 编码断言辅助数据，失败直接终止用例。
func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("JSON 编码失败：%v", err)
	}
	return raw
}

// identityTargetReader 在固定快照读取器上补充目标运行时会话身份，模拟计划账号登录成功。
type identityTargetReader struct {
	workspaceTargetReader
}

// FormRuntimeSession 返回固定的计划账号身份。
func (r identityTargetReader) FormRuntimeSession(context.Context, string) (target.FormRuntimeSession, error) {
	return target.FormRuntimeSession{
		SID: "session-a", BaseURL: "http://target.example", AccountName: "计划账号",
		UserID: "user-plan", CompanyID: "company-plan", CompanyName: "计划公司",
		DepartmentID: "dept-plan", DepartmentName: "计划部门",
	}, nil
}

// TestRepaceUserIdentityValuesOnlyTouchesGlobalContextField 锁定替换只作用于目标登录人上下文字段本身。
func TestReplaceUserIdentityValuesOnlyTouchesGlobalContextField(t *testing.T) {
	values := map[string]any{
		"global_user_basic_information": map[string]any{"userId": "old-user", "userName": "原发起人", "dutyId": "旧岗位"},
		"expenseUserName":               "原发起人",
	}
	replaced := service.RuntimeUserIdentityForTest("user-plan", "计划账号", "company-plan", "计划公司", "dept-plan", "计划部门")
	service.ReplaceUserIdentityValuesForTest(values, replaced)
	identity := values["global_user_basic_information"].(map[string]any)
	if identity["userId"] != "user-plan" || identity["userName"] != "计划账号" || identity["companyId"] != "company-plan" {
		t.Fatalf("登录人上下文没有替换为计划账号：%+v", identity)
	}
	if identity["departmentId"] != "dept-plan" || identity["departmentName"] != "计划部门" {
		t.Fatalf("部门身份没有替换为计划账号：%+v", identity)
	}
	if identity["dutyId"] != "" || identity["dutyName"] != "" {
		t.Fatalf("运行时会话不含岗位信息，不允许伪造岗位值：%+v", identity)
	}
	if values["expenseUserName"] != "原发起人" {
		t.Fatalf("业务选择字段被误当作登录人上下文替换：%v", values["expenseUserName"])
	}
	absent := map[string]any{"amount": 1}
	service.ReplaceUserIdentityValuesForTest(absent, service.RuntimeUserIdentityForTest("user-plan", "计划账号", "company-plan", "计划公司", "dept-plan", "计划部门"))
	if _, exists := absent["global_user_basic_information"]; exists {
		t.Fatalf("历史数据没有该字段时不应新增：%+v", absent)
	}
}

// TestGetDataReplacesUserIdentityWithPlanAccount 锁定读取边界的身份替换：打开表单数据页看到的就是计划账号身份。
func TestGetDataReplacesUserIdentityWithPlanAccount(t *testing.T) {
	plan := model.Plan{ID: 671, Account: "account-plan", FlowSource: "new", TargetObjectID: "flow-pay", Status: model.PlanStatusNotStarted}
	path := model.ExecutionPath{ID: 671, PlanID: plan.ID, SequenceNo: 1, Name: "身份路径"}
	store := newWorkspaceHistoryStore()
	store.snapshots[901] = model.HistorySnapshot{
		ID: 901, PlanID: plan.ID, CandidateKey: "candidate-identity", FlowCode: "flow-pay", FormName: "请款单", FlowName: "请款单流程",
		RuntimeType: string(target.FormRenderTypeFormMaking), TemplateSummary: map[string]any{}, RawFormData: map[string]any{},
	}
	store.defaults[plan.ID] = repository.HistoryDefaultRecord{PlanID: plan.ID, SnapshotID: 901, Revision: 1}
	store.configs[path.ID] = repository.HistoryPathConfigRecord{
		PathID: path.ID, Revision: 3, DataRevision: 3, SourceMode: model.HistorySourceModeDefault,
		DataStatus:        model.HistoryDataStatusReady,
		EffectiveFormData: []byte(`{"global_user_basic_information":{"userId":"old-user","userName":"原发起人","companyId":"old-company","companyName":"原公司","departmentId":"old-dept","departmentName":"原部门","dutyId":"旧岗位","dutyName":"旧岗位名"},"amount":1}`),
	}
	reader := identityTargetReader{workspaceTargetReader: workspaceTargetReader{snapshot: target.PathConfigurationSnapshot{
		Tree: &target.FlowNodeTemplate{ID: "start", Type: "start", Child: &target.FlowNodeTemplate{ID: "end", Type: "end"}}, EntryNodeIDs: []string{"start"},
		FlowCode: "flow-pay", FlowName: "请款单流程", RenderType: target.FormRenderTypeFormMaking,
		Forms: []target.FormRuntimeTemplate{{Name: "请款单", TemplateData: `{"list":[{"type":"number","model":"amount"}]}`}},
	}}}
	configService := service.NewPathConfigService(service.NewPlanService(&historyPlanRepository{plan: plan}), reader,
		analyzer.NewFlowGraphAnalyzer(), analyzer.NewExecutionPathAnalyzer(), analyzer.NewPathConfigAnalyzer(),
		&workspacePathRepository{paths: []model.ExecutionPath{path}})
	configService.SetHistoryWorkspaceStores(store, store)
	configuration, err := configService.GetData(context.Background(), plan.ID, path.ID)
	if err != nil {
		t.Fatalf("读取数据工作区失败：%v", err)
	}
	identity, ok := configuration.EffectiveFormData["global_user_basic_information"].(map[string]any)
	if !ok {
		t.Fatalf("登录人上下文字段丢失：%+v", configuration.EffectiveFormData)
	}
	if identity["userId"] != "user-plan" || identity["userName"] != "计划账号" || identity["departmentName"] != "计划部门" {
		t.Fatalf("读取边界没有替换为计划账号身份：%+v", identity)
	}
	if identity["dutyId"] != "" {
		t.Fatalf("岗位信息被伪造：%+v", identity)
	}
	if fmt.Sprintf("%v", configuration.EffectiveFormData["amount"]) != "1" {
		t.Fatalf("业务字段被身份替换波及：%v", configuration.EffectiveFormData["amount"])
	}
}

// TestRetryTransientTargetReadRecoversWithinBoundedAttempts 锁定瞬断重试：可重试错误在窗口内恢复即成功，
// 非瞬断错误立即原样返回，持续故障重试耗尽后返回末次错误且不被掩盖。
func TestRetryTransientTargetReadRecoversWithinBoundedAttempts(t *testing.T) {
	transient := target.NewError(target.ErrorUnavailable, errors.New("目标暂不可用"))
	calls := 0
	err := service.RetryTransientTargetReadForTest(context.Background(), 3, func(context.Context) error {
		calls++
		if calls < 3 {
			return transient
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("窗口内恢复的重试没有成功：err=%v calls=%d", err, calls)
	}

	fatal := target.NewError(target.ErrorPermissionDenied, errors.New("无权限"))
	calls = 0
	err = service.RetryTransientTargetReadForTest(context.Background(), 3, func(context.Context) error {
		calls++
		return fatal
	})
	if calls != 1 || !target.IsKind(err, target.ErrorPermissionDenied) {
		t.Fatalf("非瞬断错误没有立即返回：calls=%d err=%v", calls, err)
	}

	calls = 0
	err = service.RetryTransientTargetReadForTest(context.Background(), 3, func(context.Context) error {
		calls++
		return transient
	})
	if calls != 3 || !target.IsKind(err, target.ErrorUnavailable) {
		t.Fatalf("持续瞬断应重试耗尽并返回末次错误：calls=%d err=%v", calls, err)
	}
}
