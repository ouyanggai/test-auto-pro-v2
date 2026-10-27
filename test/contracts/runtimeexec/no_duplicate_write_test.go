// T010A 无重复写出口契约：静态扫描 runtime 侧源码，确保八个 Go 直连端点绝不出现；
// 登记层面 runtime 动作与 Go 直连动作端点集合互斥。
package runtimeexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
	"test-auto-pro-v2/internal/runtimeexec"
)

// goDirectEndpointLiterals 是八个 Go 直连动作的端点字面量；
// runtime 执行侧（executor TS 与页面桥接）绝不允许出现这些字符串。
var goDirectEndpointLiterals = []string{
	target.WriteEndpointUpdateFlowProxy,
	target.WriteEndpointApproverAppend,
	target.WriteEndpointRollBack,
	target.WriteEndpointRetrieve,
	target.WriteEndpointRevocation,
	target.WriteEndpointUrge,
	target.WriteEndpointFlowTracking,
}

// runtimeSources 返回独立执行服务源码文件清单。
func runtimeSources(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	patterns := []string{
		filepath.Join(root, "executor-service", "src", "*.ts"),
	}

	var files []string
	for _, pattern := range patterns {
		matched, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("扫描模式非法 %s：%v", pattern, err)
		}
		files = append(files, matched...)
	}
	if len(files) == 0 {
		t.Fatal("未扫描到 runtime 侧源码文件")
	}
	return files
}

// TestRuntimeSourcesNeverSendGoDirectEndpoints 静态扫描：Go 直连端点不得出现在 runtime 源码中。
// 出现即意味着同一动作存在两条写出口（runtime 重放或绕过唯一出口）。
func TestRuntimeSourcesNeverSendGoDirectEndpoints(t *testing.T) {
	for _, file := range runtimeSources(t) {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读取源码失败 %s：%v", file, err)
		}
		for _, endpoint := range goDirectEndpointLiterals {
			if strings.Contains(string(content), endpoint) {
				t.Fatalf("%s 出现了 Go 直连端点 %s：runtime 不得直接发送该接口", filepath.Base(file), endpoint)
			}
		}
	}
}

// TestRuntimeRegistryDisjointFromGoDirect 登记层面：runtime 动作的写端点集合与 Go 直连端点互斥。
func TestRuntimeRegistryDisjointFromGoDirect(t *testing.T) {
	goDirectSet := map[string]bool{}
	for _, endpoint := range runtimeexec.GoDirectEndpoints {
		goDirectSet[endpoint] = true
	}
	for _, action := range runtimeexec.RuntimeActionNames() {
		entry, err := runtimeexec.EntryOf(action)
		if err != nil {
			t.Fatalf("登记读取失败：%v", err)
		}
		for _, endpoint := range entry.WriteEndpoints {
			if goDirectSet[endpoint] {
				t.Fatalf("runtime 动作 %s 与 Go 直连端点 %s 冲突", action, endpoint)
			}
		}
	}
}

// TestMatrixCoversAllWriteEndpoints 协议矩阵覆盖全部已登记写端点（含转发附言），
// 保证任何一条写路径都有逐字段存在性登记，不允许未经矩阵校验的写出口。
func TestMatrixCoversAllWriteEndpoints(t *testing.T) {
	matrix := target.ProtocolMatrix()
	required := append([]string{
		target.WriteEndpointSubmit,
		target.RuntimeSubmitEndpoint,
		target.RuntimeReSubmitEndpoint,
		target.RuntimeAuditEndpoint,
		target.WriteEndpointAudit,
		target.WriteEndpointReSubmit,
		target.WriteEndpointStorageForm,
		target.WriteEndpointTranspond,
		target.FlowPostscriptEndpoint,
	}, goDirectEndpointLiterals...)
	for _, endpoint := range required {
		if _, ok := matrix[endpoint]; !ok {
			t.Fatalf("协议矩阵缺少端点登记：%s", endpoint)
		}
	}
}
