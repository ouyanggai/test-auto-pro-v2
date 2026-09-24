export class UnsupportedFormError extends Error {
  constructor (message: string) {
    super(message)
    this.name = 'UnsupportedFormError'
    this.code = 'unsupported'
  }

  code: string
}

export interface FormDataApi {
  setData?: (values: Record<string, unknown>) => Promise<void> | void
  getValues?: () => Promise<Record<string, unknown>> | Record<string, unknown>
  getData?: (validate?: boolean) => Promise<unknown> | unknown
}

export interface IdentityHint {
  // 计划账号的唯一标识：表单初始化事件已按它填好「当前用户」类字段，这些字段不再用历史值覆盖。
  userId?: string
  companyId?: string
}

export interface PatchOptions {
  allowedKeys: string[]
  identity?: IdentityHint
}

// isCurrentUserField 判断某字段初始化后的值是否是表单按注入身份自动填好的「当前用户」字段：
// 依据是该值内嵌了当前计划账号的唯一标识（userId/companyId），而不是字段名——所以对任意表单键都成立，
// 且不会误伤业务里真实选定的他人（初始化前那些业务字段为空，不含当前账号标识）。
export function isCurrentUserField (value: unknown, identity?: IdentityHint): boolean {
  if (!identity) return false
  const ids = [identity.userId, identity.companyId]
    .filter((id): id is string => typeof id === 'string' && id.trim() !== '')
  if (ids.length === 0 || value === null || value === undefined) return false
  let text: string
  try {
    text = typeof value === 'string' ? value : JSON.stringify(value)
  } catch {
    return false
  }
  if (!text) return false
  return ids.some(id => text.includes(id))
}

// pickAllowed 只取当前节点允许字段及其人员选择器伴生键（__virtualName/__formPersonId/__condition）。
function pickAllowed (source: Record<string, unknown>, allowedKeys: string[]): Record<string, unknown> {
  const next: Record<string, unknown> = {}
  const companions = ['__virtualName', '__formPersonId', '__condition']
  for (const key of allowedKeys) {
    if (Object.prototype.hasOwnProperty.call(source, key)) next[key] = source[key]
    for (const suffix of companions) {
      const companion = `${key}${suffix}`
      if (Object.prototype.hasOwnProperty.call(source, companion)) next[companion] = source[companion]
    }
  }
  return next
}

// dropCurrentUserFields 从待写入补丁中剔除表单已按注入身份填好的当前用户字段（含其伴生键），
// 让「初始化事件覆盖」的正确身份保留，不被历史发起人值覆盖（F-043 缺陷②）。
function dropCurrentUserFields (
  allowed: Record<string, unknown>,
  initValues: Record<string, unknown> | null | undefined,
  allowedKeys: string[],
  identity?: IdentityHint
): void {
  if (!initValues || typeof initValues !== 'object') return
  const companions = ['__virtualName', '__formPersonId', '__condition']
  for (const key of allowedKeys) {
    if (!isCurrentUserField(initValues[key], identity)) continue
    delete allowed[key]
    for (const suffix of companions) delete allowed[`${key}${suffix}`]
  }
}

// applyNodePatch 只写入当前节点允许字段（含人员选择器伴生键），写完即回读完整值。
// F-043 缺陷②：先读初始化后的表单值——表单初始化事件已按注入身份填好「当前用户」类字段，
// 这些字段从补丁中剔除、不用历史发起人值覆盖；其余业务/路径字段照常应用。
// 就绪等待由调用方负责（waitForOptionsReady：trace 无在途请求 + 稳定窗）——就绪必须「等」出来，
// 不允许任何硬编码 true/false 标志（评审 P1-5 的语义反置根因）。
export async function applyNodePatch (
  api: FormDataApi | null | undefined,
  patch: Record<string, unknown>,
  options: PatchOptions
): Promise<Record<string, unknown>> {
  if (!api || typeof api.setData !== 'function' || typeof api.getValues !== 'function') {
    throw new UnsupportedFormError('页面缺少 setData/getValues 数据接口')
  }
  const initValues = await api.getValues()
  const allowed = pickAllowed(patch, options.allowedKeys)
  dropCurrentUserFields(allowed, initValues as Record<string, unknown> | null, options.allowedKeys, options.identity)
  await api.setData(allowed)
  const values = await api.getValues()
  if (values === null || values === undefined || typeof values !== 'object') {
    throw new UnsupportedFormError('getValues 回读结果不是对象，表单数据接口不可用')
  }
  return values
}
