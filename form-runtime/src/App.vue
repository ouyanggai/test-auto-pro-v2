<template>
  <main class="form-runtime" :aria-busy="loading">
    <fm-generate-form
      v-if="sessionId && renderType === 'formmaking'"
      ref="generateForm"
      :key="sessionId"
      :data="template"
      :value="values"
      :edit="!readOnly"
      @on-change="markDirty"
    />
    <host-vue-page v-else-if="sessionId && renderType === 'vue_custom'" ref="vueHost" :page="vuePage" :initial-values="values" :permissions="runtimePermissions" :field-rules="runtimeFieldRules" :read-only="readOnly" />
    <div v-else class="form-runtime__placeholder">正在等待表单工作区初始化…</div>
  </main>
</template>

<script>
import { FORM_RUNTIME_VERSION, isRuntimeCommand } from './runtime/protocol'
import { captureFormValues, clonePlain, formRuntimeStats, prepareTemplate, diffManualPaths, refreshPreparedForm } from './runtime/formTemplate'
import { installReadOnlyRequestPolicy } from './runtime/requestPolicy'
import { clearRuntimeAuth, installRuntimeStorageFacade, setRuntimeAuth } from './runtime/memoryAuth'
import { setConfig as setRuntimeEnvironment } from './runtime/runtimeEnvironment'
import HostVuePage from './HostVuePage.vue'

