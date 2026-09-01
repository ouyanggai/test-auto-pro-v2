package branchoverlay

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"test-auto-pro-v2/internal/adapter/target"
)

// Evaluation 是一次目标条件计算结果；Evaluable 为 false 时禁止猜测分支结果。
type Evaluation struct {
	Satisfied bool
	Evaluable bool
	Reason    string
}

// EvaluateCondition 严格执行目标 Java JudgeEnum 的十种已核实操作符。
func EvaluateCondition(condition target.FlowCondition, values map[string]any) Evaluation {
	path := strings.TrimSpace(condition.FieldA)
	if path == "" {
		return unevaluable("条件左值字段路径为空")
	}
	left, found := getPath(values, path)
	if !found {
		return unevaluable("条件左值字段缺失")
	}
	judge := normalizeJudge(condition.Judge)
	if !supportedJudge(judge) {
		return unevaluable("目标条件操作符无法证明")
	}

	if judge == "boolean_value" {
		return evaluateBooleanValue(left)
	}
	if judge == "is_not_null" {
		return Evaluation{Satisfied: isNotNullJava(left), Evaluable: true}
	}

	right, rightOK := conditionRightValue(condition, values)
	if !rightOK {
		return unevaluable("条件右值字段缺失")
	}
	switch judge {
	case "lt", "gt", "lte", "gte":
		leftNumber, leftOK := toBigDecimal(left)
		rightNumber, rightOK := toBigDecimal(right)
		if !leftOK || !rightOK {
			return unevaluable("条件数值转换失败")
		}
		comparison := leftNumber.Cmp(rightNumber)
		satisfied := false
		switch judge {
		case "lt":
			satisfied = comparison < 0
		case "gt":
			satisfied = comparison > 0
		case "lte":
			satisfied = comparison <= 0
		case "gte":
			satisfied = comparison >= 0
		}
		return Evaluation{Satisfied: satisfied, Evaluable: true}
	case "eq":
		// Java 的 eq 只有左值属于 Number 时才走 BigDecimal；其他类型使用 equals。
		if isJavaNumber(left) {
			leftNumber, leftScale, leftOK := toBigDecimalWithScale(left)
			rightNumber, rightScale, rightOK := toBigDecimalWithScale(right)
			if !leftOK || !rightOK {
				return unevaluable("条件数值转换失败")
			}
			// 目标代码使用 BigDecimal.equals（而非 compareTo），因此数值和小数位都必须相同。
			return Evaluation{Satisfied: leftNumber.Cmp(rightNumber) == 0 && leftScale == rightScale, Evaluable: true}
		}
		if left == nil || right == nil {
			return Evaluation{Satisfied: false, Evaluable: true}
		}
		return Evaluation{Satisfied: javaEquals(left, right), Evaluable: true}
	case "neq":
		// Java 的 neq 对 null 直接返回 false，不能把缺失值改写为“不相等”。
		if left == nil || right == nil {
			return Evaluation{Satisfied: false, Evaluable: true}
		}
		return Evaluation{Satisfied: !javaEquals(left, right), Evaluable: true}
	case "contains":
		leftText, leftOK := left.(string)
		rightText, rightOK := right.(string)
		if !leftOK || !rightOK {
			return unevaluable("contains 的左右值不是目标字符串")
		}
		// 目标实现调用 bStr.contains(aStr)，方向固定为 B 包含 A。
		return Evaluation{Satisfied: strings.Contains(rightText, leftText), Evaluable: true}
	case "is_update":
		if left == nil || right == nil {
			return unevaluable("is_update 的左右值为空")
		}
		return Evaluation{Satisfied: !javaEquals(left, right), Evaluable: true}
	default:
		return unevaluable("目标条件操作符无法证明")
	}
}

