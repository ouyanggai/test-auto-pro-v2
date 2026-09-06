<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import { NTag } from 'naive-ui'
import { computed } from 'vue'

import type { FlowNodeData } from './types'

const props = defineProps<{ data: FlowNodeData }>()
const emit = defineEmits<{ openForm: [] }>()

const tagType = computed<'default' | 'success' | 'warning' | 'error' | 'info'>(() => {
  switch (props.data.type) {
    case 'start': return 'success'
    case 'end': return 'error'
    case 'condition':
    case 'manual':
    case 'parallel': return 'warning'
    case 'synergy': return 'info'
    default: return 'default'
  }
})

</script>

<template>
  <div v-if="data.runMode" class="flow-node-shell">
    <div
      class="flow-node flow-node--run"
      :class="[
        data.runStatus ? `flow-node--run-${data.runStatus}` : '',
        { 'flow-node--run-current': data.runCurrent, 'flow-node--run-busy': data.runCurrent && data.runBusy },
      ]"
      :aria-label="`${data.name}，运行态：${data.runStatusName || '未开始'}${data.runCurrent ? (data.runBusy ? '，正在执行这一步' : '，当前步') : ''}`"
      :title="`${data.name}，运行态：${data.runStatusName || '未开始'}`"
    >
      <handle type="target" :position="Position.Top" :connectable="false" />
      <span class="flow-node__type">{{ data.typeName }}</span>
      <span class="flow-node__name">{{ data.name }}</span>
      <span class="flow-node__run-status">
        <!-- 执行动画由本地 CSS 时钟驱动，不跟轮询节奏跳；文字始终写清运行态，不靠颜色单独表意。 -->
        <span v-if="data.runCurrent && data.runBusy" class="flow-node__run-spinner" aria-hidden="true" />
        {{ data.runStatusName || '未开始' }}
      </span>
      <span v-if="data.runCurrent" class="flow-node__run-current-badge" role="status">
        {{ data.runBusy ? '正在执行' : '当前步' }}
      </span>
      <handle type="source" :position="Position.Bottom" :connectable="false" />
    </div>
  </div>
  <div v-else-if="data.configurationMode" class="flow-node-shell">
    <button
      type="button"
      class="flow-node flow-node--configuration"
      :class="{
        'flow-node--configuration-selected': data.configurationSelected,
        'flow-node--configuration-disabled': !data.configurationInteractive,
      }"
      :disabled="!data.configurationInteractive"
      :aria-pressed="data.configurationSelected"
      :title="data.configurationInteractive ? `${data.name}，${data.configurationStatusName}` : `${data.name}，不在当前路径`"
    >
      <handle type="target" :position="Position.Top" :connectable="false" />
      <span class="flow-node__type">{{ data.typeName }}</span>
      <span class="flow-node__name">{{ data.name }}</span>
      <span
        class="flow-node__configuration-status"
        :class="`flow-node__configuration-status--${data.configurationStatus || 'not_required'}`"
        :aria-label="data.configurationInteractive ? data.configurationStatusName : '路径外上下文'"
        :title="data.configurationInteractive ? data.configurationStatusName : '路径外上下文'"
      />
      <handle type="source" :position="Position.Bottom" :connectable="false" />
    </button>
    <button
      v-if="data.type === 'start' && data.configurationInteractive && data.configurationFormStatus"
      type="button"
      class="flow-node__form-entry"
      :class="`flow-node__form-entry--${data.configurationFormStatus}`"
      :aria-label="`打开表单数据，${data.configurationFormStatusName || '待配置'}`"
      :title="`表单数据：${data.configurationFormStatusName || '待配置'}`"
      @click.stop="emit('openForm')"
    >
      表
    </button>
    <span
      v-if="data.type === 'start' && data.configurationInteractive && data.configurationFormStatus === 'valid'"
      class="flow-node__form-configured-badge"
      aria-label="表单数据已配置"
    >表单已配置</span>
  </div>
  <n-tag v-else class="flow-node" :type="tagType" :bordered="true" :title="data.name">
    <handle type="target" :position="Position.Top" :connectable="false" />
    <span class="flow-node__type">{{ data.typeName }}</span>
    <span class="flow-node__name">{{ data.name }}</span>
    <handle type="source" :position="Position.Bottom" :connectable="false" />
  </n-tag>
</template>

