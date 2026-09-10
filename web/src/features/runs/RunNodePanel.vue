<script setup lang="ts">
import { NButton, NEmpty, NModal, NTag, useMessage, useThemeVars } from 'naive-ui'
import { computed, ref, watch } from 'vue'

import { formatElapsed, formatTime } from './api'
import type { PathRunDetail, RunNodePlanAction, RunPreview, RunStep, RunStepAttempt } from './api'

// RunNodePanel 是点击画布节点后才出现的检视面板：三个页签只给简要事实，
// 完整内容（门禁逐项、阶段耗时、日志位置与可重放 curl）一律进详情弹窗，
// 避免把一屏堆满说明文字。写结果不确定只呈现结论与依据，不渲染任何重试或继续入口（纲领第 4.4 节）。
const props = defineProps<{
  detail: PathRunDetail
  nodeKey: string
  nodeName: string
  nodeTypeName: string
}>()

const emit = defineEmits<{ close: [] }>()

// message 给复制操作明确的成功与失败反馈：复制失败不再静默（返工任务书缺口 9）。
const message = useMessage()

type PanelTab = 'plan' | 'facts' | 'errors'
const activeTab = ref<PanelTab>('facts')

// nodeSteps 是这个节点上已落账的步骤：按图节点 ID 过滤（旧数据回退令牌键）。
const nodeSteps = computed<RunStep[]>(() => props.detail.steps.filter(step => (step.nodeId || step.nodeKey) === props.nodeKey))

// nodeStateName 是这个节点的运行态中文名。
const nodeStateName = computed(() => props.detail.nodeStates[props.nodeKey]?.statusName || '未开始')
const nodeStatus = computed(() => props.detail.nodeStates[props.nodeKey]?.status || 'not_started')

// isCurrentNode 用图节点 ID 比对：预览自带的 nodeKey 是配置令牌键，两套键空间不能混用，
// 混用会让当前步的预览永远显示不出来（本次改版修掉的既有缺陷）。
const isCurrentNode = computed(() => (props.detail.currentPreview?.nodeId || '') === props.nodeKey)
const currentPreview = computed<RunPreview | null>(() => (isCurrentNode.value ? props.detail.currentPreview ?? null : null))

// planActions 是本次运行在这个节点上的已配置计划（服务端由编译场景归组，全部中文）。
const planActions = computed<RunNodePlanAction[]>(() => props.detail.nodePlans?.[props.nodeKey] ?? [])

// NodeErrorRow 是错误页签的一行：标题、结论、一句话原因，详情留给弹窗。
interface NodeErrorRow {
  key: string
  title: string
  verdict: string
  reason: string
  step?: RunStep
}

// isSuccessfulAttempt 兼容历史运行记录，同时让新记录统一使用“执行成功”。
function isSuccessfulAttempt(attempt: RunStepAttempt): boolean {
  return attempt.verdictName === '执行成功' || attempt.verdictName === '确定成功'
}

// nodeErrors 汇总这个节点上需要人看一眼的事实：门禁未通过、执行失败的尝试、以及停在这里的原因。
const nodeErrors = computed<NodeErrorRow[]>(() => {
  const rows: NodeErrorRow[] = []
  const preview = currentPreview.value
  if (preview && !preview.gateAllowed) {
    rows.push({
      key: 'gate',
      title: `当前步：${preview.actionName || preview.action}`,
      verdict: '条件未满足',
      reason: preview.gateReason || preview.blockReason || '见下面的检查结果',
    })
  } else if (preview?.blockReason) {
    rows.push({ key: 'block', title: `当前步：${preview.actionName || preview.action}`, verdict: '放行被阻塞', reason: preview.blockReason })
  }
  for (const step of nodeSteps.value) {
    for (const attempt of step.attempts) {
      if (isSuccessfulAttempt(attempt)) continue
      rows.push({
        key: `${step.stepNo}-${attempt.attemptNo}`,
        title: `第 ${step.stepNo} 步 · ${step.actionName}`,
        verdict: attempt.verdictName,
        reason: attempt.reason,
        step,
      })
    }
  }
  if ((nodeStatus.value === 'failed' || nodeStatus.value === 'awaiting_reconciliation') && props.detail.stopReason) {
    rows.push({
      key: 'stop',
      title: '路径运行停在这个节点',
      verdict: props.detail.failureClassName || nodeStateName.value,
      reason: props.detail.stopReason,
    })
  }
  return rows
})

