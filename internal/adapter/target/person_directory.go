package target

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	maxAuditDirectoryScopes     = 100
	maxAuditDirectoryCandidates = 1000
)

type rawAuditDirectoryNode struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	RealName     string                  `json:"realName"`
	DisplayName  string                  `json:"displayName"`
	Type         string                  `json:"type"`
	ChildrenList []rawAuditDirectoryNode `json:"childrenList"`
}

type rawAuditNamedItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RealName    string `json:"realName"`
	DisplayName string `json:"displayName"`
	UserVo      *struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		RealName    string `json:"realName"`
		DisplayName string `json:"displayName"`
	} `json:"userVo"`
}

type rawAuditPersonnelPage struct {
	DataList []rawAuditNamedItem `json:"dataList"`
}

type auditDirectoryResolver struct {
	client        *Client
	ctx           context.Context
	active        Session
	trees         map[string][]rawAuditDirectoryNode
	named         map[string][]rawAuditNamedItem
	roleUsers     map[string][]FlowAuditCandidate
	positionUsers map[string][]FlowAuditCandidate
}

// resolveFlowAuditMetadata 使用目标工作台已核实的只读目录接口补全节点人员范围；失败只形成明确阻塞项，不中断其他节点投影。
func (c *Client) resolveFlowAuditMetadata(ctx context.Context, active Session, tree *FlowNodeTemplate) {
	resolver := &auditDirectoryResolver{
		client: c, ctx: ctx, active: active,
		trees: make(map[string][]rawAuditDirectoryNode), named: make(map[string][]rawAuditNamedItem),
		roleUsers: make(map[string][]FlowAuditCandidate), positionUsers: make(map[string][]FlowAuditCandidate),
	}
	visited := make(map[string]bool)
	var walk func(*FlowNodeTemplate)
	walk = func(node *FlowNodeTemplate) {
		if node == nil || visited[node.ID] {
			return
		}
		visited[node.ID] = true
		resolver.resolve(node.AuditConfig)
		walk(node.Child)
		for index := range node.ConditionNodes {
			walk(node.ConditionNodes[index].Child)
		}
		for index := range node.ParallelNodes {
			walk(node.ParallelNodes[index].Child)
		}
	}
	walk(tree)
}

// resolve 补齐固定详情、运行节点范围名称和合法候选；未知范围或目录失败均阻止误保存为完成。
func (r *auditDirectoryResolver) resolve(config *FlowNodeAuditConfig) {
	if config == nil {
		return
	}
	if len(config.Details)+len(config.Scopes) > maxAuditDirectoryScopes {
		appendAuditResolutionIssue(config, "处理人员", "目标模板人员范围超过当前安全解析上限")
		return
	}
	for index := range config.Details {
		detail := &config.Details[index]
		if strings.TrimSpace(detail.Name) != "" {
			continue
		}
		category := auditDirectoryCategory(detail.Type, config.AuditType)
		if strings.TrimSpace(detail.ID) == "" {
			appendAuditResolutionIssue(config, category, category+"配置缺少可解析标识")
			continue
		}
		name, err := r.resolveName(category, detail.ID)
		if err != nil {
			appendAuditResolutionIssue(config, category, category+"名称读取失败")
			continue
		}
		detail.Name = name
	}
	if strings.TrimSpace(config.AuditType) == "level" && strings.TrimSpace(config.AuditCondition) != "" && len(config.Details) == 0 {
		name, err := r.resolveName("岗级", config.AuditCondition)
		if err != nil {
			appendAuditResolutionIssue(config, "岗级", "岗级名称读取失败")
		} else {
			config.Details = append(config.Details, FlowAuditDetail{ID: config.AuditCondition, Name: name, Type: "level"})
		}
	}
	for index := range config.Scopes {
		scope := &config.Scopes[index]
		category := auditDirectoryCategory(scope.Type, config.AuditType)
		if strings.TrimSpace(scope.ID) == "" {
			appendAuditResolutionIssue(config, category, category+"范围缺少可解析标识")
			continue
		}
		name, candidates, err := r.resolveScope(scope.Type, scope.ID)
		if err != nil {
			appendAuditResolutionIssue(config, category, category+"范围读取失败")
			continue
		}
		scope.Name = name
		config.Candidates = appendUniqueAuditCandidates(config.Candidates, candidates...)
	}
	config.Candidates = appendUniqueAuditCandidates(nil, config.Candidates...)
	if len(config.Candidates) > maxAuditDirectoryCandidates {
		config.Candidates = config.Candidates[:maxAuditDirectoryCandidates]
		appendAuditResolutionIssue(config, "处理人员", "合法候选数量超过当前安全上限")
	}
}

