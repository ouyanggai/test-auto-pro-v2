package target_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestSubmittedFlowHandlerParsesUserIds 锁定加签插入节点的处理人解析：目标手动加签生成的 assign
// 审批节点，其 currentAuditUserInfo 的 bizIds/userList 为空，真实处理人只在 userIds 里。
// 解析必须把 userIds 当作用户 ID 收进处理人事实，否则会「当前处理人未能解析出登录账号」而卡死。
func TestSubmittedFlowHandlerParsesUserIds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"isSuccess":true,"data":[{
			"id":"inst-1","flowProxyId":"proxy-1","status":"run","currentNodeProxyId":"node-x",
			"currentAuditUserInfo":{"node-x":{"auditType":"assign","nodeId":"node-x","roleName":null,
				"userIds":["user-added-1"],"bizIds":[],"userList":[]}}
		}]}`))
	}))
	defer server.Close()

	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	facts, err := client.FindSubmittedFlowFacts(context.Background(), target.Session{SID: "sid"}, "inst-1")
	if err != nil {
		t.Fatalf("读取已发实例事实失败：%v", err)
	}
	if !facts.Found || len(facts.Handlers) != 1 {
		t.Fatalf("应解析出一个当前处理人节点：%+v", facts)
	}
	handler := facts.Handlers[0]
	if handler.NodeID != "node-x" {
		t.Fatalf("处理人节点键不正确：%+v", handler)
	}
	found := false
	for _, id := range handler.BizIDs {
		if id == "user-added-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("加签处理人 userIds 未被收进处理人用户 ID，无法解析登录账号：%+v", handler)
	}
}
