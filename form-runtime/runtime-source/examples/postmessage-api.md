# postMessage 协议文档

## 概述

流程应用 (rsh-flow-components) 通过 iframe 嵌入宿主应用。两者之间通过 `window.postMessage` 进行双向通信。

**当前实际使用方式**：宿主应用（Vue 3 新平台）通过 URL 参数传递 SID/customerCode，不依赖 postMessage 进行认证。postMessage 主要用于流程操作完成后的事件回调。

## 消息格式

所有消息为 JSON 对象，必须包含 `type` 字段。

## Parent → iframe 消息

### RSH_FLOW_AUTH
注入认证信息（可选，URL 参数已包含认证信息时无需发送）。

```json
{
  "type": "RSH_FLOW_AUTH",
  "sid": "用户会话ID",
  "userData": {
    "customerCode": "客户代码",
    "userId": "用户ID",
    "userName": "用户名"
  }
}
```

### RSH_FLOW_CONFIG
注入运行时配置（可选，用于覆盖 env.js 中的默认值）。

```json
{
  "type": "RSH_FLOW_CONFIG",
  "baseUrl": "http://192.168.1.220:38081/api",
  "viewFileUrl": "http://192.168.1.220:38081",
  "onlyOfficeUrl": "http://192.168.1.218:8085"
}
```

### RSH_FLOW_NAVIGATE
控制 iframe 内路由导航。

```json
{
  "type": "RSH_FLOW_NAVIGATE",
  "path": "/flow/detail/FLOW_INSTANCE_ID",
  "query": { "mode": "audit" }
}
```

## iframe → Parent 消息

### RSH_FLOW_READY
iframe 应用加载完成，已准备好接收消息。

```json
{
  "type": "RSH_FLOW_READY"
}
```

### RSH_FLOW_AUTH_EXPIRED
认证过期（收到 401 响应）。父窗口应重新获取 SID 并刷新 iframe。

```json
{
  "type": "RSH_FLOW_AUTH_EXPIRED"
}
```

### RSH_FLOW_EVENT
流程业务事件通知。FlowDetail.vue 在操作完成后通过此消息通知宿主。

```json
{
  "type": "RSH_FLOW_EVENT",
  "eventName": "flow-action-done",
  "data": {
    "flowInstanceId": "xxx",
    "mode": "audit"
  }
}
```

**eventName 值：**
| 事件名 | 说明 | 触发场景 |
|--------|------|----------|
| `flow-action-done` | 流程操作完成 | FlowDetail.vue 中审批/编辑完成后 callback 触发 |
| `flow-submitted` | 流程发起成功 | 流程发起页面提交后 |
| `flow-approved` | 流程审批完成 | 审核操作完成后 |

## 通信时序（URL 参数模式，当前使用）

```
Parent (Vue 3)                      iframe (rsh-flow-components)
  |                                   |
  |  -- 创建 iframe, URL含sid/cc -->  |  (iframe 加载 FlowDetail.vue)
  |                                   |
  |                                   |  → 从 URL hash 读取参数
  |                                   |  → axios 拦截器追加 SID 到请求
  |                                   |  → $flowDetail() 打开弹窗
  |                                   |
  |  <-- RSH_FLOW_EVENT -----------  |  (操作完成回调)
  |                                   |
  |  -- 关闭 iframe 弹窗 ---------->  |  (parent 控制关闭)
  |                                   |
```

## 通信时序（postMessage 完整模式，预留）

```
Parent                              iframe
  |                                   |
  |  <------ RSH_FLOW_READY --------  |  (iframe 加载完成)
  |                                   |
  |  ------- RSH_FLOW_AUTH -------->  |  (注入 SID + userData)
  |  ------- RSH_FLOW_CONFIG ------>  |  (注入 API 地址)
  |                                   |
  |  ------- RSH_FLOW_NAVIGATE ---->  |  (导航到指定页面)
  |                                   |
  |  <------ RSH_FLOW_EVENT --------  |  (流程操作完成)
  |                                   |
  |  <------ RSH_FLOW_AUTH_EXPIRED -  |  (401 需要重新认证)
  |  ------- RSH_FLOW_AUTH -------->  |  (重新注入 SID)
  |                                   |
```

## 宿主应用关键实现

宿主应用 (Vue 3 PortalView.vue) 的实际集成方式：

1. **构建 iframe URL**：`${FLOW_APP_BASE}/#/flow/detail/${flowInstanceId}?sid=...&platformCode=...&customerCode=...&mode=...`
2. **全屏弹窗**：通过 `<Teleport to="body">` 创建 z-index: 9999 的全屏遮罩 + 96vw x 94vh 的 iframe 容器
3. **关闭弹窗后刷新**：关闭 iframe 弹窗后自动刷新列表数据和各 Tab 计数
