<script setup lang="ts">
import { computed, ref } from 'vue'

import { formatElapsed, formatTime } from './api'
import type { RunStep, PathRunDetail, RunPreview } from './api'

// RunNodePanel 是固定侧栏：展示一个节点上已发生的运行事实。
// 写结果不确定只给结论与依据，不渲染任何重试或继续入口（纲领第 4.4 节）。
const props = defineProps<{
  detail: PathRunDetail
  nodeKey: string
}>()

const emit = defineEmits<{ close: [] }>()

// nodeSteps 是该节点上已落账的步骤。
const nodeSteps = computed<RunStep[]>(() => props.detail.steps.filter((step) => step.nodeKey === props.nodeKey))

// nodeStateName 是该节点的运行态中文。
const nodeStateName = computed(() => props.detail.nodeStates[props.nodeKey]?.statusName || '未开始')

// nodeDisplayName 优先用步骤事实里的节点名，其次用当前预览名。
const nodeDisplayName = computed(() => {
  if (props.nodeKey === props.detail.currentPreview?.nodeKey && props.detail.currentPreview?.nodeName) {
    return props.detail.currentPreview.nodeName
  }
  return nodeSteps.value[0]?.nodeName || props.nodeKey
})

// isCurrentNode 判断侧栏当前节点是否是等待放行的当前步；是则展示预览与门禁结论。
const isCurrentNode = computed(() => props.detail.currentPreview?.nodeKey === props.nodeKey)
const currentPreview = computed<RunPreview | null>(() => (isCurrentNode.value ? props.detail.currentPreview ?? null : null))

// expandedCurl 控制请求与响应正文的展开；默认折叠避免超大正文卡住界面。
const expandedCurl = ref<string>('')
const CURL_LIMIT = 20000

// curlText 对超限正文做有界渲染：超出部分给中文说明并引导去日志文件看全文。
function curlText(step: RunStep): string {
  const block = step.attempts[0]?.curlBlock || ''
  if (block.length <= CURL_LIMIT) return block
  return block.slice(0, CURL_LIMIT) + `\n……正文超过 ${CURL_LIMIT} 字符，已截断显示；完整内容请在日志目录的 curl.log 查看。`
}

// toggleCurl 展开或收起某一步的请求与响应正文。
function toggleCurl(stepNo: number): void {
  expandedCurl.value = expandedCurl.value === String(stepNo) ? '' : String(stepNo)
}

// copyCurl 复制原始字节内容（不做界面美化），供直接重放。
async function copyCurl(step: RunStep): Promise<void> {
  const block = step.attempts[0]?.curlBlock || ''
  if (!block) return
  try {
    await navigator.clipboard.writeText(block)
  } catch {
    // 剪贴板不可用时静默失败：用户仍可在 curl.log 里复制。
  }
}

// phaseOrder 用于按七阶段顺序展示耗时。
const phaseOrder: Array<[string, string]> = [
  ['plan', '取步'], ['gate', '门禁'], ['control', '控制'], ['prepare', '演员'],
  ['submit', '提交'], ['verify', '核验'], ['settle', '落账'],
]

// gateSnapshotLines 解析门禁结论快照为中文行（评审缺陷 10 的修复点）：
// 对已执行的步骤还原放行当时的门禁判定与逐项条件满足情况，不出现内部字段英文名。
interface GateSnapshotShape {
  allowed?: boolean
  reason?: string
  items?: Array<{ label?: string; key?: string; required?: boolean; present?: boolean }>
}

function gateSnapshotLines(step: RunStep): string[] {
  if (!step.gateSnapshot) return []
  let snapshot: GateSnapshotShape
  try {
    snapshot = JSON.parse(step.gateSnapshot) as GateSnapshotShape
  } catch {
    return []
  }
  const lines: string[] = [snapshot.allowed ? '门禁：放行时已通过' : `门禁：放行时未通过${snapshot.reason ? `（${snapshot.reason}）` : ''}`]
  for (const item of snapshot.items || []) {
    const name = item.label || item.key || '条件'
    if (item.present) lines.push(`${name}：已满足`)
    else if (item.required) lines.push(`${name}：未满足`)
    else lines.push(`${name}：未提供（非必填）`)
  }
  return lines
}

