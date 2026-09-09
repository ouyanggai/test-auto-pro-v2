<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { classifyRuntimeMessage, FORM_RUNTIME_VERSION, type RuntimeMessage } from './runtimeProtocol'
import type { PathConfigurationDataWorkspace, PathFormRuntimeSession } from './types'

const props = defineProps<{
  // viewKey 是当前按节点权限渲染的视图身份；变化时按新权限重新装载表单。
  form: PathConfigurationDataWorkspace & { readOnly?: boolean, viewKey?: string }
  runtimeSession: PathFormRuntimeSession
}>()
const emit = defineEmits<{
  loading: []
  ready: [payload: Record<string, unknown>]
  state: [payload: Record<string, unknown>]
  error: [message: string]
}>()

const iframe = ref<HTMLIFrameElement | null>(null)
const sessionId = ref(crypto.randomUUID())
const iframeSource = computed(() => import.meta.env.DEV ? 'http://127.0.0.1:19001/form-runtime/#/test-auto-form' : '/form-runtime/#/test-auto-form')
const runtimeOrigin = computed(() => new URL(iframeSource.value, window.location.href).origin)
const RUNTIME_LOAD_TIMEOUT_MS = 60_000
const pending = new Map<string, { resolve: (payload: Record<string, unknown>) => void, reject: (error: Error) => void, timer?: number, cleanup: () => void }>()
let disposed = false
let runtimeActive = false
let runtimeGeneration = 0
let iframeBootPending = true
let runtimeReady = false
let bootTimer: number | undefined
let documentLoadCount = 0
let bootFailed = false

// plainPayload 在 postMessage 前移除 Vue Proxy；不使用 structuredClone 处理响应式对象。
function plainPayload(value: unknown): Record<string, unknown> {
  return JSON.parse(JSON.stringify(value ?? {})) as Record<string, unknown>
}

// clearBootTimeout 清理 iframe 文档已挂载但运行时未发送 boot 的本地屏障，避免卸载后定时器继续改写宿主状态。
function clearBootTimeout() {
  if (bootTimer === undefined) return
  window.clearTimeout(bootTimer)
  bootTimer = undefined
}

// startBootTimeout 为首次 boot 建立独立总超时；父页面拿到数据后会清理自己的请求计时器，不能只依赖父层兜底。
function startBootTimeout() {
  if (disposed || !iframeBootPending || bootTimer !== undefined) return
  bootTimer = window.setTimeout(() => {
    bootTimer = undefined
    if (disposed || !iframeBootPending) return
    bootFailed = true
    iframeBootPending = false
    resetRuntime(false)
    emit('error', '表单 iframe 初始化超时，请返回节点画布后重试')
  }, RUNTIME_LOAD_TIMEOUT_MS)
}

// cancelRuntimeOperation 通知 iframe 停止当前异步操作；宿主取消 pending Promise 不能阻止 runtime 内部迟到回调继续改值。
function cancelRuntimeOperation() {
  const target = iframe.value?.contentWindow
  if (!target || disposed || !runtimeActive) return
  try {
    target.postMessage({
      version: FORM_RUNTIME_VERSION,
      sessionId: sessionId.value,
      requestId: crypto.randomUUID(),
      type: 'cancel',
      payload: {},
    }, runtimeOrigin.value)
  } catch (_) {
    // iframe 已经失效时无法发送取消通知，本地 pending 仍会按调用方的超时或取消继续清理。
  }
}

// postCommand 绑定当前 iframe、会话、请求号和协议版本，迟到响应无法串到新路径；每个等待都必须有上限。
function postCommand(type: string, payload: Record<string, unknown> = {}, signal?: AbortSignal, timeoutMs = 15_000, timeoutMessage = '表单运行时响应超时，当前表单数据未丢失'): Promise<Record<string, unknown>> {
  const target = iframe.value?.contentWindow
  if (!target || disposed || !runtimeActive) return Promise.reject(new Error('表单运行时尚未就绪'))
  if (signal?.aborted) return Promise.reject(signal.reason instanceof Error ? signal.reason : new DOMException('操作已取消', 'AbortError'))
  const requestId = crypto.randomUUID()
  return new Promise((resolve, reject) => {
    const abort = () => {
      const request = pending.get(requestId)
      if (!request) return
      cancelRuntimeOperation()
      window.clearTimeout(request.timer)
      request.cleanup()
      pending.delete(requestId)
      reject(signal?.reason instanceof Error ? signal.reason : new DOMException('操作已取消', 'AbortError'))
    }
    const cleanup = () => signal?.removeEventListener('abort', abort)
    let timer: number | undefined
    if (timeoutMs > 0) {
      timer = window.setTimeout(() => {
        cancelRuntimeOperation()
        cleanup()
        pending.delete(requestId)
        reject(new Error(timeoutMessage))
      }, timeoutMs)
    }
    if (timer === undefined) {
      pending.set(requestId, { resolve, reject, cleanup })
    } else {
      pending.set(requestId, { resolve, reject, timer, cleanup })
    }
    signal?.addEventListener('abort', abort, { once: true })
    try {
      target.postMessage({ version: FORM_RUNTIME_VERSION, sessionId: sessionId.value, requestId, type, payload: plainPayload(payload) }, runtimeOrigin.value)
    } catch (caught) {
      if (timer !== undefined) window.clearTimeout(timer)
      cleanup()
      pending.delete(requestId)
      reject(caught instanceof Error ? caught : new Error('表单运行时消息发送失败'))
    }
  })
}