export default {
  name: 'FormRuntimeApp',
  components: { HostVuePage },
  data () {
    return {
      parentOrigin: '',
      sessionId: '',
      template: { list: [], config: {} },
      values: {},
      savedValues: {},
      generatedValues: {},
      generatedFieldPaths: [],
      manualOverridePaths: [],
      renderType: 'formmaking',
      vuePage: { pageName: '', fields: [], issues: [] },
      runtimePermissions: [],
      runtimeFieldRules: [],
      savedGeneratedFieldPaths: [],
      savedManualOverridePaths: [],
      unsupported: [],
      isolatedHooks: [],
      allFields: [],
      editableFields: [],
      hiddenFields: [],
		protectedFields: [],
		requiredEditableFields: [],
      readOnly: false,
      loading: false,
      dirty: false,
		removeRequestPolicy: null,
		removeStorageFacade: null,
		stateTimer: null
    }
  },
  mounted () {
    this.parentOrigin = this.getParentOrigin()
    window.addEventListener('message', this.onMessage)
    this.post({ version: FORM_RUNTIME_VERSION, sessionId: '', requestId: 'boot', type: 'ready' })
  },
  beforeDestroy () {
    window.removeEventListener('message', this.onMessage)
    this.destroySession()
  },
  methods: {
    getParentOrigin () {
      try {
        return new URL(document.referrer).origin
      } catch (_) {
        return ''
      }
    },
    form () {
      return this.$refs.generateForm || null
    },
    post (message) {
      if (!this.parentOrigin || window.parent === window) return
      window.parent.postMessage(message, this.parentOrigin)
    },
    async onMessage (event) {
      if (event.source !== window.parent || event.origin !== this.parentOrigin || !isRuntimeCommand(event.data)) return
      const command = event.data
      if (command.type !== 'load' && command.type !== 'destroy' && command.sessionId !== this.sessionId) return
      try {
        await this.execute(command)
      } catch (caught) {
        this.post({
          version: FORM_RUNTIME_VERSION,
          sessionId: command.sessionId,
          requestId: command.requestId,
          type: 'error',
          payload: { message: caught instanceof Error ? caught.message : '表单运行时操作失败' }
        })
      }
    },
    async execute (command) {
      const payload = command.payload || {}
      if (command.type === 'destroy') {
        this.destroySession()
        this.result(command, { destroyed: true })
        return
      }
      if (command.type === 'load') {
        this.destroySession()
        this.sessionId = command.sessionId
        this.loading = true
        this.readOnly = Boolean(payload.readOnly)
        this.renderType = String(payload.renderType || 'formmaking')
        this.vuePage = payload.vuePage || { pageName: '', fields: [], issues: [] }
        this.runtimePermissions = Array.isArray(payload.permissions) ? payload.permissions : []
        this.runtimeFieldRules = Array.isArray(payload.fieldRules) ? payload.fieldRules : []
        const baseURL = String(payload.baseURL || '')
        const targetOrigin = baseURL ? new URL(baseURL).origin : ''
        // 上游 axios 可能已捕获同步源码的旧默认地址；会话环境与请求策略双重收敛到后端核实的当前网关。
        setRuntimeEnvironment({ baseUrl: baseURL, viewFileUrl: targetOrigin })
        this.removeRequestPolicy = installReadOnlyRequestPolicy({
          sid: String(payload.sid || ''),
          baseURL
        })
        // 目标组件继续走 rsh-flow-components 原生 Vuex/axios 链；认证只写当前 iframe 内存适配，销毁会话即清除。
        const runtimeIdentity = {
          token: String(payload.sid || ''), sid: String(payload.sid || ''), userName: String(payload.accountName || ''),
          userId: String(payload.userId || ''), companyId: String(payload.companyId || ''), customerCode: String(payload.customerCode || ''),
          topCompanyId: String(payload.companyId || ''),
          companyName: String(payload.companyName || ''), userDepartmentId: String(payload.departmentId || ''),
          departmentId: String(payload.departmentId || ''), userDepartmentName: String(payload.departmentName || ''),
          currentCompanyName: String(payload.companyName || ''), currentDepartment: String(payload.departmentName || ''),
          currentDepName: String(payload.departmentName || ''), initiatorId: String(payload.userId || ''),
          initiatorName: String(payload.accountName || ''), initiatorCompanyId: String(payload.companyId || ''),
          initiatorCompanyName: String(payload.companyName || ''), initiatorDepartmentId: String(payload.departmentId || ''),
          initiatorDepartmentName: String(payload.departmentName || ''),
          user: {
            id: String(payload.userId || ''), name: String(payload.accountName || ''),
            companyId: String(payload.companyId || ''), departmentId: String(payload.departmentId || ''),
            companyName: String(payload.companyName || ''), departmentName: String(payload.departmentName || '')
          }
        }
        // 模板可能绕过认证工具直接读 native localStorage；这里安装仅存活于 iframe 会话的 facade。
        setRuntimeAuth(runtimeIdentity)
        this.removeStorageFacade = installRuntimeStorageFacade(runtimeIdentity)
        if (window.$store) {
          window.$store.commit('user/SET_TOKEN', String(payload.sid || ''))
          window.$store.commit('user/SET_USER_NAME', String(payload.accountName || ''))
          // 公司/人员选择组件从目标登录上下文读取本地公司树；只装载后端已核实的非空字段，避免空值覆盖既有会话。
          if (payload.userId) window.$store.commit('user/SET_USER_ID', String(payload.userId))
          if (payload.companyId) window.$store.commit('user/SET_COMPANY_ID', String(payload.companyId))
          if (payload.customerCode) window.$store.commit('user/SET_CUSTOMERCODE', String(payload.customerCode))
          if (payload.companyName) window.$store.commit('user/SET_COMPANY_NAME', String(payload.companyName))
          if (payload.departmentName && window.$store._mutations['user/SET_DEPARTMENT_NAME']) window.$store.commit('user/SET_DEPARTMENT_NAME', String(payload.departmentName))
          if (payload.departmentId && window.$store._mutations['user/SET_DEPARTMENTID']) window.$store.commit('user/SET_DEPARTMENTID', String(payload.departmentId))
        }
        const prepared = prepareTemplate(payload.template || {}, payload.permissions || [], this.readOnly, payload.fieldRules || [])
        this.template = prepared.template
        this.unsupported = prepared.unsupported
        this.isolatedHooks = prepared.isolatedHooks
        this.allFields = prepared.allFields
        this.editableFields = prepared.editableFields
        this.hiddenFields = prepared.hiddenFields
        this.protectedFields = prepared.protectedFields
        this.requiredEditableFields = prepared.requiredEditableFields
        this.values = clonePlain(payload.values || {})
        this.savedValues = clonePlain(this.values)
        this.generatedValues = clonePlain(payload.generatedValues || this.values)
        this.generatedFieldPaths = Array.isArray(payload.generatedFieldPaths) ? payload.generatedFieldPaths.map(String) : []
        this.manualOverridePaths = Array.isArray(payload.manualOverridePaths) ? payload.manualOverridePaths.map(String) : []
        this.savedGeneratedFieldPaths = [...this.generatedFieldPaths]
        this.savedManualOverridePaths = [...this.manualOverridePaths]
        await this.$nextTick()
        await this.setData(this.values)
        await this.refresh()
        this.loading = false
        this.result(command, { ready: true, unsupported: this.unsupported, isolatedHooks: this.isolatedHooks, stats: this.stats() })
        return
      }
      if (command.type === 'setData') {
        const nextValues = clonePlain(payload.values || {})
        await this.setData(nextValues)
        this.generatedValues = clonePlain(nextValues)
        this.generatedFieldPaths = Array.isArray(payload.generatedFieldPaths) ? payload.generatedFieldPaths.map(String) : []
        this.manualOverridePaths = Array.isArray(payload.manualOverridePaths) ? payload.manualOverridePaths.map(String) : []
        this.dirty = true
        await this.refresh()
        this.result(command, await this.capture(false))
        return
      }
      if (command.type === 'restore') {
        await this.setData(this.savedValues)
        this.generatedValues = clonePlain(this.savedValues)
        this.generatedFieldPaths = [...this.savedGeneratedFieldPaths]
        this.manualOverridePaths = [...this.savedManualOverridePaths]
        this.dirty = false
        await this.refresh()
        this.result(command, await this.capture(false))
        return
      }
      if (command.type === 'refresh') {
        await this.refresh()
        this.result(command, await this.capture(false))
        return
      }
      if (command.type === 'getValues') {
        this.result(command, await this.capture(false))
        return
      }
      if (command.type === 'validateAndGetValues') this.result(command, await this.capture(true))
    },
    async setData (values) {
      if (this.renderType === 'vue_custom') {
        this.values = clonePlain(values)
        const page = this.$refs.vueHost
        if (!page || typeof page.setData !== 'function') throw new Error('宿主 Vue 业务页面尚未完成装载')
        await page.setData(this.values)
        return
      }
      const form = this.form()
      if (!form || typeof form.setData !== 'function') throw new Error('目标 FormMaking 运行时缺少 setData 能力')
      await form.setData(clonePlain(values))
      this.values = clonePlain(values)
    },
    async refresh () {
      if (this.renderType === 'vue_custom') return
      // 字段权限已在 FormMaking 装载前写入每个组件 options；refresh 后统一调用 disabled 会击穿缺少 disabledElement 的已注册组件。
      await refreshPreparedForm(this.form())
    },
    async capture (validate) {
      if (this.renderType === 'vue_custom') {
        const page = this.$refs.vueHost
        if (!page || typeof page.capture !== 'function') throw new Error('宿主 Vue 业务页面尚未完成装载')
        const values = await page.capture(validate)
        if (validate && this.vuePage.fields.some(field => field.required && this.isEmptyCustomValue(this.customPageValue(values, field.path)))) throw new Error('请先完成表单中的必填项')
        const manualOverridePaths = [...new Set([...this.manualOverridePaths, ...diffManualPaths(this.generatedValues, values)])].sort()
        return { values, validated: validate, unsupported: [], dirty: this.dirty, generatedFieldPaths: this.generatedFieldPaths, manualOverridePaths, stats: this.stats(values, manualOverridePaths) }
      }
      const form = this.form()
      const values = await captureFormValues(form, validate)
      this.values = values
		const manualOverridePaths = [...new Set([...this.manualOverridePaths, ...diffManualPaths(this.generatedValues, values)])].sort()
		return {
        values,
        validated: validate,
        unsupported: this.unsupported,
        dirty: this.dirty,
        generatedFieldPaths: this.generatedFieldPaths,
			manualOverridePaths,
			stats: this.stats(values, manualOverridePaths)
		}
	},
    stats (values = this.values, manualOverridePaths = this.manualOverridePaths) {
      return formRuntimeStats(values, this.generatedFieldPaths, manualOverridePaths, this.editableFields, this.protectedFields, this.requiredEditableFields)
    },
    isEmptyCustomValue (value) {
      return value == null || String(value).trim() === ''
    },
    customPageValue (values, path) {
      return String(path || '').split('.').filter(Boolean).reduce((current, key) => current && typeof current === 'object' ? current[key] : undefined, values)
    },
    markDirty () {
		if (this.loading || this.readOnly) return
		this.dirty = true
		if (this.stateTimer) window.clearTimeout(this.stateTimer)
		this.stateTimer = window.setTimeout(() => { void this.reportState() }, 0)
	},
	async reportState () {
		if (!this.sessionId || this.loading || this.readOnly) return
		try {
			const captured = await this.capture(false)
			this.post({ version: FORM_RUNTIME_VERSION, sessionId: this.sessionId, requestId: 'state', type: 'state', payload: { stats: captured.stats, manualOverridePaths: captured.manualOverridePaths } })
		} catch (_) {
			// 输入过程中的临时组件状态不影响实际保存；下一次稳定变更会重新对账。
		}
    },
    result (command, payload) {
      this.post({ version: FORM_RUNTIME_VERSION, sessionId: command.sessionId, requestId: command.requestId, type: 'result', payload })
    },
	destroySession () {
		if (this.stateTimer) window.clearTimeout(this.stateTimer)
		this.stateTimer = null
      if (typeof this.removeRequestPolicy === 'function') this.removeRequestPolicy()
      this.removeRequestPolicy = null
		if (typeof this.removeStorageFacade === 'function') this.removeStorageFacade()
		this.removeStorageFacade = null
      this.sessionId = ''
      this.template = { list: [], config: {} }
      this.values = {}
      this.savedValues = {}
      this.generatedValues = {}
      this.generatedFieldPaths = []
      this.manualOverridePaths = []
      this.renderType = 'formmaking'
      this.vuePage = { pageName: '', fields: [], issues: [] }
      this.runtimePermissions = []
      this.runtimeFieldRules = []
      this.savedGeneratedFieldPaths = []
      this.savedManualOverridePaths = []
      this.unsupported = []
      this.isolatedHooks = []
      this.allFields = []
      this.editableFields = []
		this.hiddenFields = []
		this.protectedFields = []
		this.requiredEditableFields = []
      this.dirty = false
      this.loading = false
      setRuntimeEnvironment({ baseUrl: '', viewFileUrl: '', onlyOfficeUrl: '' })
      clearRuntimeAuth()
      if (window.$store && window.$store._mutations['user/RESET_STATE']) window.$store.commit('user/RESET_STATE')
      // SID 只存在于请求策略闭包和内存认证适配；销毁会话后不保留到 storage、Vuex 或全局变量。
    }
  }
}
</script>

<style>
html,
body,
#app {
  color-scheme: light;
  min-height: 100%;
  margin: 0;
  color: #262626 !important;
  background: #fff !important;
}

body {
  color: #262626;
  background: #fff;
}

.form-runtime {
  box-sizing: border-box;
  min-height: 100vh;
  padding: 18px 22px 32px;
  color-scheme: light;
  color: #262626;
  background: #fff;
}

.form-runtime__placeholder {
  padding: 48px 20px;
  color: #8c8c8c;
  text-align: center;
}

</style>
