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
      @openCompanyPersonFramwork="openCompanyPersonFramwork"
      @openRelationOrganizationDialog="openRelationOrganizationDialog"
    />
    <host-vue-page v-else-if="sessionId && renderType === 'vue_custom'" ref="vueHost" :page="vuePage" :initial-values="values" :permissions="runtimePermissions" :read-only="readOnly" />
    <div v-else class="form-runtime__placeholder">正在等待表单工作区初始化…</div>
    <IndicatorHeaderDialog
      v-if="indicatorHeaderVisible"
      :visible.sync="indicatorHeaderVisible"
      :fielSelectType="fielSelectType"
      :companyId="companyId"
      :selectUserCompanyId="selectUserCompanyId"
      :departmentId="departmentId"
      :relDepartmentId="relDepartmentId"
      :isRelative="isRelative"
      @selectHeader="selectHeader"
    />
    <RelationOrganizationDialog
      v-if="relationOrganizationVisible"
      :visible.sync="relationOrganizationVisible"
      :fielSelectType="fielSelectType"
      @selectValue="selectValue"
    />
  </main>
</template>

<script>
import { FORM_RUNTIME_VERSION, isRuntimeCommand } from './runtime/protocol'
import { buildValuesEnvelope, captureFormValues, clonePlain, coordinateOptionPatches, formRuntimeStats, hiddenFieldKeys, optionCoordinationIssues, prepareTemplate, refreshPreparedForm, replayFieldChangeEvents } from './runtime/formTemplate'
import { installReadOnlyRequestPolicy } from './runtime/requestPolicy'
import { clearRuntimeAuth, installRuntimeStorageFacade, setRuntimeAuth } from './runtime/memoryAuth'
import { setConfig as setRuntimeEnvironment } from './runtime/runtimeEnvironment'
import HostVuePage from './HostVuePage.vue'
import IndicatorHeaderDialog from '@runtime/components/IndicatorHeaderDialog.vue'
import RelationOrganizationDialog from '@runtime/components/RelationOrganizationDialog.vue'