<style scoped>
.flow-node {
  display: grid;
  width: 180px;
  height: 72px;
  padding: 9px 14px;
  align-content: center;
  justify-items: center;
  gap: 3px;
  border-radius: 4px;
  white-space: normal;
}

.flow-node-shell {
  position: relative;
  width: 180px;
  height: 72px;
}

.flow-node--configuration {
  position: relative;
  color: var(--flow-label-color);
  cursor: pointer;
  background: var(--flow-surface-color);
  border: 1px solid var(--flow-edge-color);
  transition: border-color 120ms ease, background-color 120ms ease, transform 120ms ease;
}

.flow-node--configuration:hover:not(:disabled),
.flow-node--configuration:focus-visible,
.flow-node--configuration-selected {
  border-color: var(--flow-direction-color);
  outline: none;
}

.flow-node--configuration:hover:not(:disabled) {
  transform: translateY(-2px);
}

.flow-node--configuration:focus-visible {
  outline: 2px solid var(--flow-direction-color);
  outline-offset: 2px;
}

.flow-node--configuration-selected {
  background: color-mix(in srgb, var(--flow-direction-color) 9%, var(--flow-surface-color));
  border-width: 2px;
}

.flow-node--configuration-disabled {
  cursor: default;
  opacity: 0.52;
}

.flow-node__configuration-status {
  position: absolute;
  right: 7px;
  bottom: 7px;
  width: 8px;
  height: 8px;
  background: var(--flow-edge-color);
  border: 2px solid var(--flow-surface-color);
  border-radius: 50%;
}

