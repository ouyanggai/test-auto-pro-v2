<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'

import type { PathFormConfiguration, PathFormRuntimeSession } from './types'

const FORM_RUNTIME_VERSION = 'f007-form-runtime/v1'

const props = defineProps<{
  form: PathFormConfiguration
  runtimeSession: PathFormRuntimeSession
}>()
const emit = defineEmits<{
  ready: [payload: Record<string, unknown>]
  state: [payload: Record<string, unknown>]
  error: [message: string]
}>()

const iframe = ref<HTMLIFrameElement | null>(null)
const sessionId = ref(crypto.randomUUID())
const iframeSource = computed(() => import.meta.env.DEV ? 'http://127.0.0.1:19001/form-runtime/#/test-auto-form' : '/form-runtime/#/test-auto-form')
const runtimeOrigin = computed(() => new URL(iframeSource.value, window.location.href).origin)
const pending = new Map<string, { resolve: (payload: Record<string, unknown>) => void, reject: (error: Error) => void, timer: number }>()
let disposed = false
let runtimeActive = false
let runtimeGeneration = 0
let iframeBootPending = true

// plainPayload 在 postMessage 前移除 Vue Proxy；不使用 structuredClone 处理响应式对象。
function plainPayload(value: unknown): Record<string, unknown> {
  return JSON.parse(JSON.stringify(value ?? {})) as Record<string, unknown>
}

// postCommand 绑定当前 iframe、会话、请求号和协议版本，迟到响应无法串到新路径。
function postCommand(type: string, payload: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
  const target = iframe.value?.contentWindow
  if (!target || disposed || !runtimeActive) return Promise.reject(new Error('表单运行时尚未就绪'))
  const requestId = crypto.randomUUID()
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => {
      pending.delete(requestId)
      reject(new Error('表单运行时响应超时，当前表单数据未丢失'))
    }, 15_000)
    pending.set(requestId, { resolve, reject, timer })
    target.postMessage({ version: FORM_RUNTIME_VERSION, sessionId: sessionId.value, requestId, type, payload: plainPayload(payload) }, runtimeOrigin.value)
  })
}

// loadRuntime 只把 SID 传给当前 iframe 内存会话，并装载完整模板、权限、字段规则和 values。
async function loadRuntime(): Promise<Record<string, unknown>> {
  if (disposed) return {}
  const generation = ++runtimeGeneration
  runtimeActive = true
  try {
    const payload = await postCommand('load', {
      sid: props.runtimeSession.sid,
      baseURL: props.runtimeSession.baseURL,
      accountName: props.runtimeSession.accountName,
      userId: props.runtimeSession.userId,
      companyId: props.runtimeSession.companyId,
      customerCode: props.runtimeSession.customerCode,
      companyName: props.runtimeSession.companyName,
      readOnly: props.form.readOnly,
      template: props.form.template,
      permissions: props.form.permissions,
      fieldRules: props.form.fieldRules,
      values: props.form.values,
      generatedValues: props.form.values,
      generatedFieldPaths: props.form.generatedFieldPaths,
      manualOverridePaths: props.form.manualOverridePaths,
    })
    if (disposed || !runtimeActive || generation !== runtimeGeneration) return {}
    emit('ready', payload)
    return payload
  }
  catch (caught) {
    if (disposed || !runtimeActive || generation !== runtimeGeneration) return {}
    resetRuntime(false)
    if (!disposed) emit('error', caught instanceof Error ? caught.message : '表单运行时加载失败')
    return {}
  }
}

// handleMessage 严格核对 origin、source、版本、会话和请求号。
function handleMessage(event: MessageEvent) {
  if (event.origin !== runtimeOrigin.value || event.source !== iframe.value?.contentWindow) return
  const message = event.data as { version?: string, sessionId?: string, requestId?: string, type?: string, payload?: Record<string, unknown> }
  if (message.version !== FORM_RUNTIME_VERSION) return
  if (message.type === 'ready' && message.requestId === 'boot') {
    if (disposed || runtimeActive || !iframeBootPending) return
    iframeBootPending = false
    void loadRuntime()
    return
  }
  if (message.sessionId !== sessionId.value || !message.requestId) return
  if (message.type === 'state') {
    if (runtimeActive && !disposed) emit('state', message.payload || {})
    return
  }
  const request = pending.get(message.requestId)
  if (!request) return
  window.clearTimeout(request.timer)
  pending.delete(message.requestId)
  if (message.type === 'error') request.reject(new Error(String(message.payload?.message || '表单运行时操作失败')))
  else request.resolve(message.payload || {})
}

// setGeneratedData 把服务端生成结果交给真实 FormMaking setData/refresh。
function setGeneratedData(values: Record<string, unknown>, generatedFieldPaths: string[], manualOverridePaths: string[]) {
  return postCommand('setData', { values, generatedFieldPaths, manualOverridePaths })
}

// reloadRuntime 在新字段规则到达后销毁旧组件实例，再在真实组件创建前应用最新规则和 values。
async function reloadRuntime() {
  destroyRuntime()
  return loadRuntime()
}

// restoreSaved 恢复本次载入时的已保存值。
function restoreSaved() {
  return postCommand('restore')
}

// getValues 不触发必填校验，用于换一组前捕获人工修改和生成器所有权。
function getValues() {
  return postCommand('getValues')
}

// validateAndGetValues 先执行 getData(true)，再抓取包含虚拟字段的 getValues。
function validateAndGetValues() {
  return postCommand('validateAndGetValues')
}

// resetRuntime 统一终止当前会话并拒绝待处理请求；同一会话重复调用不会再次操作 iframe。
function resetRuntime(notifyFrame: boolean) {
  if (!runtimeActive && pending.size === 0) return
  if (notifyFrame && iframe.value?.contentWindow && !disposed) {
    iframe.value.contentWindow.postMessage({
      version: FORM_RUNTIME_VERSION, sessionId: sessionId.value, requestId: crypto.randomUUID(), type: 'destroy', payload: {},
    }, runtimeOrigin.value)
  }
  runtimeGeneration += 1
  runtimeActive = false
  sessionId.value = crypto.randomUUID()
  for (const request of pending.values()) {
    window.clearTimeout(request.timer)
    request.reject(new Error('表单工作区已经关闭'))
  }
  pending.clear()
}

// destroyRuntime 是对外暴露的幂等 teardown；父页面不需要也不应在子组件卸载时重复调用。
function destroyRuntime() {
  resetRuntime(true)
}

watch(() => [props.form.revision, props.runtimeSession.sid], () => {
  if (!iframe.value?.contentWindow) return
  destroyRuntime()
  void loadRuntime()
})

window.addEventListener('message', handleMessage)
onBeforeUnmount(() => {
  destroyRuntime()
  disposed = true
  window.removeEventListener('message', handleMessage)
})

defineExpose({ setGeneratedData, reloadRuntime, restoreSaved, getValues, validateAndGetValues, destroyRuntime })
</script>

<template>
  <iframe
    ref="iframe"
    class="form-runtime-frame"
    :src="iframeSource"
    title="真实 FormMaking 表单数据工作区"
    sandbox="allow-scripts allow-forms allow-same-origin allow-popups"
  />
</template>

<style scoped>
.form-runtime-frame {
  display: block;
  width: 100%;
  height: 100%;
  min-height: 0;
  background: var(--path-config-card-color);
  border: 0;
}
</style>