export default {
  name: 'FormRuntimeApp',
  components: { HostVuePage, IndicatorHeaderDialog, RelationOrganizationDialog },
  data () {
    return {
      parentOrigin: '',
      sessionId: '',
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
      dirty: false,
		removeRequestPolicy: null,
		requestPolicyObservations: [],
		requestPolicyIssues: [],
		runtimeIssues: [],
		indicatorHeaderVisible: false,
		relationOrganizationVisible: false,
		fielSelectType: '',
		companyId: '',
		selectUserCompanyId: '',
		departmentId: '',
		relDepartmentId: '',
		isRelative: true,
		currentField: '',
		currentRowIndex: null,
		currentTable: '',
		setDataProps: '',
		setId: {
			user_name: 'user_id',
			company_name: 'company_id',
			department_name: 'department_id',
			post_name: 'post_id',
			mentor_name: 'mentor_id',
			department_manager_name: 'department_manager_id'
		},
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
		const message = caught instanceof Error ? caught.message : '表单运行时操作失败'
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
        this.loading = true
        this.readOnly = Boolean(payload.readOnly)
        this.renderType = String(payload.renderType || 'formmaking')
		this.vuePage = payload.vuePage || { status: 'blocked', pageName: '', fields: [], issues: [] }
        this.runtimePermissions = Array.isArray(payload.permissions) ? payload.permissions : []
        const baseURL = String(payload.baseURL || '')
        const targetOrigin = baseURL ? new URL(baseURL).origin : ''
        // 上游 axios 可能已捕获同步源码的旧默认地址；会话环境与请求策略双重收敛到后端核实的当前网关。
        setRuntimeEnvironment({ baseUrl: baseURL, viewFileUrl: targetOrigin })
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
		  }
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
        await this.$nextTick()
        await this.setData(this.values)
        await this.refresh()
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
        const nextValues = clonePlain(payload.values || {})
        this.changedFields = Array.isArray(payload.changedFields) ? payload.changedFields.map(String) : this.changedFields
        this.optionPatchTriggers = [...this.changedFields]
        await this.$nextTick()
        await this.setData(nextValues)
        // setData 只用于来源切换后的整份原始值替换；替换成功即成为新的恢复基线，避免恢复按钮回到旧来源快照。
        this.savedValues = clonePlain(nextValues)
        this.dirty = false
        await this.refresh()
        this.savedValues = clonePlain(this.values)
        this.result(command, await this.capture(false))
        return
      }
      if (command.type === 'restore') {
        await this.setData(this.savedValues)
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
        const result = await page.setData(this.values)
        this.runtimeIssues = Array.isArray(result && result.issues) ? result.issues : []
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
      const form = this.form()
      await refreshPreparedForm(form)
      // 选项型字段补丁协调：等待控件自己的远程选项就绪，按名称唯一匹配回填绑定值并重放联动；
      // 无法唯一匹配且显示仍停留在历史值的字段产生阻断问题。
      const coordination = await coordinateOptionPatches(form, this.template, this.values, this.optionPatchTriggers)
      this.values = coordination.values
      this.optionCoordinationIssues = coordination.issues
      await replayFieldChangeEvents(form, this.changedFields)
      if (this.changedFields.length > 0 && typeof form.getValues === 'function') this.values = clonePlain(form.getValues() || {})
      this.changedFields = []
    },
    // normalizeEventData 兼容 FormMaking 事件传对象或 JSON 字符串，确保所有表单组件共享同一事件入口。
    normalizeEventData (data) {
      if (data && typeof data === 'object') return data
      if (typeof data !== 'string' || !data.trim()) return {}
      try {
        const parsed = JSON.parse(data)
        return parsed && typeof parsed === 'object' ? parsed : {}
      } catch (_) {
        return {}
      }
    },
    // openCompanyPersonFramwork 对齐目标 OtherSteps2 的人员、公司、部门、岗位选择事件。
    openCompanyPersonFramwork (data) {
      const event = this.normalizeEventData(data)
      const argument = this.normalizeEventData(event.argument)
      this.selectUserCompanyId = String(event.userCompanyId || '')
      this.fielSelectType = String(event.fielSelectType || '')
      this.companyId = String(this.form() && typeof this.form().getValue === 'function' ? this.form().getValue('currentCompanyId') || event.companyId || '' : event.companyId || '')
      this.departmentId = ''
      this.relDepartmentId = ''
      this.setDataProps = String(event.setData || '')
      this.currentField = String(argument.field || '')
      this.currentRowIndex = Number.isInteger(argument.rowIndex) ? argument.rowIndex : null
      this.currentTable = String(argument.table || argument.group || '')
      if (this.fielSelectType === 'duty' && this.currentTable && this.currentRowIndex !== null) {
        const rows = this.form() && this.form().getValue(this.currentTable)
        this.departmentId = String(rows && rows[this.currentRowIndex] && rows[this.currentRowIndex].department_id || '')
      }
      this.isRelative = event.isRelative !== false
      this.indicatorHeaderVisible = true
    },
    // openRelationOrganizationDialog 打开目标岗位关联选择，并保存本次回写上下文。
    openRelationOrganizationDialog (data) {
      const event = this.normalizeEventData(data)
      const argument = this.normalizeEventData(event.argument)
      this.selectUserCompanyId = String(event.userCompanyId || '')
      this.fielSelectType = String(event.fielSelectType || '')
      this.companyId = String(event.companyId || '')
      this.setDataProps = String(event.setData || '')
      this.currentField = String(argument.field || '')
      this.currentRowIndex = Number.isInteger(argument.rowIndex) ? argument.rowIndex : null
      this.currentTable = String(argument.table || argument.group || '')
      this.relationOrganizationVisible = true
    },
    // setFormValues 只通过 FormMaking 的公开 setData 回写，保持虚拟字段和变更统计一致。
    async setFormValues (values) {
      const form = this.form()
      if (!form || typeof form.setData !== 'function' || !values || typeof values !== 'object') return
      await form.setData(values)
      this.markDirty()
    },
    // selectHeader 将人员、公司或部门选择结果回填到当前字段及其关联 ID 字段。
    selectHeader (data, selectCompany, depart) {
      data = this.normalizeEventData(data)
      selectCompany = this.normalizeEventData(selectCompany)
      depart = this.normalizeEventData(depart)
      if (!Object.keys(data).length) return
      const field = this.currentField
      if (!field) return
      if (field === 'company_dept') {
        const values = data.type === 1 && selectCompany && selectCompany.type === 1
          ? { type: 'company', userName_dept: '', userName_deptid: '', userName_company: data.name, userName_companyid: data.id }
          : { type: 'dep', userName_dept: data.name, userName_deptid: data.id, userName_company: selectCompany && selectCompany.name || '', userName_companyid: selectCompany && selectCompany.id || '' }
        void this.setFormValues(values)
        this.indicatorHeaderVisible = false
        return
      }
      const values = {}
      if (this.currentTable && this.currentRowIndex !== null) {
        const rows = clonePlain(this.form() && this.form().getValue(this.currentTable) || [])
        if (Array.isArray(rows) && rows[this.currentRowIndex]) {
          rows[this.currentRowIndex][field] = data.name
          rows[this.currentRowIndex][`${field}_id`] = data.id
          if (depart) {
            rows[this.currentRowIndex][`${field}_dept`] = depart.name
            rows[this.currentRowIndex][`${field}_deptid`] = depart.id
          }
          if (this.setId[field]) rows[this.currentRowIndex][this.setId[field]] = data.id
          if (field === 'company_name' || field === 'department_name') {
            rows[this.currentRowIndex].post_name = ''
            rows[this.currentRowIndex].post_id = ''
          }
          values[this.currentTable] = rows
        }
      } else {
        values[field] = data.name
        values[`${field}_id`] = data.id
        if (this.fielSelectType === 'company') {
          const prefix = field.split('_')[0]
          values[`${prefix}_name`] = data.name
          values[`${prefix}_id`] = data.id
          values[`${prefix}_dept`] = depart && depart.name || ''
          values[`${prefix}_deptid`] = depart && depart.id || ''
          values[`${prefix}_company`] = selectCompany && selectCompany.name || ''
          values[`${prefix}_companyid`] = selectCompany && selectCompany.id || ''
        } else if (this.fielSelectType === 'department' && field.includes('_dept')) {
          const prefix = field.replace('_dept', '')
          values[prefix] = ''
          values[`${prefix}_id`] = ''
          values[`${prefix}_dept`] = data.name
          values[`${prefix}_deptid`] = data.id
          values[`${prefix}_company`] = selectCompany && selectCompany.name || ''
          values[`${prefix}_companyid`] = selectCompany && selectCompany.id || ''
        }
        if (this.setId[field]) values[this.setId[field]] = data.id
      }
      void this.setFormValues(values)
      this.indicatorHeaderVisible = false
    },
    // selectValue 将岗位、部门和公司三联结果回填到目标表单的标准字段。
    selectValue (post, department, company) {
      post = this.normalizeEventData(post)
      department = this.normalizeEventData(department)
      company = this.normalizeEventData(company)
      if (!Object.keys(post).length || !Object.keys(department).length || !Object.keys(company).length) return
      if (this.currentField === 'post_name') {
        void this.setFormValues({ post_name: post.name, post_id: post.id, department_name: department.name, department_id: department.id, company_name: company.name, company_id: company.id })
      } else if (this.currentField === 'company_department_post_name') {
        void this.setFormValues({ company_department_post_name: `${company.name}/${department.name}/${post.name}`, post_id: post.id, department_id: department.id, company_name: company.name, department_name: department.name })
      }
      this.relationOrganizationVisible = false
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
		this.indicatorHeaderVisible = false
		this.relationOrganizationVisible = false
		this.fielSelectType = ''
		this.companyId = ''
		this.selectUserCompanyId = ''
		this.departmentId = ''
		this.relDepartmentId = ''
		this.isRelative = true
		this.currentField = ''
		this.currentRowIndex = null
		this.currentTable = ''
		this.setDataProps = ''
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