// EvaluateConditions 按目标 isLastResult 保留条件列表和 and/or 的保存顺序。
func EvaluateConditions(conditions []target.FlowCondition, values map[string]any) Evaluation {
	if len(conditions) == 0 {
		return Evaluation{Satisfied: false, Evaluable: true}
	}
	lastResult := false
	var previousResult *bool
	previousType := ""
	for _, condition := range conditions {
		current := EvaluateCondition(condition, values)
		if !current.Evaluable {
			return current
		}
		conditionType := normalizeConditionType(condition.ConditionType)
		if conditionType == "unknown" {
			return unevaluable("目标条件连接符无法证明")
		}
		if conditionType == "" && previousType == "" {
			return Evaluation{Satisfied: current.Satisfied, Evaluable: true}
		}
		switch previousType {
		case "and":
			if previousResult != nil && current.Satisfied && *previousResult && conditionType == "" {
				lastResult = true
			}
			merged := previousResult != nil && current.Satisfied && *previousResult
			previousResult = boolPointer(merged)
		case "or":
			if previousResult != nil && (current.Satisfied || *previousResult) && conditionType == "" {
				lastResult = true
			}
			merged := previousResult != nil && (current.Satisfied || *previousResult)
			previousResult = boolPointer(merged)
		default:
			previousResult = boolPointer(current.Satisfied)
		}
		previousType = conditionType
	}
	return Evaluation{Satisfied: lastResult, Evaluable: true}
}

// conditionRightValue 复刻 Java 对 bvalue 的空白判断和字段右值回退。
func conditionRightValue(condition target.FlowCondition, values map[string]any) (any, bool) {
	if strings.TrimSpace(condition.ValueB) != "" {
		return condition.ValueB, true
	}
	path := strings.TrimSpace(condition.FieldB)
	if path == "" {
		return nil, false
	}
	return getPath(values, path)
}

// evaluateBooleanValue 保留目标 Y/N 校验后与“是”比较的不可满足语义。
func evaluateBooleanValue(value any) Evaluation {
	text, ok := value.(string)
	if !ok || (text != "Y" && text != "N") {
		return unevaluable("boolean_value 仅接受目标 Y/N 值")
	}
	// 目标 Java 当前实现从不返回 true：合法 Y/N 之后仍比较中文“是”。
	return Evaluation{Satisfied: false, Evaluable: true, Reason: "目标 boolean_value 规则不可满足"}
}

// normalizeJudge 只保留目标枚举名称，不接受旧生成器的符号或别名。
func normalizeJudge(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// normalizeConditionType 归一目标条件连接符，未知值保持不可证明。
func normalizeConditionType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "and", "or":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

// supportedJudge 判断操作符是否属于当前目标 Java 已核实集合。
func supportedJudge(value string) bool {
	switch value {
	case "lt", "gt", "lte", "gte", "eq", "neq", "contains", "is_update", "is_not_null", "boolean_value":
		return true
	default:
		return false
	}
}

// isNotNullJava 复刻目标对 null、字符串、数组和 Iterable 的判定边界。
func isNotNullJava(value any) bool {
	if value == nil {
		return false
	}
	if text, ok := value.(string); ok {
		return text != ""
	}
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return false
	}
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		return rv.Len() > 0
	case reflect.Interface, reflect.Pointer:
		return !rv.IsNil()
	default:
		return true
	}
}

// javaEquals 使用目标 equals 的非数值类型行为，避免把 neq 错误归一为数值比较。
func javaEquals(left, right any) bool {
	if left == nil || right == nil {
		return false
	}
	leftType, rightType := reflect.TypeOf(left), reflect.TypeOf(right)
	if leftType != rightType {
		return false
	}
	return reflect.DeepEqual(left, right)
}

// isJavaNumber 判断左值是否触发目标 eq 的 BigDecimal 分支。
func isJavaNumber(value any) bool {
	if value == nil {
		return false
	}
	switch value.(type) {
	case json.Number, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, *big.Int, *big.Float, *big.Rat:
		return true
	default:
		return false
	}
}

