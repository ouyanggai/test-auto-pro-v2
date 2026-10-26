// nativeAuthQuery 对齐目标 postMessage 的 URL 初始化契约；只传 companyId 会触发 handleAuth 并清空其余缺失身份。
// 原生页面启动前必须把同一会话的完整身份放进 URL，不能依赖启动后两次定时补 store 来碰运气。
export function nativeAuthQuery (payload: Record<string, unknown>): URLSearchParams {
  const fields: Record<string, unknown> = {
    sid: payload.sid,
    customerCode: payload.customerCode,
    userId: payload.userId,
    userName: payload.displayName || payload.accountName,
    companyId: payload.companyId,
    companyName: payload.companyName,
    userDepartmentName: payload.departmentName,
    topCompanyId: payload.companyId,
    groupDepartment: payload.groupDepartment
  }
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(fields)) if (value !== undefined && value !== null && value !== '') params.set(key, String(value))
  return params
}
