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

export interface PatchOptions {
  allowedKeys: string[]
}

function pickAllowed (source: Record<string, unknown>, allowedKeys: string[]): Record<string, unknown> {
  const next: Record<string, unknown> = {}
  for (const key of allowedKeys) {
    if (Object.prototype.hasOwnProperty.call(source, key)) next[key] = source[key]
    const virtual = `${key}__virtualName`
    if (Object.prototype.hasOwnProperty.call(source, virtual)) next[virtual] = source[virtual]
  }
  return next
}

// applyNodePatch 只写入当前节点允许字段（含 <key>__virtualName 虚拟字段），写完即回读完整值。
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
  const allowed = pickAllowed(patch, options.allowedKeys)
  await api.setData(allowed)
  const values = await api.getValues()
  if (values === null || values === undefined || typeof values !== 'object') {
    throw new UnsupportedFormError('getValues 回读结果不是对象，表单数据接口不可用')
  }
  return values
}
