package history_replay_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata/fieldpower"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

// f024Tree 构造一条真实形状的路线：发起人能改分类与金额，审批节点独占会计意见，
// 另有一个条件字段任何节点都不能改。条件节点混在中间，验证结构节点不会被当成"能填的节点"。
func f024Tree() *target.FlowNodeTemplate {
	audit := &target.FlowNodeTemplate{ID: "audit", Type: "common", Name: "部门审批", FieldPowers: []target.FlowNodeFieldPower{
		{EnglishName: "accountantOpinion", Power: "edit"},
		{EnglishName: "contractSum", Power: "edit"},
	}}
	branch := &target.FlowNodeTemplate{ID: "branch", Type: "condition", Name: "金额分支", ConditionNodes: []target.FlowBranchTemplate{
		{ID: "big", Child: audit},
	}}
	return &target.FlowNodeTemplate{ID: "start", Type: "start", Name: "发起人", Child: branch,
		FieldPowers: []target.FlowNodeFieldPower{
			{EnglishName: "classificationId", Power: "edit"},
			{EnglishName: "contractSum", Power: "edit"},
			{EnglishName: "legalOpinion", Power: "hide"},
		}}
}

// f024Reachable 是这条路线的可达节点集合。
func f024Reachable() []string {
	return []string{"start", "branch", "audit"}
}

// TestF024KeyFieldFillHintsPointAtTheNodeThatCanFillIt 锁定填写时机提示：
// 发起人能编辑的条件字段标为发起时提交；发起人无权编辑但后续节点能改的，
// 指向那个节点（目标条件求值只认本次写请求带上来的数据，语义清单第 17 条）；
// 谁都改不了的字段不给节点名，由阻断问题接手。
func TestF024KeyFieldFillHintsPointAtTheNodeThatCanFillIt(t *testing.T) {
	fields := []model.HistoryKeyField{
		{Path: "classificationId__virtualName", Label: "施工分类", Decisive: true},
		{Path: "accountantOpinion", Label: "会计意见", Decisive: true},
		{Path: "outsideField", Label: "外部字段", Decisive: true},
	}
	hinted := service.KeyFieldFillHintsForTest(f024Tree(), f024Reachable(), fields)
	byPath := map[string]model.HistoryKeyField{}
	for _, field := range hinted {
		byPath[field.Path] = field
	}
	if got := byPath["classificationId__virtualName"]; !got.FillableAtStart || got.FillNodeName != "发起人" {
		t.Fatalf("发起人可编辑控件的伴生条件字段应标为发起时填写：%+v", got)
	}
	if got := byPath["accountantOpinion"]; got.FillableAtStart || got.FillNodeName != "部门审批" {
		t.Fatalf("发起人无权编辑的条件字段应指向真正能填它的节点：%+v", got)
	}
	if got := byPath["outsideField"]; got.FillableAtStart || got.FillNodeName != "" {
		t.Fatalf("没有任何节点能编辑的字段不得凭空给出填写节点：%+v", got)
	}
}

// TestF024UnfillableDecisiveConditionFieldBlocks 锁定阻断：决定性条件字段在这条路线上
// 一个节点都填不了时必须阻断，不能让运行到那一步再按目标现有数据莫名走分支。
func TestF024UnfillableDecisiveConditionFieldBlocks(t *testing.T) {
	fields := []model.HistoryKeyField{
		{Path: "outsideField", Label: "外部字段", Decisive: true},
		{Path: "accountantOpinion", Label: "会计意见", Decisive: true},
		{Path: "otherUnfillable", Label: "非决定字段", Decisive: false},
	}
	issues := service.UnfillableKeyFieldIssuesForTest(f024Tree(), f024Reachable(), fields)
	if len(issues) != 1 {
		t.Fatalf("只应为决定性且不可填的字段产生一条阻断：%+v", issues)
	}
	if issues[0].Path != "outsideField" || !issues[0].Blocking {
		t.Fatalf("阻断问题的字段或阻断标记不对：%+v", issues[0])
	}
	if issues[0].Code != "CONDITION_FIELD_NOT_FILLABLE" {
		t.Fatalf("阻断问题码不稳定：%+v", issues[0])
	}
}

// TestF024NodeFormViewsFollowTargetDeclaration 锁定按节点视图：视图按路线顺序，
// 发起人在最前；每个视图只放开该节点声明为 edit 的字段，hide 与未声明字段都不放开；
// 条件、并行等结构节点没有表单也没有待办，不产生视图。
func TestF024NodeFormViewsFollowTargetDeclaration(t *testing.T) {
	views := service.NodeFormViewsForTest(f024Tree(), f024Reachable())
	if len(views) != 2 {
		t.Fatalf("只有人工节点才有填写视图，实际 %d 个：%+v", len(views), views)
	}
	if !views[0].IsInitiator || views[0].NodeName != "发起人" {
		t.Fatalf("发起人视图必须在最前：%+v", views)
	}
	initiator := map[string]string{}
	for _, permission := range views[0].Permissions {
		initiator[permission.Field] = permission.Power
	}
	if initiator["classificationId"] != "edit" || initiator["contractSum"] != "edit" {
		t.Fatalf("发起人视图缺少声明可编辑字段：%+v", views[0].Permissions)
	}
	if _, exists := initiator["legalOpinion"]; exists {
		t.Fatalf("hide 字段不得出现在可编辑清单里：%+v", views[0].Permissions)
	}
	if _, exists := initiator["accountantOpinion"]; exists {
		t.Fatalf("审批节点独占字段不得在发起人视图里放开：%+v", views[0].Permissions)
	}
	audit := map[string]string{}
	for _, permission := range views[1].Permissions {
		audit[permission.Field] = permission.Power
	}
	if audit["accountantOpinion"] != "edit" {
		t.Fatalf("审批节点视图缺少它自己声明的可编辑字段：%+v", views[1].Permissions)
	}
	if _, exists := audit["classificationId"]; exists {
		t.Fatalf("审批节点不能改的字段不得放开：%+v", views[1].Permissions)
	}
}

// TestF024FieldPowerCoversTargetConventions 锁定字段权限判据的三条目标约定：
// 伴生键跟随控件本体、名称字段跟随同前缀 Id 控件、子表单容器由它的列权限覆盖。
func TestF024FieldPowerCoversTargetConventions(t *testing.T) {
	editable := []string{"classificationId", "expenseDetailList.amount", "accountantUserName"}
	for _, key := range []string{
		"classificationId",
		"classificationId__virtualName",
		"classificationName",
		"expenseDetailList",
		"accountantUserName__formPersonId",
	} {
		if !fieldpower.Covers(editable, key) {
			t.Fatalf("按目标约定 %s 应被覆盖：%v", key, editable)
		}
	}
	for _, key := range []string{"legalOpinion", "classification", "expenseDetail"} {
		if fieldpower.Covers(editable, key) {
			t.Fatalf("未声明的字段不得被判为可编辑：%s", key)
		}
	}
	if got := fieldpower.NormalizeFieldPath("expenseDetailList_$$_amount"); got != "expenseDetailList.amount" {
		t.Fatalf("嵌套字段分隔符必须与目标前端一致地归一，实际 %s", got)
	}
}
