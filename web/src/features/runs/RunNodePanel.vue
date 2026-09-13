<script setup lang="ts">
import { NButton, NEmpty, NModal, NTag, useMessage, useThemeVars } from 'naive-ui'
import { computed, ref, watch } from 'vue'

import { formatElapsed, formatTime } from './api'
import { actionLabel } from './presentation'
import type { PathRunDetail, RunNodePlanAction, RunPreview, RunRequestItem, RunStep, RunStepAttempt } from './api'

// RunNodePanel 是点击画布节点后才出现的检视面板：三个页签只给简要事实，
// 完整内容（门禁逐项、阶段耗时、日志位置与可重放 curl）一律进详情弹窗，
// 避免把一屏堆满说明文字。写结果不确定只呈现结论与依据，不渲染任何重试或继续入口（纲领第 4.4 节）。
const props = defineProps<{
  detail: PathRunDetail
  nodeKey: string
  nodeName: string
  nodeTypeName: string
}>()

const emit = defineEmits<{ close: [], retry: [] }>()

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
// 失败或结果待确认时，如果没有 currentPreview，检查这个节点是否是最后一步所在节点。
const isCurrentNode = computed(() => {
  if (props.detail.currentPreview?.nodeId || props.detail.currentPreview?.nodeKey) {
    return (props.detail.currentPreview.nodeId || props.detail.currentPreview.nodeKey) === props.nodeKey
  }
  // 终态时，如果这是最后一步的节点，也视为当前节点
  if (props.detail.steps && props.detail.steps.length > 0) {
    const lastStep = props.detail.steps[props.detail.steps.length - 1]
    return (lastStep.nodeId || lastStep.nodeKey) === props.nodeKey
  }
  return false
})
const currentPreview = computed<RunPreview | null>(() => (isCurrentNode.value ? props.detail.currentPreview ?? null : null))

// planActions 是本次运行在这个节点上的已配置计划（服务端由编译场景归组，全部中文）。
const planActions = computed<RunNodePlanAction[]>(() => props.detail.nodePlans?.[props.nodeKey] ?? [])

// previewActionLabel 当前步动作的用户可见名称：集中映射避免原始动作键出现在界面（F-034 T03）。
const previewActionLabel = computed(() => actionLabel(currentPreview.value?.actionName, currentPreview.value?.action))

// stepActionLabel 已落账步骤的动作名：旧记录只有动作键时走同一张白名单。
function stepActionLabel(step: RunStep): string {
  return actionLabel(step.actionName, step.action)
}

// NodeErrorRow 是错误页签的一行：标题、结论、一句话原因，详情留给弹窗。
interface NodeErrorRow {
  key: string
  title: string
  verdict: string
  reason: string
  step?: RunStep
}

// isSuccessfulAttempt 兼容历史运行记录，同时让新记录统一使用“执行成功”。
// 「目标跳过」是目标平台自动跳过节点的只读结论，不是错误，不进错误页签。
function isSuccessfulAttempt(attempt: RunStepAttempt): boolean {
  return attempt.verdictName === '执行成功' || attempt.verdictName === '确定成功' || attempt.verdictName === '目标跳过'
}