// resolveName 按目标真实目录类别解析固定配置名称，不用内部 ID 或经验值猜测名称。
func (r *auditDirectoryResolver) resolveName(category, id string) (string, error) {
	switch category {
	case "人员":
		return r.nameFromPersonnel(id)
	case "岗位":
		return r.nameFromTree("4", id)
	case "岗级":
		return r.nameFromNamed("level", "/web/user/api/dutyLevel/list", map[string]any{"data": map[string]any{"enableType": "enable"}}, id)
	case "角色":
		return r.nameFromNamed("role", "/web/flowRoleApi/list", map[string]any{
			"data": map[string]any{"customerCode": r.active.CustomerCode, "scope": "invest"}, "pagination": false,
		}, id)
	case "部门":
		return r.nameFromTree("2", id)
	case "公司":
		return r.nameFromTree("7", id)
	case "扩展属性":
		return r.nameFromNamed("extendedAttribute", "/web/user/api/expandAttr/list", map[string]any{
			"data": map[string]any{"name": "", "expandAttrType": nil, "enableType": "enable"},
		}, id)
	default:
		return "", fmt.Errorf("unsupported audit directory category")
	}
}

// resolveScope 同时解析运行节点选择范围名称与该范围内候选，随机和手动策略不得越过这里的结果。
func (r *auditDirectoryResolver) resolveScope(scopeType, id string) (string, []FlowAuditCandidate, error) {
	switch strings.TrimSpace(scopeType) {
	case "personnel":
		items, err := r.personnelItems()
		if err != nil {
			return "", nil, err
		}
		for _, item := range items {
			candidate := auditCandidateFromNamed(item)
			if candidate.ID == strings.TrimSpace(id) {
				return candidate.Name, []FlowAuditCandidate{candidate}, nil
			}
		}
	case "position":
		name, err := r.nameFromTree("4", id)
		if err != nil {
			return "", nil, err
		}
		candidates, err := r.positionCandidates(id)
		return name, candidates, err
	case "role":
		name, err := r.resolveName("角色", id)
		if err != nil {
			return "", nil, err
		}
		candidates, err := r.roleCandidates(id)
		return name, candidates, err
	case "department":
		return r.organizationalScope("2", id)
	case "company":
		return r.organizationalScope("7", id)
	default:
		return "", nil, fmt.Errorf("unsupported audit scope type")
	}
	return "", nil, fmt.Errorf("audit scope not found")
}

// organizationalScope 分别使用名称目录和 flag=3 人员树，避免错误假定 flag=2/7 含人员节点。
func (r *auditDirectoryResolver) organizationalScope(nameFlag, id string) (string, []FlowAuditCandidate, error) {
	name, err := r.nameFromTree(nameFlag, id)
	if err != nil {
		return "", nil, err
	}
	// 目标工作台明确只从 flag=3 的公司部门人员树按 bizId 截取候选，不能扩大到整棵公司树。
	peopleTree, err := r.companyTree("3")
	if err != nil {
		return "", nil, err
	}
	node := findAuditDirectoryNode(peopleTree, id)
	if node == nil {
		return "", nil, fmt.Errorf("audit scope not found")
	}
	candidates := make([]FlowAuditCandidate, 0)
	collectAuditDirectoryUsers(*node, &candidates)
	candidates = appendUniqueAuditCandidates(nil, candidates...)
	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("audit scope has no personnel")
	}
	return name, candidates, nil
}

// nameFromTree 在目标公司目录树中按内部 ID 查找中文名称。
func (r *auditDirectoryResolver) nameFromTree(flag, id string) (string, error) {
	tree, err := r.companyTree(flag)
	if err != nil {
		return "", err
	}
	node := findAuditDirectoryNode(tree, id)
	if node == nil || strings.TrimSpace(node.Name) == "" {
		return "", fmt.Errorf("audit directory item not found")
	}
	return strings.TrimSpace(node.Name), nil
}

// companyTree 读取并缓存同一会话的目标公司目录；缺少登录公司上下文时明确失败而不是扩大到全平台。
func (r *auditDirectoryResolver) companyTree(flag string) ([]rawAuditDirectoryNode, error) {
	if cached, exists := r.trees[flag]; exists {
		return cached, nil
	}
	if strings.TrimSpace(r.active.CompanyID) == "" {
		return nil, fmt.Errorf("login response missing company id")
	}
	resp, err := r.client.call(r.ctx, "/web/user/api/company/children", r.active.SID, map[string]any{
		"data": map[string]any{"flag": flag, "id": r.active.CompanyID, "customerCode": r.active.CustomerCode},
	})
	if err != nil || !responseSucceeded(resp) {
		return nil, fmt.Errorf("company directory unavailable")
	}
	var result []rawAuditDirectoryNode
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("invalid company directory")
	}
	r.trees[flag] = result
	return result, nil
}

