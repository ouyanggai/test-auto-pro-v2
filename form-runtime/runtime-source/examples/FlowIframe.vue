<template>
  <!--
    FlowIframe.vue - 通用流程 iframe 包装组件 (Vue 3)

    在 Vue 3 宿主应用中嵌入 rsh-flow-components 流程页面。
    支持两种认证模式：
    1. URL 参数模式（当前使用）：SID/customerCode 直接拼入 iframe URL
    2. postMessage 模式（预留）：通过 postMessage 注入认证信息
  -->
  <div class="flow-iframe-container">
    <iframe
      ref="iframeRef"
      :src="iframeSrc"
      frameborder="0"
      :style="iframeStyle"
      allow="clipboard-read; clipboard-write"
      @load="onIframeLoad"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  /** 流程应用部署地址，如 http://192.168.1.212:9528/flow */
  flowAppUrl: string
  /** 后端 API 地址，如 http://192.168.1.220:38081/api */
  apiBaseUrl?: string
  /** 用户会话 SID */
  sid: string
  /** 平台编码 */
  platformCode?: string
  /** 客户编码 */
  customerCode?: string
  /** 流程实例 ID */
  flowInstanceId: string
  /** 操作模式: audit=审核, view=查看, edit=编辑, flow=流程图 */
  mode?: 'audit' | 'view' | 'edit' | 'flow'
  /** 审批方式 */
  auditWay?: string
  width?: string
  height?: string
}>()

const emit = defineEmits<{
  ready: []
  'flow-action-done': [data: { flowInstanceId: string; mode: string }]
  'auth-expired': []
}>()

const iframeRef = ref<HTMLIFrameElement | null>(null)

// 构建 iframe URL（hash mode 格式）
const iframeSrc = computed(() => {
  const params = new URLSearchParams()
  params.set('sid', props.sid)
  params.set('platformCode', props.platformCode || '200001')
  if (props.customerCode) params.set('customerCode', props.customerCode)
  if (props.mode) params.set('mode', props.mode)
  if (props.auditWay) params.set('auditWay', props.auditWay)

  return `${props.flowAppUrl}/#/flow/detail/${props.flowInstanceId}?${params.toString()}`
})

const iframeStyle = computed(() => ({
  width: props.width || '100%',
  height: props.height || '100%',
  border: 'none',
}))

function onIframeLoad() {
  emit('ready')
}

// 监听 iframe 发出的 postMessage 事件
let messageHandler: ((event: MessageEvent) => void) | null = null

onMounted(() => {
  messageHandler = (event: MessageEvent) => {
    const { type, eventName, data } = event.data || {}
    if (type === 'RSH_FLOW_EVENT' && eventName === 'flow-action-done') {
      emit('flow-action-done', data)
    }
    if (type === 'RSH_FLOW_AUTH_EXPIRED') {
      emit('auth-expired')
    }
  }
  window.addEventListener('message', messageHandler)
})

onUnmounted(() => {
  if (messageHandler) {
    window.removeEventListener('message', messageHandler)
  }
})

defineExpose({ iframeRef })
</script>

<style scoped>
.flow-iframe-container {
  width: 100%;
  height: 100%;
}
</style>
