package target_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestFindTaskSnapshotUsesProtocolTaskIdentity 锁定任务链接读取使用目标协议字段，
// 审批动作必须拿到 jobTaskId 和当前批次，不能把关联表 id 当作任务 ID。
func TestFindTaskSnapshotUsesProtocolTaskIdentity(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("请求正文解析失败：%v", err)
		}
		requests = append(requests, body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"isSuccess":true,"data":[{"id":"row-id","jobTaskId":"job-task-7","flowInstanceId":"instance-7","flowNodeProxyId":"node-7","batchNo":"batch-3","taskStatus":"pending","executorId":"executor-7","formProxyId":"form-7","flowProxyId":"proxy-7","auditWay":"single","flowNextNodeAuditType":"person"}]}`))
	}))
	defer server.Close()

	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	snapshot, err := client.FindTaskSnapshot(context.Background(), target.Session{SID: "sid-7"}, "instance-7", "node-7", "pending")
	if err != nil {
		t.Fatalf("读取待办任务快照失败：%v", err)
	}
	if snapshot.JobTaskID != "job-task-7" || snapshot.JobTaskID == "row-id" {
		t.Fatalf("必须使用 jobTaskId，实际 %+v", snapshot)
	}
	if snapshot.BatchNo != "batch-3" || snapshot.FormProxyID != "form-7" || snapshot.FlowProxyID != "proxy-7" {
		t.Fatalf("任务快照身份字段不完整：%+v", snapshot)
	}
	if len(requests) != 1 {
		t.Fatalf("应只发起一次读取请求，实际 %d 次", len(requests))
	}
	data, _ := requests[0]["data"].(map[string]any)
	if data["taskStatus"] != "pending" || data["flowInstanceId"] != "instance-7" {
		t.Fatalf("待办读取参数错误：%v", data)
	}
}

// TestFindTaskSnapshotScopesDoneTaskToExecutor 锁定已办读取必须带上实际执行人，
// 防止同一实例和节点存在历史任务时误取他人的任务批次。
func TestFindTaskSnapshotScopesDoneTaskToExecutor(t *testing.T) {
	var requestData map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("请求正文解析失败：%v", err)
		}
		requestData, _ = body["data"].(map[string]any)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"isSuccess":true,"data":[{"jobTaskId":"done-task","flowInstanceId":"instance-8","flowNodeProxyId":"node-8","batchNo":"batch-8","executorId":"user-8"}]}`))
	}))
	defer server.Close()

	client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("创建目标客户端失败：%v", err)
	}
	snapshot, err := client.FindTaskSnapshot(context.Background(), target.Session{SID: "sid-8", UserID: "user-8"}, "instance-8", "node-8", "done")
	if err != nil || snapshot.JobTaskID != "done-task" {
		t.Fatalf("读取已办任务快照失败：snapshot=%+v err=%v", snapshot, err)
	}
	if requestData["taskStatus"] != "done" || requestData["executorId"] != "user-8" {
		t.Fatalf("已办读取必须限定执行人：%v", requestData)
	}
}

// TestFindTaskSnapshotRejectsAmbiguousOrIncompleteTask 锁定不完整和重复任务不能静默选一条，
// 否则后续写动作可能落到错误任务，且无法安全对账。
func TestFindTaskSnapshotRejectsAmbiguousOrIncompleteTask(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "missing jobTaskId", data: `[{"id":"row-id","flowInstanceId":"instance-9","flowNodeProxyId":"node-9"}]`},
		{name: "multiple tasks", data: `[{"jobTaskId":"task-a","flowInstanceId":"instance-9","flowNodeProxyId":"node-9"},{"jobTaskId":"task-b","flowInstanceId":"instance-9","flowNodeProxyId":"node-9"}]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"isSuccess":true,"data":` + test.data + `}`))
			}))
			defer server.Close()

			client, err := target.NewClient(target.ClientConfig{BaseURL: server.URL, Timeout: time.Second})
			if err != nil {
				t.Fatalf("创建目标客户端失败：%v", err)
			}
			_, err = client.FindTaskSnapshot(context.Background(), target.Session{SID: "sid-9"}, "instance-9", "node-9", "pending")
			if err == nil {
				t.Fatalf("%s 必须返回协议错误", test.name)
			}
		})
	}
}
