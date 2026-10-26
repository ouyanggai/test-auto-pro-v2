type Values = Record<string, unknown>
interface NativeNode extends Values {
  $children?: NativeNode[]
  $refs?: Record<string, unknown>
  $options?: Values
}

export interface NativeFormInput {
  phase: 'hydrate' | 'verify'
  patch?: Values
  keys?: string[]
  identity?: { userId: string, companyId: string }
  expected?: Values
}

export interface NativeFormValues {
  values: Values
  expected: Values
}

// synchronizeNativeForm 是实际 page.evaluate 执行的自包含函数；只处理已授权字段，不改目标模板、请求正文或节点权限。
// 装载使用原生初始化语义，核对阶段只读；未知偏离必须阻断，不能把非空默认值或被联动搬移的金额当成正确结果。
export async function synchronizeNativeForm (input: NativeFormInput): Promise<NativeFormValues | null> {
  const native = (window as unknown as { __executorNative?: { initFlow?: NativeNode, flowDetail?: NativeNode, forwardList?: NativeNode } }).__executorNative
  const container = native?.initFlow || native?.flowDetail || native?.forwardList
  if (!container) return null
  const queue: unknown[] = [container]
  const seen = new Set<unknown>()
  let form: NativeNode | undefined
  while (queue.length > 0 && !form) {
    const candidate = queue.shift()
    if (!candidate || typeof candidate !== 'object' || seen.has(candidate)) continue
    seen.add(candidate)
    const node = candidate as NativeNode
    if ((typeof node.setData === 'function' && typeof node.getValues === 'function') ||
      (typeof node.setValues === 'function' && typeof node.getValues === 'function') ||
      (typeof node.setFormData === 'function' && typeof node.getFormData === 'function')) form = node
    queue.push(...(node.$children || []), ...Object.values(node.$refs || {}))
  }
  if (!form) return null
  const getter = (form.getValues || form.getFormData) as () => Values | Promise<Values>
  const clone = <T>(value: T): T => value === undefined ? value : JSON.parse(JSON.stringify(value)) as T
  const read = (value: unknown, path: string[]): unknown => {
    if (!path.length) return value
    if (!value || typeof value !== 'object') return undefined
    if (Array.isArray(value) && !/^\d+$/.test(path[0]!)) return value.map(row => read(row, path))
    return read((value as Values)[path[0]!], path.slice(1))
  }
  // 对齐原生 setData 的点路径与子表逐行赋值；不能把 rows.amount 写成一个额外的顶层字段。
  const write = (value: Values | unknown[], path: string[], next: unknown): void => {
    const key = path[0]!
    if (Array.isArray(value) && !/^\d+$/.test(key)) {
      for (const row of value) if (row && typeof row === 'object') write(row as Values, path, next)
    } else if (path.length === 1) {
      ;(value as Values)[key] = clone(next)
    } else {
      const child = (value as Values)[key]
      if (child && typeof child === 'object') write(child as Values, path.slice(1), next)
      else throw new Error(`表单字段路径不存在，拒绝忽略回填：${path.join('.')}`)
    }
  }
  const current = await getter.call(form)
  if (!current || typeof current !== 'object' || Array.isArray(current)) throw new Error('表单 getValues 回读结果不是对象')
  const matches = (expected: unknown, actual: unknown): boolean => {
    if (Array.isArray(expected)) return Array.isArray(actual) && expected.length === actual.length && expected.every((item, index) => matches(item, actual[index]))
    if (expected && typeof expected === 'object') return !!actual && typeof actual === 'object' && !Array.isArray(actual) && Object.keys(expected).every(key => matches((expected as Values)[key], (actual as Values)[key]))
    return Object.is(expected, actual)
  }
  if (input.phase === 'verify') {
    const expected = clone(input.expected || {})
    const fields = new Map<string, Values>()
    const visit = (node: unknown): void => {
      if (!node || typeof node !== 'object') return
      const item = node as Values
      if (typeof item.model === 'string') fields.set(item.model, item)
      for (const child of Object.values(item)) if (child && typeof child === 'object') visit(child)
    }
    visit(form.formData)
    const changed: string[] = []
    for (const key of Object.keys(expected)) {
      const actual = read(current, key.split('.'))
      if (matches(expected[key], actual)) continue
      const field = fields.get(key)
      // 目标通用关联控件用 JSON 字符串同时携带配置与选择结果；审批 refresh 会把 [] 补为仅含 flowType 的空选择配置。
      // 只接受这个控件的配置补齐，不接受新增/丢失选中 ID、非空数组或其他字段的类型变化，也不按业务字段名放行。
      if (field?.type === 'custom' && field.el === 'general-list-select-show' && Array.isArray(expected[key]) && (expected[key] as unknown[]).length === 0 && typeof actual === 'string') {
        let config: Values | null = null
        try { config = JSON.parse(actual) as Values } catch {}
        if (config && !Array.isArray(config) && Object.keys(config).length === 1 && typeof config.flowType === 'string' && config.flowType.trim() !== '') {
          // 接受原生规范值后将它固定下来；动作前不得再次变成别的流程类型或选中别的流程。
          expected[key] = actual
          continue
        }
      }
      changed.push(key)
    }
    if (changed.length) {
      const detail = changed.map(key => `${fields.get(key)?.name || key}（${key}）：预期 ${JSON.stringify(expected[key])}，实际 ${JSON.stringify(read(current, key.split('.')))}`).join('；')
      throw new Error(`表单值已偏离路径配置，拒绝执行：${detail}`)
    }
    return { values: current, expected }
  }

  const patch = input.patch || {}
  const ids = [input.identity?.userId, input.identity?.companyId].filter((id): id is string => !!id)
  const isCurrentUser = (value: unknown): boolean => value !== null && value !== undefined && ids.some(id => String(typeof value === 'string' ? value : JSON.stringify(value)).includes(id))
  const picked: Values = {}
  for (const key of input.keys || []) {
    if (isCurrentUser(read(current, key.split('.'))) && !isCurrentUser(patch[key])) continue
    if (Object.prototype.hasOwnProperty.call(patch, key)) picked[key] = patch[key]
    for (const suffix of ['__virtualName', '__formPersonId', '__condition']) {
      const companion = key + suffix
      if (Object.prototype.hasOwnProperty.call(patch, companion)) picked[companion] = patch[companion]
    }
  }
  const desired = clone(current)
  for (const [key, value] of Object.entries(picked)) write(desired, key.split('.'), value)
  const expected: Values = {}
  for (const key of Object.keys(picked)) expected[key] = clone(read(desired, key.split('.')))

  if (form.formData && 'formValue' in form && typeof form.refresh === 'function') {
    // 宿主会在装载后通过 setOptions/disabled 注入业务上下文与节点权限；刷新不能把这些运行时选项退回模板默认值。
    const schemaOptions = new Map<string, Values>()
    const rememberOptions = (node: unknown): void => {
      if (!node || typeof node !== 'object') return
      const item = node as Values
      const key = typeof item.key === 'string' ? `key:${item.key}` : typeof item.model === 'string' ? `model:${item.model}` : ''
      if (key && item.options && typeof item.options === 'object') schemaOptions.set(key, clone(item.options as Values))
      for (const [name, child] of Object.entries(item)) if (name !== 'options' && child && typeof child === 'object') rememberOptions(child)
    }
    rememberOptions(form.formData)
    const previousHidden = clone((form.dynamicHideFields || {}) as Values)
    let permissions = form.$parent as NativeNode | undefined
    while (permissions && !(Array.isArray(permissions.disabledData) && Array.isArray(permissions.enableData))) permissions = permissions.$parent as NativeNode | undefined
    // 原生容器用了 Vue 3 钩子名，但当前宿主是 Vue 2。统一补上它们已有的注销逻辑，禁止引用越刷新越旧。
    const cleanup = function (this: NativeNode): void {
      const options = this.$options || {}
      const inject = options.inject as Values | undefined
      if (inject?.generateComponentInstance && inject.deleteComponentInstance && typeof options.beforeUnmount === 'function') options.beforeUnmount.call(this)
    }
    // 弹窗根实例可能是 Vue.extend 的子类；子组件实际由 _base 创建，桥接必须装到同一个 Vue 基类。
    const root = form.$root as NativeNode | undefined
    const Vue = root?.$options?._base as { mixin?: (options: unknown) => void, __executorFormCleanup?: boolean } | undefined
    if (!Vue || typeof Vue.mixin !== 'function') throw new Error('原生表单缺少 Vue 生命周期注册能力')
    if (!Vue.__executorFormCleanup) {
      // 已挂载实例不会重新合并全局 mixin，先补当前实例；之后的新容器由全局桥接统一覆盖。
      const attach = (node: NativeNode): void => {
        const options = node.$options
        const inject = options?.inject as Values | undefined
        if (options && inject?.generateComponentInstance && inject.deleteComponentInstance && typeof options.beforeUnmount === 'function') {
          const hooks = options.beforeDestroy
          node.$options = { ...options, beforeDestroy: [...(Array.isArray(hooks) ? hooks : typeof hooks === 'function' ? [hooks] : []), cleanup] }
        }
        for (const child of node.$children || []) attach(child)
      }
      attach(form)
      Vue.mixin({ beforeDestroy: cleanup })
      Vue.__executorFormCleanup = true
    }
    const registry = form.instanceObject as Values
    for (const [key, value] of Object.entries(registry)) {
      const alive = (Array.isArray(value) ? value : [value]).filter(item => !(item as Values)?._isDestroyed)
      if (!alive.length) delete registry[key]
      else registry[key] = alive.length === 1 ? alive[0] : alive
    }
    // 原生 refresh 从 formValue 生成 models，再运行模板 refresh；不能先按默认值初始化后逐项 setData。
    form.formValue = clone(desired)
    const refreshed = form.refresh.call(form)
    // generateModel 只生成模板字段，人员伴生键和虚拟名称不会自动进入新 models；须在初始化脚本执行前保留这些原生载荷数据。
    const preserveUnmodeled = (target: Values, source: Values): void => {
      for (const [key, value] of Object.entries(source)) {
        if (!Object.prototype.hasOwnProperty.call(target, key)) {
          if (typeof form.$set === 'function') form.$set.call(form, target, key, clone(value))
          else target[key] = clone(value)
        } else if (target[key] && value && typeof target[key] === 'object' && typeof value === 'object') {
          preserveUnmodeled(target[key] as Values, value as Values)
        }
      }
    }
    preserveUnmodeled(form.models as Values, desired)
    const restoreOptions = (node: unknown): void => {
      if (!node || typeof node !== 'object') return
      const item = node as Values
      const key = typeof item.key === 'string' ? `key:${item.key}` : typeof item.model === 'string' ? `model:${item.model}` : ''
      if (key && schemaOptions.has(key)) item.options = clone(schemaOptions.get(key))
      for (const [name, child] of Object.entries(item)) if (name !== 'options' && child && typeof child === 'object') restoreOptions(child)
    }
    // 原生 _initForm 在第一个 await 之前生成新模板，此时重放纯选项/权限，不调用字段 onChange 或业务事件。
    restoreOptions(form.formData)
    if (permissions && typeof form._setSubDisabled === 'function') {
      const list = (form.formData as Values).list
      for (const key of permissions.disabledData as string[]) form._setSubDisabled.call(form, list, key.split('.'), true)
      for (const key of permissions.enableData as string[]) form._setSubDisabled.call(form, list, key.split('.'), false)
    }
    await refreshed
    const hidden = form.dynamicHideFields as Values
    for (const [key, value] of Object.entries(previousHidden)) {
      // 初始化已明确计算过的条件显隐以新结果为准；没有被初始化触及的权限隐藏仍须保留。
      if (!Object.prototype.hasOwnProperty.call(hidden, key)) {
        if (typeof form.$set === 'function') form.$set.call(form, hidden, key, value)
        else hidden[key] = value
      }
    }
  } else {
    const setter = (form.setData || form.setValues || form.setFormData) as (values: Values) => unknown
    await setter.call(form, clone(picked))
  }
  return { values: await getter.call(form), expected }
}
