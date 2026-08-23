package model

// RuleCompleteness 描述模板规则的完整性级别和分级问题。
type RuleCompleteness struct {
	Blocking  []string `json:"blocking"`  // 阻断生成的严重问题
	Warning   []string `json:"warning"`   // 需处理但不阻断
	Info      []string `json:"info"`      // 仅供参考
	Readiness string   `json:"readiness"` // complete/partial/blocked
}

// RuleReadiness 是规则就绪状态枚举。
const (
	RuleReadinessReady   = "complete" // 规则完整，可直接生成
	RuleReadinessPartial = "partial"  // 规则部分完整，可生成但有限制
	RuleReadinessBlocked = "blocked"  // 规则不完整，无法生成
)

// ClassifyRuleIssues 将问题列表按严重程度分级。
func ClassifyRuleIssues(issues []string) RuleCompleteness {
	result := RuleCompleteness{
		Blocking: []string{},
		Warning:  []string{},
		Info:     []string{},
	}

	for _, issue := range issues {
		severity := classifyIssueSeverity(issue)
		switch severity {
		case "blocking":
			result.Blocking = append(result.Blocking, issue)
		case "warning":
			result.Warning = append(result.Warning, issue)
		case "info":
			result.Info = append(result.Info, issue)
		}
	}

	// complete 只允许没有任何待核对项的规则；未知但未明确阻断的问题也必须进入 partial。
	if len(result.Blocking) > 0 {
		result.Readiness = RuleReadinessBlocked
	} else if len(result.Warning) > 0 {
		result.Readiness = RuleReadinessPartial
	} else {
		result.Readiness = RuleReadinessReady
	}

	return result
}

// classifyIssueSeverity 判断单个问题的严重程度。
func classifyIssueSeverity(issue string) string {
	// 未知页面、脚本和条件形态无法证明真实回填与保存语义，必须阻断生成而不能按名称猜测。
	blockingKeywords := []string{
		"表单渲染协议尚未识别",
		"未知自定义组件：",
		"页面规则尚未完成分析",
		"页面入口尚未识别",
		"auditWay 缺失",
		"动态脚本需要人工核对",
		"业务校验脚本需要人工核对",
		"字段规则尚未完整识别",
		"值形态尚未识别",
		"动态候选来源尚未识别",
		"条件形态",
		"条件规则尚未",
		"流程条件",
		"未识别的比较方式",
		"模板详情读取失败",
		"模板正文无法解析",
		"尚未建立本地模板规则",
		"模板规则不可用",
		"规则数据异常",
		"模板类型缺失",
		"流程编码缺失",
	}

	for _, keyword := range blockingKeywords {
		if contains(issue, keyword) {
			return "blocking"
		}
	}

	// 已识别协议里的可选数据源或错误回显缺口允许安全部分生成，但必须显式进入 partial。
	warningKeywords := []string{
		"数据源无可用记录",
		"字段级错误路径未证明",
		"Vue 请求常量",
		"Java 控制器未识别端点",
		"宿主保存请求常量",
	}

	for _, keyword := range warningKeywords {
		if contains(issue, keyword) {
			return "warning"
		}
	}

	// 新问题默认按需处理，禁止因为分类器尚未登记就误报 complete。
	return "warning"
}

// contains 检查字符串是否包含子串。
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

// findSubstring 简单的子串查找。
func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