// 切换节点时重选页签：有错误先给错误，否则给运行信息；运行中出现新错误不抢走用户当前视图。
watch(() => props.nodeKey, () => {
  activeTab.value = nodeErrors.value.length > 0 ? 'errors' : 'facts'
}, { immediate: true })

// 错误全部消失（例如重放后判定确定成功）时退回运行信息，不留一个空页签。
watch(() => nodeErrors.value.length, (count) => {
  if (count === 0 && activeTab.value === 'errors') activeTab.value = 'facts'
})

// 详情弹窗：当前步预览、已执行步骤、计划动作各自一个，互不干扰。
const previewDialogOpen = ref(false)
const stepDialog = ref<RunStep | null>(null)
const planDialog = ref<RunNodePlanAction | null>(null)

// openStepDialog 打开某一步的完整事实；错误页签点详情走同一个弹窗，不做第二套。
function openStepDialog(step?: RunStep): void {
  if (!step) {
    previewDialogOpen.value = true
    return
  }
  expandedCurl.value = ''
  stepDialog.value = step
}

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

// copyLogRef 复制日志相对路径与行号：粘贴到 code-server 搜索即可直达（三次点击标准）。
async function copyLogRef(attempt: RunStepAttempt): Promise<void> {
  try {
    await navigator.clipboard.writeText(`${attempt.logPath} 第 ${attempt.logLine} 行`)
    message.success('已复制日志位置')
  } catch {
    // 剪贴板不可用必须明说：告知用户去日志文件里定位，不静默吞掉。
    message.error('复制失败：浏览器剪贴板不可用，请打开日志目录的对应文件查看')
  }
}

// copyCurl 复制原始字节内容（不做界面美化），供直接重放。
async function copyCurl(step: RunStep): Promise<void> {
  const block = step.attempts[0]?.curlBlock || ''
  if (!block) {
    message.warning('本次尝试没有可重放的 curl 记录')
    return
  }
  try {
    await navigator.clipboard.writeText(block)
    message.success('已复制可重放 curl')
  } catch {
    // 剪贴板不可用必须明说：告知用户去 curl.log 里复制，不静默吞掉。
    message.error('复制失败：浏览器剪贴板不可用，请在日志目录的 curl.log 中查看')
  }
}

// phaseOrder 用于按七阶段顺序展示耗时（阶段流水只在详情里给排查者看）。
const phaseOrder: Array<[string, string]> = [
  ['plan', '准备'], ['gate', '检查'], ['control', '等待放行'], ['prepare', '准备执行'],
  ['submit', '发送请求'], ['verify', '确认结果'], ['settle', '记录结果'],
]

// GateSnapshotShape 是门禁结论快照的结构：逐项中文条件与满足情况。
interface GateSnapshotShape {
  allowed?: boolean
  reason?: string
  items?: Array<{ label?: string; key?: string; required?: boolean; present?: boolean }>
  // 按节点权限构造表单数据的两个清单（F-024）：解释“这个字段为什么没提交/为什么变了”。
  formOverlaid?: string[]
  formWithheld?: string[]
}

// gateItemText 把服务端检查项翻译成用户看得懂的判断结果，不显示抽象条件名。
function gateItemText(item: { label?: string; key?: string; required?: boolean; present?: boolean }): string {
  const present = item.present === true
  switch (item.key) {
    case 'active_task':
      return present ? '目标已查到当前处理人的待处理待办' : '目标没有查到待处理的待办：可能已流转给其他处理人，或已经被处理'
    case 'running_instance':
      return present ? '流程当前正在运行' : '流程当前不在运行中'
    case 'human_node':
      return present ? '当前节点是人工审批节点' : '当前节点不是人工审批节点'
    case 'not_forwarded':
      return present ? '当前不是转发辅助流程' : '当前是转发辅助流程，不能直接处理主流程'
    case 'initiator':
      return present ? '当前账号是流程发起人' : '当前账号不是流程发起人'
    case 'new_or_draft':
      return present ? '实例处于新建或草稿状态' : '实例当前不在新建或草稿状态'
    case 'new_instance':
      return present ? '实例尚未提交，可以新建提交' : '实例已经存在，不能再次新建提交'
    case 'resubmittable_status':
      return present ? '实例处于可重新提交状态' : '实例当前状态不能重新提交'
    case 'not_ended':
      return present ? '实例尚未结束' : '实例已经结束'
    default:
      if (item.label) return `${item.label}：${present ? '已满足' : (item.required === false ? '不适用' : '未满足')}`
      return present ? '检查项已满足' : '检查项未满足'
  }
}

