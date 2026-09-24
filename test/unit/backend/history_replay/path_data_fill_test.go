package history_replay_test

import (
	"encoding/json"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/service"
)

// fillClock 是补齐测试的固定时钟，保证日期类字段的确定性可断言。
var fillClock = time.Date(2026, 9, 24, 10, 30, 0, 0, time.UTC)

// TestRequiredFieldFillDeterministicDefaults 锁定普通 FormMaking 字段按类型确定性补齐：
// 文本占位、数字保真、单多选取有效选项、日期取今天、开关取 true，且历史已满足的字段不被覆盖。
func TestRequiredFieldFillDeterministicDefaults(t *testing.T) {
	fields := []target.FormFieldDetail{
		{EnglishName: "reason", Name: "出入事由", ComponentType: "textarea", Required: true},
		{EnglishName: "days", Name: "天数", ComponentType: "number", Required: true},
		{EnglishName: "level", Name: "级别", ComponentType: "select", Required: true, Options: []target.FormFieldOption{{Label: "一级", Value: "L1"}, {Label: "二级", Value: "L2"}}},
		{EnglishName: "tags", Name: "标签", ComponentType: "checkbox", Required: true, Multiple: true, Options: []target.FormFieldOption{{Label: "甲", Value: "A"}}},
		{EnglishName: "enterDate", Name: "进入日期", ComponentType: "date", Required: true, DateMode: "date"},
		{EnglishName: "urgent", Name: "紧急", ComponentType: "switch", Required: true},
		{EnglishName: "keepReason", Name: "保留事由", ComponentType: "input", Required: true},
	}
	values := map[string]any{"keepReason": "原有值不动"}
	issues := service.RequiredFieldFillForTest(fields, nil, nil, fillClock, values, nil)
	if len(issues) != 0 {
		t.Fatalf("普通字段应全部安全补齐，不应产生问题：%+v", issues)
	}
	if values["reason"] != "自动补齐" {
		t.Fatalf("文本必填未按占位补齐：%#v", values["reason"])
	}
	if got, ok := values["days"].(json.Number); !ok || got.String() != "0" {
		t.Fatalf("数字必填未按 json.Number 保真补齐：%#v", values["days"])
	}
	if values["level"] != "L1" {
		t.Fatalf("单选必填未取第一个有效选项：%#v", values["level"])
	}
	tags, ok := values["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "A" {
		t.Fatalf("多选必填未取单元素有效选项数组：%#v", values["tags"])
	}
	if values["enterDate"] != "2026-09-24" {
		t.Fatalf("日期必填未取今天：%#v", values["enterDate"])
	}
	if values["urgent"] != true {
		t.Fatalf("开关必填未取 true：%#v", values["urgent"])
	}
	if values["keepReason"] != "原有值不动" {
		t.Fatalf("历史已满足字段被误改：%#v", values["keepReason"])
	}
}

// TestRequiredFieldFillSkipsReadonlyAndOptional 锁定只读/隐藏字段与非必填字段不被补齐：
// 发起态不可编辑的字段值由目标自动带出，工具不得代填。
func TestRequiredFieldFillSkipsReadonlyAndOptional(t *testing.T) {
	fields := []target.FormFieldDetail{
		{EnglishName: "readonlyField", Name: "只读", ComponentType: "input", Required: true},
		{EnglishName: "hiddenField", Name: "隐藏", ComponentType: "input", Required: true},
		{EnglishName: "optionalField", Name: "非必填", ComponentType: "input", Required: false},
	}
	powers := map[string]string{"readonlyField": "only_read", "hiddenField": "hide"}
	values := map[string]any{}
	issues := service.RequiredFieldFillForTest(fields, powers, nil, fillClock, values, nil)
	if len(issues) != 0 {
		t.Fatalf("只读/隐藏/非必填字段不应产生补齐或阻断：%+v", issues)
	}
	if len(values) != 0 {
		t.Fatalf("只读/隐藏/非必填字段不应被补值：%#v", values)
	}
}

