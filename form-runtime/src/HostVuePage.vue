<template>
  <section class="host-vue-page">
    <component
      :is="pageComponent"
      v-if="pageComponent"
      ref="page"
      :key="`${page.componentName}:${valuesVersion}`"
      v-bind="pageProps"
    />
    <div v-else class="host-vue-page__error">当前 Vue 业务页面没有可加载的宿主组件</div>
  </section>
</template>

<script>
import { clonePlain } from './runtime/formTemplate'
import { resolveHostVuePage } from './runtime/hostVuePages'

const FORM_MODEL_KEYS = new Set([
  'form', 'formData', 'dataForm', 'model', 'initForm', 'initMainForm', 'initContentForm',
  'infoForm', 'quarterData', 'workTargetData', 'originData', 'mainFormData', 'baseInfo',
  'detailForm', 'companyDeta', 'departmentData'
])

// isVueInstance 只允许递归进入 Vue 组件实例，避免把跨域 Window 或 DOM ref 当成业务对象访问。
function isVueInstance (value) {
  try {
    return Boolean(value && typeof value === 'object' && value._isVue === true)
  } catch (_) {
    // 跨域 Window 读取非白名单属性会抛 SecurityError，ref 过滤必须把它当作非 Vue 对象。
    return false
  }
}

// hasOwnPropertySafe 读取第三方对象属性时吞掉跨域 Window 的安全异常，避免诊断递归反过来破坏渲染。
function hasOwnPropertySafe (value, key) {
  try {
    return Object.prototype.hasOwnProperty.call(value, key)
  } catch (_) {
    return false
  }
}

// isConfigObject 限制字段配置递归只进入普通对象或数组，不枚举 Window、DOM 等宿主对象。
function isConfigObject (value) {
  try {
    return Array.isArray(value) || Object.prototype.toString.call(value) === '[object Object]'
  } catch (_) {
    return false
  }
}

