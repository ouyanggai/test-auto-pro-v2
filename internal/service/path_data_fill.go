package service

import (
	"encoding/json"
	"strings"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
)

// fieldKind 是补齐引擎按 FormMaking 组件真实类型归纳出的赋值门类；
// 只用于决定「能否安全确定一个有效值」，不改变目标字段本身语义。
type fieldKind int

const (
	fieldKindText        fieldKind = iota // 文本/多行/富文本：占位即满足必填
	fieldKindNumber                       // 数字/计分/滑块：默认值或 0，按 json.Number 保真
	fieldKindBool                         // 开关：默认值或 true
	fieldKindSingleChoice                 // 单选/下拉：取默认命中项或第一个有效选项
	fieldKindMultiChoice                  // 多选/复选：取一个有效选项的单元素数组
	fieldKindDate                         // 日期/时间：按组件模式取今天
	fieldKindColor                        // 颜色：默认值或主题色
	fieldKindUpload                       // 文件/图片上传：需真实上传目标才有有效引用
	fieldKindCascader                     // 级联/地区/树：扁平选项无法安全构造有效路径
	fieldKindPerson                       // 人员/组织选择器：需真实实体，禁止臆造
	fieldKindUnknown                      // 未知自定义组件：赋值方式不明
)

// requiredFieldFillInput 汇集补齐所需的只读事实：当前模板字段、发起态字段权限、字段中文名、当前时钟。
// uploadFill 为文件上传字段提供「一次性真实上传并缓存」的引用值；为 nil 表示当前入口不实际上传（只读预览）。
type requiredFieldFillInput struct {
	fields     []target.FormFieldDetail
	powers     map[string]string
	labels     map[string]string
	now        time.Time
	uploadFill func(field target.FormFieldDetail) (any, bool)
}

// requiredFieldFill 对「当前模板必填、发起态可编辑、当前值为空」的 FormMaking 字段做确定性安全补齐。
// 能安全确定有效值的字段就地补齐；无法安全确定有效值的字段落 needs_input 阻断问题（带字段中文名）。
// 只处理 FormMaking 字段字典；自定义页面(vue_custom)由页面自身规则处理，不在此补齐。
// 关键约束：当前用户人员字段由身份注入在本函数之前处理，历史已满足的字段不覆盖、不误报。
func requiredFieldFill(in requiredFieldFillInput, values map[string]any) []model.HistoryDataIssue {
	issues := []model.HistoryDataIssue{}
	if values == nil {
		return issues
	}
	for _, field := range in.fields {
		key := strings.TrimSpace(field.EnglishName)
		// 只补当前模板真正必填、发起态可编辑且当前值为空的字段：三者缺一都跳过，避免误报与误改。
		if key == "" || !field.Required || !fieldEditableAtInitiator(in.powers, key) || !formValueIsEmpty(values[key]) {
			continue
		}
		label := fieldLabel(in.labels, field)
		if classifyFormField(field) == fieldKindUpload {
			issues = fillUploadField(issues, values, key, label, field, in.uploadFill)
			continue
		}
		if value, ok := deterministicFieldValue(field, in.now); ok {
			values[key] = value
			continue
		}
		issues = append(issues, model.HistoryDataIssue{
			Code: "REQUIRED_FIELD_EMPTY", Fields: []string{key},
			Message: "「" + label + "」当前为必填但历史数据为空，且无法安全自动补齐，请手工填写", Blocking: true,
		})
	}
	return issues
}

