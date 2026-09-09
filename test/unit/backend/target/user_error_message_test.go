package target_test

import (
	"errors"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestUserFacingErrorMessagePrefersResponseText 验证页面优先展示目标响应里的原始错误。
func TestUserFacingErrorMessagePrefersResponseText(t *testing.T) {
	message := target.UserFacingErrorMessage(target.WriteResponse{
		Code:    "FLOW_409",
		Message: "当前节点已被其他人处理",
	}, errors.New("内部错误分类"))
	if message != "当前节点已被其他人处理" {
		t.Fatalf("应展示目标原文，实际 %q", message)
	}
}

// TestUserFacingErrorMessageUnwrapsTargetCause 验证没有响应包文案时仍能展示传输层原始错误。
func TestUserFacingErrorMessageUnwrapsTargetCause(t *testing.T) {
	message := target.UserFacingErrorMessage(target.WriteResponse{}, target.NewError(target.ErrorUnavailable, errors.New("connection refused")))
	if message != "connection refused" {
		t.Fatalf("应展开错误原因，实际 %q", message)
	}
}