// gateSnapshotLines 还原放行当时的门禁判定与逐项条件满足情况，不出现内部字段英文名。
function gateSnapshotLines(step: RunStep): string[] {
  if (!step.gateSnapshot) return []
  let snapshot: GateSnapshotShape
  try {
    snapshot = JSON.parse(step.gateSnapshot) as GateSnapshotShape
  } catch {
    return []
  }
  const lines: string[] = [snapshot.allowed ? '检查结果：通过，可以放行' : `检查结果：未通过${snapshot.reason ? `（${snapshot.reason}）` : ''}`]
  for (const item of snapshot.items || []) {
    lines.push(gateItemText(item))
  }
  if (snapshot.formOverlaid && snapshot.formOverlaid.length > 0) {
    lines.push(`按本节点权限覆盖的字段：${snapshot.formOverlaid.join('、')}`)
  }
  if (snapshot.formWithheld && snapshot.formWithheld.length > 0) {
    lines.push(`按本节点权限未携带的字段：${snapshot.formWithheld.join('、')}`)
  }
  return lines
}

// finalFactsText 把最终目标事实摘要渲染为中文行，不直接输出英文键 JSON。
// 主实例 ID 属内部标识不上界面（用户裁决）：需要在目标平台定位实例时走日志目录的运行记录。
const finalFactsText = computed<string[]>(() => {
  const facts = (props.detail.finalTarget ?? {}) as Record<string, unknown>
  const lines: string[] = []
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
  if (typeof facts.currentTaskFound === 'boolean') {
    const assignee = facts.currentTaskAssigneeName ? `，处理人：${String(facts.currentTaskAssigneeName)}` : ''
    lines.push(facts.currentTaskFound ? `当前待办：已查到待处理任务${assignee}` : `当前待办：未查到待处理任务（目标没有返回当前处理人的待办，可能已流转或已处理）`)
  } else {
    const due = facts.dueNodes as string[] | undefined
    lines.push(due && due.length > 0 ? `当前待办：${due.length} 个` : '当前待办：无')
  }
  if (facts.readError) lines.push(`读取异常：${String(facts.readError)}`)
  return lines
})

// gateBrief 是当前步门禁的一句话结论（详情里才展开逐项条件）。
const gateBrief = computed(() => {
  const preview = currentPreview.value
  if (!preview) return ''
  if (preview.gateAllowed) return '条件已满足，可以放行'
  return `条件未满足：${preview.gateReason || '查看下面的检查结果'}`
})

// stateTagType 让运行态标签的颜色与语义一致；颜色之外始终有中文文字。
const stateTagType = computed<'default' | 'info' | 'success' | 'warning' | 'error'>(() => {
  switch (nodeStatus.value) {
    case 'completed': return 'success'
    case 'running':
    case 'verifying': return 'info'
    case 'failed':
    case 'awaiting_reconciliation': return 'error'
    case 'paused':
    case 'stopped':
    case 'cancelled': return 'warning'
    default: return 'default'
  }
})

// statusTagType 把运行事实状态映射为弹窗标题旁的语义色，不让用户只靠文字判断。
function statusTagType(statusName: string): 'success' | 'error' | 'warning' | 'info' {
  if (statusName.includes('失败') || statusName.includes('异常')) return 'error'
  if (statusName.includes('成功') || statusName.includes('完成')) return 'success'
  if (statusName.includes('待确认') || statusName.includes('停止')) return 'warning'
  return 'info'
}

// attemptTone 给每次尝试加左边框语义色：成功绿色、失败红色、不确定橙色。
function attemptTone(attempt: RunStepAttempt): string {
  if (attempt.verdictName.includes('成功')) return 'run-panel__attempt--success'
  if (attempt.verdictName.includes('不确定') || attempt.verdictName.includes('待确认')) return 'run-panel__attempt--warning'
  return 'run-panel__attempt--error'
}

