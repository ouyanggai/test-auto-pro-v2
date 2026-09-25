package executor

import (
	"testing"

	"test-auto-pro-v2/internal/engine/step"
)

// TestDeepEqualNormalizedSelectorJSON 验证选择器 JSON 在目标平台只返回/保存 id 时与携带 name 的原值等价。
func TestDeepEqualNormalizedSelectorJSON(t *testing.T) {
	written := `{"id":"6e1235cf416e4143947311c6cc93d78e","name":"广东斯能投资有限责任公司"}`
	actual := `{"id":"6e1235cf416e4143947311c6cc93d78e"}`

	// 1. 发送带 name，目标返回瘦 id 对象，应判定为等价
	if !step.DeepEqualNormalized(written, actual) {
		t.Fatalf("期望 written 与 actual 判定为等价，但判定为不等")
	}

	// 2. 发送不带 name，目标返回带 name，应判定为等价
	if !step.DeepEqualNormalized(actual, written) {
		t.Fatalf("期望 actual 与 written 反向也判定为等价")
	}

	// 3. 两者 id 不等，必须判定为不相等
	diffID := `{"id":"other_id","name":"广东斯能投资有限责任公司"}`
	if step.DeepEqualNormalized(written, diffID) {
		t.Fatalf("不同 id 不得判定为等价")
	}

	// 4. 两者均有 name 但 name 不等，必须判定为不相等
	diffName := `{"id":"6e1235cf416e4143947311c6cc93d78e","name":"其他公司"}`
	if step.DeepEqualNormalized(written, diffName) {
		t.Fatalf("相同 id 但不同 name 不得判定为等价")
	}

	// 5. 目标返回带 name:"" 空字符串的瘦对象，应判定为等价
	emptyNameActual := `{"id":"6e1235cf416e4143947311c6cc93d78e","name":""}`
	if !step.DeepEqualNormalized(written, emptyNameActual) {
		t.Fatalf("发送带 name，目标返回 name 为空串，应判定为等价")
	}
	if !step.DeepEqualNormalized(emptyNameActual, written) {
		t.Fatalf("发送 name 为空串，目标返回完整 name，反向应判定为等价")
	}

	// 6. 普通 JSON 字符串正常比对
	if !step.DeepEqualNormalized("abc", "abc") {
		t.Fatalf("普通字符串相等测试失败")
	}
	if step.DeepEqualNormalized("abc", "def") {
		t.Fatalf("普通字符串不等测试失败")
	}
}
