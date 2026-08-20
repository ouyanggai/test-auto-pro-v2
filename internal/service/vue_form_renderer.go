package service

import (
	"test-auto-pro-v2/internal/adapter/target"
)

// VueFormRenderer 将 Vue 页面规则转换为动态表单配置。
type VueFormRenderer struct{}

// NewVueFormRenderer 创建 Vue 表单渲染器。
func NewVueFormRenderer() *VueFormRenderer {
	return &VueFormRenderer{}
}

// RenderConfig 将 Vue 页面规则转换为 Element Form 配置。
type VueFormRenderConfig struct {
	Fields      []VueFormFieldConfig `json:"fields"`
	Rules       map[string][]VueFormFieldRule `json:"rules"`
	InitialData map[string]any       `json:"initialData"`
	ReadOnly    bool                 `json:"readOnly"`
}

// VueFormFieldConfig 是单个表单字段的渲染配置。
type VueFormFieldConfig struct {
	Prop        string                    `json:"prop"`
	Label       string                    `json:"label"`
	Type        string                    `json:"type"` // input/number/date/select/checkbox
	Required    bool                      `json:"required"`
	ReadOnly    bool                      `json:"readOnly"`
	Disabled    bool                      `json:"disabled"`
	Hidden      bool                      `json:"hidden"`
	Placeholder string                    `json:"placeholder,omitempty"`
	Options     []VueFormFieldOption      `json:"options,omitempty"`
	DefaultValue any                      `json:"defaultValue,omitempty"`
	Dependencies []string                  `json:"dependencies,omitempty"` // 依赖的其他字段
}

// VueFormFieldOption 是字段选项。
type VueFormFieldOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// VueFormFieldRule 是字段验证规则。
type VueFormFieldRule struct {
	Required bool   `json:"required,omitempty"`
	Message  string `json:"message,omitempty"`
	Trigger  string `json:"trigger,omitempty"` // blur/change
	Pattern  string `json:"pattern,omitempty"`
	Type     string `json:"type,omitempty"` // string/number/date/array
}

// Render 将 Vue 页面规则转换为表单配置。
func (r *VueFormRenderer) Render(page *target.VueCustomPageRule) VueFormRenderConfig {
	if page == nil {
		return VueFormRenderConfig{
			Fields:      []VueFormFieldConfig{},
			Rules:       map[string][]VueFormFieldRule{},
			InitialData: map[string]any{},
		}
	}

	config := VueFormRenderConfig{
		Fields:      make([]VueFormFieldConfig, 0, len(page.Fields)),
		Rules:       make(map[string][]VueFormFieldRule),
		InitialData: page.InitialState,
		ReadOnly:    false,
	}

	// 构建依赖映射
	dependencies := buildDependencyMap(page.Dependencies)

	// 转换字段
	for _, field := range page.Fields {
		// 跳过隐藏字段
		if field.Hidden {
			continue
		}

		fieldConfig := VueFormFieldConfig{
			Prop:         field.Path,
			Label:        field.Name,
			Type:         normalizeVueFieldTypeForRender(field.ValueType),
			Required:     field.Required,
			ReadOnly:     field.ReadOnly,
			Disabled:     field.Disabled,
			Hidden:       field.Hidden,
			Placeholder:  "请输入" + field.Name,
			DefaultValue: field.DefaultValue,
			Dependencies: dependencies[field.Path],
		}

		// 转换选项
		if len(field.Options) > 0 {
			fieldConfig.Options = make([]VueFormFieldOption, 0, len(field.Options))
			for _, option := range field.Options {
				fieldConfig.Options = append(fieldConfig.Options, VueFormFieldOption{
					Label: option.Label,
					Value: option.Value,
				})
			}
		}

		// 设置占位符
		if fieldConfig.Type == "select" || fieldConfig.Type == "checkbox" {
			fieldConfig.Placeholder = "请选择" + field.Name
		} else if fieldConfig.Type == "date" {
			fieldConfig.Placeholder = "请选择日期"
		}

		config.Fields = append(config.Fields, fieldConfig)

		// 生成验证规则
		rules := buildFieldRules(field)
		if len(rules) > 0 {
			config.Rules[field.Path] = rules
		}
	}

	return config
}

// buildDependencyMap 构建字段依赖映射。
func buildDependencyMap(dependencies []target.VueCustomDependencyRule) map[string][]string {
	result := make(map[string][]string)
	for _, dep := range dependencies {
		if dep.Kind == "assignment" {
			result[dep.Field] = append(result[dep.Field], dep.Depends...)
		}
	}
	return result
}

// buildFieldRules 构建字段验证规则。
func buildFieldRules(field target.VueCustomFieldRule) []VueFormFieldRule {
	rules := make([]VueFormFieldRule, 0)

	// 必填规则
	if field.Required {
		trigger := "blur"
		if field.ValueType == "select" || field.ValueType == "checkbox" {
			trigger = "change"
		}

		rules = append(rules, VueFormFieldRule{
			Required: true,
			Message:  field.Name + "不能为空",
			Trigger:  trigger,
		})
	}

	// 类型规则
	if field.ValueType == "number" {
		rules = append(rules, VueFormFieldRule{
			Type:    "number",
			Message: field.Name + "必须为数字",
			Trigger: "blur",
		})
	} else if field.ValueType == "date" {
		rules = append(rules, VueFormFieldRule{
			Type:    "date",
			Message: field.Name + "必须为日期",
			Trigger: "change",
		})
	} else if field.ValueType == "checkbox" {
		rules = append(rules, VueFormFieldRule{
			Type:    "array",
			Message: field.Name + "必须为数组",
			Trigger: "change",
		})
	}

	// 格式规则
	if field.Format == "email" {
		rules = append(rules, VueFormFieldRule{
			Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			Message: field.Name + "格式不正确",
			Trigger: "blur",
		})
	} else if field.Format == "pattern" {
		rules = append(rules, VueFormFieldRule{
			Message: field.Name + "格式不正确",
			Trigger: "blur",
		})
	}

	return rules
}

// normalizeVueFieldTypeForRender 将 Vue 字段类型转换为前端渲染类型。
func normalizeVueFieldTypeForRender(valueType string) string {
	switch valueType {
	case "number":
		return "number"
	case "date":
		return "date"
	case "select":
		return "select"
	case "checkbox":
		return "checkbox"
	case "file":
		return "file"
	case "runtime":
		return "input" // 运行时类型默认为输入框
	default:
		return "input"
	}
}
