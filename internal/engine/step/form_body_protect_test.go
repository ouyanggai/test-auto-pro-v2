package step

import (
	"context"
	"encoding/json"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/runtimeexec"
)

type mockFormTarget struct {
	TargetClient
	currentData map[string]any
}

func (m mockFormTarget) ReadInstanceCurrentData(ctx context.Context, session target.Session, instanceID string) (map[string]any, error) {
	return m.currentData, nil
}

// TestProtectRuntimeFormBodyIgnoresEmptyKeyDifference 验证写前带有空字符串键但无实质业务字段丢失时，不误判为字段减少。
func TestProtectRuntimeFormBodyIgnoresEmptyKeyDifference(t *testing.T) {
	// 写前数据：41个字段，包含一个空键 ""
	beforeData := map[string]any{
		"":              "",
		"contractId":    "123",
		"contractSum":   100,
		"businessType":  "001",
		"initiatorName": "张三",
	}
	// 写后数据：目标后端落库返回的 40 个真实业务字段（去除了空键 ""）
	afterData := map[string]any{
		"contractId":    "123",
		"contractSum":   100,
		"businessType":  "001",
		"initiatorName": "张三",
	}

	summaryBytes, _ := json.Marshal(map[string]any{
		"formBefore":        beforeData,
		"fieldCountBefore":  5, // 原生页面 Object.keys 计数为 5（包含空键）
	})

	executor := &Executor{target: mockFormTarget{currentData: afterData}}
	runCtx := RunContext{
		PathRun: model.PathRun{
			MainInstanceRef: "inst-1",
		},
	}
	step := model.CompiledActionStep{
		Action:   model.ActionStorageFormData,
		Sequence: 8,
	}

	res := executor.protectRuntimeFormBody(
		context.Background(),
		runCtx,
		step,
		target.Session{},
		runtimeexec.ActionRunResult{Status: "success", BusinessSummary: summaryBytes},
		nil,
		1,
	)

	if res.Status != "success" {
		t.Fatalf("有效业务字段完全一致时应保持成功，实际状态：%s，消息：%s", res.Status, res.Message)
	}
}

// TestProtectRuntimeFormBodyDetectsRealMissingField 验证真实业务字段丢失时，能够准确拦截并报告业务失败。
func TestProtectRuntimeFormBodyDetectsRealMissingField(t *testing.T) {
	beforeData := map[string]any{
		"contractId":    "123",
		"contractSum":   100,
		"businessType":  "001",
		"initiatorName": "张三",
	}
	// 写后丢失了 contractSum
	afterData := map[string]any{
		"contractId":    "123",
		"businessType":  "001",
		"initiatorName": "张三",
	}

	summaryBytes, _ := json.Marshal(map[string]any{
		"formBefore":       beforeData,
		"fieldCountBefore": 4,
	})

	executor := &Executor{target: mockFormTarget{currentData: afterData}}
	runCtx := RunContext{
		PathRun: model.PathRun{
			MainInstanceRef: "inst-1",
		},
	}
	step := model.CompiledActionStep{
		Action:   model.ActionStorageFormData,
		Sequence: 8,
	}

	res := executor.protectRuntimeFormBody(
		context.Background(),
		runCtx,
		step,
		target.Session{},
		runtimeexec.ActionRunResult{Status: "success", BusinessSummary: summaryBytes},
		nil,
		1,
	)

	if res.Status != "business_failed" {
		t.Fatalf("业务字段真实丢失时应判定为 business_failed，实际状态：%s", res.Status)
	}
}
