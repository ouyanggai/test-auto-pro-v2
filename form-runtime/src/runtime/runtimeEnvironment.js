// 目标 axios 在模块加载时读取 baseUrl；保持为空可让会话请求策略把 /web 请求转向本次核实网关。
export let baseUrl = ''
export let viewFileUrl = ''
export let onlyOfficeUrl = ''
export let wsUrl = ''

// setConfig 只更新非认证运行参数，SID 由独立内存认证适配持有。
export function setConfig (config = {}) {
  if (config.baseUrl) baseUrl = config.baseUrl
  if (config.viewFileUrl) viewFileUrl = config.viewFileUrl
  if (config.onlyOfficeUrl) onlyOfficeUrl = config.onlyOfficeUrl
}
