<template>
  <main class="form-runtime" :aria-busy="loading">
    <el-alert
      v-if="unsupported.length"
      type="warning"
      :closable="false"
      show-icon
      class="form-runtime__unsupported"
      :title="`当前模板有 ${unsupported.length} 项需要宿主适配，不能保存为可执行配置`"
      :description="unsupported.join('；')"
    />
    <fm-generate-form
      v-if="sessionId"
      ref="generateForm"
      :key="sessionId"
      :data="template"
      :value="values"
      :edit="!readOnly"
      @on-change="markDirty"
    />
    <div v-else class="form-runtime__placeholder">正在等待表单工作区初始化…</div>
  </main>
</template>

<script>
import { FORM_RUNTIME_VERSION, isRuntimeCommand } from './runtime/protocol'
import { clonePlain, prepareTemplate, diffManualPaths } from './runtime/formTemplate'
import { installReadOnlyRequestPolicy } from './runtime/requestPolicy'

export default {
  name: 'FormRuntimeApp',
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
      savedGeneratedFieldPaths: [],
      savedManualOverridePaths: [],
      unsupported: [],
      readOnly: false,
      loading: false,
      dirty: false,
      removeRequestPolicy: null
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
        this.removeRequestPolicy = installReadOnlyRequestPolicy({
          sid: String(payload.sid || ''),
          baseURL: String(payload.baseURL || '')
        })
        const prepared = prepareTemplate(payload.template || {}, payload.permissions || [], this.readOnly)
        this.template = prepared.template
        this.unsupported = prepared.unsupported
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
        this.result(command, { ready: true, unsupported: this.unsupported })
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
      const form = this.form()
      if (!form || typeof form.setData !== 'function') throw new Error('目标 FormMaking 运行时缺少 setData 能力')
      await form.setData(clonePlain(values))
      this.values = clonePlain(values)
    },
    async refresh () {
      const form = this.form()
      if (!form || typeof form.refresh !== 'function') throw new Error('目标 FormMaking 运行时缺少 refresh 能力')
      await form.refresh()
    },
    async capture (validate) {
      const form = this.form()
      if (!form) throw new Error('目标 FormMaking 尚未就绪')
      if (validate) {
        try {
          await form.getData(true)
        } catch (_) {
          throw new Error('请先完成表单中的必填项')
        }
      }
      if (typeof form.getValues !== 'function') throw new Error('目标 FormMaking 运行时缺少 getValues 能力')
      const values = clonePlain(form.getValues())
      this.values = values
      return {
        values,
        validated: validate,
        unsupported: this.unsupported,
        dirty: this.dirty,
        generatedFieldPaths: this.generatedFieldPaths,
        manualOverridePaths: [...new Set([...this.manualOverridePaths, ...diffManualPaths(this.generatedValues, values)])].sort()
      }
    },
    markDirty () {
      if (!this.loading && !this.readOnly) this.dirty = true
    },
    result (command, payload) {
      this.post({ version: FORM_RUNTIME_VERSION, sessionId: command.sessionId, requestId: command.requestId, type: 'result', payload })
    },
    destroySession () {
      if (typeof this.removeRequestPolicy === 'function') this.removeRequestPolicy()
      this.removeRequestPolicy = null
      this.sessionId = ''
      this.template = { list: [], config: {} }
      this.values = {}
      this.savedValues = {}
      this.generatedValues = {}
      this.generatedFieldPaths = []
      this.manualOverridePaths = []
      this.savedGeneratedFieldPaths = []
      this.savedManualOverridePaths = []
      this.unsupported = []
      this.dirty = false
      this.loading = false
      // SID 只存在于请求策略闭包；销毁会话后不保留到 storage、Vuex 或全局变量。
    }
  }
}
</script>

<style>
html,
body,
#app {
  min-height: 100%;
  margin: 0;
}

body {
  color: #262626;
  background: #fff;
}

.form-runtime {
  box-sizing: border-box;
  min-height: 100vh;
  padding: 18px 22px 32px;
}

.form-runtime__unsupported {
  margin-bottom: 16px;
}

.form-runtime__placeholder {
  padding: 48px 20px;
  color: #8c8c8c;
  text-align: center;
}

@media (prefers-color-scheme: dark) {
  body { color: #e5eaf0; background: #18181c; }
}
</style>
