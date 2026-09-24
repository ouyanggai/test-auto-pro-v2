// F-043 缺陷②:注入身份后,表单初始化事件会把「当前用户」类字段(申请人/部门/公司等)填成计划账号。
// 本模块负责在应用历史业务数据前,识别并剔除这些当前用户字段,使初始化结果保留,不被历史发起人值覆盖。

// CURRENT_USER_COMPANION_SUFFIXES 是人员/公司选择器写入的伴生键(虚拟名、人员对象、条件判据),
// 与主键同属一个当前用户字段,剔除主键时必须一并剔除,避免残留历史发起人的显示名或条件值。
const CURRENT_USER_COMPANION_SUFFIXES = ['__virtualName', '__formPersonId', '__condition']

// deleteKey 只删除对象自有键,不触碰原型链,避免误删或对非对象抛错。
function deleteKey (target, key) {
  if (target && typeof target === 'object' && Object.prototype.hasOwnProperty.call(target, key)) {
    delete target[key]
  }
}

// stripCurrentUserFields 依据「初始化后的字段值是否内嵌当前账号唯一标识」判定当前用户字段,
// 从待写入值 values 与恢复基线 savedValues 中同步剔除该字段及其伴生键,返回被剔除的主键列表(供测试与日志)。
// 判据只看值不看字段名:初始化前当前用户字段为空、不含当前账号标识,因此不会误伤业务里真实选定的他人;
// identityIds 为空或初始化值不可用时不剔除任何字段,回落为按历史数据整份覆盖的原行为。
export function stripCurrentUserFields (values, savedValues, initValues, identityIds) {
  const ids = (identityIds || []).filter(id => typeof id === 'string' && id.trim() !== '')
  if (ids.length === 0 || !initValues || typeof initValues !== 'object') return []
  const stripped = []
  for (const key of Object.keys(initValues)) {
    const value = initValues[key]
    if (value === undefined || value === null) continue
    let text
    try {
      // 值可能是字符串、对象或数组;统一序列化后判断是否内嵌当前账号标识,序列化失败的循环引用值直接跳过。
      text = typeof value === 'string' ? value : JSON.stringify(value)
    } catch (_) {
      continue
    }
    if (!text || !ids.some(id => text.includes(id))) continue
    stripped.push(key)
    deleteKey(values, key)
    deleteKey(savedValues, key)
    for (const suffix of CURRENT_USER_COMPANION_SUFFIXES) {
      deleteKey(values, key + suffix)
      deleteKey(savedValues, key + suffix)
    }
  }
  return stripped
}