// gateTone 按门禁项文字是否包含未满足判断展示色，不改变服务端的事实口径。
function gateTone(line: string): string {
  if (line.includes('未满足') || line.includes('未通过') || line.includes('失败')) return 'run-panel__condition--failed'
  return 'run-panel__condition--passed'
}

// dialogStyle 是三个详情弹窗共用的尺寸与配色：弹窗内容被传送到 body，
// 拿不到页面里声明的自定义属性，主题色必须随弹窗自己带过去。
const themeVars = useThemeVars()
const dialogStyle = computed(() => ({
  width: '760px',
  maxWidth: '94vw',
  '--run-border-color': themeVars.value.dividerColor,
  '--run-secondary-text-color': themeVars.value.textColor3,
  '--info-color': themeVars.value.infoColor,
  '--success-color': themeVars.value.successColor,
  '--error-color': themeVars.value.errorColor,
}))
</script>

<template>
  <aside class="run-panel" aria-label="节点检视面板">
    <header class="run-panel__header">
      <div class="run-panel__identity">
        <h3 class="run-panel__title">{{ nodeName || '流程节点' }}</h3>
        <p class="run-panel__state">
          <span v-if="nodeTypeName">{{ nodeTypeName }}</span>
          <n-tag size="tiny" :bordered="false" :type="stateTagType">{{ nodeStateName }}</n-tag>
          <n-tag v-if="isCurrentNode" size="tiny" :bordered="false" type="info">当前步</n-tag>
        </p>
      </div>
      <n-button quaternary size="tiny" aria-label="关闭节点检视面板" @click="emit('close')">关闭</n-button>
    </header>

    <nav class="run-panel__tabs" role="tablist" aria-label="节点信息分类">
      <button
        type="button" class="run-panel__tab" :class="{ 'run-panel__tab--active': activeTab === 'plan' }"
        role="tab" :aria-selected="activeTab === 'plan'" @click="activeTab = 'plan'"
      >配置</button>
      <button
        type="button" class="run-panel__tab" :class="{ 'run-panel__tab--active': activeTab === 'facts' }"
        role="tab" :aria-selected="activeTab === 'facts'" @click="activeTab = 'facts'"
      >运行信息</button>
      <button
        v-if="nodeErrors.length > 0"
        type="button" class="run-panel__tab run-panel__tab--danger" :class="{ 'run-panel__tab--active': activeTab === 'errors' }"
        role="tab" :aria-selected="activeTab === 'errors'" @click="activeTab = 'errors'"
      >错误（{{ nodeErrors.length }}）</button>
    </nav>

    <div class="run-panel__body">
      <!-- 配置：只列动作序列与来源，完整口径进详情弹窗。 -->
      <section v-show="activeTab === 'plan'" aria-label="节点配置">
        <p v-if="planActions.length > 0" class="run-panel__lead">本次运行在这个节点上的已配置动作，按执行顺序：</p>
        <ul class="run-panel__rows">
          <li v-for="action in planActions" :key="action.sequence" class="run-panel__row">
            <div class="run-panel__row-main">
              <span class="run-panel__row-title">{{ action.sequence }}. {{ action.actionName }}</span>
              <span class="run-panel__row-sub">{{ action.sourceName }} · {{ action.scopeName }}<template v-if="action.releaseRequired"> · 放行边界</template></span>
            </div>
            <n-button text size="tiny" type="info" @click="planDialog = action">详情</n-button>
          </li>
        </ul>
        <n-empty
          v-if="planActions.length === 0"
          size="small"
          description="这个节点在本次运行的计划里没有动作：它不在已配置路线上，或由目标引擎自动通过。"
        />
      </section>

      <!-- 运行信息：当前步一句话结论 + 已执行步骤清单，明细进详情弹窗。 -->
      <section v-show="activeTab === 'facts'" aria-label="节点运行信息">
        <div v-if="currentPreview" class="run-panel__block">
          <div class="run-panel__block-head">
            <span class="run-panel__block-title">当前步：{{ currentPreview.actionName || currentPreview.action }}</span>
            <n-button text size="tiny" type="info" @click="openStepDialog()">详情</n-button>
          </div>
          <p>处理人：{{ currentPreview.actorName || '执行时按已配置人员策略确定' }}</p>
          <p v-if="currentPreview.expectedEffect">预期效果：{{ currentPreview.expectedEffect }}</p>
          <p :class="currentPreview.gateAllowed ? 'run-panel__ok' : 'run-panel__bad'">{{ gateBrief }}</p>
        </div>

        <p v-if="nodeSteps.length > 0" class="run-panel__lead">已执行的步骤（{{ nodeSteps.length }}）：</p>
        <ul class="run-panel__rows">
          <li v-for="step in nodeSteps" :key="step.stepNo" class="run-panel__row">
            <div class="run-panel__row-main">
              <span class="run-panel__row-title">第 {{ step.stepNo }} 步 · {{ step.actionName }}</span>
              <span class="run-panel__row-sub">{{ step.statusName }} · 耗时 {{ formatElapsed(step.durationMs) }} · 开始于 {{ formatTime(step.startedAt) }}</span>
            </div>
            <n-button text size="tiny" type="info" @click="openStepDialog(step)">详情</n-button>
          </li>
        </ul>
        <n-empty
          v-if="nodeSteps.length === 0 && !currentPreview"
          size="small"
          description="这个节点还没有执行过步骤；运行推进到这里后会在这里显示已发生的事实。"
        />
      </section>

      <!-- 错误：一行一条结论与一句话原因，依据、日志位置与可重放 curl 进详情弹窗。 -->
      <section v-show="activeTab === 'errors'" aria-label="节点错误信息">
        <ul class="run-panel__rows">
          <li v-for="row in nodeErrors" :key="row.key" class="run-panel__row run-panel__row--bad">
            <div class="run-panel__row-main">
              <span class="run-panel__row-title">{{ row.title }}</span>
              <span class="run-panel__row-verdict">{{ row.verdict }}</span>
              <span class="run-panel__row-sub" :title="row.reason">{{ row.reason }}</span>
            </div>
            <n-button v-if="row.step" text size="tiny" type="error" @click="openStepDialog(row.step)">详情</n-button>
            <n-button v-else-if="currentPreview" text size="tiny" type="error" @click="openStepDialog()">详情</n-button>
          </li>
        </ul>
      </section>
    </div>

    <!-- 计划动作详情：编译场景已经用中文写好前置条件与预期效果，这里原样呈现。 -->
    <n-modal
      :show="planDialog !== null"
      preset="card"
      :style="dialogStyle"
      :title="`计划动作：${planDialog?.actionName || ''}`"
      @update:show="planDialog = null"
    >
      <dl v-if="planDialog" class="run-panel__facts">
        <div><dt>动作</dt><dd>{{ planDialog.actionName }}</dd></div>
        <div><dt>来源</dt><dd>{{ planDialog.sourceName }}</dd></div>
        <div><dt>作用范围</dt><dd>{{ planDialog.scopeName }}</dd></div>
        <div v-if="planDialog.precondition"><dt>前置条件</dt><dd>{{ planDialog.precondition }}</dd></div>
        <div v-if="planDialog.expectedEffect"><dt>预期效果</dt><dd>{{ planDialog.expectedEffect }}</dd></div>
        <div v-if="planDialog.stopOnFailure"><dt>失败处理</dt><dd>{{ planDialog.stopOnFailure }}</dd></div>
        <div v-if="planDialog.recoveryPolicy"><dt>恢复策略</dt><dd>{{ planDialog.recoveryPolicy }}</dd></div>
          <div><dt>执行前确认当前状态</dt><dd>{{ planDialog.reloadRequired ? '需要' : '不需要' }}</dd></div>
        <div>
          <dt>动作参数</dt>
          <dd>{{ planDialog.parameterCount > 0 ? `${planDialog.parameterCount} 项，原文在 step.log 与 curl.log` : '无' }}</dd>
        </div>
      </dl>
    </n-modal>

    <!-- 当前步详情：门禁逐项、目标实时事实与即将发出的请求。 -->
    <n-modal
      :show="previewDialogOpen"
      preset="card"
      :style="dialogStyle"
      title="当前步详情（等待放行）"
      @update:show="previewDialogOpen = false"
    >
      <template v-if="currentPreview">
        <dl class="run-panel__facts">
          <div><dt>动作</dt><dd>{{ currentPreview.actionName || currentPreview.action }}</dd></div>
          <div><dt>步序</dt><dd>第 {{ currentPreview.stepNo }} 步，共 {{ currentPreview.totalSteps }} 步</dd></div>
          <div><dt>处理人</dt><dd>{{ currentPreview.actorName || '执行时按已配置人员策略确定' }}</dd></div>
          <div v-if="currentPreview.expectedEffect"><dt>预期效果</dt><dd>{{ currentPreview.expectedEffect }}</dd></div>
          <div v-if="currentPreview.endpoint"><dt>目标端点</dt><dd class="run-panel__mono">{{ currentPreview.endpoint }}</dd></div>
          <div>
            <dt>检查结论</dt>
            <dd :class="currentPreview.gateAllowed ? 'run-panel__ok' : 'run-panel__bad'">{{ gateBrief }}</dd>
          </div>
        </dl>
        <ul v-if="currentPreview.gateItems.length > 0" class="run-panel__conditions">
          <li v-for="(item, index) in currentPreview.gateItems" :key="index" :class="item.passed ? '' : 'run-panel__bad'">
            {{ item.description }}：{{ item.passed ? '满足' : '不满足' }}
          </li>
        </ul>
        <p v-if="currentPreview.blockReason" class="run-panel__bad">{{ currentPreview.blockReason }}</p>
        <p class="run-panel__block-title">目标当前状态</p>
        <p v-for="line in previewFactsText" :key="line">{{ line }}</p>
        <details class="run-panel__request">
          <summary>即将发出的请求</summary>
          <pre class="run-panel__pre">{{ currentPreview.requestPreview || '（这一步不发写请求）' }}</pre>
        </details>
      </template>
    </n-modal>

    <n-modal
      :show="stepDialog !== null"
      preset="card"
      class="run-panel__dialog"
      :style="dialogStyle"
      :title="stepDialog ? `第 ${stepDialog.stepNo} 步详情 · ${stepDialog.actionName}` : ''"
      @update:show="stepDialog = null"
    >
      <template v-if="stepDialog">
        <div class="run-panel__dialog-head">
          <n-tag size="small" :type="statusTagType(stepDialog.statusName)">{{ stepDialog.statusName }}</n-tag>
          <span class="run-panel__dialog-meta">第 {{ stepDialog.stepNo }} 步 · {{ stepDialog.actionName }}</span>
        </div>

        <section class="run-panel__section">
          <p class="run-panel__section-title">步骤事实</p>
          <dl class="run-panel__facts run-panel__facts--card">
            <div><dt>结论</dt><dd :class="stepDialog.statusName.includes('失败') ? 'run-panel__bad' : 'run-panel__ok'">{{ stepDialog.statusName }}</dd></div>
            <div><dt>演员</dt><dd>{{ stepDialog.actorName || '—' }}</dd></div>
            <div><dt>开始</dt><dd>{{ formatTime(stepDialog.startedAt) }}</dd></div>
            <div><dt>结束</dt><dd>{{ formatTime(stepDialog.finishedAt) }}</dd></div>
            <div><dt>总耗时</dt><dd>{{ formatElapsed(stepDialog.durationMs) }}</dd></div>
          </dl>
        </section>

        <section v-if="gateSnapshotLines(stepDialog).length > 0" class="run-panel__section">
          <p class="run-panel__section-title">这一步的检查</p>
          <ul class="run-panel__conditions">
            <li v-for="(line, index) in gateSnapshotLines(stepDialog)" :key="index" class="run-panel__condition" :class="gateTone(line)">
              <span class="run-panel__condition-dot"></span>
              <span>{{ line }}</span>
            </li>
          </ul>
        </section>

        <section class="run-panel__section">
          <p class="run-panel__section-title">尝试记录</p>
          <article v-for="attempt in stepDialog.attempts" :key="attempt.attemptNo" class="run-panel__attempt" :class="attemptTone(attempt)">
            <div class="run-panel__attempt-head">
              <span>第 {{ attempt.attemptNo }} 次尝试</span>
              <n-tag size="tiny" :type="statusTagType(attempt.verdictName)">{{ attempt.verdictName }}</n-tag>
              <span v-if="attempt.isReplay" class="run-panel__row-sub">（这次是重放）</span>
            </div>
            <p class="run-panel__reason">{{ attempt.reason }}</p>
            <p v-if="attempt.traceId" class="run-panel__attempt-meta">
              <span>耗时 {{ formatElapsed(attempt.durationMs) }}</span>
              <span class="run-panel__mono">{{ attempt.traceId }}</span>
            </p>
            <p v-else class="run-panel__attempt-meta">耗时 {{ formatElapsed(attempt.durationMs) }}，写请求发出前已停止，无 trace_id</p>
            <div v-if="attempt.phaseDurations" class="run-panel__phases">
              <span v-for="[phase, label] in phaseOrder" :key="phase" class="run-panel__phase">
                {{ label }} {{ formatElapsed(attempt.phaseDurations[phase] ?? -1) }}
              </span>
            </div>
            <p v-else class="run-panel__row-sub">{{ attempt.phaseDurationsNote || '暂无阶段耗时' }}</p>
            <p class="run-panel__row-sub run-panel__log-ref">
              日志：{{ attempt.logPath }} 第 {{ attempt.logLine }} 行
              <n-button text size="tiny" type="info" @click="copyLogRef(attempt)">复制日志位置</n-button>
            </p>
            <div v-if="attempt.curlBlock" class="run-panel__curl-actions">
              <n-button text size="tiny" type="info" @click="toggleCurl(stepDialog.stepNo)">
                {{ expandedCurl === String(stepDialog.stepNo) ? '收起请求与响应正文' : '展开请求与响应正文' }}
              </n-button>
              <n-button text size="tiny" type="info" @click="copyCurl(stepDialog)">复制可重放 curl</n-button>
            </div>
            <p v-else class="run-panel__no-curl">这次尝试没有发出写请求（例如没有查到当前处理人的待处理待办）；失败请求的原始 curl 与目标响应见同目录 curl.log（日志目录：{{ attempt.logPath.replace(/step\.log.*$/, '') }}curl.log）。</p>
            <pre v-if="expandedCurl === String(stepDialog.stepNo) && attempt.curlBlock" class="run-panel__pre">{{ curlText(stepDialog) }}</pre>
          </article>
        </section>
      </template>
    </n-modal>
  </aside>
