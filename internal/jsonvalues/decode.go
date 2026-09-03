// Package jsonvalues 统一目标表单原始数据的 JSON 解码方式。
//
// 目标 Java 用 fastjson 把表单正文解析成 Map：带小数点或指数的字面量成为 BigDecimal，
// 整数字面量成为 Integer/Long。JudgeEnum 的 eq 使用 BigDecimal.equals，同时比较数值与
// 小数位，因此字面量写法必须原样保留。Go 默认把 JSON 数字解码为 float64，会把 "128.0"
// 变成 128 并丢掉小数位，直接改变条件分支结果。本包所有解码都启用 UseNumber。
package jsonvalues

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// ErrNotObject 表示正文不是 JSON 对象。
var ErrNotObject = errors.New("JSON 正文不是对象")

// Decode 使用 UseNumber 把字节解码到目标结构，保留数字字面量。
func Decode(raw []byte, destination any) error {
	return NewDecoder(bytes.NewReader(raw)).Decode(destination)
}

// NewDecoder 返回启用 UseNumber 的解码器，供请求体等流式场景使用。
func NewDecoder(reader io.Reader) *json.Decoder {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	return decoder
}

// DecodeObject 解码 JSON 对象；数组、标量与 null 都视为非对象。
func DecodeObject(raw []byte) (map[string]any, error) {
	var decoded map[string]any
	if err := Decode(raw, &decoded); err != nil {
		return nil, err
	}
	if decoded == nil {
		return nil, ErrNotObject
	}
	return decoded, nil
}

// DeepCopyObject 深复制目标表单数据：JSON 原生容器逐层复制，字面量类型原样保留。
//
// 不整体重新编解码，避免深复制顺带改变值的 Go 类型：json.Number 保留目标字面量的小数位，
// 调用方自带的原生标量也不会被换成别的数字类型。非 JSON 原生类型仍通过编解码断开引用。
func DeepCopyObject(values map[string]any) (map[string]any, error) {
	if values == nil {
		return map[string]any{}, nil
	}
	copied := make(map[string]any, len(values))
	for key, value := range values {
		duplicated, err := deepCopy(value)
		if err != nil {
			return nil, err
		}
		copied[key] = duplicated
	}
	return copied, nil
}

// DeepCopyValue 深复制任意目标值；无法复制时返回 nil。
func DeepCopyValue(value any) any {
	copied, err := deepCopy(value)
	if err != nil {
		return nil
	}
	return copied
}

// deepCopy 复制单个目标值；复制失败向上返回错误，禁止降级为空值。
func deepCopy(value any) (any, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case bool, string, json.Number,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return typed, nil
	case map[string]any:
		return DeepCopyObject(typed)
	case []any:
		copied := make([]any, len(typed))
		for index, item := range typed {
			duplicated, err := deepCopy(item)
			if err != nil {
				return nil, err
			}
			copied[index] = duplicated
		}
		return copied, nil
	default:
		return copyThroughJSON(typed)
	}
}

// copyThroughJSON 对结构体、指针等非 JSON 原生类型重新编解码，确保引用被断开。
func copyThroughJSON(value any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result any
	if err := Decode(encoded, &result); err != nil {
		return nil, err
	}
	return result, nil
}
