package backend_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/model"
	"test-auto-pro-v2/internal/service"
)

type f010CandidateIssueProvider struct {
	err error
}

// ComponentCandidates 返回指定的真实候选结果或稳定错误分类，供生成降级投影回归使用。
func (p f010CandidateIssueProvider) ComponentCandidates(context.Context, string, string, string) ([]any, error) {
	if p.err != nil {
		return nil, p.err
	}
	return nil, nil
}

// TestF010GenerationProjectsEmptyCandidateAsFieldIssue 验证无可见候选只形成字段级 partial，不伪造外部对象。
func TestF010GenerationProjectsEmptyCandidateAsFieldIssue(t *testing.T) {
	serviceUnderTest := newF010SlowGenerationService()
	serviceUnderTest.SetComponentCandidateCache(service.NewComponentCandidateCache(f010CandidateIssueProvider{}, 10, time.Minute))
	result, err := serviceUnderTest.GenerateForm(context.Background(), 110, 111, 1, map[string]any{}, nil, false)
	if err != nil {
		t.Fatalf("无候选生成不应变成 HTTP/技术错误：%v", err)
	}
	if result.GenerationState != "partial" || result.Values["project"] != nil {
		t.Fatalf("无候选必须保留安全空值并返回 partial：%+v", result)
	}
	if !hasF010CandidateIssue(result.Issues, "项目", "当前数据源无可用记录") {
		t.Fatalf("缺少无候选字段级原因：%+v", result.Issues)
	}
}

// TestF010GenerationProjectsPermissionIssue 验证权限不足与无记录使用不同公开原因。
func TestF010GenerationProjectsPermissionIssue(t *testing.T) {
	serviceUnderTest := newF010SlowGenerationService()
	serviceUnderTest.SetComponentCandidateCache(service.NewComponentCandidateCache(f010CandidateIssueProvider{err: target.NewError(target.ErrorPermissionDenied, nil)}, 10, time.Minute))
	result, err := serviceUnderTest.GenerateForm(context.Background(), 110, 111, 1, map[string]any{}, nil, false)
	if err != nil {
		t.Fatalf("权限降级不应变成 HTTP/技术错误：%v", err)
	}
	if !hasF010CandidateIssue(result.Issues, "项目", "当前账号无权读取该数据源候选") {
		t.Fatalf("权限原因未按字段投影：%+v", result.Issues)
	}
	if strings.Contains(f010GenerationIssueText(result.Issues), "当前数据源无可用记录") {
		t.Fatalf("权限不足不应误报成无记录：%+v", result.Issues)
	}
}

// hasF010CandidateIssue 查找字段和稳定原因，避免测试绑定内部接口地址或原始响应。
func hasF010CandidateIssue(issues []model.PathFormGenerationIssue, field, reason string) bool {
	for _, issue := range issues {
		if issue.Field == field && issue.Reason == reason && !issue.Blocking {
			return true
		}
	}
	return false
}
