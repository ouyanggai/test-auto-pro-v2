package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"test-auto-pro-v2/internal/adapter/target"
)

// TestF018DimensionReadsAgainstRealTarget 用真实目标账号验证对账新增的两个只读维度：
// 已办记录（/web/flowJobTaskLink/list，taskStatus=done）与动作痕迹（/web/flowAuditRecord/list）。
//
// 为什么必须真跑：这两维决定「未生效 → 允许重放」是否成立，而重放会再写一次。
// 只靠源码推断响应形状不够——形状不符时代码会走"读不到 → 缺失降级"，
// 结论回到仍无法判定，虽然安全，但重放链路永远拿不到真实证据。
// 本用例全程只读：只调这两个读端点与一个已发列表读取，不发任何写请求。
func TestF018DimensionReadsAgainstRealTarget(t *testing.T) {
	_, client, session := requireF014Session(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// 从目标自己的已发列表取一条真实实例，避免依赖某个历史实例是否还在。
	submitted, err := client.ListSubmitted(ctx, session, "", 1, 5)
	if err != nil {
		t.Fatalf("读取已发列表失败：%v", err)
	}
	if len(submitted.Items) == 0 {
		t.Fatal("该账号没有已发实例，无法验证对账维度读取；请用有已发数据的账号重跑")
	}
	instance := submitted.Items[0]
	instanceID := strings.TrimSpace(instance.ID)
	if instanceID == "" {
		t.Fatalf("已发列表没有返回实例标识：%+v", instance)
	}
	t.Logf("取用真实实例：id=%s 名称=%s 状态=%s", instanceID, instance.Name, instance.Status)

	// 维度一：已办记录。不带节点标识时回答"这个实例上是否已经有已办"。
	doneAny, err := client.FindDoneTaskOnNode(ctx, session, instanceID, "")
	if err != nil {
		t.Fatalf("已办记录读取失败（对账会因此降级，不能上线）：%v", err)
	}
	t.Logf("已办记录维度：实例级 found=%v", doneAny)

	// 维度二：动作痕迹。审核记录是流程日志同源读取，条数进对账依据说明。
	traceAny, traceTotal, err := client.FindAuditTraceOnNode(ctx, session, instanceID, "")
	if err != nil {
		t.Fatalf("审核记录读取失败（对账会因此降级，不能上线）：%v", err)
	}
	t.Logf("动作痕迹维度：实例级 found=%v 该实例审核记录条数=%d", traceAny, traceTotal)

	// 一条已发实例至少应当有发起这条审核记录；条数为 0 说明响应形状或字段名不对，
	// 那正是"源码推断"会踩的坑，必须在这里暴露而不是等到真实对账时静默降级。
	if traceTotal == 0 {
		t.Fatalf("已发实例的审核记录条数为 0，响应形状与推断不符，需要重新勘定 /web/flowAuditRecord/list")
	}

	// 节点过滤必须真的生效：用一个必然不存在的节点标识，两个维度都应当返回未命中而不是报错。
	doneOnFakeNode, err := client.FindDoneTaskOnNode(ctx, session, instanceID, "node-that-does-not-exist")
	if err != nil {
		t.Fatalf("已办记录按节点过滤读取失败：%v", err)
	}
	if doneOnFakeNode {
		t.Fatal("不存在的节点不应命中已办记录，节点过滤没有生效")
	}
	traceOnFakeNode, _, err := client.FindAuditTraceOnNode(ctx, session, instanceID, "node-that-does-not-exist")
	if err != nil {
		t.Fatalf("审核记录按节点过滤读取失败：%v", err)
	}
	if traceOnFakeNode {
		t.Fatal("不存在的节点不应命中动作痕迹，节点过滤没有生效")
	}

	// 空实例标识按"没有事实"处理，不发请求也不报错（对账收集器会因此标缺失）。
	if found, err := client.FindDoneTaskOnNode(ctx, session, "", ""); err != nil || found {
		t.Fatalf("空实例标识应当直接返回未命中：found=%v err=%v", found, err)
	}
}

// TestF018AuditTraceMatchesRealNode 在真实数据上验证动作痕迹的节点过滤确实按节点区分：
// 取审核记录里真实出现过的节点标识，必须命中；这是「未生效」判据的正向证据。
func TestF018AuditTraceMatchesRealNode(t *testing.T) {
	clientConfig, client, session := requireF014Session(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	submitted, err := client.ListSubmitted(ctx, session, "", 1, 5)
	if err != nil {
		t.Fatalf("读取已发列表失败：%v", err)
	}
	if len(submitted.Items) == 0 {
		t.Fatal("该账号没有已发实例，无法验证节点级动作痕迹")
	}
	instanceID := strings.TrimSpace(submitted.Items[0].ID)

	// 直接读一次原始响应，取出目标真实返回的 flowNodeProxyId，再用它回过头验证过滤命中。
	status, body, err := f014Post(clientConfig.BaseURL, "/web/flowAuditRecord/list", session.SID,
		map[string]any{"data": map[string]any{"flowInstanceId": instanceID}}, clientConfig.Timeout)
	if err != nil {
		t.Fatalf("原始审核记录读取失败：%v", err)
	}
	if status != 200 {
		t.Fatalf("审核记录端点返回非 200：%d %s", status, body)
	}
	nodeID := firstAuditRecordNodeID(body)
	if nodeID == "" {
		t.Fatalf("审核记录响应里没有 flowNodeProxyId 字段，需重新勘定字段名：%s", truncateBody(body))
	}
	t.Logf("真实审核记录节点标识：%s", nodeID)

	found, total, err := client.FindAuditTraceOnNode(ctx, session, instanceID, nodeID)
	if err != nil {
		t.Fatalf("按真实节点读取动作痕迹失败：%v", err)
	}
	if !found {
		t.Fatalf("审核记录里出现过的节点必须命中动作痕迹（该实例共 %d 条）", total)
	}
	t.Logf("节点级动作痕迹命中，实例审核记录共 %d 条", total)
}

// TestF018DoneRecordMatchesRealDoneTask 在真实数据上验证已办记录维度的正向命中：
// 从目标自己的已办列表取一条真实记录，用它的实例与节点回过头读，必须命中。
// 这是「未生效 → 允许重放」判据的正向证据：只有 done 过滤值与字段名都对，这一维才算真的读到了。
func TestF018DoneRecordMatchesRealDoneTask(t *testing.T) {
	clientConfig, client, session := requireF014Session(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	status, body, err := f014Post(clientConfig.BaseURL, "/web/flowJobTaskLink/list", session.SID, map[string]any{
		"data": map[string]any{
			"taskStatus":                   "done",
			"auditWayList":                 []string{},
			"useScope":                     "invest",
			"flowInstanceBizRelevance":     map[string]any{},
			"flowInstanceBizRelevanceList": []any{},
		},
		"pagination": true, "pages": 1, "size": 5,
	}, clientConfig.Timeout)
	if err != nil {
		t.Fatalf("已办列表原始读取失败：%v", err)
	}
	if status != 200 {
		t.Fatalf("已办列表返回非 200：%d %s", status, truncateBody(body))
	}
	instanceID := firstJSONStringField(body, "flowInstanceId")
	nodeID := firstJSONStringField(body, "flowNodeProxyId")
	if instanceID == "" {
		t.Fatalf("该账号没有已办记录，或响应字段名与推断不符，需重新勘定：%s", truncateBody(body))
	}
	t.Logf("真实已办记录：实例=%s 节点=%s", instanceID, nodeID)

	found, err := client.FindDoneTaskOnNode(ctx, session, instanceID, "")
	if err != nil {
		t.Fatalf("已办记录读取失败：%v", err)
	}
	if !found {
		t.Fatal("目标自己的已办列表里出现过的实例必须命中已办记录维度")
	}
	if nodeID != "" {
		onNode, err := client.FindDoneTaskOnNode(ctx, session, instanceID, nodeID)
		if err != nil {
			t.Fatalf("按真实节点读取已办记录失败：%v", err)
		}
		if !onNode {
			t.Fatal("已办记录里出现过的节点必须命中，节点过滤口径不对")
		}
	}
}

// firstJSONStringField 从原始响应文本里取第一个指定字段的字符串值；取不到返回空串。
// 刻意按原始文本取：验证的就是"目标真的返回了这个字段名"，不经结构体解码掩盖字段名差异。
func firstJSONStringField(body, field string) string {
	key := `"` + field + `":"`
	index := strings.Index(body, key)
	if index < 0 {
		return ""
	}
	rest := body[index+len(key):]
	end := strings.Index(rest, `"`)
	if end <= 0 {
		return ""
	}
	return rest[:end]
}

// firstAuditRecordNodeID 从原始响应文本里取第一个 flowNodeProxyId 值；解析失败返回空串。
// 这里刻意按原始文本取，验证的就是"目标真的返回了这个字段名"。
func firstAuditRecordNodeID(body string) string {
	return firstJSONStringField(body, "flowNodeProxyId")
}

// truncateBody 截断响应正文用于失败说明，避免把超长正文刷进测试输出。
func truncateBody(body string) string {
	if len(body) <= 600 {
		return body
	}
	return body[:600] + "……（已截断）"
}

// 保持与既有只读集成测试同一套目标类型引用。
var _ = target.Session{}
