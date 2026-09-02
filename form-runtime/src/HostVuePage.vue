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
import { captureVueFieldValues, mergeVueFieldIssues, writeVueFieldValues } from './runtime/vueFieldBridge'

export default {
  name: 'HostVuePage',
  props: {
    page: { type: Object, required: true },
    initialValues: { type: Object, default: () => ({}) },
    permissions: { type: Array, default: () => [] },
    readOnly: { type: Boolean, default: false }
  },
  data () {
    return { values: clonePlain(this.initialValues || {}), bridgeIssues: [] }
  },
  computed: {
    pageComponent () { return resolveHostVuePage(this.page.componentName) }
  },
  methods: {
    async setData (values) {
      this.values = clonePlain(values || {})
      await this.$nextTick()
      const written = writeVueFieldValues(this.$refs.page, this.page.fields, this.values, (owner, key, value) => this.$set(owner, key, value))
      this.bridgeIssues = written.issues
      this.applyFieldStates(this.$refs.page, new Set(), 0)
      return { issues: this.bridgeIssues, writtenFieldPaths: written.writtenFieldPaths }
    },
    async capture (validate) {
      const captured = captureVueFieldValues(this.$refs.page, this.page.fields, this.values)
      if (validate) await this.validatePage(this.$refs.page)
      this.values = captured.values
      return {
        values: captured.values,
        issues: mergeVueFieldIssues(this.bridgeIssues, captured.issues),
        capturedFieldPaths: captured.capturedFieldPaths
      }
    },
    applyFieldStates (instance, visited, depth) {
      if (!instance || visited.has(instance) || depth > 7) return
      visited.add(instance)
      const permission = new Map(this.permissions.map(item => [String(item.field || ''), String(item.power || '')]))
      const visitConfig = (value, configVisited, configDepth) => {
        if (!value || typeof value !== 'object' || configVisited.has(value) || configDepth > 10) return
        configVisited.add(value)
        if (value.prop) {
          const path = String(value.prop)
          const power = permission.get(path)
          value.hidden = power === 'hide' || value.hidden === true
          value.disabled = this.readOnly || power !== 'edit'
          if (value.disabled && Array.isArray(value.rules)) value.rules = value.rules.filter(rule => !rule || !rule.required)
        }
        for (const child of Array.isArray(value) ? value : Object.values(value)) visitConfig(child, configVisited, configDepth + 1)
      }
      visitConfig(instance.$data, new Set(), 0)
      if (instance.$options && instance.$options.name === 'ElFormItem' && instance.prop) {
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
    fieldLocked (prop, permission) {
      const exact = permission.get(prop)
      const suffix = [...permission.entries()].filter(([path]) => path.endsWith(`.${prop}`))
      const power = exact || (suffix.length === 1 ? suffix[0][1] : '')
      return this.readOnly || power !== 'edit'
    },
    setDescendantsDisabled (instance, disabled, visited, depth) {
      if (!instance || visited.has(instance) || depth > 5) return
      visited.add(instance)
      if (instance._props && Object.prototype.hasOwnProperty.call(instance._props, 'disabled')) this.$set(instance._props, 'disabled', disabled)
      for (const child of this.childInstances(instance)) this.setDescendantsDisabled(child, disabled, visited, depth + 1)
    }
  }
}
</script>

<style scoped>
.host-vue-page { min-height: 100%; background: #fff; }
.host-vue-page__error { padding: 48px 20px; color: #8c8c8c; text-align: center; }
.host-vue-page :deep(.footer-bt), .host-vue-page :deep(.dialog-footer) { display: none !important; }
</style>