export default {
  name: 'HostVuePage',
  props: {
    page: { type: Object, required: true },
    initialValues: { type: Object, default: () => ({}) },
    permissions: { type: Array, default: () => [] },
    readOnly: { type: Boolean, default: false }
  },
  data () {
    return { values: clonePlain(this.initialValues || {}), valuesVersion: 0 }
  },
  computed: {
    pageComponent () { return resolveHostVuePage(this.page.componentName) },
    // 同一份快照按目标页面既有的参数命名透传，避免不同表单因参数名差异丢失初始值。
    pageProps () {
      const values = this.values || {}
      const firstValue = (...keys) => {
        for (const key of keys) {
          const value = values[key]
          if (value !== undefined && value !== null && value !== '') return value
        }
        return ''
      }
      const workspaceMode = this.readOnly ? 'preview' : 'edit'
      return {
        // 宿主只负责数据工作区；禁止目标页面进入新建/提交流程，也不触发按流程类型读取模板。
        operaType: workspaceMode,
        actionType: workspaceMode,
        showType: workspaceMode,
        selectFlowType: '',
        selectFlowName: this.page.pageName,
        value: values,
        propData: values,
        params: values,
        paramsInfo: values,
        param: values,
        initialValues: values,
        data: values,
        // 业务快照中的 id 不是目标页面初始化所需的代理 id，不能再驱动目标接口覆盖已传入的数据。
        id: '',
        bizId: '',
        otherBizId: '',
        flowInstanceId: '',
        flowProxyId: '',
        flowNodeProxyId: '',
        createrId: firstValue('createrId', 'creatorId', 'creator_id', 'userId', 'user_id'),
        isExamine: Boolean(values.isExamine || values.is_examine),
        isReInitiate: Boolean(values.isReInitiate || values.is_re_initiate),
        logTableData: values.logTableData || values.log_table_data || []
      }
    }
  },
  methods: {
    // 更新快照后递增版本，强制目标页面重新走既有 created 初始化流程。
    async setData (values) {
      this.values = clonePlain(values || {})
      this.valuesVersion += 1
      await this.$nextTick()
      // 目标页面的 created/mounted 初始化可能还要经过一轮子组件挂载，第二次 tick 后再回填内部模型。
      await this.$nextTick()
      this.hydrateInitialValues(this.$refs.page, this.values, new Set(), 0)
      this.applyFieldStates(this.$refs.page, new Set(), 0)
      return { issues: [] }
    },
    async capture (validate) {
      if (validate) await this.validatePage(this.$refs.page)
      const values = this.collectPageValues(this.$refs.page, clonePlain(this.values || {}), new Set(), 0)
      this.values = values
      return {
        values,
        issues: [],
        capturedFieldPaths: []
      }
    },
    // hydrateInitialValues 在目标组件完成既有初始化后按同名字段回填快照，不改变目标页面的字段结构和请求逻辑。
    hydrateInitialValues (instance, source, visited, depth) {
      if (!isVueInstance(instance) || visited.has(instance) || depth > 8) return
      visited.add(instance)
      if (source && typeof source === 'object') {
        Object.keys(source).forEach(key => {
          const incoming = source[key]
          if (!hasOwnPropertySafe(instance, key)) return
          const current = instance[key]
          if (current && typeof current === 'object' && incoming && typeof incoming === 'object' && !Array.isArray(current) && !Array.isArray(incoming)) {
            Object.assign(current, clonePlain(incoming))
          } else {
            this.$set(instance, key, clonePlain(incoming))
          }
        })
        // 复制页面通常把字段放在 initForm/formData 等模型对象中，快照是扁平字段时要回填这些模型。
        for (const key of FORM_MODEL_KEYS) {
          const current = instance[key]
          if (!current || typeof current !== 'object' || Array.isArray(current)) continue
          const modelKeys = Object.keys(current)
          const matching = modelKeys.filter(modelKey => hasOwnPropertySafe(source, modelKey))
          if (matching.length) matching.forEach(modelKey => this.$set(current, modelKey, clonePlain(source[modelKey])))
        }
      }
      for (const child of this.childInstances(instance)) this.hydrateInitialValues(child, source, visited, depth + 1)
    },
    // collectPageValues 优先读取目标页面公开的 getValues，保留无表单页面编辑后的内部模型。
    collectPageValues (instance, values, visited, depth) {
      if (!isVueInstance(instance) || visited.has(instance) || depth > 8) return values
      visited.add(instance)
      if (typeof instance.getValues === 'function') {
        const pageValues = instance.getValues()
        if (pageValues && typeof pageValues === 'object' && Object.keys(pageValues).length) values = { ...values, ...clonePlain(pageValues) }
      }
      for (const child of this.childInstances(instance)) values = this.collectPageValues(child, values, visited, depth + 1)
      return values
    },
    applyFieldStates (instance, visited, depth) {
      if (!isVueInstance(instance) || visited.has(instance) || depth > 7) return
      visited.add(instance)
      const permission = new Map(this.permissions.map(item => [String(item.field || ''), String(item.power || '')]))
      const visitConfig = (value, configVisited, configDepth) => {
        if (!isConfigObject(value) || configVisited.has(value) || configDepth > 10) return
        configVisited.add(value)
        if (hasOwnPropertySafe(value, 'prop') && value.prop) {
          const path = String(value.prop)
          const power = permission.get(path)
          value.hidden = power === 'hide' || value.hidden === true
          value.disabled = this.readOnly || power !== 'edit'
          if (value.disabled && Array.isArray(value.rules)) value.rules = value.rules.filter(rule => !rule || !rule.required)
        }
        for (const child of Array.isArray(value) ? value : Object.values(value)) visitConfig(child, configVisited, configDepth + 1)
      }
      visitConfig(instance.$data, new Set(), 0)
      if (instance.$options && instance.$options.name === 'ElFormItem' && hasOwnPropertySafe(instance, 'prop') && instance.prop) {
        const power = permission.get(String(instance.prop))
        if (instance.$el && instance.$el.style) instance.$el.style.display = power === 'hide' ? 'none' : ''
        const locked = this.fieldLocked(String(instance.prop), permission)
        this.setDescendantsDisabled(instance, locked, new Set(), 0)
      }
      for (const child of this.childInstances(instance)) this.applyFieldStates(child, visited, depth + 1)
    },
    async validatePage (instance) {
      const forms = []
      this.collectForms(instance, forms, new Set(), 0)
      for (const form of forms) {
        await new Promise((resolve, reject) => form.validate(valid => valid ? resolve() : reject(new Error('请先完成页面中的必填项'))))
      }
    },
    collectForms (instance, forms, visited, depth) {
      if (!isVueInstance(instance) || visited.has(instance) || depth > 7) return
      visited.add(instance)
      if (instance.$options && instance.$options.name === 'ElForm' && typeof instance.validate === 'function') forms.push(instance)
      for (const child of this.childInstances(instance)) this.collectForms(child, forms, visited, depth + 1)
    },
    refInstances (refs) {
      const result = []
      for (const value of Object.values(refs || {})) {
        if (Array.isArray(value)) result.push(...value.filter(isVueInstance))
        else if (isVueInstance(value)) result.push(value)
      }
      return result
    },
    childInstances (instance) {
      return [...new Set([
        ...(Array.isArray(instance && instance.$children) ? instance.$children.filter(isVueInstance) : []),
        ...this.refInstances(instance && instance.$refs)
      ])]
    },
    fieldLocked (prop, permission) {
      const exact = permission.get(prop)
      const suffix = [...permission.entries()].filter(([path]) => path.endsWith(`.${prop}`))
      const power = exact || (suffix.length === 1 ? suffix[0][1] : '')
      return this.readOnly || power !== 'edit'
    },
    setDescendantsDisabled (instance, disabled, visited, depth) {
      if (!isVueInstance(instance) || visited.has(instance) || depth > 5) return
      visited.add(instance)
      if (instance._props && hasOwnPropertySafe(instance._props, 'disabled')) this.$set(instance._props, 'disabled', disabled)
      for (const child of this.childInstances(instance)) this.setDescendantsDisabled(child, disabled, visited, depth + 1)
    }
  }
}
</script>

<style>
.host-vue-page { min-height: 100%; background: #fff; }
.host-vue-page__error { padding: 48px 20px; color: #8c8c8c; text-align: center; }
.host-vue-page .footer-bt, .host-vue-page .botton-group { display: none !important; }
</style>
