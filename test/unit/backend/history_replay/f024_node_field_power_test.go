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
	views := service.NodeFormViewsForTest(f024Tree(), f024Reachable(), map[string]any{
		"classificationId":  []any{"c-1"},
		"contractSum":       12,
		"accountantOpinion": "上游还没填",
		"systemField":       "表单自己维护",
	})
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
	if initiator["legalOpinion"] == "edit" {
		t.Fatalf("目标声明隐藏的字段不得被放开为可编辑：%+v", views[0].Permissions)
	}
	if initiator["accountantOpinion"] == "edit" {
		t.Fatalf("审批节点独占字段不得在发起人视图里放开为可编辑：%+v", views[0].Permissions)
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
	// 只有后续节点才能编辑的字段：组件照常显示（不隐藏、按只读渲染），但不回显样本值，
	// 否则用户会以为这一步就会提交它；执行到真正拥有它的节点时再自动填入。
	if _, exists := initiator["accountantOpinion"]; exists {
		t.Fatalf("只有后续节点能填的字段不得出现在权限清单里（应按只读渲染）：%+v", views[0].Permissions)
	}
	if !containsField(views[0].BlankFields, "accountantOpinion") {
		t.Fatalf("只有后续节点能填的字段应在发起人视图不回显样本值：%+v", views[0].BlankFields)
	}
	if containsField(views[0].BlankFields, "systemField") {
		t.Fatalf("没有任何节点声明的表单自身字段照常回显：%+v", views[0].BlankFields)
	}
	if containsField(views[0].BlankFields, "contractSum") {
		t.Fatalf("本节点可编辑的字段必须照常回显：%+v", views[0].BlankFields)
	}
	if power, exists := audit["accountantOpinion"]; !exists || power != "edit" {
		t.Fatalf("字段在真正拥有它的节点视图里必须可编辑：%+v", views[1].Permissions)
	}
	if len(views[1].BlankFields) != 0 {
		t.Fatalf("最后一个节点之后没有别的节点，不该有不回显字段：%+v", views[1].BlankFields)
	}
	// 目标自己声明 hide 的字段照旧隐藏：hide 是目标的显示约定，不是我们的样本数据策略。
	if initiator["legalOpinion"] != "hide" {
		t.Fatalf("目标声明隐藏的字段必须沿用隐藏：%+v", views[0].Permissions)
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

// containsField 判断字段清单是否包含目标字段。
func containsField(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestF024SaveRestoresFieldsOutsideCurrentView 锁定保存边界：一个节点视图只能改该节点有编辑权限的字段。
// 没有权限的字段在这个视图里刻意不回显样本值，若照浏览器回传原样落盘就会把样本数据存成空，
// 执行到真正拥有它的节点时就没得可填了。因此这些键一律恢复为服务端基线值。
func TestF024SaveRestoresFieldsOutsideCurrentView(t *testing.T) {
	submitted := map[string]any{
		"contractSum":       500001,       // 本视图可编辑：用户改过，保留
		"accountantOpinion": "",           // 本视图无权限且未回显：必须恢复
		"classificationId":  []any{"c-9"}, // 本视图无权限：必须恢复
		"extraCompanion":    "表单自己新增的键",   // 基线里没有：保留提交值，不误删
	}
	baseline := map[string]any{
		"contractSum":       10000,
		"accountantOpinion": "样本里的会计意见",
		"classificationId":  []any{"c-1"},
	}
	restored := service.RestoreFieldsOutsideViewForTest(submitted, baseline, []string{"contractSum"})
	if submitted["contractSum"] != 500001 {
		t.Fatalf("本视图可编辑字段不得被基线覆盖：%v", submitted["contractSum"])
	}
	if submitted["accountantOpinion"] != "样本里的会计意见" {
		t.Fatalf("无权限字段必须恢复为基线值，实际 %v", submitted["accountantOpinion"])
	}
	if got, ok := submitted["classificationId"].([]any); !ok || len(got) != 1 || got[0] != "c-1" {
		t.Fatalf("无权限字段的数组取值必须恢复为基线值，实际 %v", submitted["classificationId"])
	}
	if submitted["extraCompanion"] != "表单自己新增的键" {
		t.Fatalf("基线里没有的键必须保留提交值：%v", submitted["extraCompanion"])
	}
	if !containsField(restored, "accountantOpinion") || !containsField(restored, "classificationId") {
		t.Fatalf("恢复过的字段必须如实返回：%v", restored)
	}
	if containsField(restored, "contractSum") {
		t.Fatalf("没有变化的可编辑字段不该记为恢复：%v", restored)
	}
}

// f024TreeWithoutPowers 构造一条完全没有字段权限声明的路线：目标库里有 280 个流程模板就是这样。
func f024TreeWithoutPowers() *target.FlowNodeTemplate {
	audit := &target.FlowNodeTemplate{ID: "audit", Type: "common", Name: "部门审批"}
	return &target.FlowNodeTemplate{ID: "start", Type: "start", Name: "发起人", Child: audit}
}

// TestF024NoFieldPowerDeclarationDegradesInsteadOfBlocking 锁定通用性：
// 一条声明都没有的流程不能被工具凭空判死——不分节点视图、不产生不可填阻断，
// 只给一条中文说明告诉用户分段填写在这条路线上不生效。
func TestF024NoFieldPowerDeclarationDegradesInsteadOfBlocking(t *testing.T) {
	tree := f024TreeWithoutPowers()
	reachable := []string{"start", "audit"}
	fields := []model.HistoryKeyField{{Path: "anyField", Label: "任意字段", Decisive: true}}

	if views := service.NodeFormViewsForTest(tree, reachable, map[string]any{"anyField": "值"}); len(views) != 0 {
		t.Fatalf("没有字段权限声明时不应产生按节点视图：%+v", views)
	}
	if issues := service.UnfillableKeyFieldIssuesForTest(tree, reachable, fields); len(issues) != 0 {
		t.Fatalf("没有字段权限声明时不得产生不可填阻断：%+v", issues)
	}
	hinted := service.KeyFieldFillHintsForTest(tree, reachable, fields)
	if len(hinted) != 1 || hinted[0].FillNodeName != "" || hinted[0].FillableAtStart {
		t.Fatalf("没有依据时不得凭空给出填写节点：%+v", hinted)
	}
	degraded := service.FieldPowerDegradationIssuesForTest(tree, reachable)
	if len(degraded) != 1 || degraded[0].Code != "NODE_FIELD_POWER_NOT_DECLARED" || degraded[0].Blocking {
		t.Fatalf("必须给出一条不阻断的中文说明：%+v", degraded)
	}
}

// TestF024InitiatorWithoutDeclarationIsExplained 锁定另一种缺失：只有发起节点没有声明。
// 按目标口径发起态整张表单只读，用户会困惑"为什么什么都填不了"，必须给出原因说明。
func TestF024InitiatorWithoutDeclarationIsExplained(t *testing.T) {
	audit := &target.FlowNodeTemplate{ID: "audit", Type: "common", Name: "部门审批", FieldPowers: []target.FlowNodeFieldPower{
		{EnglishName: "accountantOpinion", Power: "edit"},
	}}
	tree := &target.FlowNodeTemplate{ID: "start", Type: "start", Name: "发起人", Child: audit}
	degraded := service.FieldPowerDegradationIssuesForTest(tree, []string{"start", "audit"})
	if len(degraded) != 1 || degraded[0].Code != "INITIATOR_FIELD_POWER_EMPTY" || degraded[0].Blocking {
		t.Fatalf("发起节点没有声明时必须给出不阻断的原因说明：%+v", degraded)
	}
}