// TestRequiredFieldFillBlocksUnsafeFields 锁定无法安全确定有效值的字段落 needs_input 阻断，并带字段中文名：
// 人员/组织选择器、级联、未知自定义组件都不臆造值。
func TestRequiredFieldFillBlocksUnsafeFields(t *testing.T) {
	fields := []target.FormFieldDetail{
		{EnglishName: "approver", Name: "审批人", ComponentType: "select", ComponentName: "person-select", Required: true},
		{EnglishName: "region", Name: "所在地区", ComponentType: "cascader", Required: true},
		{EnglishName: "bizWidget", Name: "自定义业务控件", ComponentType: "custom-biz-widget", ComponentName: "custom-biz-widget", Required: true},
		{EnglishName: "noOption", Name: "无选项下拉", ComponentType: "select", Required: true},
	}
	labels := map[string]string{"approver": "审批人", "region": "所在地区"}
	values := map[string]any{}
	issues := service.RequiredFieldFillForTest(fields, nil, labels, fillClock, values, nil)
	if len(issues) != 4 {
		t.Fatalf("四个无法安全确定的必填字段都应落 needs_input：%+v", issues)
	}
	for _, issue := range issues {
		if issue.Code != "REQUIRED_FIELD_EMPTY" || !issue.Blocking {
			t.Fatalf("无法安全补齐的字段必须落阻断 REQUIRED_FIELD_EMPTY：%+v", issue)
		}
		if len(issue.Fields) == 0 {
			t.Fatalf("阻断问题必须带字段键：%+v", issue)
		}
	}
	if len(values) != 0 {
		t.Fatalf("无法安全确定的字段不得被臆造赋值：%#v", values)
	}
}

// TestRequiredFieldFillUploadPendingWithoutUploader 锁定只读预览下必填附件给非阻断待上传提示，
// 不实际上传，也不放行成有效值。
func TestRequiredFieldFillUploadPendingWithoutUploader(t *testing.T) {
	fields := []target.FormFieldDetail{
		{EnglishName: "attachment", Name: "附件", ComponentType: "upload", Required: true},
	}
	values := map[string]any{}
	issues := service.RequiredFieldFillForTest(fields, nil, nil, fillClock, values, nil)
	if len(issues) != 1 || issues[0].Code != "REQUIRED_UPLOAD_PENDING" || issues[0].Blocking {
		t.Fatalf("只读预览下附件应给非阻断待上传提示：%+v", issues)
	}
	if _, exists := values["attachment"]; exists {
		t.Fatalf("只读预览不应给附件赋值：%#v", values)
	}
}

// TestRequiredFieldFillUploadUsesUploader 锁定有上传能力时必填附件一次性回填目标引用，不再报缺失。
func TestRequiredFieldFillUploadUsesUploader(t *testing.T) {
	fields := []target.FormFieldDetail{
		{EnglishName: "attachment", Name: "附件", ComponentType: "upload", Required: true},
	}
	ref := []any{map[string]any{"fileId": "F-1", "fileUrl": "/f/1"}}
	uploadFill := func(target.FormFieldDetail) (any, bool) { return ref, true }
	values := map[string]any{}
	issues := service.RequiredFieldFillForTest(fields, nil, nil, fillClock, values, uploadFill)
	if len(issues) != 0 {
		t.Fatalf("上传成功不应产生问题：%+v", issues)
	}
	if _, ok := values["attachment"].([]any); !ok {
		t.Fatalf("附件未回填目标引用：%#v", values["attachment"])
	}
}

// TestUploadValueForFieldSupersetShape 锁定上传回显值为超集单元素数组，
// 覆盖目标各上传组件读取的关键引用键（name/id/fileId/fileUrl/absolutelyFileUrl），且优先绝对地址。
func TestUploadValueForFieldSupersetShape(t *testing.T) {
	value := service.UploadValueForFieldForTest(target.UploadedFile{
		ID: "REL-1", FileID: "FILE-9", FileURL: "/f/9", AbsolutelyFileURL: "http://t/f/9",
		OriginFileName: "占位.txt", FileType: "ordinaryFile",
	})
	list, ok := value.([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("上传值必须是单元素数组：%#v", value)
	}
	entry, ok := list[0].(map[string]any)
	if !ok {
		t.Fatalf("上传值元素必须是对象：%#v", list[0])
	}
	if entry["fileId"] != "FILE-9" || entry["id"] != "REL-1" {
		t.Fatalf("文件标识引用键不正确：%#v", entry)
	}
	if entry["absolutelyFileUrl"] != "http://t/f/9" || entry["url"] != "http://t/f/9" {
		t.Fatalf("文件地址应优先绝对地址：%#v", entry)
	}
	if entry["name"] != "占位.txt" || entry["originFileName"] != "占位.txt" {
		t.Fatalf("文件名引用键不正确：%#v", entry)
	}
	if entry["status"] != "uploaded" {
		t.Fatalf("上传状态键缺失：%#v", entry)
	}
}