// fillUploadField 处理必填文件上传字段：有上传能力就一次性真实上传并回填引用，失败阻断；
// 无上传能力（只读预览）时给非阻断提示，交由一键配置或保存时真实上传，避免每次预览触发慢上传。
func fillUploadField(issues []model.HistoryDataIssue, values map[string]any, key, label string, field target.FormFieldDetail, uploadFill func(target.FormFieldDetail) (any, bool)) []model.HistoryDataIssue {
	if uploadFill == nil {
		return append(issues, model.HistoryDataIssue{
			Code: "REQUIRED_UPLOAD_PENDING", Fields: []string{key},
			Message: "「" + label + "」为必填附件，将在一键配置或保存时自动上传占位文件", Blocking: false,
		})
	}
	if value, ok := uploadFill(field); ok {
		values[key] = value
		return issues
	}
	return append(issues, model.HistoryDataIssue{
		Code: "REQUIRED_UPLOAD_FAILED", Fields: []string{key},
		Message: "「" + label + "」必填附件自动上传失败，请稍后重试或手工上传", Blocking: true,
	})
}

// fieldEditableAtInitiator 判断字段在发起态是否可编辑：目标未声明权限的字段默认可填，
// 仅当目标明确声明为只读/隐藏时才跳过（隐藏或只读字段的值由目标自动带出，不属于发起人补齐范围）。
func fieldEditableAtInitiator(powers map[string]string, field string) bool {
	power, declared := powers[field]
	if !declared {
		return true
	}
	return power == "edit"
}

// formValueIsEmpty 判定字段当前值是否为空：nil、空白字符串、空数组、空对象都算空；
// 数字(json.Number)、布尔 false 等已设定值不算空，避免把有效值当缺失重复补齐。
func formValueIsEmpty(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

// deterministicFieldValue 按字段门类给出确定性有效默认值；不随机，避免触发长度/正则/枚举约束。
// 返回 ok=false 表示无法安全确定（级联/人员/未知自定义），交调用方落 needs_input。
func deterministicFieldValue(field target.FormFieldDetail, now time.Time) (any, bool) {
	switch classifyFormField(field) {
	case fieldKindText:
		return firstNonEmptyString(field.DefaultValue, "自动补齐"), true
	case fieldKindNumber:
		if n := strings.TrimSpace(field.DefaultValue); n != "" {
			// 数字按 json.Number 保真落库，与 F-012 两级数字保真口径一致。
			return json.Number(n), true
		}
		return json.Number("0"), true
	case fieldKindBool:
		if value, ok := parseBoolDefault(field.DefaultValue); ok {
			return value, true
		}
		return true, true
	case fieldKindSingleChoice:
		if value, ok := chooseOptionValue(field); ok {
			return value, true
		}
		return nil, false
	case fieldKindMultiChoice:
		if value, ok := chooseOptionValue(field); ok {
			return []any{value}, true
		}
		return nil, false
	case fieldKindDate:
		return defaultDateValue(field.DateMode, now), true
	case fieldKindColor:
		return firstNonEmptyString(field.DefaultValue, "#409EFF"), true
	default:
		// fieldKindCascader / fieldKindPerson / fieldKindUnknown：无法安全确定有效值。
		return nil, false
	}
}

// classifyFormField 按 FormMaking 组件真实类型与注册名归纳门类；文件上传与人员/组织选择器优先识别。
func classifyFormField(field target.FormFieldDetail) fieldKind {
	name := strings.ToLower(strings.TrimSpace(field.ComponentName))
	typ := strings.ToLower(strings.TrimSpace(field.ComponentType))
	// 文件/图片上传优先识别：其值必须是目标文件服务返回的引用，本地静态文件目标不认。
	if containsAny(typ, "upload", "imgupload", "file") || containsAny(name, "upload", "file", "attach", "附件") {
		return fieldKindUpload
	}
	// 人员/组织选择器需要真实实体，禁止臆造；当前用户字段已由身份注入在此之前覆盖。
	if containsAny(name, "person", "user", "staff", "depart", "dept", "company", "role", "organization", "选人", "人员", "部门", "公司") {
		return fieldKindPerson
	}
	switch typ {
	case "input", "textarea", "text", "editor", "richtext", "":
		if len(field.Options) > 0 {
			if field.Multiple {
				return fieldKindMultiChoice
			}
			return fieldKindSingleChoice
		}
		return fieldKindText
	case "number", "rate", "slider", "money", "calculate":
		return fieldKindNumber
	case "select", "radio":
		if field.Multiple {
			return fieldKindMultiChoice
		}
		return fieldKindSingleChoice
	case "checkbox":
		return fieldKindMultiChoice
	case "switch":
		return fieldKindBool
	case "date", "time", "datepicker", "timepicker":
		return fieldKindDate
	case "color", "colorpicker":
		return fieldKindColor
	case "cascader", "region", "treeselect", "tree":
		return fieldKindCascader
	default:
		return fieldKindUnknown
	}
}

// chooseOptionValue 从字段真实选项中取一个有效值：默认值命中选项时优先，否则取第一个非空选项值。
func chooseOptionValue(field target.FormFieldDetail) (any, bool) {
	def := strings.TrimSpace(field.DefaultValue)
	if def != "" {
		for _, option := range field.Options {
			if strings.TrimSpace(option.Value) != "" && option.Value == def {
				return option.Value, true
			}
		}
	}
	for _, option := range field.Options {
		if strings.TrimSpace(option.Value) != "" {
			return option.Value, true
		}
	}
	return nil, false
}

// defaultDateValue 按组件日期模式返回今天的合法值；区间型返回今天到今天的两元数组。
func defaultDateValue(mode string, now time.Time) any {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "datetime":
		return now.Format("2006-01-02 15:04:05")
	case "time":
		return now.Format("15:04:05")
	case "year":
		return now.Format("2006")
	case "month":
		return now.Format("2006-01")
	case "daterange":
		day := now.Format("2006-01-02")
		return []any{day, day}
	case "datetimerange":
		stamp := now.Format("2006-01-02 15:04:05")
		return []any{stamp, stamp}
	default:
		return now.Format("2006-01-02")
	}
}

