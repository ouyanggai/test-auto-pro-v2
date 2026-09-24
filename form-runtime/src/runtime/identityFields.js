// F-043 缺陷②:目标表单的初始化脚本(eventScript key='refresh',名「初始化当前登录人信息」)从
// window.localStorage 的 invest-power-system-* 键读当前登录人,填「申请人/部门/公司」等当前用户字段。
// 但脚本用「仅空值填充」守卫(if(!this.getValue('applicantName'))...),历史回放时这些字段已被历史发起人
// 值填满,守卫跳过,当前用户字段保留历史值。本模块提供:把「空值探针重跑初始化脚本」捕获到的补丁里,
// 只挑出「取值确实来自注入的计划账号身份」的字段,用于覆盖历史值,业务默认值(如无陪同='无')不挑出。

// pickIdentityDerived 从探针捕获的补丁中,挑出取值内嵌任一计划账号身份串(userId/userName/公司/部门等)的字段。
// 判据只看值不看字段名:历史真实选定的他人取值不含当前计划账号标识,因此不会被误挑;业务字面默认值同理不含标识被排除。
// identityValues 为空或补丁为空时返回空对象,调用方据此回落为不覆盖任何字段。
export function pickIdentityDerived (capturedPatch, identityValues) {
  const ids = (identityValues || []).filter(value => typeof value === 'string' && value.trim() !== '')
  const picked = {}
  if (!capturedPatch || typeof capturedPatch !== 'object' || ids.length === 0) return picked
  for (const [key, value] of Object.entries(capturedPatch)) {
    if (value === undefined || value === null || value === '') continue
    let text
    try {
      // 值可能是字符串、{id,name} 对象或数组;统一序列化后判断是否内嵌计划账号标识,循环引用等序列化失败值跳过。
      text = typeof value === 'string' ? value : JSON.stringify(value)
    } catch (_) {
      continue
    }
    if (text && ids.some(id => text.includes(id))) picked[key] = value
  }
  return picked
}