.flow-node__configuration-status--configured { background: var(--success-color, #18a058); }
.flow-node__configuration-status--pending,
.flow-node__configuration-status--partial { background: var(--warning-color, #f0a020); }
.flow-node__configuration-status--affected { background: var(--error-color, #d03050); }
.flow-node__configuration-status--runtime { background: var(--info-color, #2080f0); }

.flow-node__form-entry {
  position: absolute;
  top: -10px;
  right: -10px;
  z-index: 2;
  display: grid;
  width: 26px;
  height: 26px;
  padding: 0;
  color: var(--flow-label-color);
  font-size: 12px;
  cursor: pointer;
  background: var(--flow-surface-color);
  border: 1px solid var(--flow-edge-color);
  border-radius: 50%;
  place-items: center;
}

.flow-node__form-entry:hover,
.flow-node__form-entry:focus-visible {
  border-color: var(--flow-direction-color);
  outline: 2px solid color-mix(in srgb, var(--flow-direction-color) 34%, transparent);
}

.flow-node__form-entry--valid { color: var(--success-color, #18a058); }
.flow-node__form-entry--affected,
.flow-node__form-entry--unsupported { color: var(--error-color, #d03050); }
.flow-node__form-entry--empty,
.flow-node__form-entry--draft { color: var(--warning-color, #f0a020); }

.flow-node__form-configured-badge {
  position: absolute;
  top: -10px;
  left: 50%;
  z-index: 2;
  padding: 1px 8px;
  color: #fff;
  font-size: 11px;
  line-height: 16px;
  white-space: nowrap;
  background: var(--success-color, #18a058);
  border-radius: 9px;
  transform: translateX(-50%);
}

.flow-node__type {
  font-size: 12px;
  line-height: 1.2;
  opacity: 0.78;
}

.flow-node__name {
  display: -webkit-box;
  overflow: hidden;
  max-width: 150px;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.35;
  text-align: center;
  text-overflow: ellipsis;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.flow-node :deep(.vue-flow__handle) {
  width: 7px;
  height: 7px;
  border: 1px solid currentcolor;
  background: currentcolor;
  pointer-events: none;
}

.flow-node--run {
  display: grid;
  position: relative;
  height: 100%;
  align-content: center;
  justify-items: center;
  gap: 2px;
  color: var(--flow-label-color);
  cursor: pointer;
  background: var(--flow-surface-color);
  border: 1px solid var(--flow-edge-color);
  /* 运行画布只让当前步亮起来，其余节点整体压暗：读图时视线自然落在正在跑的那一步。 */
  opacity: 0.56;
  transition: opacity 140ms ease, border-color 140ms ease, box-shadow 140ms ease;
}

/* 九个中文运行态各有视觉档：颜色之外节点上必须有中文文字（run-status），不靠颜色单独表意。 */
.flow-node--run-not_started { border-style: dashed; opacity: 0.4; }
.flow-node--run-waiting { border-style: dashed; }
.flow-node--run-running { border-color: var(--info-color, #2080f0); }
.flow-node--run-running .flow-node__run-status { color: var(--info-color, #2080f0); }
.flow-node--run-verifying { border-color: var(--info-color, #2080f0); border-style: dotted; }
.flow-node--run-verifying .flow-node__run-status { color: var(--info-color, #2080f0); }
.flow-node--run-completed {
  border-color: color-mix(in srgb, var(--success-color, #18a058) 55%, transparent);
  opacity: 0.7;
}
.flow-node--run-completed .flow-node__run-status { color: var(--success-color, #18a058); }
/* 失败与待对账不压暗：出问题的节点必须一眼看到，压暗等于把故障藏起来。 */
.flow-node--run-failed {
  border-color: var(--error-color, #d03050);
  border-width: 2px;
  opacity: 1;
}
.flow-node--run-failed .flow-node__run-status { color: var(--error-color, #d03050); }
.flow-node--run-awaiting_reconciliation {
  border-color: var(--error-color, #d03050);
  border-width: 2px;
  border-style: double;
  opacity: 1;
}
.flow-node--run-awaiting_reconciliation .flow-node__run-status { color: var(--error-color, #d03050); }
.flow-node--run-paused { border-color: var(--warning-color, #f0a020); opacity: 0.86; }
.flow-node--run-paused .flow-node__run-status { color: var(--warning-color, #f0a020); }
.flow-node--run-stopped,
.flow-node--run-cancelled { opacity: 0.6; }
.flow-node--run-stopped .flow-node__run-status,
.flow-node--run-cancelled .flow-node__run-status { color: var(--warning-color, #f0a020); }

/* 当前步是整张画布唯一的高亮节点：亮边 + 淡色底 + 外圈光环，放在状态档之后好覆盖压暗。 */
.flow-node--run-current {
  z-index: 1;
  color: var(--flow-label-color);
  background: color-mix(in srgb, var(--info-color, #2080f0) 10%, var(--flow-surface-color));
  border-color: var(--info-color, #2080f0);
  border-width: 2px;
  border-style: solid;
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--info-color, #2080f0) 18%, transparent);
  opacity: 1;
}

/* 正在执行：光环按本地时钟呼吸，表示这一步真的在跑；只是停在这里等放行时保持静态高亮。 */
.flow-node--run-busy {
  animation: flow-node-run-pulse 1.6s ease-out infinite;
}

.flow-node__run-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 600;
  line-height: 1.2;
}

/* 执行动画：CSS 旋转环，不依赖轮询节奏；减少动态效果时退化为静态圆环，文字信息不减。 */
.flow-node__run-spinner {
  display: inline-block;
  width: 10px;
  height: 10px;
  border: 1.5px solid color-mix(in srgb, var(--info-color, #2080f0) 28%, transparent);
  border-top-color: var(--info-color, #2080f0);
  border-radius: 50%;
  animation: flow-node-run-spin 720ms linear infinite;
}

.flow-node__run-current-badge {
  position: absolute;
  top: -11px;
  left: 50%;
  padding: 1px 8px;
  color: #fff;
  font-size: 11px;
  line-height: 16px;
  white-space: nowrap;
  background: var(--info-color, #2080f0);
  border-radius: 9px;
  transform: translateX(-50%);
}

@keyframes flow-node-run-spin {
  to { transform: rotate(360deg); }
}

@keyframes flow-node-run-pulse {
  0% { box-shadow: 0 0 0 3px color-mix(in srgb, var(--info-color, #2080f0) 26%, transparent); }
  70% { box-shadow: 0 0 0 11px color-mix(in srgb, var(--info-color, #2080f0) 0%, transparent); }
  100% { box-shadow: 0 0 0 3px color-mix(in srgb, var(--info-color, #2080f0) 26%, transparent); }
}

@media (prefers-reduced-motion: reduce) {
  .flow-node--configuration {
    transition: none;
  }

  .flow-node--run {
    transition: none;
  }

  /* 减少动态效果：旋转与呼吸全部停掉，当前步保留静态高亮，「正在执行」四个字仍在。 */
  .flow-node--run-busy,
  .flow-node__run-spinner {
    animation: none;
  }
}
</style>