// fieldLabel 取字段中文名：优先模板布局标签，其次组件名，最后回退英文名，保证阻断提示可读。
func fieldLabel(labels map[string]string, field target.FormFieldDetail) string {
	if labels != nil {
		if label := strings.TrimSpace(labels[strings.TrimSpace(field.EnglishName)]); label != "" {
			return label
		}
	}
	if name := strings.TrimSpace(field.Name); name != "" {
		return name
	}
	return strings.TrimSpace(field.EnglishName)
}

// parseBoolDefault 解析开关组件的默认值文本，无法识别时返回 ok=false。
func parseBoolDefault(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true, true
	case "false", "0", "no", "off":
		return false, true
	default:
		return false, false
	}
}

// firstNonEmptyString 返回第一个非空白字符串。
func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// containsAny 判断目标字符串是否包含任一子串（子串已按小写传入或本身大小写无关）。
func containsAny(value string, substrings ...string) bool {
	for _, substring := range substrings {
		if substring != "" && strings.Contains(value, substring) {
			return true
		}
	}
	return false
}

// buildInitiatorPowerMap 把发起态字段权限投影为「字段->权限」映射，供补齐引擎判断可编辑性。
func buildInitiatorPowerMap(permissions []model.PathFormPermission) map[string]string {
	powers := make(map[string]string, len(permissions))
	for _, permission := range permissions {
		field := strings.TrimSpace(permission.Field)
		if field == "" {
			continue
		}
		powers[field] = permission.Power
	}
	return powers
}

// RequiredFieldFillForTest 暴露缺失必填安全补齐，供 test 目录锁定确定性补齐与 needs_input 边界。
// powers 为 nil 视为无字段权限声明（发起态全部可填）；uploadFill 为 nil 模拟只读预览（附件不实际上传）。
func RequiredFieldFillForTest(fields []target.FormFieldDetail, powers, labels map[string]string, now time.Time, values map[string]any, uploadFill func(target.FormFieldDetail) (any, bool)) []model.HistoryDataIssue {
	return requiredFieldFill(requiredFieldFillInput{fields: fields, powers: powers, labels: labels, now: now, uploadFill: uploadFill}, values)
}