// nodeErrors 汇总这个节点上需要人看一眼的事实：门禁未通过、执行失败的尝试、以及停在这里的原因。
const nodeErrors = computed<NodeErrorRow[]>(() => {
  const rows: NodeErrorRow[] = []
  const preview = currentPreview.value
  if (preview && !preview.gateAllowed) {
    rows.push({
      key: 'gate',
      title: `当前步：${actionLabel(preview.actionName, preview.action)}`,
      verdict: '条件未满足',
      reason: preview.gateReason || preview.blockReason || '见下面的检查结果',
    })
  } else if (preview?.blockReason) {
    rows.push({ key: 'block', title: `当前步：${actionLabel(preview.actionName, preview.action)}`, verdict: '放行被阻塞', reason: preview.blockReason })
  }
  for (const step of nodeSteps.value) {
    for (const attempt of step.attempts) {
      if (isSuccessfulAttempt(attempt)) continue
      rows.push({
        key: `${step.stepNo}-${attempt.attemptNo}`,
        title: `第 ${step.stepNo} 步 · ${stepActionLabel(step)}`,
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
  // 中断步定位：中断时正在执行的那一步尚未落账，节点上没有任何步骤记录，
  // 若不在这里补一行，用户点开节点一片空白，无从知道问题出在哪（实测缺陷）。
  // F-034 T06：不再无条件显示“结果待确认”——结构化阻塞 > 确定失败 > 执行中恢复 > 结果待确认。
  if (props.detail.interruptedNodeId && props.detail.interruptedNodeId === props.nodeKey && props.detail.interruptedNote) {
    rows.push({
      key: 'interrupted',
      title: '路径运行中断在这个节点',
      verdict: interruptedVerdict.value,
      reason: props.detail.interruptedNote,
    })
  }
  return rows
})

// interruptedVerdict 按运行事实判定中断节点的展示语义：
// 只有写结果不确定才用“结果待确认”；目标明确前置拒绝为“阻塞”；其他失败为“失败”。
const interruptedVerdict = computed(() => {
  if (props.detail.stopKind === 'blocked') return '阻塞'
  if (props.detail.pathRunStatus === 'failed') return '失败'
  if (props.detail.stepInFlight) return '正在执行，等待结果'
  return '结果待确认'
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
  ['plan', '确认要做什么'], ['gate', '检查能不能做'], ['control', '等你确认执行'], ['prepare', '正在准备实际操作人'],
  ['submit', '向目标提交'], ['verify', '回读确认结果'], ['settle', '保存结果'],
]

// phaseFlow 是本地执行过程的展示序列：只列实际发生过的阶段，用 F-030 的用户口径命名。
const phaseFlow = computed<Array<[string, string]>>(() => {
  const durations = stepDialog.value?.attempts[0]?.phaseDurations
  if (!durations) return phaseOrder
  return phaseOrder.filter(([phase]) => durations[phase] !== undefined)
})

// attemptRequests 返回某一步的请求明细；多尝试时合并展示（按发生顺序已在后端排好）。
function attemptRequests(step: RunStep): RunRequestItem[] {
  return step.attempts.flatMap(attempt => attempt.requests ?? [])
}

// stepOutcomeTitle 是步骤结果摘要的状态词（F-034 T06）：
// 优先级：结构化阻塞 > 确定失败 > 结果待确认 > 运行中；只有写结果不确定才用“结果待确认”。
const stepOutcomeTitle = computed(() => {
  const step = stepDialog.value
  if (!step) return ''
  // F-034 评审 #2：路径级阻塞只绑定触发它的步骤，其他步骤用自身的执行状态，
  // 防止把第 3 步的阻塞原因显示到已成功的第 1 步上。
  const stepBlocked = props.detail.stopKind === 'blocked' && props.detail.stopStepNo === step.stepNo
  if (stepBlocked) return '阻塞'
  if (step.statusName.includes('失败')) return '失败'
  if (step.statusName.includes('待确认') || step.statusName.includes('不确定')) return '结果待确认'
  if (step.statusName.includes('成功') || step.statusName.includes('完成') || step.statusName.includes('跳过')) return '成功'
  return step.statusName
})

// stepOutcomeTone 给结果摘要配语义色：阻塞与失败同色但文字不同，待确认单独色。
const stepOutcomeTone = computed<'success' | 'error' | 'warning' | 'info'>(() => {
  const title = stepOutcomeTitle.value
  if (title === '成功') return 'success'
  if (title === '阻塞' || title === '失败') return 'error'
  if (title === '结果待确认') return 'warning'
  return 'info'
})

// stepWhyText 回答“为什么不能/没能继续”（F-034 T06）：优先受控阻塞说明，其次尝试原因。
const stepWhyText = computed(() => {
  const step = stepDialog.value
  if (!step) return ''
  const stepBlocked = props.detail.stopKind === 'blocked' && props.detail.stopStepNo === step.stepNo
  if (stepBlocked) return props.detail.stopKindNote || '目标在执行前明确拒绝了请求：前置条件未满足'
  const failedAttempt = step.attempts.find(attempt => !isSuccessfulAttempt(attempt))
  return failedAttempt?.reason || ''
})

// stepNextText 回答“下一步怎么处理”（F-034 T06）：按结果语义给出可操作建议，不展示内部阶段。
const stepNextText = computed(() => {
  const title = stepOutcomeTitle.value
  if (title === '阻塞') return '请按原因修正路径或人员配置后重新发起运行；本次没有产生任何目标写入'
  if (title === '失败') return props.detail.retryable ? '可在节点面板点击「重试失败动作」，从失败步骤重新装填' : '请检查原因修正配置后重新发起运行'
  if (title === '结果待确认') return '请先在目标平台确认实例当前状态，再决定是否重新发起；工具不会自动重发同一写请求'
  return ''
})

// stepTimingNote 用一句白话解释步骤总耗时与接口等待的关系（F-034 T06）。
const stepTimingNote = computed(() => {
  const step = stepDialog.value
  if (!step) return ''
  const summary = stepRequestSummary.value
  if (!summary || summary.count === 0) return `这一步共 ${formatElapsed(step.durationMs)}，暂无真实接口耗时记录`
  const total = summary.allDurationsKnown ? formatElapsed(summary.totalMs) : `${formatElapsed(summary.totalMs)}+（部分未知）`
  return `这一步共 ${formatElapsed(step.durationMs)}，其中目标接口等待 ${total}，其余时间用于读取当前待办和保存结果`
})

// requestResultText 请求行结果：优先后端业务摘要，无摘要时退回 HTTP/传输状态（F-034 T06）。
function requestResultText(req: RunRequestItem): string {
  if (req.resultSummary) return req.resultSummary
  if (req.result === 'success') return '成功'
  if (!req.statusCode) return '传输失败'
  return `失败（HTTP ${req.statusCode}）`
}

// stepRequestSummary 多尝试汇总：聚合全部 attempt 的指标，保证明细列表与顶部指标是同一批数据
// （审查 P1：此前只取第一条 attempt，重试或重新核验时明细与指标不一致）。
const stepRequestSummary = computed(() => {
  const attempts = stepDialog.value?.attempts ?? []
  const merged = { totalMs: 0, writeMs: 0, count: 0, writeCount: 0, allDurationsKnown: true }
  let hasSummary = false
  for (const attempt of attempts) {
    const summary = attempt.requestSummary
    if (!summary) continue
    hasSummary = true
    merged.count += summary.count
    merged.writeCount += summary.writeCount
    merged.writeMs += summary.writeMs
    merged.totalMs += summary.totalMs
    if (!summary.allDurationsKnown) merged.allDurationsKnown = false
  }
  return hasSummary ? merged : null
})

// copyTrace 复制完整 trace_id，供在 curl.log / network.log 中检索原文。
async function copyTrace(traceId: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(traceId)
    message.success('已复制 trace，可在 curl.log 中检索')
  } catch {
    message.error('复制失败：浏览器剪贴板不可用，请手动从日志目录检索')
  }
}

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
  // 弹窗高度约束：内容被传送到 body，不加最大高度会冲破视口且无滚动（实测缺陷）。
  maxHeight: '86vh',
  overflowY: 'auto',
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
              <span class="run-panel__row-title">{{ action.sequence }}. {{ actionLabel(action.actionName, action.action) }}</span>
              <span class="run-panel__row-sub">{{ action.sourceName }} · {{ action.scopeName }}<template v-if="action.releaseRequired"> · 放行边界</template></span>
            </div>
            <n-button text size="tiny" type="info" @click="planDialog = action">详情</n-button>
          </li>
        </ul>
        <p v-if="planActions.length === 0" class="run-panel__lead" style="color: var(--run-secondary-text-color, #909090);">
          这个节点在本次运行的计划里没有动作：它不在已配置路线上，或由目标引擎自动通过。
        </p>
      </section>

      <!-- 运行信息：当前步一句话结论 + 已执行步骤清单，明细进详情弹窗。 -->
      <section v-show="activeTab === 'facts'" aria-label="节点运行信息">
        <div v-if="currentPreview" class="run-panel__block">
          <div class="run-panel__block-head">
            <span class="run-panel__block-title">当前步：{{ previewActionLabel }}</span>
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
              <span class="run-panel__row-title">第 {{ step.stepNo }} 步 · {{ stepActionLabel(step) }}</span>
              <span class="run-panel__row-sub">{{ step.statusName }} · 耗时 {{ formatElapsed(step.durationMs) }} · 开始于 {{ formatTime(step.startedAt) }}</span>
            </div>
            <n-button text size="tiny" type="info" @click="openStepDialog(step)">详情</n-button>
          </li>
        </ul>
        <p v-if="nodeSteps.length === 0 && !currentPreview" class="run-panel__lead" style="color: var(--run-secondary-text-color, #909090);">
          这个节点还没有执行过步骤；运行推进到这里后会在这里显示已发生的事实。
        </p>
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
        <!-- F-028 失败动作重试：只在服务端判定可重试（确定失败、无目标副作用）时出现。
             重试本身不发任何写请求，只把运行从失败步骤重新装填；人工控制模式装填后仍需按「放行」。 -->
        <div v-if="props.detail.retryable" class="run-panel__retry">
          <n-button size="small" type="primary" ghost @click="emit('retry')">重试失败动作</n-button>
          <span class="run-panel__retry-note">从失败的这一步重新装填；重试不发任何写请求</span>
        </div>
      </section>
    </div>

    <!-- 计划动作详情：编译场景已经用中文写好前置条件与预期效果，这里原样呈现。 -->
    <n-modal
      :show="planDialog !== null"
      preset="card"
      :style="dialogStyle"
      :title="`计划动作：${actionLabel(planDialog?.actionName, planDialog?.action)}`"
      @update:show="planDialog = null"
    >
      <dl v-if="planDialog" class="run-panel__facts">
        <div><dt>动作</dt><dd>{{ actionLabel(planDialog.actionName, planDialog.action) }}</dd></div>
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
      title="当前步详情"
      @update:show="previewDialogOpen = false"
    >
      <template v-if="currentPreview">
        <dl class="run-panel__facts">
          <div><dt>动作</dt><dd>{{ previewActionLabel }}</dd></div>
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
      :title="stepDialog ? `第 ${stepDialog.stepNo} 步 · ${stepActionLabel(stepDialog)}` : ''"
      @update:show="stepDialog = null"
    >
      <template v-if="stepDialog">
        <!-- F-034 T06：首屏只回答五件事——做了什么、调哪个接口、耗时多久、返回什么、为什么能/不能继续。
             内部阶段、逐项检查、参数键、日志与 curl 全部收进折叠的排查区。 -->
        <div class="run-panel__summary">
          <div class="run-panel__summary-main">
            <n-tag size="small" :type="stepOutcomeTone">{{ stepOutcomeTitle }}</n-tag>
            <span class="run-panel__summary-title">{{ stepActionLabel(stepDialog) }}</span>
            <span class="run-panel__summary-sub">{{ nodeName || stepDialog.nodeKey }} · 处理人：{{ stepDialog.actorName || '—' }}</span>
          </div>
          <span class="run-panel__summary-time">步骤总耗时 {{ formatElapsed(stepDialog.durationMs) }}</span>
        </div>

        <!-- 为什么不能/不能继续 + 下一步怎么处理：阻塞与失败时紧邻结论显示。 -->
        <div v-if="stepOutcomeTitle !== '成功'" class="run-panel__why">
          <p v-if="stepWhyText" class="run-panel__why-reason">为什么不能继续：{{ stepWhyText }}</p>
          <p v-if="stepNextText" class="run-panel__why-next">下一步：{{ stepNextText }}</p>
        </div>

        <!-- 接口请求：按真实发生顺序的简洁行列表，每行只有类型、接口、真实耗时与结果摘要。 -->
        <section class="run-panel__section">
          <p class="run-panel__section-title">接口请求</p>
          <ul v-if="attemptRequests(stepDialog).length > 0" class="run-panel__req-lines">
            <li v-for="(req, index) in attemptRequests(stepDialog)" :key="index" class="run-panel__req-line">
              <span class="run-panel__req-kind" :class="req.requestClass === 'write' ? 'run-panel__req-kind--write' : 'run-panel__req-kind--read'">
                {{ req.requestClass === 'write' ? '写入' : '读取' }}
              </span>
              <span class="run-panel__req-path">{{ req.endpoint }}</span>
              <span class="run-panel__req-num run-panel__mono">{{ req.durationKnown ? `${req.durationMs} ms` : '耗时未知' }}</span>
              <span class="run-panel__req-outcome" :class="req.result === 'success' ? 'run-panel__ok' : 'run-panel__bad'">
                {{ requestResultText(req) }}
              </span>
            </li>
          </ul>
          <p v-else class="run-panel__muted">这一步没有发出真实接口请求（历史运行或读取类步骤未产生网络日志）</p>
          <p class="run-panel__timing-note">{{ stepTimingNote }}</p>
        </section>

        <!-- 尝试结论：只保留每次尝试的一句话结论；依据、门禁逐项、阶段流水进排查区。 -->
        <section class="run-panel__section">
          <p class="run-panel__section-title">尝试结论</p>
          <p v-for="attempt in stepDialog.attempts" :key="attempt.attemptNo" class="run-panel__attempt-line">
            第 {{ attempt.attemptNo }} 次尝试：{{ attempt.verdictName }}——{{ attempt.reason }}
          </p>
        </section>

        <!-- 排查入口：默认折叠；日志位置、trace、curl、门禁逐项、阶段流水与原始正文都在这里。 -->
        <details class="run-panel__debug">
          <summary>查看日志与原始请求（排查用）</summary>
          <dl class="run-panel__debug-list">
            <template v-for="attempt in stepDialog.attempts" :key="attempt.attemptNo">
              <div><dt>日志位置</dt><dd class="run-panel__debug-path">{{ attempt.logPath }} 第 {{ attempt.logLine }} 行</dd></div>
              <div><dt></dt><dd><n-button text size="tiny" type="info" @click="copyLogRef(attempt)">复制日志位置</n-button></dd></div>
              <div v-if="attempt.traceId"><dt>trace</dt><dd class="run-panel__debug-path">{{ attempt.traceId }}</dd></div>
              <div v-if="attempt.traceId"><dt></dt><dd><n-button text size="tiny" type="info" @click="copyTrace(attempt.traceId)">复制 trace</n-button></dd></div>
            </template>
          </dl>
          <div v-if="stepDialog.attempts[0]?.curlBlock" class="run-panel__curl-actions">
            <n-button text size="tiny" type="info" @click="copyCurl(stepDialog)">复制可重放 curl</n-button>
            <n-button text size="tiny" type="info" @click="toggleCurl(stepDialog.stepNo)">
              {{ expandedCurl === String(stepDialog.stepNo) ? '收起原始请求与响应' : '展开原始请求与响应' }}
            </n-button>
          </div>
          <p v-else class="run-panel__no-curl">这次尝试没有发出写请求；完整网络日志见运行目录的 curl.log。</p>
          <pre v-if="expandedCurl === String(stepDialog.stepNo) && stepDialog.attempts[0]?.curlBlock" class="run-panel__pre">{{ curlText(stepDialog) }}</pre>

          <!-- 内部排查内容统一收在这里，标签用人话；不把阶段名当主标题。 -->
          <section v-if="gateSnapshotLines(stepDialog).length > 0" class="run-panel__section">
            <p class="run-panel__section-title">这一步的检查项</p>
            <ul class="run-panel__conditions">
              <li v-for="(line, index) in gateSnapshotLines(stepDialog)" :key="index" class="run-panel__condition" :class="gateTone(line)">
                <span class="run-panel__condition-dot"></span>
                <span>{{ line }}</span>
              </li>
            </ul>
          </section>
          <section v-if="stepDialog.attempts[0]?.phaseDurations" class="run-panel__section">
            <p class="run-panel__section-title">本地执行各环节用时</p>
            <ul class="run-panel__phase-lines">
              <li v-for="[phase, label] in phaseFlow" :key="phase">
                {{ label }}：{{ formatElapsed(stepDialog.attempts[0].phaseDurations?.[phase] ?? -1) }}
              </li>
            </ul>
          </section>
          <p v-else class="run-panel__muted">{{ stepDialog.attempts[0]?.phaseDurationsNote || '暂无本地执行过程记录' }}</p>
        </details>
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

/* F-028 重试入口：错误列表底部一行，按钮加一句不发写请求的说明，不与错误行混排。 */
.run-panel__retry {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
}

.run-panel__retry-note {
  color: var(--run-secondary-text-color, #909090);
  font-size: 12px;
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

/* F-034 T06：结果摘要下的“为什么/下一步”两行说明，紧跟结论不抢层级。 */
.run-panel__why {
  display: grid;
  gap: 2px;
  margin: 0 0 12px;
  padding: 8px 10px;
  border-left: 3px solid var(--error-color, #d03050);
  background: color-mix(in srgb, var(--error-color, #d03050) 5%, transparent);
  border-radius: 0 6px 6px 0;
}

.run-panel__why p { margin: 0; line-height: 1.6; word-break: break-word; }

.run-panel__why-next { color: var(--run-secondary-text-color, #909090); }

/* 接口请求行式布局：类型、路径（可换行）、耗时、结果一行可扫读，窄屏不溢出。 */
.run-panel__req-lines {
  display: grid;
  gap: 6px;
  margin: 0 0 8px;
  padding: 0;
  list-style: none;
}

.run-panel__req-line {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 4px 10px;
  padding: 7px 10px;
  border: 1px solid var(--run-border-color, var(--flow-edge-color, #ccc));
  border-radius: 6px;
}

.run-panel__req-path {
  flex: 1 1 200px;
  min-width: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  word-break: break-all;
}

.run-panel__req-outcome { min-width: 0; }

.run-panel__timing-note {
  margin: 0;
  color: var(--run-secondary-text-color, #909090);
  line-height: 1.6;
}

.run-panel__attempt-line {
  margin: 0 0 4px;
  line-height: 1.6;
}

/* 排查折叠区：日志路径可换行/横向滚动，复制按钮独立成行，不挤压主结论。 */
.run-panel__debug {
  margin: 4px 0 0;
  border: 1px solid var(--run-border-color, var(--flow-edge-color, #ccc));
  border-radius: 6px;
  padding: 8px 10px;
}

.run-panel__debug > summary {
  cursor: pointer;
  color: var(--run-secondary-text-color, #909090);
}

.run-panel__debug-list {
  display: grid;
  gap: 6px;
  margin: 10px 0;
}

.run-panel__debug-list > div {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr);
  gap: 8px;
  align-items: baseline;
}

.run-panel__debug-list dt { color: var(--run-secondary-text-color, #909090); }

.run-panel__debug-list dd { margin: 0; }

.run-panel__debug-path {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  word-break: break-all;
}

.run-panel__phase-lines {
  margin: 0;
  padding-left: 18px;
  line-height: 1.8;
}

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
<style scoped>
/* ===== F-030/T05 节点动作详情排版：摘要区、指标带、两 separates 结构、折叠原文。 ===== */

/* 摘要区：结论与主体信息一行，时间靠右；浅色底与卡片描边拉开层次。 */
.run-panel__summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  padding: 10px 12px;
  border: 1px solid var(--run-border-color, var(--flow-edge-color, #ccc));
  border-radius: 8px;
  background: color-mix(in srgb, var(--run-border-color, #ccc) 8%, transparent);
}

.run-panel__summary-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex-wrap: wrap;
}

.run-panel__summary-title {
  font-size: 14px;
  font-weight: 600;
}

.run-panel__summary-sub {
  color: var(--run-secondary-text-color, #909090);
}

.run-panel__summary-time {
  flex: none;
  color: var(--run-secondary-text-color, #909090);
  font-size: 12px;
  white-space: nowrap;
}

/* 指标带：四等分网格，数值大字、标签小字，视觉重心在数字上。 */
.run-panel__metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 14px;
}

.run-panel__metric {
  display: grid;
  gap: 2px;
  padding: 10px 12px;
  border: 1px solid var(--run-border-color, var(--flow-edge-color, #ccc));
  border-radius: 8px;
}

.run-panel__metric-value {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.2;
  font-variant-numeric: tabular-nums;
}

.run-panel__metric-label {
  color: var(--run-secondary-text-color, #909090);
  font-size: 12px;
}

/* 本地执行过程：横向阶段流，有耗时的阶段正常展示，无记录的阶段弱化。 */
.run-panel__phase-flow {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.run-panel__phase-step {
  display: grid;
  gap: 1px;
  padding: 6px 10px;
  border: 1px solid color-mix(in srgb, var(--run-border-color, #ccc) 52%, transparent);
  border-radius: 7px;
  background: color-mix(in srgb, var(--info-color, #2080f0) 6%, transparent);
}

.run-panel__phase-step--empty {
  opacity: 0.55;
}

.run-panel__phase-name {
  font-size: 12px;
  color: var(--run-secondary-text-color, #909090);
}

.run-panel__phase-ms {
  font-size: 13px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.run-panel__muted {
  margin: 0;
  color: var(--run-secondary-text-color, #909090);
  line-height: 1.6;
}

/* 目标请求明细表：接口名列可换行不撐破；数字右对齐等宽字体。 */
.run-panel__req-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12.5px;
}

.run-panel__req-table th,
.run-panel__req-table td {
  padding: 7px 10px;
  text-align: left;
  border-bottom: 1px solid color-mix(in srgb, var(--run-border-color, #ccc) 40%, transparent);
}

.run-panel__req-table th {
  color: var(--run-secondary-text-color, #909090);
  font-weight: 500;
}

.run-panel__req-table tbody tr:last-child td {
  border-bottom: none;
}

.run-panel__req-num {
  text-align: right !important;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.run-panel__req-kind {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 9px;
  font-size: 12px;
}

.run-panel__req-kind--write {
  color: var(--warning-color, #f0a020);
  background: color-mix(in srgb, var(--warning-color, #f0a020) 14%, transparent);
  font-weight: 600;
}

.run-panel__req-kind--read {
  color: var(--info-color, #2080f0);
  background: color-mix(in srgb, var(--info-color, #2080f0) 10%, transparent);
}

.run-panel__req-endpoint {
  min-width: 0;
}

.run-panel__req-path {
  display: block;
  word-break: break-all;
  line-height: 1.4;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
}

.run-panel__req-retry {
  display: inline-block;
  margin-top: 2px;
  color: var(--warning-color, #f0a020);
  font-size: 12px;
}

/* 窄窗口退化：指标两列、摘要换行，摘要先看、明细可滚动的布局不变。 */
@media (max-width: 720px) {
  .run-panel__metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .run-panel__summary {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }
}
</style>
<style scoped>
/* 阶段列与 trace 列：小字号弱化，trace 可点击复制。 */
.run-panel__req-phase {
  color: var(--run-secondary-text-color, #909090);
  white-space: nowrap;
}

.run-panel__req-trace {
  display: inline-block;
  margin-top: 2px;
  cursor: pointer;
  color: var(--info-color, #2080f0);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
}
</style>
