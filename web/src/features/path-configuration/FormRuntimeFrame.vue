<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'

import { classifyRuntimeMessage, FORM_RUNTIME_VERSION, type RuntimeMessage } from './runtimeProtocol'
import type { PathConfigurationDataWorkspace, PathFormRuntimeSession } from './types'

const props = defineProps<{
  form: PathConfigurationDataWorkspace & { readOnly?: boolean }
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
const pending = new Map<string, { resolve: (payload: Record<string, unknown>) => void, reject: (error: Error) => void, timer?: number, cleanup: () => void }>()
let disposed = false
let runtimeActive = false
let runtimeGeneration = 0
let iframeBootPending = true

// plainPayload 在 postMessage 前移除 Vue Proxy；不使用 structuredClone 处理响应式对象。
function plainPayload(value: unknown): Record<string, unknown> {
  return JSON.parse(JSON.stringify(value ?? {})) as Record<string, unknown>
}

// postCommand 绑定当前 iframe、会话、请求号和协议版本，迟到响应无法串到新路径。
function postCommand(type: string, payload: Record<string, unknown> = {}, signal?: AbortSignal, timeoutMs = 15_000): Promise<Record<string, unknown>> {
  // timeoutMs <= 0 表示不设超时：宿主页面装载要等目标表单自身的异步初始化与数据源请求，
  // 目标服务抖动时耗时不可预估；真失败会由 iframe 以 error 消息回报，离开页面由 AbortController 兜底。
  const target = iframe.value?.contentWindow
  if (!target || disposed || !runtimeActive) return Promise.reject(new Error('表单运行时尚未就绪'))
  if (signal?.aborted) return Promise.reject(signal.reason instanceof Error ? signal.reason : new DOMException('操作已取消', 'AbortError'))
  const requestId = crypto.randomUUID()
  return new Promise((resolve, reject) => {
    const abort = () => {
      const request = pending.get(requestId)
      if (!request) return
      window.clearTimeout(request.timer)
      request.cleanup()
      pending.delete(requestId)
      reject(signal?.reason instanceof Error ? signal.reason : new DOMException('操作已取消', 'AbortError'))
    }
    const cleanup = () => signal?.removeEventListener('abort', abort)
    let timer: number | undefined
    if (timeoutMs > 0) {
      timer = window.setTimeout(() => {
        cleanup()
        pending.delete(requestId)
        reject(new Error('表单运行时响应超时，当前表单数据未丢失'))
      }, timeoutMs)
    }
    if (timer === undefined) {
      pending.set(requestId, { resolve, reject, cleanup })
    } else {
      pending.set(requestId, { resolve, reject, timer, cleanup })
    }
    signal?.addEventListener('abort', abort, { once: true })
    target.postMessage({ version: FORM_RUNTIME_VERSION, sessionId: sessionId.value, requestId, type, payload: plainPayload(payload) }, runtimeOrigin.value)
  })
}

// loadRuntime 只把 SID 传给当前 iframe 内存会话，并按目标 runtime 协议装载原始模板、权限、页面和 values。
async function loadRuntime(): Promise<Record<string, unknown>> {
  if (disposed) return {}
  const generation = ++runtimeGeneration
  runtimeActive = true
  try {
    // 宿主页面装载包含目标表单自身的异步初始化与数据源请求，超时预算比其他命令更长。
    const payload = await postCommand('load', {
      sid: props.runtimeSession.sid,
      baseURL: props.runtimeSession.baseURL,
      accountName: props.runtimeSession.accountName,
      userId: props.runtimeSession.userId,
      companyId: props.runtimeSession.companyId,
      customerCode: props.runtimeSession.customerCode,
      companyName: props.runtimeSession.companyName,
      departmentId: props.runtimeSession.departmentId,
      departmentName: props.runtimeSession.departmentName,
      readOnly: props.form.readOnly === true,
      renderType: props.form.runtimeType,
      readRequestManifest: props.form.readRequests,
      vuePage: props.form.vuePage,
      template: props.form.template,
      permissions: props.form.permissions,
      values: props.form.effectiveFormData,
      changedFields: props.form.branchPatches.map(patch => patch.path),
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
  const message = event.data as RuntimeMessage
  const disposition = classifyRuntimeMessage(message, {
    sessionId: sessionId.value,
    pendingRequestIds: new Set(pending.keys()),
    runtimeActive,
    disposed,
    bootPending: iframeBootPending,
  })
  if (disposition === 'boot') {
    iframeBootPending = false
    void loadRuntime()
    return
  }
  if (disposition === 'state') {
    emit('state', message.payload || {})
    return
  }
  if (disposition === 'ignore' || !message.requestId) return
  const request = pending.get(message.requestId)
  if (!request) return
  window.clearTimeout(request.timer)
  request.cleanup()
  pending.delete(message.requestId)
  if (disposition === 'error') request.reject(new Error(String(message.payload?.message || '表单运行时操作失败')))
  else request.resolve(message.payload || {})
}

// setValues 把用户明确恢复的原始 values 交给 runtime，不附带额外元数据或字段映射。
function setValues(values: Record<string, unknown>, signal?: AbortSignal) {
  return postCommand('setData', {
    values,
    changedFields: props.form.branchPatches.map(patch => patch.path),
  }, signal)
}

// restoreSaved 恢复本次载入时的已保存值。
function restoreSaved() {
  return postCommand('restore')
}

// getValues 不触发必填校验，用于保存前捕获 runtime 当前原始 values。
function getValues(signal?: AbortSignal) {
  return postCommand('getValues', {}, signal)
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
    request.cleanup()
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

defineExpose({ setValues, restoreSaved, getValues, validateAndGetValues, destroyRuntime })
</script>

<template>
  <iframe
    ref="iframe"
    class="form-runtime-frame"
    :src="iframeSource"
    title="目标表单原始数据工作区"
    sandbox="allow-scripts allow-forms allow-same-origin allow-popups"
  />
</template>

<style scoped>
.form-runtime-frame {
  display: block;
  width: 100%;
  height: 100%;
  min-height: 0;
  background: #fff;
  border: 0;
}
</style>
