// T010 动作入口契约：七个 runtime 动作的宿主与目标入口唯一登记、
// 写端点按序唯一、转发复合动作双写、Go 直连端点绝不进入 runtime 登记。
package formruntime

import (
	"sort"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/runtimeexec"
)

// TestRuntimeActionEntries 七个业务动作必须逐一登记且入口唯一。
func TestRuntimeActionEntries(t *testing.T) {
	for _, action := range []string{"save_draft", "submit", "storage_form_data", "approve", "reject", "resubmit", "forward"} {
		entry, err := runtimeexec.EntryOf(action)
		if err != nil {
			t.Fatalf("动作 %s 未登记：%v", action, err)
		}
		if strings.TrimSpace(entry.EntryMethod) == "" {
			t.Fatalf("动作 %s 缺少目标入口方法", action)
		}
		if len(entry.WriteEndpoints) == 0 {
			t.Fatalf("动作 %s 缺少写端点登记", action)
		}
	}
}

// TestForwardIsTheOnlyCompositeAction 转发是唯一复合动作：固定两个写请求
// （先建辅助实例、成功后保存附言）；其余动作都只允许一个写端点。
func TestForwardIsTheOnlyCompositeAction(t *testing.T) {
	expected := map[string]string{
		"save_draft":        "/web/flowInstanceApi/submit",
		"submit":            target.RuntimeSubmitEndpoint,
		"storage_form_data": "/web/flowInstanceApi/storageFormData",
		"approve":           target.RuntimeAuditEndpoint,
		"reject":            target.RuntimeAuditEndpoint,
		"resubmit":          target.RuntimeReSubmitEndpoint,
	}
	for _, action := range runtimeexec.RuntimeActionNames() {
		entry, err := runtimeexec.EntryOf(action)
		if err != nil {
			t.Fatalf("登记读取失败：%v", err)
		}
		if action == "forward" {
			if len(entry.WriteEndpoints) != 2 {
				t.Fatalf("转发必须登记两个写端点（主请求+附言），实际 %v", entry.WriteEndpoints)
			}
			if entry.WriteEndpoints[0] != "/web/flowInstanceApi/transpond" || entry.WriteEndpoints[1] != "/web/flowInstancePostscript/save" {
				t.Fatalf("转发写序必须是 transpond → postscript：%v", entry.WriteEndpoints)
			}
			continue
		}
		if action == "choice_continue" {
			continue
		}
		if len(entry.WriteEndpoints) != 1 {
			t.Fatalf("普通动作 %s 只允许一个写端点，实际 %v", action, entry.WriteEndpoints)
		}
		if expected[action] != "" && entry.WriteEndpoints[0] != expected[action] {
			t.Fatalf("动作 %s 写端点应为 %s，实际 %v", action, expected[action], entry.WriteEndpoints)
		}
	}
}

// TestGoDirectEndpointsNeverInRuntimeRegistry Go 直连动作的端点绝不允许出现在 runtime 登记中，
// 这是「同一动作一次执行只能选择一个入口」的静态保证。
func TestGoDirectEndpointsNeverInRuntimeRegistry(t *testing.T) {
	for _, action := range runtimeexec.RuntimeActionNames() {
		entry, err := runtimeexec.EntryOf(action)
		if err != nil {
			t.Fatalf("登记读取失败：%v", err)
		}
		for _, endpoint := range entry.WriteEndpoints {
			for _, goDirect := range runtimeexec.GoDirectEndpoints {
				if endpoint == goDirect {
					t.Fatalf("runtime 动作 %s 登记了 Go 直连端点 %s", action, endpoint)
				}
			}
		}
	}
}

// TestSelectionClosedLoopContinuation 提交/重提/同意必须登记选择闭环继续入口，
// 且继续入口与原动作同一宿主链（不由 Go 另发接口绕过表单前后置代码）。
func TestSelectionClosedLoopContinuation(t *testing.T) {
	for _, action := range []string{"submit", "resubmit", "approve"} {
		entry, err := runtimeexec.EntryOf(action)
		if err != nil {
			t.Fatalf("动作 %s 未登记：%v", action, err)
		}
		if strings.TrimSpace(entry.ContinueMethod) == "" {
			t.Fatalf("动作 %s 缺少选择闭环继续入口", action)
		}
		for _, method := range []string{"handleSaveBranchChooseNode", "handleSaveParallelChooseNode", "getSelectPerson"} {
			if !strings.Contains(entry.ContinueMethod, method) {
				t.Fatalf("动作 %s 的继续入口缺少 %s", action, method)
			}
		}
	}
}

// TestEntryOfRejectsUnknown 未登记动作（含 Go 直连动作键）必须被拒绝。
func TestEntryOfRejectsUnknown(t *testing.T) {
	for _, action := range []string{"add_sign", "transfer", "rollback_previous", "retrieve", "withdraw", "urge", "follow", "unfollow", "system_automatic", "unknown"} {
		if _, err := runtimeexec.EntryOf(action); err == nil {
			t.Fatalf("动作 %s 不应出现在 runtime 登记", action)
		}
	}
	names := runtimeexec.RuntimeActionNames()
	sort.Strings(names)
	if len(names) != 8 {
		t.Fatalf("runtime 登记应为 7 个业务动作 + choice_continue，实际 %v", names)
	}
}