// finalFactsText 把最终目标事实摘要渲染为中文行，不再直接输出英文键 JSON（评审低优先级 20 的修复点）。
const finalFactsText = computed<string[]>(() => {
  const facts = (props.detail.finalTarget ?? {}) as Record<string, unknown>
  const lines: string[] = []
  if (facts.instanceRef) lines.push(`主实例：${String(facts.instanceRef)}`)
  if (facts.statusName) lines.push(`实例状态：${String(facts.statusName)}`)
  else if (facts.status) lines.push(`实例状态：${String(facts.status)}`)
  const current = facts.currentNodeNames as string[] | undefined
  lines.push(current && current.length > 0 ? `当前节点：${current.join('、')}` : '当前节点：无')
  const due = facts.dueNodeNames as string[] | undefined
  lines.push(due && due.length > 0 ? `当前待办：${due.join('、')}` : '当前待办：无')
  return lines
})

// previewFactsText 把当前目标事实渲染为中文行。
const previewFactsText = computed<string[]>(() => {
  const facts = currentPreview.value?.facts || {}
  const lines: string[] = []
  lines.push(facts.instanceFound ? '实例：已存在' : '实例：尚未创建')
  if (facts.instanceStatus) lines.push(`实例状态：${String(facts.instanceStatus)}`)
  const due = facts.dueNodes as string[] | undefined
  lines.push(due && due.length > 0 ? `当前待办：${due.length} 个` : '当前待办：无')
  if (facts.readError) lines.push(`读取异常：${String(facts.readError)}`)
  return lines
})
</script>

<template>
  <aside class="run-panel" aria-label="节点运行信息">
    <header class="run-panel__header">
      <div>
        <h3 class="run-panel__title">{{ nodeDisplayName }}</h3>
        <p class="run-panel__state">运行态：{{ nodeStateName }}</p>
      </div>
      <button type="button" class="run-panel__close" aria-label="关闭节点运行信息" @click="emit('close')">关闭</button>
    </header>

    <div v-if="isCurrentNode && currentPreview" class="run-panel__section">
      <h4 class="run-panel__section-title">下一步预览（等待放行）</h4>
      <p>动作：{{ currentPreview.actionName || currentPreview.action }}</p>
      <p>处理人：{{ currentPreview.actorName }}</p>
      <p v-if="currentPreview.expectedEffect">预期效果：{{ currentPreview.expectedEffect }}</p>
      <p v-if="currentPreview.endpoint">目标端点：{{ currentPreview.endpoint }}</p>
      <p class="run-panel__gate" :class="currentPreview.gateAllowed ? 'run-panel__gate--ok' : 'run-panel__gate--blocked'">
        门禁结论：{{ currentPreview.gateAllowed ? '通过' : `未通过（${currentPreview.gateReason || '见阻塞原因'}）` }}
      </p>
      <ul class="run-panel__gate-items">
        <li v-for="(item, index) in currentPreview.gateItems" :key="index" :class="item.passed ? '' : 'run-panel__gate-item--failed'">
          {{ item.description }}：{{ item.passed ? '满足' : '不满足' }}
        </li>
      </ul>
      <p v-if="currentPreview.blockReason" class="run-panel__blocked">{{ currentPreview.blockReason }}</p>
      <details class="run-panel__request">
        <summary>即将发出的请求</summary>
        <pre class="run-panel__pre">{{ currentPreview.requestPreview || '（暂无请求载荷）' }}</pre>
      </details>
      <div class="run-panel__facts">
        <h5>目标实时事实</h5>
        <p v-for="line in previewFactsText" :key="line">{{ line }}</p>
      </div>
    </div>

    <div class="run-panel__section">
      <h4 class="run-panel__section-title">已执行的步骤</h4>
      <p v-if="nodeSteps.length === 0" class="run-panel__empty">该节点还没有已执行的步骤。</p>
      <article v-for="step in nodeSteps" :key="step.stepNo" class="run-panel__step">
        <header>
          <strong>第 {{ step.stepNo }} 步：{{ step.actionName }}</strong>
          <span>{{ step.statusName }}，总耗时 {{ formatElapsed(step.durationMs) }}</span>
        </header>
        <p>演员：{{ step.actorName || '—' }}；开始于 {{ formatTime(step.startedAt) }}</p>
        <ul v-if="gateSnapshotLines(step).length > 0" class="run-panel__gate">
          <li v-for="(line, index) in gateSnapshotLines(step)" :key="index">{{ line }}</li>
        </ul>
        <div v-for="attempt in step.attempts" :key="attempt.attemptNo" class="run-panel__attempt">
          <p>判定：{{ attempt.verdictName }}（耗时 {{ formatElapsed(attempt.durationMs) }}）</p>
          <p class="run-panel__reason">{{ attempt.reason }}</p>
          <p class="run-panel__basis">依据：{{ attempt.basis }}</p>
          <p>trace_id：{{ attempt.traceId }}</p>
          <div v-if="attempt.phaseDurations" class="run-panel__phases">
            <span v-for="[phase, label] in phaseOrder" :key="phase" class="run-panel__phase">
              {{ label }} {{ formatElapsed(attempt.phaseDurations[phase] ?? -1) }}
            </span>
          </div>
          <p v-else class="run-panel__phase-note">{{ attempt.phaseDurationsNote || '暂无阶段耗时' }}</p>
          <p class="run-panel__log">日志：{{ attempt.logPath }} 第 {{ attempt.logLine }} 行</p>
          <div class="run-panel__curl-actions">
            <button type="button" class="run-panel__link" @click="toggleCurl(step.stepNo)">
              {{ expandedCurl === String(step.stepNo) ? '收起请求与响应正文' : '展开请求与响应正文' }}
            </button>
            <button v-if="attempt.curlBlock" type="button" class="run-panel__link" @click="copyCurl(step)">复制可重放 curl</button>
          </div>
          <pre v-if="expandedCurl === String(step.stepNo)" class="run-panel__pre">{{ curlText(step) }}</pre>
          <p v-else-if="!attempt.curlBlock" class="run-panel__phase-note">curl.log 中没有该次尝试的记录。</p>
        </div>
      </article>
    </div>

    <div v-if="detail.finalTarget" class="run-panel__section">
      <h4 class="run-panel__section-title">最终目标事实</h4>
      <p v-for="line in finalFactsText" :key="line">{{ line }}</p>
    </div>
  </aside>