</template>

<style scoped>
.run-panel {
  /* 宽度由使用方（运行详情的右侧列）决定，面板自己不声明宽度，避免两处宽度互相覆盖。 */
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  font-size: 13px;
  background: var(--run-surface-color, var(--flow-surface-color, #fff));
  border: 1px solid var(--run-border-color, var(--flow-edge-color, #ccc));
  border-radius: 8px;
}

.run-panel__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 12px;
}

.run-panel__title {
  margin: 0;
  font-size: 15px;
  line-height: 1.3;
}

.run-panel__state {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 5px 0 0;
  color: var(--run-secondary-text-color, #909090);
}

/* 页签：与主区页签同一套下划线样式，错误页签用错误色标出。 */
.run-panel__tabs {
  display: flex;
  gap: 2px;
  padding: 0 8px;
  border-bottom: 1px solid var(--run-border-color, var(--flow-edge-color, #ccc));
}

.run-panel__tab {
  padding: 7px 10px;
  color: var(--run-secondary-text-color, #909090);
  font: inherit;
  cursor: pointer;
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}

.run-panel__tab--active {
  color: var(--run-primary-color, #18a058);
  font-weight: 500;
  border-bottom-color: var(--run-primary-color, #18a058);
}

.run-panel__tab--danger.run-panel__tab--active {
  color: var(--error-color, #d03050);
  border-bottom-color: var(--error-color, #d03050);
}

.run-panel__body {
  flex: 1 1 auto;
  min-height: 0;
  padding: 10px 12px 14px;
  overflow-y: auto;
}

.run-panel__lead {
  margin: 0 0 8px;
  color: var(--run-secondary-text-color, #909090);
}

.run-panel__rows {
  display: grid;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.run-panel__row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--run-border-color, var(--flow-edge-color, #ccc));
  border-radius: 6px;
}

.run-panel__row--bad {
  border-color: color-mix(in srgb, var(--error-color, #d03050) 46%, transparent);
  background: color-mix(in srgb, var(--error-color, #d03050) 6%, transparent);
}

.run-panel__row-main {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.run-panel__row-title {
  font-weight: 600;
  line-height: 1.4;
}

.run-panel__row-verdict {
  color: var(--error-color, #d03050);
  font-weight: 600;
}

.run-panel__row-sub {
  display: -webkit-box;
  overflow: hidden;
  color: var(--run-secondary-text-color, #909090);
  line-height: 1.5;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.run-panel__block {
  display: grid;
  gap: 4px;
  margin: 0 0 12px;
  padding: 9px 10px;
  border: 1px solid var(--run-border-color, var(--flow-edge-color, #ccc));
  border-radius: 6px;
}

.run-panel__block p { margin: 0; }

.run-panel__block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.run-panel__block-title { font-weight: 600; }

.run-panel__ok { color: var(--success-color, #18a058); }
.run-panel__bad { color: var(--error-color, #d03050); }

/* 详情弹窗内的事实表：左列中文标题，右列事实，长文本自动换行。 */
.run-panel__facts {
  display: grid;
  gap: 6px;
  margin: 0 0 10px;
}

.run-panel__facts > div {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 10px;
}

.run-panel__facts dt {
  color: var(--run-secondary-text-color, #909090);
}

.run-panel__facts dd {
  margin: 0;
  line-height: 1.6;
  word-break: break-word;
}

.run-panel__mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }

.run-panel__conditions {
  margin: 0 0 10px;
  padding-left: 18px;
  line-height: 1.7;
}

.run-panel__attempt {
  display: grid;
  gap: 3px;
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px dashed var(--run-border-color, var(--flow-edge-color, #ccc));
}

.run-panel__attempt p { margin: 0; }
.run-panel__attempt-head { font-weight: 600; }
.run-panel__reason { line-height: 1.6; }

.run-panel__phases { display: flex; flex-wrap: wrap; gap: 6px; }

.run-panel__phase {
  padding: 1px 6px;
  background: color-mix(in srgb, var(--info-color, #2080f0) 10%, transparent);
  border-radius: 8px;
}

.run-panel__curl-actions { display: flex; gap: 12px; margin-top: 2px; }

.run-panel__request { margin-top: 6px; }

.run-panel__pre {
  max-height: 320px;
  margin: 6px 0 0;
  padding: 8px;
  overflow: auto;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-all;
  background: color-mix(in srgb, var(--run-border-color, #ccc) 16%, transparent);
  border-radius: 4px;
}

/* 详情弹窗的分区、事实卡片与尝试卡片：让每一步的事实、门禁和尝试层次分明。 */
.run-panel__dialog :deep(.n-card__content) {
  max-height: 72vh;
  overflow: auto;
}

.run-panel__dialog-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}

.run-panel__dialog-meta {
  color: var(--run-secondary-text-color, #909090);
}

.run-panel__section {
  margin-top: 14px;
}

.run-panel__section:first-child {
  margin-top: 0;
}

.run-panel__section-title {
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--run-secondary-text-color, #666);
}

.run-panel__facts--card {
  padding: 10px 12px;
  background: color-mix(in srgb, var(--run-border-color, #ccc) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--run-border-color, #ccc) 52%, transparent);
  border-radius: 8px;
}

.run-panel__conditions {
  display: grid;
  gap: 6px;
  padding-left: 0;
  list-style: none;
}

.run-panel__condition {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr);
  gap: 8px;
  align-items: start;
  padding: 7px 10px;
  border-radius: 7px;
  background: color-mix(in srgb, var(--run-border-color, #ccc) 8%, transparent);
}

.run-panel__condition-dot {
  width: 8px;
  height: 8px;
  margin-top: 5px;
  border-radius: 50%;
  background: var(--info-color, #2080f0);
}

.run-panel__condition--passed .run-panel__condition-dot {
  background: var(--success-color, #18a058);
}

.run-panel__condition--failed {
  background: color-mix(in srgb, var(--error-color, #d03050) 8%, transparent);
}

.run-panel__condition--failed .run-panel__condition-dot {
  background: var(--error-color, #d03050);
}

.run-panel__attempt {
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--run-border-color, #ccc) 52%, transparent);
  border-left-width: 3px;
  border-radius: 8px;
}

.run-panel__attempt--success { border-left-color: var(--success-color, #18a058); }
.run-panel__attempt--warning { border-left-color: var(--warning-color, #f0a020); }
.run-panel__attempt--error { border-left-color: var(--error-color, #d03050); }

.run-panel__attempt + .run-panel__attempt {
  margin-top: 8px;
}

.run-panel__attempt-head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.run-panel__attempt-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
  color: var(--run-secondary-text-color, #909090);
}

.run-panel__log-ref {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.run-panel__no-curl {
  margin: 4px 0 0;
  color: var(--run-secondary-text-color, #909090);
  line-height: 1.6;
}

</style>
