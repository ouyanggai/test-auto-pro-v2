// T004 契约测试：Go 侧协议校验与共享夹具逐条对照。
// 夹具 test/fixtures/runtimeexec/protocol 同时被 TS 侧
// test/unit/form-runtime/protocol.test.ts 消费，保证两侧校验规则一致。
package formruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"test-auto-pro-v2/internal/runtimeexec"
)

// fixturesDir 返回协议夹具目录；与本文件位置绑定，移动文件时必须同步。
func fixturesDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "fixtures", "runtimeexec", "protocol")
}

// fixtureNames 列出目录下全部夹具文件名（稳定排序），空目录直接判错。
func fixtureNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取夹具目录失败：%v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatalf("夹具目录 %s 为空", dir)
	}
	return names
}

// TestProtocolValidFixtures 全部合法夹具必须通过 ValidateMessage，且字段原样保留。
func TestProtocolValidFixtures(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "valid")
	for _, name := range fixtureNames(t, dir) {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("读取夹具 %s 失败：%v", name, err)
		}
		message, err := runtimeexec.ValidateMessage(raw)
		if err != nil {
			t.Fatalf("合法夹具 %s 被拒绝：%v", name, err)
		}
		if message.Version != runtimeexec.ProtocolVersion {
			t.Fatalf("夹具 %s 版本回读不一致：%s", name, message.Version)
		}
	}
}

// TestProtocolInvalidFixtures 全部非法夹具必须被拒绝，错误码与文件名一致；
// 文件名即期望的稳定错误码（如 version_rejected.json 期望 Code=version_rejected）。
func TestProtocolInvalidFixtures(t *testing.T) {
	dir := filepath.Join(fixturesDir(t), "invalid")
	for _, name := range fixtureNames(t, dir) {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("读取夹具 %s 失败：%v", name, err)
		}
		_, validateErr := runtimeexec.ValidateMessage(raw)
		if validateErr == nil {
			t.Fatalf("非法夹具 %s 通过了校验：%s", name, raw)
		}
		var protocolErr *runtimeexec.ProtocolError
		if !errorsAs(validateErr, &protocolErr) {
			t.Fatalf("夹具 %s 的错误不是 ProtocolError：%v", name, validateErr)
		}
		expectCode := strings.TrimSuffix(name, ".json")
		if protocolErr.Code != expectCode {
			t.Fatalf("夹具 %s 期望错误码 %s，实际 %s", name, expectCode, protocolErr.Code)
		}
	}
}

// TestProtocolEncodeRejectsInvalidMessage 编码器必须复用同一套校验：
// 缺步骤号、旧版本、未知动作、非法结果状态都在发出前被拦截。
func TestProtocolEncodeRejectsInvalidMessage(t *testing.T) {
	cases := []struct {
		name    string
		message runtimeexec.Message
		code    string
	}{
		{
			name:    "缺少步骤编号",
			message: runtimeexec.Message{Kind: runtimeexec.KindLoad, RequestID: "r1", RunID: "run", PathRunID: "pr"},
			code:    "id_missing",
		},
		{
			name:    "load 缺少会话编号",
			message: runtimeexec.Message{Kind: runtimeexec.KindLoad, RequestID: "r1", RunID: "run", PathRunID: "pr", StepID: "s1"},
			code:    "session_missing",
		},
		{
			name: "未知动作",
			message: runtimeexec.Message{Kind: runtimeexec.KindExecute, RequestID: "r1", SessionID: "s", RunID: "run", PathRunID: "pr", StepID: "s1",
				Payload: json.RawMessage(`{"action":"withdraw"}`)},
			code: "action_unknown",
		},
		{
			name: "非法结果状态",
			message: runtimeexec.Message{Kind: runtimeexec.KindResult, RequestID: "r1", SessionID: "s", RunID: "run", PathRunID: "pr", StepID: "s1",
				Payload: json.RawMessage(`{"status":"probably_fine"}`)},
			code: "status_invalid",
		},
	}
	for _, item := range cases {
		_, err := runtimeexec.EncodeMessage(item.message)
		if err == nil {
			t.Fatalf("%s：非法消息通过了编码校验", item.name)
		}
		var protocolErr *runtimeexec.ProtocolError
		if !errorsAs(err, &protocolErr) || protocolErr.Code != item.code {
			t.Fatalf("%s：期望错误码 %s，实际 %v", item.name, item.code, err)
		}
	}
}

// TestProtocolCreateExemptFromSessionID create 是唯一允许没有 sessionId 的入站消息，
// 其余类型一律必须携带，保证会话编号从创建起全程闭环。
func TestProtocolCreateExemptFromSessionID(t *testing.T) {
	raw := []byte(`{"version":"` + runtimeexec.ProtocolVersion + `","kind":"create","requestId":"r1","sessionId":"","runId":"run","pathRunId":"pr","stepId":"s1"}`)
	if _, err := runtimeexec.ValidateMessage(raw); err != nil {
		t.Fatalf("create 消息不应要求 sessionId：%v", err)
	}
}

// errorsAs 包装 errors.As，避免在每个用例里重复导入。
func errorsAs(err error, target *(*runtimeexec.ProtocolError)) bool {
	if protocolErr, ok := err.(*runtimeexec.ProtocolError); ok {
		*target = protocolErr
		return true
	}
	return false
}
