package target_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestRoleNameResolutionUsesNodePlatformCode 锁定固定角色名称解析的平台码语义：
// 固定角色挂在其创建平台码（如 999999）下，而统一网关平台码（如 200001）查不到任何角色；
// 名称解析必须按节点审批配置携带的平台码查询，才能给出可展示名称而不是"读取失败"。
func TestRoleNameResolutionUsesNodePlatformCode(t *testing.T) {
	const (
		roleID       = "e6a1f92f097d4054ada5fdb627059b94"
		roleName     = "印章证照管理员（深圳）"
		customerCode = "d4036cc6581141a8b13cf39387c8d6f2"
	)
	successBody := `{"isSuccess":true,"data":[{"id":"` + roleID + `","name":"` + roleName + `"}]}`
	emptyBody := `{"isSuccess":true,"data":[]}`
	var roleListPlatform atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/web/flowRoleApi/list") {
			roleListPlatform.Store(r.URL.Query().Get("platformCode"))
			w.Header().Set("Content-Type", "application/json")
			// 目标侧按 URL 平台码过滤：只有与角色创建平台一致时才返回角色。
			if r.URL.Query().Get("platformCode") == "999999" {
				_, _ = w.Write([]byte(successBody))
				return
			}
			_, _ = w.Write([]byte(emptyBody))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptyBody))
	}))
	defer server.Close()

	client, err := target.NewClient(target.ClientConfig{
		BaseURL:      server.URL,
		CustomerCode: customerCode,
		PlatformCode: "200001",
		Timeout:      5 * time.Second,
	})
	if err != nil {
		t.Fatalf("构造目标客户端失败：%v", err)
	}

	// 模拟节点审批配置：解析前只有角色 ID，平台码指向角色创建平台。
	config := &target.FlowNodeAuditConfig{
		PlatformCode: "999999",
		AuditType:    "role",
		Details:      []target.FlowAuditDetail{{ID: roleID, Type: "role"}},
	}
	tree := &target.FlowNodeTemplate{
		ID:          "node-a",
		Type:        "common",
		Name:        "印章证照管理员（深圳）",
		AuditConfig: config,
	}
	client.ResolveFlowAuditMetadataForTest(context.Background(), target.Session{
		SID: "session-a", CustomerCode: customerCode, PlatformCode: "200001",
	}, tree)

	if got := roleListPlatform.Load(); got != "999999" {
		t.Fatalf("角色列表没有按节点平台码查询：platform=%v", got)
	}
	if len(config.Details) != 1 || config.Details[0].Name != roleName {
		t.Fatalf("角色名称没有解析出来：%+v", config.Details)
	}
	if len(config.ResolutionIssues) != 0 {
		t.Fatalf("解析成功不应有问题：%+v", config.ResolutionIssues)
	}
}