// toBigDecimal 把 Go 中的 JSON 数值和目标支持的数字文本转换为精确有理数。
func toBigDecimal(value any) (*big.Rat, bool) {
	if value == nil {
		return nil, false
	}
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case json.Number:
		text = typed.String()
	case int:
		text = strconv.FormatInt(int64(typed), 10)
	case int8:
		text = strconv.FormatInt(int64(typed), 10)
	case int16:
		text = strconv.FormatInt(int64(typed), 10)
	case int32:
		text = strconv.FormatInt(int64(typed), 10)
	case int64:
		text = strconv.FormatInt(typed, 10)
	case uint:
		text = strconv.FormatUint(uint64(typed), 10)
	case uint8:
		text = strconv.FormatUint(uint64(typed), 10)
	case uint16:
		text = strconv.FormatUint(uint64(typed), 10)
	case uint32:
		text = strconv.FormatUint(uint64(typed), 10)
	case uint64:
		text = strconv.FormatUint(typed, 10)
	case float32:
		text = strconv.FormatFloat(float64(typed), 'g', -1, 32)
	case float64:
		text = strconv.FormatFloat(typed, 'g', -1, 64)
	case *big.Int:
		if typed == nil {
			return nil, false
		}
		text = typed.String()
	case *big.Float:
		if typed == nil {
			return nil, false
		}
		text = typed.Text('g', -1)
	case *big.Rat:
		if typed == nil {
			return nil, false
		}
		return new(big.Rat).Set(typed), true
	case fmt.Stringer:
		text = typed.String()
	default:
		return nil, false
	}
	// Java BigDecimal(String) 不接受首尾空白，不能为目标条件悄悄放宽转换边界。
	if text == "" {
		return nil, false
	}
	result, ok := decimalRat(text)
	return result, ok
}

// toBigDecimalWithScale 返回目标 BigDecimal 的精确值和 scale，供 eq 复刻 equals 语义。
func toBigDecimalWithScale(value any) (*big.Rat, int, bool) {
	rat, ok := toBigDecimal(value)
	if !ok {
		return nil, 0, false
	}
	text, textOK := decimalText(value)
	if !textOK {
		return nil, 0, false
	}
	if strings.TrimSpace(text) != text {
		return nil, 0, false
	}
	if strings.HasPrefix(text, "+") || strings.HasPrefix(text, "-") {
		text = text[1:]
	}
	parts := strings.SplitN(strings.ToLower(text), "e", 2)
	scale := 0
	if dot := strings.IndexByte(parts[0], '.'); dot >= 0 {
		scale = len(parts[0]) - dot - 1
	}
	if len(parts) == 2 {
		exponent, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, 0, false
		}
		scale -= exponent
	}
	return rat, scale, true
}

// decimalText 提取目标数字转换使用的十进制文本，拒绝隐式对象格式化。
func decimalText(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case json.Number:
		return typed.String(), true
	case int:
		return strconv.FormatInt(int64(typed), 10), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'g', -1, 32), true
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64), true
	case fmt.Stringer:
		return typed.String(), true
	default:
		return "", false
	}
}

// decimalRat 解析 BigDecimal 接受的十进制及指数文本，保持比较不受 float64 舍入影响。
func decimalRat(text string) (*big.Rat, bool) {
	if strings.ContainsAny(text, "NaNnInf") {
		return nil, false
	}
	if strings.ContainsAny(text, "/") {
		return nil, false
	}
	sign := 1
	if strings.HasPrefix(text, "+") {
		text = text[1:]
	} else if strings.HasPrefix(text, "-") {
		sign = -1
		text = text[1:]
	}
	parts := strings.SplitN(strings.ToLower(text), "e", 2)
	mantissa := parts[0]
	exponent := 0
	if len(parts) == 2 {
		parsed, err := strconv.Atoi(parts[1])
		if err != nil || parsed > 10000 || parsed < -10000 {
			return nil, false
		}
		exponent = parsed
	}
	decimalParts := strings.Split(mantissa, ".")
	if len(decimalParts) > 2 || (decimalParts[0] == "" && (len(decimalParts) == 1 || decimalParts[1] == "")) {
		return nil, false
	}
	whole, fraction := decimalParts[0], ""
	if len(decimalParts) == 2 {
		fraction = decimalParts[1]
	}
	digits := whole + fraction
	if digits == "" {
		return nil, false
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return nil, false
		}
	}
	numerator := new(big.Int)
	if _, ok := numerator.SetString(digits, 10); !ok {
		return nil, false
	}
	if sign < 0 {
		numerator.Neg(numerator)
	}
	scale := len(fraction) - exponent
	if scale <= 0 {
		numerator.Mul(numerator, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-scale)), nil))
		return new(big.Rat).SetInt(numerator), true
	}
	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	return new(big.Rat).SetFrac(numerator, denominator), true
}

// boolPointer 返回供目标顺序聚合使用的独立布尔指针。
func boolPointer(value bool) *bool {
	return &value
}

// unevaluable 构造统一的不可证明结果。
func unevaluable(reason string) Evaluation {
	return Evaluation{Evaluable: false, Reason: reason}
}