// loadRuntime 只把 SID 传给当前 iframe 内存会话，并按目标 runtime 协议装载原始模板、权限、页面和 values。
async function loadRuntime(retry = true, deadlineAt = Date.now() + RUNTIME_LOAD_TIMEOUT_MS): Promise<Record<string, unknown>> {
  if (disposed) return {}
  const generation = ++runtimeGeneration
  runtimeActive = true
  runtimeReady = false
  emit('loading')
  try {
    // 目标表单包含远程选项和链式联动，允许一次较长等待，但重试不能把总预算无限翻倍。
    const remaining = Math.max(1, deadlineAt - Date.now())
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
    }, undefined, remaining, '表单 iframe 初始化响应超时，请返回节点画布后重试')
    if (disposed || !runtimeActive || generation !== runtimeGeneration) return {}
    runtimeReady = true
    emit('ready', payload)
    return payload
  }
  catch (caught) {
    if (disposed || !runtimeActive || generation !== runtimeGeneration) return {}
    // 目标表单初始化包含多个远程选项请求，单次瞬断不应让已能回显的数据被清空。
    // load 是幂等初始化，保留当前 iframe 会话并只重试一次，避免无限重试掩盖真实错误。
    if (retry && Date.now() < deadlineAt) {
      await new Promise(resolve => window.setTimeout(resolve, 300))
      if (!disposed && runtimeActive && generation === runtimeGeneration) return loadRuntime(false, deadlineAt)
      return {}
    }
    resetRuntime(true)
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
    clearBootTimeout()
    bootFailed = false
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

// handleIframeLoad 只表示 iframe 文档已经到达；文档重载必须重新进入 boot 过渡，避免旧会话把白屏当成已 ready。
function handleIframeLoad() {
  if (disposed || bootFailed) return
  const firstDocument = documentLoadCount === 0
  documentLoadCount += 1
  if (!firstDocument) {
    resetRuntime(false)
    iframeBootPending = true
    runtimeReady = false
    bootFailed = false
  }
  if (firstDocument && runtimeReady) return
  startBootTimeout()
  emit('loading')
}

// handleIframeError 把浏览器层面的 iframe 加载失败转换成页面可见错误，并终止当前 pending 命令。
function handleIframeError() {
  if (disposed) return
  bootFailed = true
  resetRuntime(true)
  emit('error', '表单 iframe 加载失败，请返回节点画布后重试')
}

// setValues 把用户明确恢复的原始 values 交给 runtime，不附带额外元数据或字段映射。
function setValues(values: Record<string, unknown>, signal?: AbortSignal) {
  return postCommand('setData', {
    values,
    changedFields: props.form.branchPatches.map(patch => patch.path),
  }, signal, 60_000)
}

// restoreSaved 恢复本次载入时的已保存值。
function restoreSaved(signal?: AbortSignal) {
  return postCommand('restore', {}, signal, 60_000)
}

// getValues 不触发必填校验，用于保存前捕获 runtime 当前原始 values。
function getValues(signal?: AbortSignal) {
  return postCommand('getValues', {}, signal, 60_000)
}

// validateAndGetValues 先执行 getData(true)，再抓取包含虚拟字段的 getValues。
function validateAndGetValues(signal?: AbortSignal) {
  return postCommand('validateAndGetValues', {}, signal, 60_000)
}

// resetRuntime 统一终止当前会话并拒绝待处理请求；同一会话重复调用不会再次操作 iframe。
function resetRuntime(notifyFrame: boolean) {
  clearBootTimeout()
  iframeBootPending = false
  if (!runtimeActive && pending.size === 0) return
  if (notifyFrame && iframe.value?.contentWindow && !disposed) {
    try {
      iframe.value.contentWindow.postMessage({
        version: FORM_RUNTIME_VERSION, sessionId: sessionId.value, requestId: crypto.randomUUID(), type: 'destroy', payload: {},
      }, runtimeOrigin.value)
    } catch (_) {
      // iframe 已经失效时销毁通知无法送达，下面仍会本地拒绝所有 pending 并推进 generation。
    }
  }
  runtimeGeneration += 1
  runtimeActive = false
  runtimeReady = false
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

watch(() => [props.form.revision, props.form.dataRevision, props.runtimeSession.sid, props.form.viewKey], () => {
  if (!iframe.value?.contentWindow || iframeBootPending) return
  destroyRuntime()
  void loadRuntime()
})

window.addEventListener('message', handleMessage)
onMounted(startBootTimeout)
onBeforeUnmount(() => {
  destroyRuntime()
  clearBootTimeout()
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
    title="表单数据"
    @load="handleIframeLoad"
    @error="handleIframeError"
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