</template>

<style scoped>
.run-panel {
  display: grid;
  align-content: start;
  gap: 12px;
  width: 336px;
  padding: 12px;
  overflow-y: auto;
  font-size: 13px;
  background: var(--run-surface, var(--flow-surface-color, #fff));
  border: 1px solid var(--run-border, var(--flow-edge-color, #ccc));
  border-radius: 8px;
}

.run-panel__header {
  display: flex;
  align-items: start;
  justify-content: space-between;
}

.run-panel__title {
  margin: 0;
  font-size: 15px;
}

.run-panel__state { margin: 4px 0 0; opacity: 0.8; }

.run-panel__close {
  padding: 3px 10px;
  cursor: pointer;
  background: transparent;
  border: 1px solid var(--run-border, var(--flow-edge-color, #ccc));
  border-radius: 4px;
}

.run-panel__section {
  display: grid;
  gap: 6px;
  padding-top: 8px;
  border-top: 1px dashed var(--run-border, var(--flow-edge-color, #ccc));
}

.run-panel__section-title { margin: 0; font-size: 13px; }
.run-panel__section p { margin: 0; }
.run-panel__empty { opacity: 0.7; }

.run-panel__gate--ok { color: var(--success-color, #18a058); }
.run-panel__gate--blocked { color: var(--error-color, #d03050); }

.run-panel__gate-items {
  margin: 0;
  padding-left: 18px;
  opacity: 0.85;
}

.run-panel__gate-item--failed { color: var(--error-color, #d03050); }
.run-panel__blocked { color: var(--error-color, #d03050); }

.run-panel__step {
  display: grid;
  gap: 4px;
  padding: 8px;
  border: 1px solid var(--run-border, var(--flow-edge-color, #ccc));
  border-radius: 6px;
}

.run-panel__step header { display: grid; }
.run-panel__attempt {
  display: grid;
  gap: 3px;
  padding-top: 6px;
  border-top: 1px dashed var(--run-border, var(--flow-edge-color, #ccc));
}

.run-panel__reason { font-weight: 600; }
.run-panel__gate { margin: 4px 0; padding-left: 18px; }
.run-panel__basis { opacity: 0.8; }

.run-panel__phases { display: flex; flex-wrap: wrap; gap: 6px; }
.run-panel__phase { padding: 1px 6px; background: color-mix(in srgb, var(--info-color, #2080f0) 10%, transparent); border-radius: 8px; }
.run-panel__phase-note { opacity: 0.7; }
.run-panel__log { opacity: 0.7; word-break: break-all; }

.run-panel__curl-actions { display: flex; gap: 10px; }
.run-panel__link {
  padding: 0;
  color: var(--info-color, #2080f0);
  cursor: pointer;
  background: none;
  border: none;
  text-decoration: underline;
}

.run-panel__pre {
  max-height: 260px;
  margin: 0;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  background: color-mix(in srgb, var(--run-border, #ccc) 14%, transparent);
  border-radius: 4px;
  padding: 6px;
}

.run-panel__facts h5 { margin: 4px 0 0; }
</style>
