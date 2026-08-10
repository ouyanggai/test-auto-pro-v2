/**
 * useFlowIframe - Vue 3 流程 iframe 集成 composable
 *
 * 用于在 Vue 3 宿主应用中管理流程 iframe 弹窗的状态。
 * 当前实际使用方式：通过 URL 参数传递认证信息，不依赖 postMessage。
 *
 * 使用示例见 vue3-usage.vue
 */
import { ref, computed, onMounted, onUnmounted } from 'vue'

// ── 常量 ──────────────────────────────────────────────
const PLATFORM_CODE = '200001'

// ── 类型定义 ──────────────────────────────────────────
export interface FlowIframeOptions {
  /** 流程应用部署地址，如 http://192.168.1.212:9528/flow */
  flowAppUrl: string
  /** 获取当前用户 SID */
  getSid: () => string
  /** 获取当前 customerCode */
  getCustomerCode: () => string
}

export interface FlowModalState {
  visible: boolean
  url: string
  title: string
}

// ── Composable ───────────────────────────────────────
export function useFlowIframe(options: FlowIframeOptions) {
  // iframe 弹窗状态
  const modalVisible = ref(false)
  const modalUrl = ref('')
  const modalTitle = ref('')

  /**
   * 构建流程详情 iframe URL
   * 格式: {flowAppUrl}/#/flow/detail/{id}?sid=...&platformCode=...&customerCode=...&mode=...
   */
  function buildFlowUrl(
    flowInstanceId: string,
    mode: 'audit' | 'view' | 'edit' | 'flow',
    extra?: { auditWay?: string }
  ): string {
    const sid = options.getSid()
    const cc = options.getCustomerCode()
    const params = new URLSearchParams({
      sid,
      platformCode: PLATFORM_CODE,
      customerCode: cc,
      mode,
    })
    if (extra?.auditWay) params.set('auditWay', extra.auditWay)

    return `${options.flowAppUrl}/#/flow/detail/${flowInstanceId}?${params.toString()}`
  }

  /**
   * 打开全屏 iframe 弹窗
   */
  function openFlowModal(
    flowInstanceId: string,
    mode: 'audit' | 'view' | 'edit' | 'flow',
    title?: string,
    extra?: { auditWay?: string }
  ) {
    const url = buildFlowUrl(flowInstanceId, mode, extra)
    modalUrl.value = url
    modalTitle.value = title || '流程详情'
    modalVisible.value = true
  }

  /**
   * 关闭 iframe 弹窗
   */
  function closeFlowModal() {
    modalVisible.value = false
    modalUrl.value = ''
    modalTitle.value = ''
  }

  // 监听 iframe postMessage 事件
  let messageHandler: ((event: MessageEvent) => void) | null = null
  const eventCallbacks: Record<string, ((data: any) => void)[]> = {}

  function onFlowEvent(eventName: string, callback: (data: any) => void) {
    if (!eventCallbacks[eventName]) {
      eventCallbacks[eventName] = []
    }
    eventCallbacks[eventName].push(callback)
    return () => {
      const idx = eventCallbacks[eventName].indexOf(callback)
      if (idx > -1) eventCallbacks[eventName].splice(idx, 1)
    }
  }

  onMounted(() => {
    messageHandler = (event: MessageEvent) => {
      const { type, eventName, data } = event.data || {}
      if (type === 'RSH_FLOW_EVENT' && eventName && eventCallbacks[eventName]) {
        eventCallbacks[eventName].forEach((cb) => cb(data))
      }
      if (type === 'RSH_FLOW_AUTH_EXPIRED') {
        console.warn('[FlowIframe] 认证过期，需要重新登录')
      }
    }
    window.addEventListener('message', messageHandler)
  })

  onUnmounted(() => {
    if (messageHandler) {
      window.removeEventListener('message', messageHandler)
    }
  })

  return {
    modalVisible,
    modalUrl,
    modalTitle,
    buildFlowUrl,
    openFlowModal,
    closeFlowModal,
    onFlowEvent,
  }
}
