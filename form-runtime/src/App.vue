<template>
  <main class="form-runtime" :aria-busy="loading">
    <router-view
      v-if="sessionId && renderType === 'formmaking'"
      ref="formHost"
      :key="sessionId"
    />
    <host-vue-page v-else-if="sessionId && renderType === 'vue_custom'" ref="vueHost" :page="vuePage" :initial-values="values" :permissions="runtimePermissions" :read-only="readOnly" />
    <div v-else class="form-runtime__placeholder">正在等待表单工作区初始化…</div>
    <section v-if="loading" class="form-runtime__loading" role="status" aria-live="polite">
      <span class="form-runtime__spinner" aria-hidden="true" />
      <strong>{{ loadingMessage }}</strong>
      <span v-if="pendingTargetRequests > 0" class="form-runtime__loading-detail">正在等待 {{ pendingTargetRequests }} 个表单数据请求完成</span>
    </section>
  </main>
</template>

<script>
import { FORM_RUNTIME_VERSION, isRuntimeCommand } from './runtime/protocol'
import { buildValuesEnvelope, captureFormValues, clonePlain, coordinateOptionPatches, formRuntimeStats, formValuesFingerprint, hiddenFieldKeys, optionCoordinationIssues, prepareTemplate, refreshPreparedForm, replayFieldChangeEvents } from './runtime/formTemplate'
import { createTargetRequestTracker, installReadOnlyRequestPolicy } from './runtime/requestPolicy'
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
		sessionGeneration: 0,
		operationGeneration: 0,
      template: { list: [], config: {} },
      values: {},
      savedValues: {},
      renderType: 'formmaking',
      vuePage: { status: 'blocked', pageName: '', fields: [], issues: [] },
      runtimePermissions: [],
      unsupported: [],
      isolatedHooks: [],
      allFields: [],
      editableFields: [],
      hiddenFields: [],
      requiredEditableFields: [],
      changedFields: [],
      readOnly: false,
      loading: false,
		loadingMessage: '正在加载表单结构和历史数据',
		pendingTargetRequests: 0,
		requestTracker: null,
      dirty: false,
		removeRequestPolicy: null,
		requestPolicyObservations: [],
		requestPolicyIssues: [],
		runtimeIssues: [],
		companyId: '',
		boundForm: null,
		formChangeHandler: null,
		removeStorageFacade: null,
		stateTimer: null,
		// 选项型字段补丁协调：补丁触及的字段集合与最近一次协调产生的阻断问题。
		optionPatchTriggers: [],
		optionCoordinationIssues: []
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
      const host = this.$refs.formHost
      return host && host.$refs && host.$refs.generateForm ? host.$refs.generateForm : null
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
		if (caught && (caught.code === 'FORM_RUNTIME_REQUEST_CANCELLED' || caught.code === 'FORM_RUNTIME_SESSION_SUPERSEDED')) return
		const message = caught instanceof Error ? caught.message : '表单运行时操作失败'
		if (command.sessionId === this.sessionId) this.loading = false
        this.post({
          version: FORM_RUNTIME_VERSION,
          sessionId: command.sessionId,
          requestId: command.requestId,
          type: 'error',
		  payload: {
			message, renderType: this.renderType,
			issues: this.combinedIssues([{
			  code: 'runtime_command_failed', status: 'blocked', source: 'iframe_runtime', fieldPath: '', fieldLabel: '',
			  operator: command.type, expected: '命令执行成功', actual: message, relatedFields: [], message, canRetry: true
			}])
		  }
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
		this.operationGeneration++
        this.loading = true
		this.loadingMessage = '正在加载表单结构和历史数据'
        this.readOnly = Boolean(payload.readOnly)
        this.renderType = String(payload.renderType || 'formmaking')
		this.vuePage = payload.vuePage || { status: 'blocked', pageName: '', fields: [], issues: [] }
		this.companyId = String(payload.companyId || '')
        this.runtimePermissions = Array.isArray(payload.permissions) ? payload.permissions : []
        const baseURL = String(payload.baseURL || '')
        const targetOrigin = baseURL ? new URL(baseURL).origin : ''
        // 上游 axios 可能已捕获同步源码的旧默认地址；会话环境与请求策略双重收敛到后端核实的当前网关。
        setRuntimeEnvironment({ baseUrl: baseURL, viewFileUrl: targetOrigin })
        this.requestTracker = createTargetRequestTracker({
		  onChange: pending => { this.pendingTargetRequests = pending }
		})
		const loadContext = this.activeSessionContext()
        this.removeRequestPolicy = installReadOnlyRequestPolicy({
          sid: String(payload.sid || ''),
          baseURL,
		  readRequestManifest: payload.readRequestManifest,
		  shadowContext: { renderType: this.renderType, componentName: String(this.vuePage.componentName || '') },
		  onDecision: observation => {
			// 影子数据只保留无敏感判定摘要并设置上限，P0 不上传、不持久化也不改变请求结果。
			this.requestPolicyObservations = [...this.requestPolicyObservations, observation].slice(-200)
		  },
		  onIssue: issue => {
			this.requestPolicyIssues = this.mergeIssues(this.requestPolicyIssues, [issue]).slice(-50)
		  },
		  requestTracker: this.requestTracker
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
        const prepared = prepareTemplate(payload.template || {}, payload.permissions || [], this.readOnly, {
          companyId: payload.companyId,
          companyName: payload.companyName,
          departmentId: payload.departmentId,
          departmentName: payload.departmentName,
          userId: payload.userId,
          accountName: payload.accountName,
          customerCode: payload.customerCode
        })
        this.template = prepared.template
        this.unsupported = prepared.unsupported
        this.isolatedHooks = prepared.isolatedHooks
        this.allFields = prepared.allFields
        this.editableFields = prepared.editableFields
        this.hiddenFields = prepared.hiddenFields
        this.requiredEditableFields = prepared.requiredEditableFields
        this.changedFields = Array.isArray(payload.changedFields) ? payload.changedFields.map(String) : []
        this.optionPatchTriggers = [...this.changedFields]
        this.values = clonePlain(payload.values || {})
        this.savedValues = clonePlain(this.values)
        this.loadingMessage = '正在加载表单组件和远程选项'
        await this.$nextTick()
		this.assertActiveSession(loadContext)
        if (this.renderType === 'formmaking') {
          const host = this.$refs.formHost
          if (!host) throw new Error('目标表单宿主尚未完成装载')
          // 适配层只注入目标宿主已有的数据模型和权限，不复制宿主组件或业务交互。
          host.enableData = [...this.editableFields]
          host.disabledData = this.allFields.filter(field => !this.editableFields.includes(field))
          host.jsonData = clonePlain(this.template)
          host.editData = clonePlain(this.values)
          await this.$nextTick()
		  this.assertActiveSession(loadContext)
          this.bindFormChange()
          const form = this.form()
          if (!form || typeof form.setData !== 'function') throw new Error('目标 FormMaking 运行时缺少 setData 能力')
          await form.setData(clonePlain(this.values))
		  this.assertActiveSession(loadContext)
          // runtime-source 宿主监听 editData 会异步 refresh，必须在该刷新完成后再落一次回放值。
          await this.$nextTick()
		  this.assertActiveSession(loadContext)
          await form.setData(clonePlain(this.values))
		  this.assertActiveSession(loadContext)
        } else {
		  await this.setData(this.values, loadContext)
        }
		await this.refresh(loadContext)
		this.assertActiveSession(loadContext)
        this.savedValues = clonePlain(this.values)
        this.loading = false
        this.result(command, {
		  ready: true, renderType: this.renderType,
		  unsupported: this.unsupported, isolatedHooks: this.isolatedHooks,
		  issues: this.combinedIssues(), stats: this.stats()
		})
        return
      }
      if (command.type === 'setData') {
		const commandContext = this.beginOperation()
        const nextValues = clonePlain(payload.values || {})
        this.changedFields = Array.isArray(payload.changedFields) ? payload.changedFields.map(String) : this.changedFields
        this.optionPatchTriggers = [...this.changedFields]
        await this.$nextTick()
		this.assertActiveSession(commandContext)
		await this.setData(nextValues, commandContext)
        // setData 只用于来源切换后的整份原始值替换；替换成功即成为新的恢复基线，避免恢复按钮回到旧来源快照。
        this.savedValues = clonePlain(nextValues)
        this.dirty = false
		await this.refresh(commandContext)
        this.savedValues = clonePlain(this.values)
        this.result(command, await this.capture(false))
        return
      }
      if (command.type === 'restore') {
		const commandContext = this.beginOperation()
		await this.setData(this.savedValues, commandContext)
        this.dirty = false
		await this.refresh(commandContext)
        this.result(command, await this.capture(false))
        return
      }
      if (command.type === 'refresh') {
        const commandContext = this.beginOperation()
        await this.refresh(commandContext)
        this.result(command, await this.capture(false))
        return
      }
      if (command.type === 'getValues') {
        this.result(command, await this.capture(false))
        return
      }
      if (command.type === 'validateAndGetValues') this.result(command, await this.capture(true))
    },
	activeSessionContext () {
	  return { sessionId: this.sessionId, generation: this.sessionGeneration, operation: this.operationGeneration, requestTracker: this.requestTracker }
	},
	beginOperation () {
	  this.operationGeneration++
	  return this.activeSessionContext()
	},
	assertActiveSession (context) {
	  if (!context || context.sessionId === this.sessionId && context.generation === this.sessionGeneration && context.operation === this.operationGeneration) return
	  const error = new Error('当前表单操作已被新的路径或数据来源替换')
	  error.code = 'FORM_RUNTIME_SESSION_SUPERSEDED'
	  throw error
	},
    async setData (values, context) {
	  this.assertActiveSession(context)
      if (this.renderType === 'vue_custom') {
        this.values = clonePlain(values)
        const page = this.$refs.vueHost
        if (!page || typeof page.setData !== 'function') throw new Error('宿主 Vue 业务页面尚未完成装载')
        const result = await page.setData(this.values)
		this.assertActiveSession(context)
        this.runtimeIssues = Array.isArray(result && result.issues) ? result.issues : []
        return
      }
      const host = this.$refs.formHost
      if (host) {
        host.editData = clonePlain(values)
        const form = this.form()
        if (!form || typeof form.setData !== 'function') throw new Error('目标 FormMaking 运行时缺少 setData 能力')
        await form.setData(clonePlain(values))
		this.assertActiveSession(context)
        await this.$nextTick()
		this.assertActiveSession(context)
        await form.setData(clonePlain(values))
		this.assertActiveSession(context)
        this.bindFormChange()
        this.values = clonePlain(values)
        return
      }
      const form = this.form()
      if (!form || typeof form.setData !== 'function') throw new Error('目标 FormMaking 运行时缺少 setData 能力')
      await form.setData(clonePlain(values))
	  this.assertActiveSession(context)
      this.values = clonePlain(values)
    },
	async refresh (context) {
      if (this.renderType === 'vue_custom') return
	  const refreshContext = context || this.activeSessionContext()
	  this.assertActiveSession(refreshContext)
	  const requestTracker = refreshContext.requestTracker
      // 字段权限已在 FormMaking 装载前写入每个组件 options；refresh 后统一调用 disabled 会击穿缺少 disabledElement 的已注册组件。
      const form = this.form()
	  if (this.loading) this.loadingMessage = '正在加载表单组件和远程选项'
      await refreshPreparedForm(form)
	  this.assertActiveSession(refreshContext)
	  // FormMaking.refresh() 在内部只启动自动数据源，并不会等待请求结束。所有补丁协调必须越过这道真实网络完成屏障。
	  await this.waitForTargetRequests(requestTracker)
	  this.assertActiveSession(refreshContext)
      // 选项型字段补丁协调：等待控件自己的远程选项就绪，按名称唯一匹配回填绑定值并重放联动；
      // 无法唯一匹配且显示仍停留在历史值的字段产生阻断问题。
      const coordination = await coordinateOptionPatches(
		form,
		this.template,
		this.values,
		this.optionPatchTriggers,
		undefined,
		async () => {
		  await this.waitForTargetRequests(requestTracker)
		  this.assertActiveSession(refreshContext)
		}
	  )
	  this.assertActiveSession(refreshContext)
      if (formValuesFingerprint(coordination.values) !== formValuesFingerprint(this.values)) {
        // 协调回填的绑定值必须同时落到宿主的 editData：runtime-source 宿主监听 editData 会异步 refresh，
        // 只改 FormMaking 模型的话，那次刷新会按旧 editData 重新渲染控件，界面又退回历史选项，
        // 随后的捕获也会把旧值读回去——这正是"右侧提示已改、表单还显示旧分类"的成因。
		await this.setData(coordination.values, refreshContext)
      } else {
        this.values = coordination.values
      }
      await replayFieldChangeEvents(form, this.changedFields)
	  this.assertActiveSession(refreshContext)
	  await this.waitForTargetRequests(requestTracker)
	  this.assertActiveSession(refreshContext)
	  if (this.loading) this.loadingMessage = '正在自动修正关键路径数据'
	  if (this.loading) await this.$nextTick()
	  // 写入 host.editData 会触发目标宿主自己的异步 refresh；上面等它及字段联动全部结束后，
	  // 再以服务端补丁目标名称和表单实时绑定值做最后一次协调。最终结果直接落 FormMaking，避免再次触发宿主刷新形成覆盖循环。
	  const finalCoordination = await coordinateOptionPatches(
		form,
		this.template,
		coordination.values,
		this.optionPatchTriggers,
		undefined,
		async () => {
		  await this.waitForTargetRequests(requestTracker)
		  this.assertActiveSession(refreshContext)
		}
	  )
	  this.assertActiveSession(refreshContext)
	  this.optionCoordinationIssues = finalCoordination.issues
	  this.values = finalCoordination.values
      this.changedFields = []
    },
	// waitForTargetRequests 等待目标请求归零并保持稳定，涵盖响应回调继续触发的链式数据源。
	async waitForTargetRequests (requestTracker = this.requestTracker) {
	  if (!requestTracker || typeof requestTracker.waitForIdle !== 'function') return
	  await requestTracker.waitForIdle()
	},
    // bindFormChange 监听 runtime-source 宿主里的 FormMaking 实例，继续沿用 iframe 状态回报协议。
    bindFormChange () {
      const form = this.form()
      if (!form || typeof form.$on !== 'function' || this.boundForm === form) return
      this.unbindFormChange()
      this.boundForm = form
      this.formChangeHandler = () => this.markDirty()
      form.$on('on-change', this.formChangeHandler)
    },
    // unbindFormChange 在切换路径或销毁会话前解除运行时实例监听，避免旧表单继续污染新会话状态。
    unbindFormChange () {
      if (this.boundForm && this.formChangeHandler && typeof this.boundForm.$off === 'function') {
        this.boundForm.$off('on-change', this.formChangeHandler)
      }
      this.boundForm = null
      this.formChangeHandler = null
    },
    async capture (validate) {
      if (this.renderType === 'vue_custom') {
        const page = this.$refs.vueHost
        if (!page || typeof page.capture !== 'function') throw new Error('宿主 Vue 业务页面尚未完成装载')
        const captured = await page.capture(validate)
        const values = captured.values
        this.runtimeIssues = Array.isArray(captured.issues) ? captured.issues : []
        return buildValuesEnvelope({
		  values, validated: validate, unsupported: [], dirty: this.dirty,
		  issues: this.combinedIssues(), renderType: this.renderType,
		  stats: this.stats(values)
		})
      }
      const form = this.form()
      const values = await captureFormValues(form, validate)
      this.values = values
      this.optionCoordinationIssues = optionCoordinationIssues(form, this.template, this.values, this.optionPatchTriggers)
		return buildValuesEnvelope({
        values, validated: validate, unsupported: this.unsupported, dirty: this.dirty,
		issues: this.combinedIssues(), renderType: this.renderType,
		stats: this.stats(values)
		})
	},
	mergeIssues (...groups) {
	  const seen = new Set()
	  const result = []
	  for (const issue of groups.flatMap(group => Array.isArray(group) ? group : [])) {
		if (!issue || typeof issue !== 'object') continue
		const key = [issue.code, issue.status, issue.source, issue.fieldPath, issue.actual, issue.message].map(value => JSON.stringify(value ?? '')).join('|')
		if (seen.has(key)) continue
		seen.add(key)
		result.push(issue)
	  }
	  return result
	},
	combinedIssues (additional = []) {
	  return this.mergeIssues(this.requestPolicyIssues, this.runtimeIssues, this.optionCoordinationIssues, additional)
	},
    // stats 返回当前原始值在可编辑字段中的填写统计，供宿主展示人工待处理数量。
    stats (values = this.values) {
      // 隐藏字段（静态隐藏容器或联动隐藏区域）不计入填写统计，避免其他合同类型封面页必填项被误报成待手工。
      return formRuntimeStats(values, this.editableFields, this.requiredEditableFields, hiddenFieldKeys(this.form(), this.template, this.hiddenFields))
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
			// 状态回报同时携带阻断问题，宿主面板可以实时展示选项协调等待人工处理的字段。
			this.post({
				version: FORM_RUNTIME_VERSION, sessionId: this.sessionId, requestId: 'state', type: 'state',
				payload: { stats: captured.stats, issues: captured.issues.filter(issue => issue.status === 'blocked' || issue.blocking === true) },
			})
		} catch (_) {
			// 输入过程中的临时组件状态不影响实际保存；下一次稳定变更会重新对账。
		}
    },
    result (command, payload) {
      this.post({ version: FORM_RUNTIME_VERSION, sessionId: command.sessionId, requestId: command.requestId, type: 'result', payload })
    },
	destroySession () {
		this.sessionGeneration++
		this.operationGeneration++
		if (this.stateTimer) window.clearTimeout(this.stateTimer)
		this.stateTimer = null
		this.unbindFormChange()
      if (typeof this.removeRequestPolicy === 'function') this.removeRequestPolicy()
      this.removeRequestPolicy = null
	  if (this.requestTracker && typeof this.requestTracker.dispose === 'function') this.requestTracker.dispose()
	  this.requestTracker = null
	  this.pendingTargetRequests = 0
		if (typeof this.removeStorageFacade === 'function') this.removeStorageFacade()
		this.removeStorageFacade = null
      this.sessionId = ''
      this.template = { list: [], config: {} }
      this.values = {}
      this.savedValues = {}
      this.renderType = 'formmaking'
      this.vuePage = { pageName: '', fields: [], issues: [] }
      this.runtimePermissions = []
      this.unsupported = []
      this.isolatedHooks = []
      this.allFields = []
      this.editableFields = []
		this.hiddenFields = []
		this.requiredEditableFields = []
		this.requestPolicyObservations = []
		this.requestPolicyIssues = []
      this.runtimeIssues = []
      this.optionPatchTriggers = []
      this.optionCoordinationIssues = []
		this.companyId = ''
      this.dirty = false
      this.loading = false
	  this.loadingMessage = '正在加载表单结构和历史数据'
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
  position: relative;
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

.form-runtime__loading {
  position: absolute;
  z-index: 1000;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  min-height: 320px;
  color: #262626;
  background: rgba(255, 255, 255, 0.94);
}

.form-runtime__spinner {
  width: 30px;
  height: 30px;
  border: 3px solid #d9f2e3;
  border-top-color: #13a05f;
  border-radius: 50%;
  animation: form-runtime-spin 0.8s linear infinite;
}

.form-runtime__loading-detail {
  color: #8c8c8c;
  font-size: 13px;
}

@keyframes form-runtime-spin {
  to { transform: rotate(360deg); }
}

</style>