// personnelItems 只按 findByCompanyIdUserList 的真实 dataList envelope 解析人员目录。
func (r *auditDirectoryResolver) personnelItems() ([]rawAuditNamedItem, error) {
	if cached, exists := r.named["personnel"]; exists {
		return cached, nil
	}
	if strings.TrimSpace(r.active.CompanyID) == "" {
		return nil, fmt.Errorf("login response missing company id")
	}
	resp, err := r.client.call(r.ctx, "/web/user/api/user/findByCompanyIdUserList", r.active.SID, map[string]any{
		"data": map[string]any{"companyId": r.active.CompanyID}, "pagination": true, "pages": 1, "size": 1000,
	})
	if err != nil || !responseSucceeded(resp) {
		return nil, fmt.Errorf("personnel directory unavailable")
	}
	var page rawAuditPersonnelPage
	if err := json.Unmarshal(resp.Data, &page); err != nil {
		return nil, fmt.Errorf("invalid personnel directory")
	}
	r.named["personnel"] = page.DataList
	return page.DataList, nil
}

// nameFromPersonnel 在真实人员 dataList 中按内部 ID 查找中文姓名。
func (r *auditDirectoryResolver) nameFromPersonnel(id string) (string, error) {
	items, err := r.personnelItems()
	if err != nil {
		return "", err
	}
	for _, item := range items {
		candidate := auditCandidateFromNamed(item)
		if candidate.ID == strings.TrimSpace(id) && candidate.Name != "" {
			return candidate.Name, nil
		}
	}
	return "", fmt.Errorf("personnel directory item not found")
}

// namedItems 只读取已核实为 data 数组的岗级、角色和扩展属性目录，不兼容猜测对象 envelope。
func (r *auditDirectoryResolver) namedItems(cacheKey, path string, body map[string]any) ([]rawAuditNamedItem, error) {
	if cached, exists := r.named[cacheKey]; exists {
		return cached, nil
	}
	resp, err := r.client.call(r.ctx, path, r.active.SID, body)
	if err != nil || !responseSucceeded(resp) {
		return nil, fmt.Errorf("audit directory unavailable")
	}
	var result []rawAuditNamedItem
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("invalid audit directory")
	}
	r.named[cacheKey] = result
	return result, nil
}

// nameFromNamed 在固定扁平目录中查找中文名称。
func (r *auditDirectoryResolver) nameFromNamed(cacheKey, path string, body map[string]any, id string) (string, error) {
	items, err := r.namedItems(cacheKey, path, body)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		candidate := auditCandidateFromNamed(item)
		if candidate.ID == strings.TrimSpace(id) && candidate.Name != "" {
			return candidate.Name, nil
		}
	}
	return "", fmt.Errorf("audit directory item not found")
}

// roleCandidates 读取固定角色下的合法人员，结果仅供当前节点策略生成不透明键。
func (r *auditDirectoryResolver) roleCandidates(roleID string) ([]FlowAuditCandidate, error) {
	if cached, exists := r.roleUsers[roleID]; exists {
		return cached, nil
	}
	resp, err := r.client.call(r.ctx, "/web/flowRoleUserApi/list", r.active.SID, map[string]any{
		"data": map[string]any{"flowRoleId": roleID}, "pagination": false,
	})
	if err != nil || !responseSucceeded(resp) {
		return nil, fmt.Errorf("role users unavailable")
	}
	var items []rawAuditNamedItem
	if err := json.Unmarshal(resp.Data, &items); err != nil {
		return nil, fmt.Errorf("invalid role users")
	}
	result := make([]FlowAuditCandidate, 0, len(items))
	for _, item := range items {
		result = appendUniqueAuditCandidates(result, auditCandidateFromNamed(item))
	}
	r.roleUsers[roleID] = result
	return result, nil
}

