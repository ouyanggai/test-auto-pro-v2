package history_replay_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/formdata/branchoverlay"
	"test-auto-pro-v2/internal/jsonvalues"
)

// historyFormDataJSON 模拟目标接口返回的原始表单正文：既有小数字面量，也有整数、指数与嵌套结构。
const historyFormDataJSON = `{
  "amount": 128.0,
  "count": 128,
  "ratio": 1.500,
  "scientific": 1.28E2,
  "invoice": {"amount": 99.90},
  "rows": [{"amount": 10.50}]
}`

// TestHistoryFormDataDecodePreservesNumberLiterals 证明目标原始表单数据的数字字面量按 fastjson/BigDecimal 语义原样保留。
//
// 目标 Java 用 fastjson 解析表单正文，128.0 成为 BigDecimal("128.0")（小数位 1），
// 128 成为 Integer（小数位 0）。Go 默认解码会把两者都变成 float64 128，
// 直接改变 JudgeEnum.eq 的 BigDecimal.equals 结果。
func TestHistoryFormDataDecodePreservesNumberLiterals(t *testing.T) {
	values, err := jsonvalues.DecodeObject([]byte(historyFormDataJSON))
	if err != nil {
		t.Fatalf("解码目标原始表单数据失败：%v", err)
	}
	expected := map[string]json.Number{
		"amount":     json.Number("128.0"),
		"count":      json.Number("128"),
		"ratio":      json.Number("1.500"),
		"scientific": json.Number("1.28E2"),
	}
	for field, want := range expected {
		got, ok := values[field].(json.Number)
		if !ok {
			t.Fatalf("字段 %s 类型 = %T，期望 json.Number 以保留字面量", field, values[field])
		}
		if got != want {
			t.Fatalf("字段 %s 字面量 = %q，期望 %q", field, got, want)
		}
	}
	invoice, ok := values["invoice"].(map[string]any)
	if !ok {
		t.Fatalf("嵌套对象类型 = %T，期望 map[string]any", values["invoice"])
	}
	if got, ok := invoice["amount"].(json.Number); !ok || got != json.Number("99.90") {
		t.Fatalf("嵌套字面量 = %#v，期望 json.Number(\"99.90\")", invoice["amount"])
	}
	rows, ok := values["rows"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("子表类型 = %T，期望单行数组", values["rows"])
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("子表行类型 = %T，期望 map[string]any", rows[0])
	}
	if got, ok := row["amount"].(json.Number); !ok || got != json.Number("10.50") {
		t.Fatalf("子表字面量 = %#v，期望 json.Number(\"10.50\")", row["amount"])
	}
}

// TestHistoryFormDataConditionMatchesJavaScale 从目标 JSON 文本直接驱动条件求值，覆盖 eq 的小数位语义与比较类操作符的不敏感性。
func TestHistoryFormDataConditionMatchesJavaScale(t *testing.T) {
	values, err := jsonvalues.DecodeObject([]byte(historyFormDataJSON))
	if err != nil {
		t.Fatalf("解码目标原始表单数据失败：%v", err)
	}
	tests := []struct {
		name      string
		field     string
		valueB    string
		judge     string
		satisfied bool
	}{
		{name: "eq-同字面量命中", field: "amount", valueB: "128.0", judge: "eq", satisfied: true},
		{name: "eq-小数位不同不命中", field: "amount", valueB: "128", judge: "eq", satisfied: false},
		{name: "eq-整数字面量不匹配小数位", field: "count", valueB: "128.0", judge: "eq", satisfied: false},
		{name: "eq-整数字面量命中", field: "count", valueB: "128", judge: "eq", satisfied: true},
		{name: "eq-多余零参与小数位", field: "ratio", valueB: "1.5", judge: "eq", satisfied: false},
		{name: "eq-保留原小数位命中", field: "ratio", valueB: "1.500", judge: "eq", satisfied: true},
		{name: "eq-指数字面量按指数折算小数位", field: "scientific", valueB: "128", judge: "eq", satisfied: true},
		{name: "eq-指数字面量小数位不为原文位数", field: "scientific", valueB: "128.00", judge: "eq", satisfied: false},
		{name: "eq-嵌套字段命中", field: "invoice.amount", valueB: "99.90", judge: "eq", satisfied: true},
		{name: "eq-子表字段命中", field: "rows[].amount", valueB: "10.50", judge: "eq", satisfied: true},
		{name: "neq-小数位不同视为不等", field: "amount", valueB: "128", judge: "neq", satisfied: true},
		{name: "gt-比较忽略小数位", field: "amount", valueB: "127.99", judge: "gt", satisfied: true},
		{name: "gte-比较忽略小数位", field: "amount", valueB: "128", judge: "gte", satisfied: true},
		{name: "lte-比较忽略小数位", field: "amount", valueB: "128.000", judge: "lte", satisfied: true},
		{name: "lt-比较忽略小数位", field: "amount", valueB: "128.0", judge: "lt", satisfied: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			condition := target.FlowCondition{FieldA: test.field, ValueB: test.valueB, Judge: test.judge}
			got := branchoverlay.EvaluateCondition(condition, values)
			if !got.Evaluable {
				t.Fatalf("条件不可求值：%#v", got)
			}
			if got.Satisfied != test.satisfied {
				t.Fatalf("求值结果 = %v，期望 %v", got.Satisfied, test.satisfied)
			}
		})
	}
}

