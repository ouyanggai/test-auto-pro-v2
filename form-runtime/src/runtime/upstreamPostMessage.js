// initPostMessage 由本项目版本化会话协议接管；禁用上游通配 origin 和 URL SID 降级入口。
export function initPostMessage () {}

// notifyAuthExpired 保留目标组件调用边界，F-007 工作区由请求错误返回当前父会话。
export function notifyAuthExpired () {}

// notifyFlowEvent 配置阶段不向父页面冒充流程提交或审批事件。
export function notifyFlowEvent () {}

// notifyResize 由 iframe 容器固定尺寸和 ResizeObserver 处理。
export function notifyResize () {}

export const MESSAGE_TYPES = Object.freeze({})