// positionCandidates 读取固定岗位下的合法人员，查询类型严格使用目标工作台的 DUTY 语义。
func (r *auditDirectoryResolver) positionCandidates(positionID string) ([]FlowAuditCandidate, error) {
	if cached, exists := r.positionUsers[positionID]; exists {
		return cached, nil
	}
	resp, err := r.client.call(r.ctx, "/web/user/api/user/getUserVosByBizIds", r.active.SID, map[string]any{
		"data": map[string]any{"queryTypeEnum": "DUTY", "bizIds": []string{positionID}},
	})
	if err != nil || !responseSucceeded(resp) {
		return nil, fmt.Errorf("position users unavailable")
	}
	var items []rawAuditNamedItem
	if err := json.Unmarshal(resp.Data, &items); err != nil {
		return nil, fmt.Errorf("invalid position users")
	}
	result := make([]FlowAuditCandidate, 0, len(items))
	for _, item := range items {
		result = appendUniqueAuditCandidates(result, auditCandidateFromNamed(item))
	}
	r.positionUsers[positionID] = result
	return result, nil
}

// auditCandidateFromNamed 兼容普通用户项和角色关系中的 userVo，不公开其内部 ID。
func auditCandidateFromNamed(item rawAuditNamedItem) FlowAuditCandidate {
	if item.UserVo != nil {
		return FlowAuditCandidate{ID: strings.TrimSpace(item.UserVo.ID), Name: firstNonEmpty(item.UserVo.Name, item.UserVo.RealName, item.UserVo.DisplayName)}
	}
	return FlowAuditCandidate{ID: strings.TrimSpace(item.ID), Name: firstNonEmpty(item.Name, item.RealName, item.DisplayName)}
}

// findAuditDirectoryNode 递归查找目标目录节点，目录由受限目标响应提供且不在公开 DTO 中透传。
func findAuditDirectoryNode(nodes []rawAuditDirectoryNode, id string) *rawAuditDirectoryNode {
	id = strings.TrimSpace(id)
	for index := range nodes {
		if strings.TrimSpace(nodes[index].ID) == id {
			return &nodes[index]
		}
		if found := findAuditDirectoryNode(nodes[index].ChildrenList, id); found != nil {
			return found
		}
	}
	return nil
}

// collectAuditDirectoryUsers 只收集目标目录明确标为人员的子节点，避免把组织或岗位 ID 当候选人。
func collectAuditDirectoryUsers(node rawAuditDirectoryNode, result *[]FlowAuditCandidate) {
	if strings.TrimSpace(node.Type) == "5" {
		*result = append(*result, FlowAuditCandidate{ID: strings.TrimSpace(node.ID), Name: firstNonEmpty(node.Name, node.RealName, node.DisplayName)})
	}
	for _, child := range node.ChildrenList {
		collectAuditDirectoryUsers(child, result)
	}
}

// appendUniqueAuditCandidates 按内部 ID 去重合法候选，空 ID 或空名称不能形成可保存 token。
func appendUniqueAuditCandidates(current []FlowAuditCandidate, candidates ...FlowAuditCandidate) []FlowAuditCandidate {
	seen := make(map[string]bool, len(current)+len(candidates))
	result := make([]FlowAuditCandidate, 0, len(current)+len(candidates))
	for _, candidate := range append(append([]FlowAuditCandidate(nil), current...), candidates...) {
		candidate.ID = strings.TrimSpace(candidate.ID)
		candidate.Name = strings.TrimSpace(candidate.Name)
		if candidate.ID == "" || candidate.Name == "" || seen[candidate.ID] {
			continue
		}
		seen[candidate.ID] = true
		result = append(result, candidate)
	}
	return result
}

// appendAuditResolutionIssue 去重目录失败说明，公开层只收到中文类别与稳定原因。
func appendAuditResolutionIssue(config *FlowNodeAuditConfig, category, reason string) {
	for _, issue := range config.ResolutionIssues {
		if issue.Category == category && issue.Reason == reason {
			return
		}
	}
	config.ResolutionIssues = append(config.ResolutionIssues, FlowAuditResolutionIssue{Category: category, Reason: reason})
}

// auditDirectoryCategory 把目标人员类型映射为目录类别；未知值不会作为目标枚举公开。
func auditDirectoryCategory(value, auditType string) string {
	switch strings.TrimSpace(value) {
	case "personnel":
		return "人员"
	case "position":
		return "岗位"
	case "level", "grade":
		return "岗级"
	case "role", "c":
		return "角色"
	case "department":
		return "部门"
	case "company":
		return "公司"
	case "extendedAttribute":
		return "扩展属性"
	}
	switch strings.TrimSpace(auditType) {
	case "assign", "initiator", "form_person":
		return "人员"
	case "position":
		return "岗位"
	case "level":
		return "岗级"
	case "role":
		return "角色"
	case "department", "department_supervisor", "branched_passage_manager":
		return "部门"
	case "company", "company_id":
		return "公司"
	case "extendedAttribute":
		return "扩展属性"
	default:
		return "处理人员"
	}
}