// TestHistoryFormDataDefaultDecodeLosesScale 固定住本次修复要防止的回退：默认解码会抹掉小数位并改变 eq 结果。
func TestHistoryFormDataDefaultDecodeLosesScale(t *testing.T) {
	var lossy map[string]any
	if err := json.Unmarshal([]byte(historyFormDataJSON), &lossy); err != nil {
		t.Fatalf("默认解码失败：%v", err)
	}
	if _, isFloat := lossy["amount"].(float64); !isFloat {
		t.Fatalf("默认解码类型 = %T，期望 float64 以说明回退风险", lossy["amount"])
	}
	condition := target.FlowCondition{FieldA: "amount", ValueB: "128.0", Judge: "eq"}
	if lossyResult := branchoverlay.EvaluateCondition(condition, lossy); lossyResult.Satisfied {
		t.Fatalf("默认解码不应命中 128.0，实际 = %#v", lossyResult)
	}
	values, err := jsonvalues.DecodeObject([]byte(historyFormDataJSON))
	if err != nil {
		t.Fatalf("解码目标原始表单数据失败：%v", err)
	}
	if fixed := branchoverlay.EvaluateCondition(condition, values); !fixed.Satisfied {
		t.Fatalf("UseNumber 解码应命中 128.0，实际 = %#v", fixed)
	}
}

// TestHistoryFormDataDeepCopyKeepsLiterals 验证快照深复制与持久化往返都不改变字面量，避免复制环节重新引入 float64。
func TestHistoryFormDataDeepCopyKeepsLiterals(t *testing.T) {
	values, err := jsonvalues.DecodeObject([]byte(historyFormDataJSON))
	if err != nil {
		t.Fatalf("解码目标原始表单数据失败：%v", err)
	}
	copied, err := jsonvalues.DeepCopyObject(values)
	if err != nil {
		t.Fatalf("深复制目标原始表单数据失败：%v", err)
	}
	if !reflect.DeepEqual(values, copied) {
		t.Fatalf("深复制结果与原值不一致：%#v", copied)
	}
	encoded, err := json.Marshal(copied)
	if err != nil {
		t.Fatalf("持久化编码失败：%v", err)
	}
	restored, err := jsonvalues.DecodeObject(encoded)
	if err != nil {
		t.Fatalf("持久化解码失败：%v", err)
	}
	condition := target.FlowCondition{FieldA: "amount", ValueB: "128.0", Judge: "eq"}
	for name, candidate := range map[string]map[string]any{"深复制": copied, "持久化往返": restored} {
		if got := branchoverlay.EvaluateCondition(condition, candidate); !got.Satisfied {
			t.Fatalf("%s后 128.0 条件未命中：%#v", name, got)
		}
	}
}
