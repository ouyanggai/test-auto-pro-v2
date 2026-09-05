<script setup lang="ts">
import { computed } from 'vue'

import { formatElapsed } from './api'

// 七个阶段的顺序与后端 step.log 的 phase= 一致；段标签是中文阶段名，不只靠颜色表意。
const phases: Array<{ key: string; name: string }> = [
  { key: 'plan', name: '取步' },
  { key: 'gate', name: '门禁' },
  { key: 'control', name: '控制' },
  { key: 'prepare', name: '演员' },
  { key: 'submit', name: '提交' },
  { key: 'verify', name: '核验' },
  { key: 'settle', name: '落账' },
]

const props = defineProps<{
  // running 表示本步正在执行（放行后到响应前）；waiting 表示停在阶段 3 等待放行。
  running: boolean
  // stale 表示超过配置预算仍无状态更新，转为疑似无响应，不无限转圈。
  stale: boolean
  // elapsedText 是三处同源的已耗时文本（服务端阶段时间为基准、本地时钟插值）。
  elapsedText: string
  // phaseDurations 是最近一次尝试各阶段耗时（毫秒），场景走完后展示。
  phaseDurations?: Record<string, number>
  // note 是重试等状态的补充说明（如「重试中，第 N 次」）。
  note?: string
  // currentPhase 是执行器实时上报的当前阶段键；为空表示没有实时数据，退化为整段执行中。
  currentPhase?: string
  // currentPhaseNote 是与实时阶段一起上报的中文补充（如「写请求发送中，同步等待目标响应」）。
  currentPhaseNote?: string
}>()

// activeIndex 计算执行中应高亮到哪个阶段：优先用执行器实时上报的当前阶段推进；
// 没有上报数据时退化为执行中高亮到落账段（旧形态），等待放行时停在控制段。
const activeIndex = computed(() => {
  if (props.stale) return -1
  if (!props.running) return 2
  const index = phases.findIndex((phase) => phase.key === props.currentPhase)
  return index >= 0 ? index : 6
})

// currentPhaseLabel 是实时阶段的中文段名。
const currentPhaseLabel = computed(() => phases.find((phase) => phase.key === props.currentPhase)?.name || '')

// activeNote 汇总两路补充说明：实时阶段说明优先，其次重试/预算提示。
const activeNote = computed(() => props.currentPhaseNote || props.note || '')

// segments 生成每段的状态文案与耗时：已落账按实测耗时展示；
// 执行中把当前阶段之前的段标记为已走过，当前段随本地时钟推进。
const segments = computed(() => {
  const active = activeIndex.value
  return phases.map((phase, index) => ({
    ...phase,
    duration: props.phaseDurations?.[phase.key],
    done: !props.running
      ? props.phaseDurations?.[phase.key] !== undefined
      : !props.stale && index < active,
  }))
})
</script>

<template>
  <div class="run-indicator" role="status" :aria-label="`当前步指示器，${stale ? '疑似无响应' : running ? (currentPhaseLabel ? `执行中：${currentPhaseLabel}` : '执行中') : '等待放行'}，已耗时 ${elapsedText}`">
    <div class="run-indicator__segments">
      <div
        v-for="(segment, index) in segments"
        :key="segment.key"
        class="run-indicator__segment"
        :class="{
          'run-indicator__segment--active': running && !stale && index === activeIndex,
          'run-indicator__segment--done': segment.done,
        }"
        :title="`${segment.name}阶段${segment.duration !== undefined ? `，耗时 ${formatElapsed(segment.duration)}` : ''}`"
      >
        <span class="run-indicator__segment-bar" />
        <span class="run-indicator__segment-name">{{ segment.name }}</span>
      </div>
    </div>
    <div class="run-indicator__meta">
      <span v-if="stale" class="run-indicator__state run-indicator__state--stale">疑似无响应，请查看日志</span>
      <span v-else-if="running" class="run-indicator__state run-indicator__state--running">
        {{ currentPhaseLabel ? `执行中：${currentPhaseLabel}` : '执行中' }}<span class="run-indicator__pulse" aria-hidden="true" />
      </span>
      <span v-else class="run-indicator__state">等待放行</span>
      <span class="run-indicator__elapsed">已耗时 {{ elapsedText }}</span>
      <span v-if="activeNote" class="run-indicator__note">{{ activeNote }}</span>
    </div>
  </div>
</template>

<style scoped>
.run-indicator {
  display: grid;
  gap: 6px;
  padding: 8px 10px;
  background: var(--run-surface, var(--flow-surface-color, #fff));
  border: 1px solid var(--run-border, var(--flow-edge-color, #ccc));
  border-radius: 6px;
}

.run-indicator__segments {
  display: flex;
  gap: 4px;
}

.run-indicator__segment {
  display: grid;
  flex: 1;
  gap: 2px;
  justify-items: center;
}

.run-indicator__segment-bar {
  width: 100%;
  height: 5px;
  background: var(--run-border, var(--flow-edge-color, #ccc));
  border-radius: 3px;
}

.run-indicator__segment--done .run-indicator__segment-bar { background: var(--success-color, #18a058); }
.run-indicator__segment--active .run-indicator__segment-bar { background: var(--info-color, #2080f0); }

.run-indicator__segment-name {
  font-size: 11px;
  line-height: 1.2;
  white-space: nowrap;
}

.run-indicator__meta {
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: 12px;
}

.run-indicator__state { font-weight: 600; }
.run-indicator__state--running { color: var(--info-color, #2080f0); }
.run-indicator__state--stale { color: var(--error-color, #d03050); }

/* 执行中的点动提示：减少动态效果时退化为纯文字（下方媒体查询关闭动画）。 */
.run-indicator__pulse {
  display: inline-block;
  width: 7px;
  height: 7px;
  margin-left: 5px;
  background: currentcolor;
  border-radius: 50%;
  animation: run-indicator-pulse 1.1s ease-in-out infinite;
}

.run-indicator__elapsed { opacity: 0.85; }
.run-indicator__note { color: var(--warning-color, #f0a020); }

@keyframes run-indicator-pulse {
  0%, 100% { opacity: 0.25; }
  50% { opacity: 1; }
}

@media (prefers-reduced-motion: reduce) {
  .run-indicator__pulse { animation: none; opacity: 1; }
}
</style>
