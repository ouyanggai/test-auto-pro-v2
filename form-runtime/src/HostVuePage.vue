<template>
  <section class="host-vue-page">
    <component
      :is="pageComponent"
      v-if="pageComponent"
      ref="page"
      :key="page.componentName"
      :opera-type="readOnly ? 'preview' : 'create'"
      :action-type="readOnly ? 'view' : 'create'"
      :show-type="readOnly ? 'preview' : 'init'"
      :select-flow-type="page.route"
      :select-flow-name="page.pageName"
      :params-info="values"
      :param="values"
    />
    <div v-else class="host-vue-page__error">当前 Vue 业务页面没有可加载的宿主组件</div>
  </section>
</template>

<script>
import { clonePlain } from './runtime/formTemplate'
import { resolveHostVuePage } from './runtime/hostVuePages'

export default {
  name: 'HostVuePage',
  props: {
    page: { type: Object, required: true },
    initialValues: { type: Object, default: () => ({}) },
    permissions: { type: Array, default: () => [] },
    fieldRules: { type: Array, default: () => [] },
    readOnly: { type: Boolean, default: false }
  },
  data () {
    return { values: clonePlain(this.initialValues || {}) }
  },
  computed: {
    pageComponent () { return resolveHostVuePage(this.page.componentName) }
  },
  methods: {
    async setData (values) {
      this.values = clonePlain(values || {})
      await this.$nextTick()
      this.applyValues(this.$refs.page, this.values, new Set(), 0)
      this.applyFieldStates(this.$refs.page, new Set(), 0)
    },
    async capture (validate) {
      const values = clonePlain(this.values)
      this.captureValues(this.$refs.page, values, new Set(), 0)
      if (validate) await this.validatePage(this.$refs.page)
      this.values = values
      return values
    },
    applyValues (instance, values, visited, depth) {
      if (!instance || visited.has(instance) || depth > 7) return
      visited.add(instance)
      const fields = Array.isArray(this.page.fields) ? this.page.fields : []
      for (const field of fields) {
        const value = this.getPath(values, field.path)
        if (value === undefined) continue
        this.writeKnownState(instance, field.path, clonePlain(value))
      }
      for (const child of this.childInstances(instance)) this.applyValues(child, values, visited, depth + 1)
    },
    captureValues (instance, values, visited, depth) {
      if (!instance || visited.has(instance) || depth > 7) return
      visited.add(instance)
      for (const field of Array.isArray(this.page.fields) ? this.page.fields : []) {
        const value = this.readKnownState(instance, field.path)
        if (value !== undefined) this.setPath(values, field.path, clonePlain(value))
      }
      for (const child of this.childInstances(instance)) this.captureValues(child, values, visited, depth + 1)
    },
    applyFieldStates (instance, visited, depth) {
      if (!instance || visited.has(instance) || depth > 7) return
      visited.add(instance)
      const permission = new Map(this.permissions.map(item => [String(item.field || ''), String(item.power || '')]))
      const protectedFields = new Set(this.fieldRules.filter(item => item && item.disabled).map(item => String(item.field || '')))
      const visitConfig = (value, configVisited, configDepth) => {
        if (!value || typeof value !== 'object' || configVisited.has(value) || configDepth > 10) return
        configVisited.add(value)
        if (value.prop) {
          const path = String(value.prop)
          value.disabled = this.readOnly || permission.get(path) !== 'edit' || protectedFields.has(path)
          if (value.disabled && Array.isArray(value.rules)) value.rules = value.rules.filter(rule => !rule || !rule.required)
        }
        for (const child of Array.isArray(value) ? value : Object.values(value)) visitConfig(child, configVisited, configDepth + 1)
      }
      visitConfig(instance.$data, new Set(), 0)
      if (instance.$options && instance.$options.name === 'ElFormItem' && instance.prop) {
        const locked = this.fieldLocked(String(instance.prop), permission, protectedFields)
        this.setDescendantsDisabled(instance, locked, new Set(), 0)
      }
      for (const child of this.childInstances(instance)) this.applyFieldStates(child, visited, depth + 1)
    },
    writeKnownState (instance, path, value) {
      const data = instance.$data || {}
      const candidates = [data, data.form, data.initForm, data.editData, data.params, data.param, data.rawData, data.detail]
      for (const candidate of candidates) {
        if (!candidate || typeof candidate !== 'object') continue
        if (this.hasPath(candidate, path)) this.setPath(candidate, path, value)
        const leaf = String(path || '').split('.').filter(Boolean).pop()
        if (leaf && Object.prototype.hasOwnProperty.call(candidate, leaf)) this.$set(candidate, leaf, value)
      }
    },
    readKnownState (instance, path) {
      const data = instance.$data || {}
      for (const candidate of [data, data.form, data.initForm, data.editData, data.params, data.param, data.rawData, data.detail]) {
        if (!candidate || typeof candidate !== 'object') continue
        const direct = this.getPath(candidate, path)
        if (direct !== undefined) return direct
        const leaf = String(path || '').split('.').filter(Boolean).pop()
        if (leaf && Object.prototype.hasOwnProperty.call(candidate, leaf)) return candidate[leaf]
      }
      return undefined
    },
    async validatePage (instance) {
      const forms = []
      this.collectForms(instance, forms, new Set(), 0)
      for (const form of forms) {
        await new Promise((resolve, reject) => form.validate(valid => valid ? resolve() : reject(new Error('请先完成页面中的必填项'))))
      }
    },
    collectForms (instance, forms, visited, depth) {
      if (!instance || visited.has(instance) || depth > 7) return
      visited.add(instance)
      if (instance.$options && instance.$options.name === 'ElForm' && typeof instance.validate === 'function') forms.push(instance)
      for (const child of this.childInstances(instance)) this.collectForms(child, forms, visited, depth + 1)
    },
    refInstances (refs) {
      const result = []
      for (const value of Object.values(refs || {})) {
        if (Array.isArray(value)) result.push(...value.filter(Boolean))
        else if (value) result.push(value)
      }
      return result
    },
    childInstances (instance) {
      return [...new Set([...(Array.isArray(instance && instance.$children) ? instance.$children : []), ...this.refInstances(instance && instance.$refs)])]
    },
    fieldLocked (prop, permission, protectedFields) {
      const exact = permission.get(prop)
      const suffix = [...permission.entries()].filter(([path]) => path.endsWith(`.${prop}`))
      const power = exact || (suffix.length === 1 ? suffix[0][1] : '')
      const protectedMatch = protectedFields.has(prop) || [...protectedFields].filter(path => path.endsWith(`.${prop}`)).length === 1
      return this.readOnly || power !== 'edit' || protectedMatch
    },
    setDescendantsDisabled (instance, disabled, visited, depth) {
      if (!instance || visited.has(instance) || depth > 5) return
      visited.add(instance)
      if (instance._props && Object.prototype.hasOwnProperty.call(instance._props, 'disabled')) this.$set(instance._props, 'disabled', disabled)
      for (const child of this.childInstances(instance)) this.setDescendantsDisabled(child, disabled, visited, depth + 1)
    },
    hasPath (input, path) { return this.getPath(input, path) !== undefined },
    getPath (input, path) { return String(path || '').split('.').filter(Boolean).reduce((current, key) => current && typeof current === 'object' ? current[key] : undefined, input) },
    setPath (input, path, value) {
      const parts = String(path || '').split('.').filter(Boolean)
      let current = input
      for (let index = 0; index < parts.length - 1; index++) {
        if (!current[parts[index]] || typeof current[parts[index]] !== 'object') this.$set(current, parts[index], {})
        current = current[parts[index]]
      }
      if (parts.length) this.$set(current, parts[parts.length - 1], value)
    }
  }
}
</script>

<style scoped>
.host-vue-page { min-height: 100%; background: #fff; }
.host-vue-page__error { padding: 48px 20px; color: #8c8c8c; text-align: center; }
.host-vue-page :deep(.footer-bt), .host-vue-page :deep(.dialog-footer) { display: none !important; }
</style>
