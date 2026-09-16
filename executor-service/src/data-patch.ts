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
  optionsReady?: boolean
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

// applyNodePatch 只写入当前节点允许字段，等选项完成后再回读。
export async function applyNodePatch (
  api: FormDataApi | null | undefined,
  patch: Record<string, unknown>,
  options: PatchOptions
): Promise<Record<string, unknown>> {
  if (!api || typeof api.setData !== 'function' || typeof api.getValues !== 'function') {
    throw new UnsupportedFormError('页面缺少 setData/getValues 数据接口')
  }
  if (options.optionsReady === false) {
    throw new UnsupportedFormError('远程选项尚未完成，不得执行写动作')
  }
  const allowed = pickAllowed(patch, options.allowedKeys)
  await api.setData(allowed)
  const values = await api.getValues()
  return values
}
