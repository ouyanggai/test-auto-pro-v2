package service

import (
	"context"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

// PathDataCompanyDirectory 数据工作区同步公司下拉真实 ID 所需的目标公司主数据只读目录。
// 目标公司下拉的真实来源是 t_company 主数据与 t_project_company 项目公司的并集，实现方必须两处都查。
type PathDataCompanyDirectory interface {
	// CompanyNameByID 按公司 ID 查询未删除公司名称；found=false 表示公司在任何来源中都不存在或已删除。
	CompanyNameByID(ctx context.Context, companyID string) (name string, found bool, err error)
	// CompanyIDByName 按公司名称查询全部未删除公司 ID；同名多家时由调用方拒绝解析。
	CompanyIDByName(ctx context.Context, name string) ([]string, error)
}

// targetCompanyDatasourceURLMarker 是目标公司接口的路径特征；只有远程数据源命中该特征的下拉才按公司主数据同步，
// 避免把部门、人员、费用归口等其他远程选择器误写成公司 ID。
const targetCompanyDatasourceURLMarker = "/api/web/user/api/company/"

// SetCompanyDirectory 注入目标公司目录；未注入时数据工作区保持历史行为，不做任何 ID 同步。
func (s *PathConfigService) SetCompanyDirectory(directory PathDataCompanyDirectory) {
	s.companyDirectory = directory
}

// linkedCompanySelectPair 是模板里"公司远程下拉 ID 字段 + 同前缀名称字段"的配对，例如
// applicationFundsVo_payCompanyId 与 applicationFundsVo_payCompanyName。
type linkedCompanySelectPair struct {
	idField   string
	nameField string
}

// syncLinkedCompanySelects 检查表单数据里的公司远程下拉，把名称与 ID 指向不一致的记录同步为真实公司。
// 分支条件只按名称字段命中，最小补丁不会改动 ID 字段，历史 ID 残留会让下拉框显示旧公司：
// FormMaking 按值匹配选项标签，匹配不到时才用 __virtualName 兜底显示。因此必须在服务端按目标公司表
// 解析名称对应的真实 ID 并补齐虚拟显示值，界面回显、提交数据与路径提示才能三者一致。
// 目录不可用或无法唯一解析时保留原值并记录阻断问题：保留即意味着控件继续显示历史公司，
// 这正是需要用户处理（改选公司或修正路径数据）的阻断场景，不能静默放过。
func (s *PathConfigService) syncLinkedCompanySelects(ctx context.Context, template map[string]any, values map[string]any) []model.HistoryDataIssue {
	if s == nil || s.companyDirectory == nil || len(values) == 0 {
		return nil
	}
	issues := []model.HistoryDataIssue{}
	for _, pair := range linkedCompanySelectPairs(template) {
		nameValue, nameOK := values[pair.nameField].(string)
		idValue, idOK := values[pair.idField].(string)
		if !nameOK || strings.TrimSpace(nameValue) == "" || !idOK || strings.TrimSpace(idValue) == "" {
			continue
		}
		// 名称与当前 ID 已指向同一家公司时数据一致，不做任何写入，也不依赖目录服务的额外查询。
		if currentName, found, err := s.companyDirectory.CompanyNameByID(ctx, idValue); err == nil && found && currentName == nameValue {
			continue
		}
		ids, err := s.companyDirectory.CompanyIDByName(ctx, nameValue)
		if err != nil {
			issues = appendHistoryIssues(issues, []model.HistoryDataIssue{{Code: "COMPANY_LINK_UNRESOLVED", Path: pair.idField, Blocking: true,
				Message: "按名称解析公司真实 ID 失败，控件仍会显示并提交历史公司：" + nameValue}})
			continue
		}
		// 同名多家公司无法确定目标身份，宁可保留历史 ID 也不能猜测，否则会写入错误的业务归属。
		if len(ids) != 1 {
			issues = appendHistoryIssues(issues, []model.HistoryDataIssue{{Code: "COMPANY_LINK_UNRESOLVED", Path: pair.idField, Blocking: true,
				Message: "目标公司目录未能唯一匹配该公司，控件仍会显示并提交历史公司：" + nameValue}})
			continue
		}
		values[pair.idField] = ids[0]
		// FormMaking 的兜底显示选项直接读取 __virtualName；选项列表按用户权限过滤后可能不含目标公司，必须补齐才能正确回显。
		values[pair.idField+"__virtualName"] = nameValue
	}
	if len(issues) == 0 {
		return nil
	}
	return issues
}

// linkedCompanySelectPairs 从 FormMaking 模板提取公司远程下拉与名称字段的配对。
func linkedCompanySelectPairs(template map[string]any) []linkedCompanySelectPair {
	config, ok := template["config"].(map[string]any)
	if !ok {
		return nil
	}
	companyKeys := map[string]bool{}
	for _, item := range templateNodeList(config["dataSource"]) {
		source, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.Contains(templateString(source["url"]), targetCompanyDatasourceURLMarker) {
			if key := templateString(source["key"]); key != "" {
				companyKeys[key] = true
			}
		}
	}
	if len(companyKeys) == 0 {
		return nil
	}
	pairs := []linkedCompanySelectPair{}
	seen := map[string]bool{}
	var visit func(node any)
	visit = func(node any) {
		switch typed := node.(type) {
		case []any:
			for _, child := range typed {
				visit(child)
			}
		case map[string]any:
			options, _ := typed["options"].(map[string]any)
			modelName := templateString(typed["model"])
			// 目标约定远程下拉绑定 ID 字段，配对的名称字段去掉 Id 后缀再加 Name；同名字段只登记一次。
			if templateString(typed["type"]) == "select" && options != nil &&
				options["remote"] == true && strings.HasSuffix(modelName, "Id") && len(modelName) > 2 &&
				companyKeys[templateString(options["remoteDataSource"])] && !seen[modelName] {
				seen[modelName] = true
				pairs = append(pairs, linkedCompanySelectPair{idField: modelName, nameField: modelName[:len(modelName)-2] + "Name"})
			}
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(template["list"])
	return pairs
}

// templateNodeList 把模板节点的子列表安全转为 any 切片，非列表输入返回空。
func templateNodeList(value any) []any {
	if list, ok := value.([]any); ok {
		return list
	}
	return nil
}

// targetGlobalUserIdentityField 是目标发起页提交表单时统一注入的登录人上下文字段名。
// 目标 FlowDialog 在每次提交前用登录态覆盖该字段（userId/userName/companyId/companyName/
// departmentId/departmentName/dutyId/dutyName），历史实例里保存的是原发起人身份。
const targetGlobalUserIdentityField = "global_user_basic_information"

// runtimeUserIdentity 是数据工作区替换登录人上下文所需的当前计划账号身份。
type runtimeUserIdentity struct {
	UserID         string
	UserName       string
	CompanyID      string
	CompanyName    string
	DepartmentID   string
	DepartmentName string
}

// currentUserIdentity 读取计划账号在目标平台的当前身份；会话由会话管理器缓存，不额外触发登录。
func (s *PathConfigService) currentUserIdentity(ctx context.Context, planID uint64) (runtimeUserIdentity, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return runtimeUserIdentity{}, err
	}
	reader, ok := s.target.(interface {
		FormRuntimeSession(context.Context, string) (target.FormRuntimeSession, error)
	})
	if !ok {
		return runtimeUserIdentity{}, &PathConfigError{Kind: PathConfigErrorStorage, Message: "表单运行时会话暂不可用"}
	}
	var active target.FormRuntimeSession
	if err := retryTransientTargetRead(ctx, 3, func(ctx context.Context) error {
		session, sessionErr := reader.FormRuntimeSession(ctx, plan.Account)
		if sessionErr != nil {
			return sessionErr
		}
		active = session
		return nil
	}); err != nil {
		return runtimeUserIdentity{}, err
	}
	return runtimeUserIdentity{
		UserID: active.UserID, UserName: active.AccountName,
		CompanyID: active.CompanyID, CompanyName: active.CompanyName,
		DepartmentID: active.DepartmentID, DepartmentName: active.DepartmentName,
	}, nil
}

// replaceUserIdentityValues 把历史表单数据里目标提交时注入的登录人上下文字段替换为当前计划账号身份。
// 目标在每次提交时都会用登录态覆盖该字段，回放值若保留原发起人身份，提交出去的数据会冒用他人身份；
// 岗位信息由目标登录态携带而运行时会话不含岗位，保持为空，不伪造岗位值。
func replaceUserIdentityValues(values map[string]any, identity runtimeUserIdentity) {
	if values == nil {
		return
	}
	if _, exists := values[targetGlobalUserIdentityField]; !exists {
		return
	}
	values[targetGlobalUserIdentityField] = map[string]any{
		"userId":         identity.UserID,
		"userName":       identity.UserName,
		"companyId":      identity.CompanyID,
		"companyName":    identity.CompanyName,
		"departmentId":   identity.DepartmentID,
		"departmentName": identity.DepartmentName,
		"dutyId":         "",
		"dutyName":       "",
	}
}

// ReplaceUserIdentityValuesForTest 暴露登录人上下文替换，供 test 目录下的定向用例锁定行为。
func ReplaceUserIdentityValuesForTest(values map[string]any, identity runtimeUserIdentity) {
	replaceUserIdentityValues(values, identity)
}

// RuntimeUserIdentityForTest 构造测试用登录人身份，避免为测试导出内部结构。
func RuntimeUserIdentityForTest(userID, userName, companyID, companyName, departmentID, departmentName string) runtimeUserIdentity {
	return runtimeUserIdentity{
		UserID: userID, UserName: userName, CompanyID: companyID,
		CompanyName: companyName, DepartmentID: departmentID, DepartmentName: departmentName,
	}
}

// RetryTransientTargetReadForTest 暴露目标读取瞬断重试，供 test 目录下的定向用例锁定行为。
func RetryTransientTargetReadForTest(ctx context.Context, attempts int, call func(context.Context) error) error {
	return retryTransientTargetRead(ctx, attempts, call)
}
