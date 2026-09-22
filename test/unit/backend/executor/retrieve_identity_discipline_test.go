package executor_test

import (
	"context"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/engine/step"
)

// TestRetrieveRejectsNameOnlyWithoutAccountMatch 验证安全纪律：
// 当目标返回的已办行只带姓名（preExecuteName）而不给 executorId 时，
// 严格禁止通过人员目录按姓名模糊反查账号（同名歧义或选错人会导致以他人名义发出取回的严重越权）。
// 必须如实将已办执行人判定为空，由门禁安全阻塞。
func TestRetrieveRejectsNameOnlyWithoutAccountMatch(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{
			instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}},
		},
		// 目标已办行只给姓名，不给 ExecutorID，且审核记录中也没有本节点通过记录
		snapshot: target.TaskSnapshot{
			LinkID:          "link-name-only",
			JobTaskID:       "task-name-only",
			FlowNodeProxyID: "node-audit",
			ExecutorID:      "", // 无 ExecutorID
			ExecutorName:    "刘慧玲",
			TaskStatus:      "done",
		},
		records: []target.AuditRecordSnapshot{}, // 无审核记录
		tree: &target.FlowNodeTemplate{
			ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
				ID: "node-audit", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"},
			},
		},
	}
	runCtx := retrieveNodePassRunContext()
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("取回预览构建失败：finished=%v err=%v preview=%+v", finished, err, preview)
	}

	// 门禁必须阻塞，绝不能因为有姓名就放行取回
	if preview.GateAllowed {
		t.Fatalf("已办行无 executorId 且无审核记录证明时取回门禁严禁放行，实际放行了：%+v", preview)
	}
	// 取回身份必须为空
	if strings.TrimSpace(preview.Facts.CompletedTaskExecutorID) != "" {
		t.Fatalf("取回身份必须为空，绝不能通过姓名反查补身份，实际为：%q", preview.Facts.CompletedTaskExecutorID)
	}
}

// TestRetrieveAcceptsExactAuditIdentityWithoutNameSearch 验证精确身份解析：
// 当审核记录中存在本节点的通过记录时，取回直接使用审核记录锁定的真实执行人 ID，
// 无需且严禁做姓名反查。
func TestRetrieveAcceptsExactAuditIdentityWithoutNameSearch(t *testing.T) {
	targetFake := &taskActionFactsTarget{
		fakeTarget: &fakeTarget{
			instance: fakeTargetView{Found: true, Status: "run", CurrentNodes: []string{"node-current"}},
		},
		snapshot: target.TaskSnapshot{
			LinkID:          "link-exact",
			JobTaskID:       "task-exact",
			FlowNodeProxyID: "node-audit",
			ExecutorID:      "user-exact-id",
			TaskStatus:      "done",
		},
		records: []target.AuditRecordSnapshot{{
			FlowJobTaskID:   "link-exact",
			FlowNodeProxyID: "node-audit",
			ExecutorID:      "user-exact-id",
			AuditStatus:     "pass",
		}},
		tree: &target.FlowNodeTemplate{
			ID: "start", Type: "start", Child: &target.FlowNodeTemplate{
				ID: "node-audit", Type: "common", Child: &target.FlowNodeTemplate{ID: "node-current", Type: "common"},
			},
		},
	}
	runCtx := retrieveNodePassRunContext()
	executor := step.NewExecutor(targetFake, &fakeSessions{}, &fakeRunState{}, &fakeFacts{}, fixedRunConfig(), nil)

	preview, finished, err := executor.BuildPreview(context.Background(), runCtx, 0)
	if err != nil || finished || preview == nil {
		t.Fatalf("取回预览构建失败：finished=%v err=%v preview=%+v", finished, err, preview)
	}

	if !preview.GateAllowed {
		t.Fatalf("有真实审核记录与执行人 ID 时门禁应放行，实际被拒：%+v", preview.GateItems)
	}
	if preview.Facts.CompletedTaskExecutorID != "user-exact-id" {
		t.Fatalf("取回执行人 ID 必须严格等于目标返回的真实 ExecutorID，实际为：%q", preview.Facts.CompletedTaskExecutorID)
	}
}
