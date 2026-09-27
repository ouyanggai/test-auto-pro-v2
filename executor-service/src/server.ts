// 独立执行服务：健康检查 + 会话生命周期。
// POST /runtime/session 只创建 BrowserContext，不导航；装载前必须先连转发通道。
import http from 'node:http'
import childProcess from 'node:child_process'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { randomUUID } from 'node:crypto'
import { WebSocketServer, type WebSocket } from 'ws'
import { launchHeadlessChromium, HEADLESS_LAUNCH_ARGS } from './browser.ts'
import { healthBody, listenPort, SERVICE_NAME } from './health.ts'
import { SESSION_EVENTS, SessionManager } from './session-manager.ts'
import { ForwardChannel, ForwardFailure } from './forward-channel.ts'
import { installTargetRoute, type TargetRouteHandle } from './target-route.ts'
import { assertForwardReady } from './navigate.ts'
import { extractPageDist, serveFlowPage } from './page-origin.ts'
import { encodeEnvelope, PROTOCOL_VERSION, validateMessage, type MessageEnvelope } from './protocol.ts'
import { InitiateBlockedError, planTemplateSelection } from './native-initiate.ts'
import { resolveDetailSemantics } from './native-detail.ts'
import { UnsupportedFormError } from './data-patch.ts'
import { canContinueChoice, CHOICE_ERRORS } from './native-actions.ts'
import { nativeChoiceContinue, nativeDispatch, type ChoiceContinueChoice, type DispatchInput } from './native-dispatch.ts'
import { isPermissionCheckSubmit, settleCompletion, WriteGate } from './completion.ts'
import type { Page } from 'playwright'

interface RuntimeSession {
  sessionId: string
  accountKey: string
  runId: string
  pathRunId: string
  stepId: string
  page: Page
  channel: ForwardChannel
  route: TargetRouteHandle | null
  loaded: boolean
  actionInFlight: boolean
  destroyed: boolean
  writeGate: WriteGate
  transpondConfirmed: boolean
  choicePending: boolean
  choicePendingAt: number
  lastChoiceError: Record<string, unknown> | null
  choiceState: { continued: boolean }
  identity: Record<string, unknown> | null
  nativeHost: Record<string, unknown> | null
  loadMode: string
  instanceId: string
  forwardPostscriptRequired: boolean
  formValidationError?: string
  formBefore: Record<string, unknown>
}

interface SessionCreateBody {
  accountKey?: string
  runId?: string
  pathRunId?: string
  stepId?: string
}

async function readJSON (request: http.IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = []
  for await (const chunk of request) chunks.push(chunk as Buffer)
  const raw = Buffer.concat(chunks).toString('utf8') || '{}'
  return JSON.parse(raw)
}

function sendEnvelope (socket: WebSocket, session: RuntimeSession, kind: MessageEnvelope['kind'], requestId: string, payload: unknown): void {
  socket.send(JSON.stringify(encodeEnvelope(kind, session, requestId, payload)))
}

// CAPTURE_TIMEOUT_MS 是捕获/就绪等待上限：原生装载包含模板、权限与远程选项读取，
// 全部经 Go 转发，30 秒在慢目标下不够；取 60 秒，超时按装载失败阻塞（保留 500 门禁）。
const CAPTURE_TIMEOUT_MS = Number(process.env.EXECUTOR_CAPTURE_TIMEOUT_MS || 60000)

// waitForExecutorNative 等待注入脚本捕获到原生宿主实例（initFlow 或 flowDetail 任一）。
export async function waitForExecutorNative (page: Page): Promise<Record<string, unknown>> {
  await page.waitForFunction(
    'window.__executorNative && (window.__executorNative.initFlow || window.__executorNative.flowDetail)',
    undefined,
    { timeout: CAPTURE_TIMEOUT_MS }
  )
  const captured = await page.evaluate(() => (window as unknown as { __executorNative: Record<string, unknown> }).__executorNative)
  if (!captured || (!captured.initFlow && !captured.flowDetail)) {
    throw new Error('原生宿主捕获失败')
  }
  return captured
}

// waitForInitiateReady 等待发起装载就绪：原生已拿到模板详情并打开 FlowDialog。
// 只看「实例非空」会把半装载页面当成功（施工令 §2 假修清单），必须等 flowJson.id。
async function waitForInitiateReady (page: Page): Promise<void> {
  await page.waitForFunction(
    `(() => {
      const instance = window.__executorNative && window.__executorNative.initFlow;
      const json = Array.isArray(instance?.flowJson) ? instance.flowJson[0] : instance?.flowJson;
      return Boolean(instance && instance.approveDialogVisible === true && json && json.id);
    })()`,
    undefined,
    { timeout: CAPTURE_TIMEOUT_MS }
  )
}

// waitForDetailReady 等待详情容器与实例正文同时就绪；仅弹窗可见不代表数据已加载。
async function waitForDetailReady (page: Page): Promise<void> {
  await page.waitForFunction(
    `(() => {
      const instance = window.__executorNative && window.__executorNative.flowDetail;
      if (!instance) return false;
      return instance.examineDialogVisible === true || instance.this_EnterpriseExamineDialogVisible === true || instance.ExpensesClaimFormVisible === true;
    })()`,
    undefined,
    { timeout: CAPTURE_TIMEOUT_MS }
  )
  const deadline = Date.now() + CAPTURE_TIMEOUT_MS
  while (Date.now() < deadline) {
    if (Object.keys(await readNativeFormSnapshot(page)).length > 0) return
    await new Promise(resolve => setTimeout(resolve, 200))
  }
  throw new UnsupportedFormError('详情容器已打开，但实例表单正文在等待窗口内始终为空')
}

// readNativeFormSnapshot 优先通过原生读写接口取值，再回退到已知业务数据对象；
// 不返回任意组件状态，避免把任务上下文或 UI 配置误当成表单正文。
async function readNativeFormSnapshot (page: Page): Promise<Record<string, unknown>> {
  return page.evaluate(async () => {
    interface VNode { $children?: VNode[], $refs?: Record<string, unknown>, [key: string]: unknown }
    const native = (window as unknown as { __executorNative?: { initFlow?: VNode, flowDetail?: VNode, forwardList?: VNode } }).__executorNative
    const root = (native?.initFlow || native?.flowDetail || native?.forwardList) as VNode | undefined
    if (!root) return {}
    const queue: unknown[] = [root]
    const seen = new Set<unknown>()
    const nodes: Record<string, unknown>[] = []
    while (queue.length > 0) {
      const candidate = queue.shift()
      if (!candidate || typeof candidate !== 'object' || seen.has(candidate)) continue
      seen.add(candidate)
      const node = candidate as Record<string, unknown>
      nodes.push(node)
      queue.push(...((node.$children as unknown[] | undefined) ?? []), ...Object.values((node.$refs as Record<string, unknown> | undefined) ?? {}))
    }
    for (const node of nodes) {
      const getter = ((typeof node.setData === 'function' || typeof node.setValues === 'function') && typeof node.getValues === 'function')
        ? node.getValues
        : typeof node.setFormData === 'function' && typeof node.getFormData === 'function' ? node.getFormData : null
      if (typeof getter !== 'function') continue
      try {
        const values = await (getter as () => unknown).call(node)
        if (values && typeof values === 'object' && !Array.isArray(values) && Object.keys(values as object).length > 0) {
          const sanitized: Record<string, unknown> = {}
          for (const [k, v] of Object.entries(values as Record<string, unknown>)) {
            if (k.trim() !== '') sanitized[k.trim()] = v
          }
          return sanitized
        }
      } catch {}
    }
    const keys = ['editData', 'currentFormData', 'formData', 'infoForm', 'formModel', 'detailData']
    for (const key of keys) {
      let best: Record<string, unknown> = {}
      for (const node of nodes) {
        const value = node[key]
        if (!value || typeof value !== 'object' || Array.isArray(value)) continue
        const record = value as Record<string, unknown>
        if (Object.keys(record).length > Object.keys(best).length) best = record
      }
      if (Object.keys(best).length > 0) {
        const sanitized: Record<string, unknown> = {}
        for (const [k, v] of Object.entries(best)) {
          if (k.trim() !== '') sanitized[k.trim()] = v
        }
        return sanitized
      }
    }
    return {}
  })
}

// prepareForwardList 在原生列表数据模型中精确选中实例与任务，调用 transpond 完成弹窗准备。
async function prepareForwardList (page: Page, input: { tab: string, instanceId: string, jobTaskId: string, receiverId: string, receiverName: string, checkList: string[] }): Promise<void> {
  try {
    await page.waitForFunction(({ tab, instanceId, jobTaskId }) => {
      interface VNode { $children?: VNode[], $refs?: Record<string, unknown> }
      const root = (window as unknown as { $vue?: VNode }).$vue
      if (!root) return false
      const queue: VNode[] = [root]
      while (queue.length > 0) {
        const node = queue.shift()!
        const child = node.$refs?.[tab] as { tableData?: Array<Record<string, unknown>> } | undefined
        if (child && Array.isArray(child.tableData)) {
          return child.tableData.some(row => String(row.flowInstanceId ?? row.id ?? '') === instanceId && (!jobTaskId || String(row.jobTaskId ?? '') === jobTaskId))
        }
        queue.push(...(node.$children ?? []))
      }
      return false
    }, { tab: input.tab, instanceId: input.instanceId, jobTaskId: input.jobTaskId }, { timeout: CAPTURE_TIMEOUT_MS })
  } catch (error) {
    // 超时不能让排障停在"等不到行"：把页面真实看到了什么一并说清楚——
    // 列表组件是否存在、渲染了多少行、有没有同实例但不同任务号的行。
    // 这是把"超时"翻译成可定位事实的关键证据，不猜测原因。
    const evidence = await page.evaluate(({ tab, instanceId }) => {
      interface VNode { $children?: VNode[], $refs?: Record<string, unknown>, $options?: { name?: string } }
      const root = (window as unknown as { $vue?: VNode }).$vue
      if (!root) return { vueRoot: false, listFound: false }
      const queue: VNode[] = [root]
      const seen = new Set<VNode>()
      let listFound = false
      let rowCount = -1
      const sameInstanceRows: Array<{ jobTaskId: string }> = []
      while (queue.length > 0) {
        const node = queue.shift()!
        if (!node || seen.has(node)) continue
        seen.add(node)
        const child = node.$refs?.[tab] as { tableData?: Array<Record<string, unknown>> } | undefined
        if (child && Array.isArray(child.tableData)) {
          listFound = true
          rowCount = child.tableData.length
          for (const row of child.tableData) {
            if (String(row.flowInstanceId ?? row.id ?? '') === instanceId) {
              sameInstanceRows.push({ jobTaskId: String(row.jobTaskId ?? '') })
            }
          }
          break
        }
        queue.push(...(node.$children ?? []))
      }
      return { vueRoot: true, listFound, rowCount, sameInstanceRows }
    }, { tab: input.tab, instanceId: input.instanceId }).catch(() => ({ evidenceFailed: true }))
    throw new Error(`转发列表装载超时：未在「${input.tab}」标签页等到实例 ${input.instanceId} / 任务 ${input.jobTaskId} 的行；页面事实=${JSON.stringify(evidence)}`)
  }
  await page.evaluate(async payload => {
    interface VNode { $children?: VNode[], $refs?: Record<string, unknown> }
    interface ListHost extends VNode {
      tableData?: Array<Record<string, unknown>>
      transpond?: (tab: string) => unknown
      transpondVisible?: boolean
      approveForm?: Record<string, unknown>
    }
    const native = (window as unknown as { __executorNative?: Record<string, unknown> }).__executorNative ?? ((window as unknown as { __executorNative: Record<string, unknown> }).__executorNative = {})
    const root = (window as unknown as { $vue?: VNode }).$vue
    const queue: VNode[] = root ? [root] : []
    let host: ListHost | undefined
    while (queue.length > 0 && !host) {
      const node = queue.shift()!
      const candidate = node.$refs?.[payload.tab] as ListHost | undefined
      if (candidate && Array.isArray(candidate.tableData)) host = candidate
      queue.push(...(node.$children ?? []))
    }
    if (!host || typeof host.transpond !== 'function') throw new Error(`原生列表页缺少 ${payload.tab}.transpond 入口`)
    const rows = (host.tableData ?? []).filter(row => String(row.flowInstanceId ?? row.id ?? '') === payload.instanceId && (!payload.jobTaskId || String(row.jobTaskId ?? '') === payload.jobTaskId))
    if (rows.length !== 1) throw new Error(`转发行匹配数量为 ${rows.length}，必须按实例和任务唯一命中`)
    const dytable = host.$refs?.dytable as { selectDatas?: Array<Record<string, unknown>>, toggleRowSelection?: (row: Record<string, unknown>, selected: boolean) => void } | undefined
    if (!dytable) throw new Error('转发列表缺少 dytable 选择接口')
    if (typeof dytable.toggleRowSelection === 'function') dytable.toggleRowSelection(rows[0]!, true)
    dytable.selectDatas = [rows[0]!]
    await host.transpond(payload.tab)
    if (host.transpondVisible !== true || !host.approveForm) throw new Error('原生转发弹窗未完成准备')
    host.approveForm.personId = payload.receiverId
    host.approveForm.personName = payload.receiverName
    host.approveForm.checkList = payload.checkList
    native.forwardList = host
  }, input)
}

// collectHostPath 从捕获实例向下递归找目标组件链（按 $options.name 唯一命中），
// 返回根到目标组件的名称链；找不到返回空数组（调用方据此阻塞，绝不静默跳过）。
async function collectHostPath (page: Page, targetNames: string[]): Promise<string[]> {
  return page.evaluate(names => {
    const native = (window as unknown as { __executorNative?: Record<string, unknown> }).__executorNative
    interface VNode { $options?: { name?: string }, $children?: VNode[] }
    const root = (native?.initFlow || native?.flowDetail) as VNode | undefined
    if (!root) return []
    const wanted = names as string[]
    const path: string[] = [String(root.$options?.name ?? '')]
    // 逐级消耗 wanted，返回匹配到的最深前缀链：审批侧 ExamineOpinion 挂载晚于
    // EnterpriseExamineDialog（内层 v-if），装载期只要求到 EnterpriseExamineDialog，
    // ExamineOpinion 在动作执行期按需再解析；调用方按链内容决定是否阻塞。
    const walk = (node: VNode, wantIndex: number): boolean => {
      if (wantIndex >= wanted.length) return true
      for (const child of node.$children ?? []) {
        if (String(child?.$options?.name ?? '') !== wanted[wantIndex]) continue
        path.push(String(child?.$options?.name ?? ''))
        if (walk(child, wantIndex + 1)) return true
        return true
      }
      return false
    }
    if (path[0] === wanted[0]) {
      walk(root, 1)
      return path
    }
    walk(root, 0)
    return path
  }, targetNames)
}

// collectPageErrorEvidence 统计「无 URL 的页面异常」（真实 JS 崩溃：TypeError 等），仅作证据。
// 带 URL 的 Network Error/404 属请求级结果：目标请求的成败已由 trace（aborted/HTTP 状态/
// isSuccess 包络）覆盖，FormMaking 外部服务（tools-server）的失败也与业务完成无关；
// 实测同一请求 trace 为 fulfilled 仍会抛 Network Error（应用层噪声），按 URL 排除。
// 选择要求的中间态业务消息（目标要求选分支/选人，选择继续后即被覆盖）同样不算终局异常：
// 写层面的真相由 trace 的 supersede 逻辑裁决（completion.ts finalize）。
async function collectPageErrorEvidence (page: Page): Promise<number> {
  return page.evaluate(() => {
    const CHOICE_MESSAGES = ['手动条件分支,请选择', '请选择流程分支', '未设置审批人', '该分支需选择审批人，请先选择']
    const errors = (window as unknown as { __executorPageErrors?: Array<{ kind?: string, message?: string, url?: string }> }).__executorPageErrors ?? []
    return errors.filter(error => {
      if (String(error?.url ?? '') !== '') return false
      const message = String(error?.message ?? '')
      return !CHOICE_MESSAGES.includes(message)
    }).length
  })
}

// describeCapturedTree 描述捕获实例的直接子组件与关键可见标志：hostPath 找不到目标组件时
// 给诊断一个真实树形快照（绝不静默返回空链）。
async function describeCapturedTree (page: Page): Promise<string> {
  return page.evaluate(() => {
    interface VNode { $options?: { name?: string }, $children?: VNode[] }
    const native = (window as unknown as { __executorNative?: Record<string, unknown> }).__executorNative
    const root = (native?.initFlow || native?.flowDetail) as (VNode & { examineDialogVisible?: unknown, ExpensesClaimFormVisible?: unknown, approveDialogVisible?: unknown }) | undefined
    if (!root) return '捕获实例为空'
    const names = (node: VNode): string[] => (node.$children ?? []).map(child => String(child?.$options?.name ?? '<anonymous>'))
    return `root=(${names(root).join(',')}) examineDialogVisible=${String(root.examineDialogVisible)} ExpensesClaimFormVisible=${String(root.ExpensesClaimFormVisible)} approveDialogVisible=${String(root.approveDialogVisible)}`
  })
}

// ChoiceContinueChoice 是 Go 回传的选择继续载荷（协议 ChoicePayload 镜像）。
// dispatchChoiceContinueOnPage 在页面内把选择继续落到原生组件（S6）。
// 与动作分发同一纪律：页面内执行 native-dispatch.ts 的同一份 nativeChoiceContinue 实现
// （经 Function 源串构造进页面），Node 侧单测直测同一实现，断言打在生产分发路径上（§8.2）。
function dispatchChoiceContinueOnPage (page: Page, errorType: string, choice: ChoiceContinueChoice, choiceError: Record<string, unknown> | null): Promise<{ component: string, method: string }> {
  const inPage = new Function('args',
    'const native = (window.__executorNative || undefined);' +
    'return (' + nativeChoiceContinue.toString() + ')(native, args.errorType, args.choice, args.choiceError);')
  // Function 构造出的执行体不在 Playwright 的函数类型收窄范围内，这里显式宽化调用。
  return (page.evaluate as (fn: unknown, arg: unknown) => Promise<{ component: string, method: string }>)(inPage, { errorType, choice, choiceError })
}

// dispatchBusinessAction 把动作落到真实原生组件（S4）。
// 页面内执行的是 native-dispatch.ts 的同一份 nativeDispatch 实现（经 Function 源串构造），
// Node 侧单元测试直接调用该实现，断言打在生产分发路径上（§8.2）。
function dispatchBusinessAction (page: Page, input: DispatchInput): Promise<{ component: string, method: string }> {
  const inPage = new Function('input',
    'const native = (window.__executorNative || undefined);' +
    'return (' + nativeDispatch.toString() + ')(native, input);')
  return page.evaluate(inPage as (input: DispatchInput) => { component: string, method: string }, input)
}

// autoConfirmMessageBox 在页面内自动点击 Element MessageBox 的确认主按钮（S4 已知差异：
// 设计禁止用 Playwright 定位器点业务控件；这里是 page.evaluate 内触发 DOM click，
// 复刻「用户点确认」的原生路径）。找不到按钮按超时失败，绝不静默跳过。
async function autoConfirmMessageBox (page: Page, timeoutMs = 10000): Promise<void> {
  const startedAt = Date.now()
  while (Date.now() - startedAt < timeoutMs) {
    const clicked = await page.evaluate(() => {
      const buttons = Array.from(document.querySelectorAll('.el-message-box__btns button'))
      const primary = buttons.find(button => button.classList.contains('el-button--primary')) as HTMLButtonElement | undefined
      const target = primary ?? buttons[buttons.length - 1] as HTMLButtonElement | undefined
      if (!target) return false
      target.click()
      return true
    })
    if (clicked) return
    await new Promise(resolve => setTimeout(resolve, 120))
  }
  throw new Error('确认弹窗未出现或没有可点击按钮')
}

// parseTemplateCandidateIds 从 trace 里 /web/flowTemplateApi/list 的真实响应体解出候选模板 ID。
// 选择判据只认这份真实响应（S2）：解析失败或没有 fulfilled 响应返回 null。
function parseTemplateCandidateIds (trace: { list: () => Array<{ url: string, state: string, response?: { httpStatus: number, body?: string, bodyEncoding?: string } }> }): string[] | null {
  const entries = trace.list().filter(entry => entry.url.includes('/web/flowTemplateApi/list'))
  const fulfilled = entries.filter(entry => entry.state === 'fulfilled' && entry.response?.httpStatus === 200)
  const last = fulfilled[fulfilled.length - 1]
  if (!last || !last.response?.body) return null
  try {
    let text = last.response.body
    if (last.response.bodyEncoding === 'base64') {
      text = Buffer.from(last.response.body, 'base64').toString('utf8')
    }
    const parsed = JSON.parse(text) as { data?: Array<{ id?: string | number }> | { list?: Array<{ id?: string | number }> } }
    const rows = Array.isArray(parsed.data) ? parsed.data : Array.isArray((parsed.data as { list?: unknown[] })?.list) ? (parsed.data as { list: Array<{ id?: string | number }> }).list : []
    return rows.map(row => String(row?.id ?? '')).filter(id => id !== '')
  } catch {
    return null
  }
}

// formApiAvailable 按“宿主是否提供读写接口”探测，不按渲染类型或组件名称分流。
async function formApiAvailable (page: Page, _mode: string): Promise<boolean> {
  return page.evaluate(() => {
    interface VNode { $children?: VNode[], $refs?: Record<string, unknown>, [key: string]: unknown }
    const native = (window as unknown as { __executorNative?: { initFlow?: unknown, flowDetail?: unknown, forwardList?: unknown } }).__executorNative
    const container = (native?.initFlow || native?.flowDetail || native?.forwardList) as VNode | undefined
    if (!container) return false
    const queue: unknown[] = [container]
    const seen = new Set<unknown>()
    while (queue.length > 0) {
      const candidate = queue.shift()
      if (!candidate || typeof candidate !== 'object' || seen.has(candidate)) continue
      seen.add(candidate)
      const value = candidate as Record<string, unknown>
      const hasPair = (typeof value.setData === 'function' && typeof value.getValues === 'function') ||
        (typeof value.setValues === 'function' && typeof value.getValues === 'function') ||
        (typeof value.setFormData === 'function' && typeof value.getFormData === 'function')
      if (hasPair) return true
      queue.push(...((value.$children as unknown[] | undefined) ?? []), ...Object.values((value.$refs as Record<string, unknown> | undefined) ?? {}))
    }
    return false
  })
}

// applyNodePatchViaPage 在页面内一次完成「数据接口解析 → 只写允许字段 → 回读完整值」。
// 返回回读值；宿主缺失返回 null，由调用方按 unsupported 阻塞。
async function applyNodePatchViaPage (page: Page, _mode: string, patch: Record<string, unknown>, allowedKeys: string[], identity: { userId: string, companyId: string }): Promise<Record<string, unknown> | null> {
  return page.evaluate(async ({ patch, keys, identity }) => {
    interface VNode { $children?: VNode[], $refs?: Record<string, unknown>, [key: string]: unknown }
    const native = (window as unknown as { __executorNative?: { initFlow?: unknown, flowDetail?: unknown, forwardList?: unknown } }).__executorNative
    const container = (native?.initFlow || native?.flowDetail || native?.forwardList) as VNode | undefined
    if (!container) return null
    const queue: unknown[] = [container]
    const seen = new Set<unknown>()
    let api: Record<string, unknown> | null = null
    while (queue.length > 0 && api === null) {
      const candidate = queue.shift()
      if (!candidate || typeof candidate !== 'object' || seen.has(candidate)) continue
      seen.add(candidate)
      const value = candidate as Record<string, unknown>
      if ((typeof value.setData === 'function' && typeof value.getValues === 'function') ||
        (typeof value.setValues === 'function' && typeof value.getValues === 'function') ||
        (typeof value.setFormData === 'function' && typeof value.getFormData === 'function')) api = value
      queue.push(...((value.$children as unknown[] | undefined) ?? []), ...Object.values((value.$refs as Record<string, unknown> | undefined) ?? {}))
    }
    if (!api) return null
    const setter = (api.setData ?? api.setValues ?? api.setFormData) as (values: Record<string, unknown>) => unknown
    const getter = (api.getValues ?? api.getFormData) as () => unknown
    // F-043 缺陷②：先读初始化后的表单值——表单初始化事件已按注入身份填好「当前用户」类字段。
    const initValues = (await getter.call(api)) as Record<string, unknown> | null
    const ids = [identity.userId, identity.companyId].filter(id => typeof id === 'string' && id.trim() !== '')
    const isCurrentUser = (value: unknown): boolean => {
      if (ids.length === 0 || value === null || value === undefined) return false
      let text: string
      try { text = typeof value === 'string' ? value : JSON.stringify(value) } catch { return false }
      return !!text && ids.some(id => text.includes(id))
    }
    const companions = ['__virtualName', '__formPersonId', '__condition']
    const picked: Record<string, unknown> = {}
    for (const key of keys) {
      // 当前用户字段由表单初始化事件按注入身份填好，不用历史发起人值覆盖（缺陷②）。
      // 但若待写入补丁自身也是当前用户的值（例如带有完整中文名的选择器），则保留补丁，不予跳过。
      if (initValues && isCurrentUser(initValues[key])) {
        const patchVal = patch[key]
        if (!(patchVal !== undefined && patchVal !== null && patchVal !== '' && isCurrentUser(patchVal))) {
          continue
        }
      }
      if (Object.prototype.hasOwnProperty.call(patch, key)) picked[key] = patch[key]
      for (const suffix of companions) {
        const companion = `${key}${suffix}`
        if (Object.prototype.hasOwnProperty.call(patch, companion)) picked[companion] = patch[companion]
      }
    }
    await setter.call(api, picked)
    const values = await getter.call(api)
    if (values === null || values === undefined || typeof values !== 'object') return null
    return values as Record<string, unknown>
  }, { patch, keys: allowedKeys, identity })
}
// restoreIntendedPatchViaPage 对比页面实时值与意图值，自动恢复被异步联动脚本误清空的有效意图值。
async function restoreIntendedPatchViaPage (page: Page, _mode: string, patch: Record<string, unknown>, allowedKeys: string[], identity: { userId: string, companyId: string }): Promise<Record<string, unknown> | null> {
  return page.evaluate(async ({ patch, keys, identity }) => {
    interface VNode { $children?: VNode[], $refs?: Record<string, unknown>, [key: string]: unknown }
    const native = (window as unknown as { __executorNative?: { initFlow?: unknown, flowDetail?: unknown, forwardList?: unknown } }).__executorNative
    const container = (native?.initFlow || native?.flowDetail || native?.forwardList) as VNode | undefined
    if (!container) return null
    const queue: unknown[] = [container]
    const seen = new Set<unknown>()
    let api: Record<string, unknown> | null = null
    while (queue.length > 0 && api === null) {
      const candidate = queue.shift()
      if (!candidate || typeof candidate !== 'object' || seen.has(candidate)) continue
      seen.add(candidate)
      const value = candidate as Record<string, unknown>
      if ((typeof value.setData === 'function' && typeof value.getValues === 'function') ||
        (typeof value.setValues === 'function' && typeof value.getValues === 'function') ||
        (typeof value.setFormData === 'function' && typeof value.getFormData === 'function')) api = value
      queue.push(...((value.$children as unknown[] | undefined) ?? []), ...Object.values((value.$refs as Record<string, unknown> | undefined) ?? {}))
    }
    if (!api) return null
    const getter = (api.getValues ?? api.getFormData) as () => unknown
    const setter = (api.setData ?? api.setValues ?? api.setFormData) as (values: Record<string, unknown>) => unknown
    const current = (await getter.call(api)) as Record<string, unknown> | null
    if (!current || typeof current !== 'object') return null
    // F-043 缺陷②：当前用户字段由表单初始化事件按注入身份填好，联动回补时不得恢复历史发起人值。
    const currentUserIds = [identity.userId, identity.companyId].filter(id => typeof id === 'string' && id.trim() !== '')
    const isCurrentUserValue = (value: unknown): boolean => {
      if (currentUserIds.length === 0 || value === null || value === undefined) return false
      let text: string
      try { text = typeof value === 'string' ? value : JSON.stringify(value) } catch { return false }
      return !!text && currentUserIds.some(id => text.includes(id))
    }

    const restorePatch: Record<string, unknown> = {}
    let needRestore = false

    const parseId = (val: unknown): string | null => {
      if (!val) return null
      if (typeof val === 'object' && val !== null && 'id' in val && (val as { id?: unknown }).id) return String((val as { id?: unknown }).id)
      if (typeof val === 'string' && val.trim().startsWith('{')) {
        try {
          const obj = JSON.parse(val)
          if (obj && obj.id) return String(obj.id)
        } catch {}
      }
      return null
    }

    for (const key of keys) {
      const intended = patch[key]
      const live = current[key]
      if (isCurrentUserValue(live)) continue
      if (intended === undefined || intended === null || intended === '') continue

      if (Array.isArray(intended)) {
        if (Array.isArray(live) && live.length > 0) {
          let arrayChanged = false
          const mergedArray = live.map((row, rIdx) => {
            const intendedRow = intended[rIdx]
            if (!intendedRow || typeof intendedRow !== 'object' || !row || typeof row !== 'object') return row
            const newRow = { ...(row as Record<string, unknown>) }
            for (const [col, colVal] of Object.entries(intendedRow as Record<string, unknown>)) {
              if (colVal !== undefined && colVal !== null && colVal !== '' && (newRow[col] === undefined || newRow[col] === null || newRow[col] === '')) {
                newRow[col] = colVal
                arrayChanged = true
              }
            }
            return newRow
          })
          if (arrayChanged) {
            restorePatch[key] = mergedArray
            needRestore = true
          }
        }
        continue
      }

      const intendedId = parseId(intended)
      if (intendedId) {
        const liveId = parseId(live)
        if (!liveId || liveId !== intendedId) {
          restorePatch[key] = intended
          needRestore = true
          continue
        }
      }

      if (live === undefined || live === null || live === '') {
        restorePatch[key] = intended
        needRestore = true
      }
    }

    for (const [key, val] of Object.entries(current)) {
      if (key.endsWith('_col') && (val === undefined || val === null || val === '')) {
        const targetCol = key.slice(0, -4)
        for (const listVal of Object.values(current)) {
          if (Array.isArray(listVal) && listVal.length > 0) {
            const firstRow = listVal[0]
            if (firstRow && typeof firstRow === 'object' && targetCol in firstRow) {
              const cVal = (firstRow as Record<string, unknown>)[targetCol]
              if (cVal !== undefined && cVal !== null && cVal !== '') {
                restorePatch[key] = cVal
                needRestore = true
                break
              }
            }
          }
        }
      }
    }

    if (needRestore) {
      await setter.call(api, restorePatch)
      return (await getter.call(api)) as Record<string, unknown>
    }
    return current
  }, { patch, keys: allowedKeys, identity })
}

// applyPlanIdentityViaPage 在数据落定后,用表单自身的初始化脚本(读注入身份的 localStorage)在空值探针下
// 重算「当前用户」类字段(申请人/部门/公司等)的计划账号取值,只覆盖取值确实来自注入身份的字段。
// 目标表单 refresh 初始化脚本对已有值字段用「仅空值填充」守卫跳过,历史回放时这些字段残留历史发起人,
// 本函数作为数据落定最后一步权威覆盖(F-043 缺陷②),业务字段与历史真实选定的他人不受影响。
async function applyPlanIdentityViaPage (page: Page, identity: { userId: string, accountName: string, companyId: string, companyName: string, departmentId: string, departmentName: string }): Promise<Record<string, unknown> | null> {
  return page.evaluate(async ({ identity }) => {
    interface VNode { $children?: VNode[], $refs?: Record<string, unknown>, [key: string]: unknown }
    const native = (window as unknown as { __executorNative?: { initFlow?: unknown, flowDetail?: unknown, forwardList?: unknown } }).__executorNative
    const container = (native?.initFlow || native?.flowDetail || native?.forwardList) as VNode | undefined
    if (!container) return null
    const queue: unknown[] = [container]
    const seen = new Set<unknown>()
    let api: Record<string, unknown> | null = null
    while (queue.length > 0 && api === null) {
      const candidate = queue.shift()
      if (!candidate || typeof candidate !== 'object' || seen.has(candidate)) continue
      seen.add(candidate)
      const value = candidate as Record<string, unknown>
      // 必须是带 eventScript 的 FormMaking 实例(有 setData/getValues + formData),否则无法探针初始化脚本。
      if (typeof value.setData === 'function' && typeof value.getValues === 'function' && value.formData) api = value
      queue.push(...((value.$children as unknown[] | undefined) ?? []), ...Object.values((value.$refs as Record<string, unknown> | undefined) ?? {}))
    }
    if (!api) return null
    // FormMaking 实例是页面内不可表达的库类型:以命名常量断言其脚本容器与编译函数表,仅本函数内读取。
    const formData = api.formData as { config?: { eventScript?: Array<{ key?: string, func?: string }> }, eventFunction?: Record<string, unknown> } | undefined
    const scripts = (formData && formData.config && Array.isArray(formData.config.eventScript)) ? formData.config.eventScript : []
    // 只重跑「读 invest-power-system-* 身份」的初始化脚本,避免触发无关业务事件。
    const identityScripts = scripts.filter(script => script && typeof script.func === 'string' && script.func.indexOf('invest-power-system-') >= 0)
    const eventFns = (api.eventFunction ?? (formData && formData.eventFunction)) as Record<string, unknown> | undefined
    if (identityScripts.length === 0 || !eventFns) return null
    const captured: Record<string, unknown> = {}
    const orig = { getValue: api.getValue, getValues: api.getValues, setData: api.setData, triggerEvent: api.triggerEvent, display: api.display, validate: api.validate }
    try {
      // 空值探针:让脚本「仅空值填充」守卫全部通过,算出当前用户字段应有的计划账号取值;
      // setData 只捕获;triggerEvent/display/validate 屏蔽,避免探针触发联动、显隐或校验副作用。
      Object.assign(api, {
        getValue: () => '',
        getValues: () => ({}),
        setData: (patch: Record<string, unknown>) => { if (patch && typeof patch === 'object') Object.assign(captured, patch) },
        triggerEvent: () => undefined,
        ...(typeof orig.display === 'function' ? { display: () => undefined } : {}),
        ...(typeof orig.validate === 'function' ? { validate: () => Promise.resolve() } : {})
      })
      for (const script of identityScripts) {
        const handler = script.key ? eventFns[script.key] : undefined
        if (typeof handler !== 'function') continue
        // 单个脚本探针失败只跳过该脚本,不影响其余,回落为不覆盖该字段。
        try { await handler.call(api, {}) } catch {}
      }
    } finally {
      Object.assign(api, {
        getValue: orig.getValue, getValues: orig.getValues, setData: orig.setData, triggerEvent: orig.triggerEvent,
        ...(typeof orig.display === 'function' ? { display: orig.display } : {}),
        ...(typeof orig.validate === 'function' ? { validate: orig.validate } : {})
      })
    }
    const ids = [identity.userId, identity.accountName, identity.companyId, identity.companyName, identity.departmentId, identity.departmentName].filter(value => typeof value === 'string' && value.trim() !== '')
    const patch: Record<string, unknown> = {}
    for (const [key, value] of Object.entries(captured)) {
      if (value === undefined || value === null || value === '') continue
      let text: string
      try { text = typeof value === 'string' ? value : JSON.stringify(value) } catch { continue }
      if (text && ids.some(id => text.includes(id))) patch[key] = value
    }
    if (Object.keys(patch).length === 0) return null
    const setData = api.setData
    if (typeof setData === 'function') await setData.call(api, patch)
    // 回读覆盖后的完整值作为本步 readback,与页面实时表单保持一致。
    const getValues = api.getValues
    if (typeof getValues !== 'function') return null
    const values = await getValues.call(api)
    return values && typeof values === 'object' ? values : null
  }, { identity })
}

// waitForOptionsReady 等待远程选项/联动收敛：trace 无在途转发请求且条目数保持稳定窗。
// 就绪必须「等」出来，不允许任何硬编码标志（施工令 §2 假修清单）。
async function waitForOptionsReady (trace: { list: () => Array<{ state: string }> }, idleMs: number): Promise<void> {
  const startedAt = Date.now()
  let stableSince: number | null = null
  let lastCount = -1
  while (Date.now() - startedAt < CAPTURE_TIMEOUT_MS) {
    const entries = trace.list()
    const inFlight = entries.some(entry => entry.state === 'forwarding')
    if (!inFlight) {
      if (entries.length !== lastCount) {
        lastCount = entries.length
        stableSince = Date.now()
      }
      if (stableSince !== null && Date.now() - stableSince >= idleMs) return
    } else {
      stableSince = null
    }
    await new Promise(resolve => setTimeout(resolve, 100))
  }
  throw new Error('等待表单选项就绪超时：目标读取未收敛')
}

function injectIdentityScript (payload: Record<string, unknown>): string {
  const accountName = String(payload.accountName ?? '')
  const userName = String(payload.displayName ?? payload.accountName ?? '')
  const companyId = String(payload.companyId ?? '')
  const companyName = String(payload.companyName ?? '')
  const departmentId = String(payload.departmentId ?? '')
  const departmentName = String(payload.departmentName ?? '')
  const dutyId = String(payload.dutyId ?? '')
  const dutyName = String(payload.dutyName ?? '')
  const customerCode = String(payload.customerCode ?? '')
  const sid = String(payload.sid ?? '')
  const userId = String(payload.userId ?? '')
  const platformCode = String(payload.platformCode ?? '999999')

  const identity = {
    sid,
    token: sid,
    customerCode,
    userId,
    userName: userName || accountName,
    userAccount: accountName,
    companyId,
    companyName,
    userDepartmentName: departmentName,
    userDepartmentId: departmentId,
    departmentId,
    departmentName,
    dutyId,
    dutyName,
    dutyLevel: dutyId,
    topCompanyId: companyId,
    platformCode
  }
  return `(() => {
    const identity = ${JSON.stringify(identity)};
    const prefix = 'invest-power-system';
    for (const [key, value] of Object.entries(identity)) {
      if (!value) continue;
      window.localStorage.setItem(key, value);
      window.localStorage.setItem(prefix + '-' + key, value);
    }
    if (identity.sid) {
      window.localStorage.setItem('token', identity.sid);
      window.localStorage.setItem(prefix + '-token', identity.sid);
    }
    try {
      const userObj = {
        id: identity.userId,
        name: identity.userName,
        customerCode: identity.customerCode,
        companyId: identity.companyId,
        companyName: identity.companyName,
        departmentId: identity.departmentId,
        departmentName: identity.userDepartmentName,
        dutyId: identity.dutyId,
        dutyName: identity.dutyName,
        flag: identity.companyId
      };
      window.sessionStorage.setItem('user', JSON.stringify(userObj));
    } catch {}
    const syncStore = () => {
      if (window.$store && window.$store.commit) {
        try {
          if (identity.token) window.$store.commit('user/SET_TOKEN', identity.token);
          if (identity.userName) window.$store.commit('user/SET_USER_NAME', identity.userName);
          if (identity.userId) window.$store.commit('user/SET_USER_ID', identity.userId);
          if (identity.customerCode) window.$store.commit('user/SET_CUSTOMERCODE', identity.customerCode);
          if (identity.companyId) window.$store.commit('user/SET_COMPANY_ID', identity.companyId);
          if (identity.companyName && window.$store._mutations && window.$store._mutations['user/SET_COMPANY_NAME']) {
            window.$store.commit('user/SET_COMPANY_NAME', identity.companyName);
          }
          if (identity.departmentId && window.$store._mutations && window.$store._mutations['user/SET_DEPARTMENTID']) {
            window.$store.commit('user/SET_DEPARTMENTID', identity.departmentId);
          }
          if (identity.dutyId && window.$store._mutations && window.$store._mutations['user/SET_DUTY_ID']) {
            window.$store.commit('user/SET_DUTY_ID', identity.dutyId);
          }
          if (identity.dutyName && window.$store._mutations && window.$store._mutations['user/SET_DUTY_NAME']) {
            window.$store.commit('user/SET_DUTY_NAME', identity.dutyName);
          }
        } catch {}
      }
    };
    syncStore();
    setTimeout(syncStore, 50);
    // 页面异常采集（S7 完成判定与 S1 装载诊断共用）：任何 pageerror / 未处理 rejection
    // 都落进 __executorPageErrors，判定阶段「页面无异常」以它为准，不再使用幻影全局标志。
    window.__executorPageErrors = [];
    window.addEventListener('error', event => {
      window.__executorPageErrors.push({ kind: 'error', message: String(event.message ?? ''), source: String(event.filename ?? ''), lineno: event.lineno ?? 0 });
    });
    window.addEventListener('unhandledrejection', event => {
      let message = '';
      let url = '';
      try {
        const reason = event.reason;
        message = reason && reason.message ? String(reason.message) : String(reason ?? 'unhandledrejection');
        // axios 类 rejection 携带 config.url：完成判定要区分「目标请求异常」与
        // 「FormMaking 外部服务（如 tools-server uptoken）异常」，后者是与流程无关的噪声。
        url = String(reason && reason.config && reason.config.url ? reason.config.url : '');
      } catch (e) { message = 'unhandledrejection'; }
      window.__executorPageErrors.push({ kind: 'unhandledrejection', message, url });
    });
    // 原生宿主捕获（P1-1 修复）：目标产物只有 window.$vue（main.js:130 全局根实例），
    // 绝不能用 window.Vue / window.app 捕获。做法：等 $vue 出现 → 取其原型 →
    // 幂等包装原型上的 $initFlow / $flowDetail（components/InitFlowDialog/index.js 注册），
    // 把返回值里的 instance 写进 __executorNative 供外部以组件方法调用。
    window.__executorNative = { initFlow: null, flowDetail: null };
    const wrap = function (name, original) {
      if (!original || original.__executorWrapped) return original;
      const wrapped = function () {
        const created = original.apply(this, arguments);
        if (created && typeof created === 'object' && created.instance) {
          window.__executorNative[name] = created.instance;
        }
        return created;
      };
      wrapped.__executorWrapped = true;
      return wrapped;
    };
    const waitVue = () => {
      // 根实例由 new Vue(...).$mount('#app') 创建，原型即 Vue 原型；
      // 兜底 constructor.prototype 应对继承/包装过的根实例。
      const root = window.$vue;
      if (!root) return false;
      const proto = Object.getPrototypeOf(root) || (root.constructor && root.constructor.prototype);
      if (!proto) return false;
      if (proto.$initFlow) proto.$initFlow = wrap('initFlow', proto.$initFlow);
      if (proto.$flowDetail) proto.$flowDetail = wrap('flowDetail', proto.$flowDetail);
      return Boolean(proto.$initFlow || proto.$flowDetail);
    };
    if (!waitVue()) {
      const timer = setInterval(() => { if (waitVue()) clearInterval(timer); }, 20);
      setTimeout(() => clearInterval(timer), 15000);
    }
  })()`
}

// nativeURL 构造原生入口地址。
// 发起（P1-2 修复）：URL 的 flowTemplateId 必须是 /web/flowTemplateApi/list 返回的模板 ID
// （载荷专字段 flowTemplateId），绝不用表单代理 ID/流程代理 ID 顶替；auditWay 是原生查询入口。
// 详情：mode 决定原生语义（audit/edit/view），taskContext 携带实例行事实的 JSON 序列化；
// 实例行主体由页面 getInstanceId 自行从目标读取，这里只带页面外部拿不到的上下文。
function nativeURL (origin: string, payload: Record<string, unknown>): string {
  const mode = String(payload.mode ?? 'initiate')
  // 平台码统一带进 URL：运行时 axios 的 resolveRuntimePlatformCode 优先读 location 上的 platformCode，
  // 让实例重建类装载按实例真实平台（200001/999999）发目标请求；缺失时页面自身回落默认平台码。
  const platformCode = String(payload.platformCode ?? '')
  if (mode === 'initiate') {
    const params = new URLSearchParams()
    if (payload.selectFlowType) params.set('auditWay', String(payload.selectFlowType))
    if (payload.flowTemplateId) params.set('flowTemplateId', String(payload.flowTemplateId))
    if (payload.companyId) params.set('companyId', String(payload.companyId))
    if (payload.toolFlowName) params.set('title', String(payload.toolFlowName))
    if (platformCode) params.set('platformCode', platformCode)
    return `${origin}#/flow/initiate?${params.toString()}`
  }
  if (mode === 'forward') {
    const tab = String(payload.forwardTab ?? 'backlog')
    const suffix = platformCode ? `&platformCode=${encodeURIComponent(platformCode)}` : ''
    return `${origin}#/flow/approve/${encodeURIComponent(tab)}?tab=${encodeURIComponent(tab)}${suffix}`
  }
  const instanceId = String(payload.instanceId ?? '')
  const detailMode = mode === 'approve' ? 'audit' : mode
  const params = new URLSearchParams()
  params.set('mode', detailMode)
  if (platformCode) params.set('platformCode', platformCode)
  const relevance = Array.isArray(payload.flowInstanceBizRelevanceList) ? payload.flowInstanceBizRelevanceList : []
  // taskContext 行事实：FlowDetail.getTaskContext 优先解析它；getInstanceId 的实例查询
  // 也依赖 flowInstanceBizRelevanceList。字段名与目标 FlowDetail 契约一一对应，不得改名。
  const taskContext: Record<string, unknown> = {
    id: instanceId,
    flowInstanceId: instanceId,
    formExist: '',
    auditWay: String(payload.selectFlowType ?? ''),
    formProxyId: String(payload.formProxyId ?? ''),
    flowProxyId: String(payload.flowProxyId ?? ''),
    jobTaskId: String(payload.jobTaskId ?? ''),
    flowNodeProxyId: String(payload.nodeProxyId ?? ''),
    nextNodeProxyId: String(payload.nextNodeProxyId ?? ''),
    initiatorId: String((payload.initiator as { userId?: string } | undefined)?.userId ?? ''),
    flowName: String(payload.toolFlowName ?? ''),
    flowInstanceBizRelevanceList: relevance
  }
  params.set('taskContext', JSON.stringify(taskContext))
  return `${origin}#/flow/detail/${encodeURIComponent(instanceId)}?${params.toString()}`
}

async function ensureLatestFlowSource (): Promise<void> {
  const serviceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const syncScript = path.join(serviceRoot, 'scripts', 'sync-flow-source.js')
  const buildScript = path.join(serviceRoot, 'scripts', 'build-native-page.js')
  // 1. 远端同步是尽力而为:内网 git 抖动/不可达时只告警,绝不因此跳过本地页面构建;
  //    EXECUTOR_DISABLE_AUTO_SYNC=1 只关掉这一步远端拉取,不影响下面的本地构建兜底。
  //    超时给到 180s:一次全量 checkout+FormMaking 补齐+全树摘要实测约 60s,旧的 60s 上限过紧、
  //    正常同步也会被判 ETIMEDOUT(叠加破坏性 cloneAt 就会清空 flow-source),故留足余量。
  if (process.env.EXECUTOR_DISABLE_AUTO_SYNC !== '1') {
    try {
      childProcess.execFileSync(process.execPath, [syncScript, '--pull-latest'], {
        cwd: serviceRoot,
        encoding: 'utf8',
        timeout: 180000,
        stdio: 'inherit'
      })
    } catch (error) {
      console.warn('[executor] 启动时自动同步 rsh-flow-components 告警(已回落到本地版本):', (error as Error)?.message ?? error)
    }
  }
  // 2. 本地页面构建独立于同步结果:无论同步成功、失败还是被禁用,都必须让 dist 对应当前源码。
  //    build-native-page 自身按输入摘要判定是否重建,dist 已最新时是快速空操作;
  //    这里单独兜底,根治「同步失败->构建被整段跳过->缺 dist->启动失败」的连锁。
  try {
    childProcess.execFileSync(process.execPath, [buildScript], {
      cwd: serviceRoot,
      encoding: 'utf8',
      timeout: 300000,
      stdio: 'inherit'
    })
  } catch (error) {
    console.warn('[executor] 原生 /flow 页面构建告警(沿用现有 dist,缺失则启动仍会失败):', (error as Error)?.message ?? error)
  }
}

async function main (): Promise<void> {
  await ensureLatestFlowSource()
  const port = listenPort()
  const unpacked = extractPageDist()
  const pageOrigin = await serveFlowPage(unpacked.distDir)
  let browser: Awaited<ReturnType<typeof launchHeadlessChromium>> | null = null
  let manager: SessionManager | null = null
  let leaseReaper: NodeJS.Timeout | null = null
  const sessions = new Map<string, RuntimeSession>()

  async function ensureManager (): Promise<SessionManager> {
    if (manager) return manager
    browser = await launchHeadlessChromium()
    manager = new SessionManager(browser)
    manager.events.on(SESSION_EVENTS.closed, closed => {
      const runtime = sessions.get(closed.sessionId)
      if (!runtime) return
      runtime.destroyed = true
      void runtime.route?.uninstall().catch(() => undefined)
      sessions.delete(closed.sessionId)
    })
    const configuredReaperMs = Number(process.env.EXECUTOR_SESSION_REAPER_MS || 10000)
    const reaperMs = Number.isFinite(configuredReaperMs) && configuredReaperMs >= 1000 ? configuredReaperMs : 10000
    leaseReaper = setInterval(() => { manager?.reapExpired() }, reaperMs)
    leaseReaper.unref()
    return manager
  }


  const server = http.createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`)
    const send = (status: number, body: unknown): void => {
      response.writeHead(status, { 'content-type': 'application/json; charset=utf-8' })
      response.end(JSON.stringify(body))
    }

    if (request.method === 'GET' && (url.pathname === '/health' || url.pathname === '/runtime/health')) {
      send(200, healthBody())
      return
    }

    if (request.method === 'POST' && url.pathname === '/runtime/session') {
      void (async () => {
        let body: SessionCreateBody
        try {
          body = await readJSON(request) as SessionCreateBody
        } catch {
          send(400, { error: '请求体必须是 JSON' })
          return
        }
        const accountKey = String(body.accountKey ?? '').trim()
        const runId = String(body.runId ?? '').trim()
        const pathRunId = String(body.pathRunId ?? '').trim()
        const stepId = String(body.stepId ?? '').trim()
        if (!accountKey || !runId || !pathRunId || !stepId) {
          send(400, { error: '创建会话必须携带 accountKey/runId/pathRunId/stepId' })
          return
        }
        const record = await (await ensureManager()).create(accountKey).catch(error => {
          send(409, { error: String((error as Error)?.message ?? error) })
          return null
        })
        if (!record || !record.page) return
        const channel = new ForwardChannel()
        sessions.set(record.sessionId, {
          sessionId: record.sessionId,
          accountKey,
          runId,
          pathRunId,
          stepId,
          page: record.page,
          channel,
          route: null,
          loaded: false,
          actionInFlight: false,
          destroyed: false,
          writeGate: new WriteGate(),
          transpondConfirmed: false,
          choicePending: false,
          choicePendingAt: 0,
          lastChoiceError: null,
          choiceState: { continued: false },
          identity: null,
          nativeHost: null,
          loadMode: '',
          instanceId: '',
          forwardPostscriptRequired: false,
          formBefore: {}
        })
        send(200, { sessionId: record.sessionId, status: record.status })
      })().catch(error => {
        send(500, { error: String((error as Error)?.message ?? error) })
      })
      return
    }

    const sessionMatch = url.pathname.match(/^\/runtime\/session\/([^/]+)(\/.*)?$/)
    if (!sessionMatch) {
      send(404, { error: 'not found' })
      return
    }
    const sessionId = sessionMatch[1]!
    const action = sessionMatch[2] ?? ''
    const session = sessions.get(sessionId)

    if (request.method === 'POST' && action === '/heartbeat') {
      if (!session) {
        send(404, { error: '会话不存在' })
        return
      }
      try {
        const leaseExpiresAt = manager?.heartbeat(sessionId)
        send(200, { sessionId, leaseExpiresAt })
      } catch (error) {
        send(409, { error: String((error as Error)?.message ?? error) })
      }
      return
    }

    if (request.method === 'GET' && action === '/status') {
      if (!session) {
        send(404, { error: '会话不存在' })
        return
      }
      // trace.entries 是验证台的端点级观测口（L1/L2 断言写端点与只读事实）；不含请求/响应正文。
      // channel 是 G5 的可机读通道健康事实：通道复用是否正常必须能被断言，
      // 不能只依赖「日志里没看到某句话」这种不可机读的观察方式。
      send(200, {
        sessionId,
        loaded: session.loaded,
        channelConnected: session.channel.connected,
        channel: session.channel.stats(),
        identity: session.identity,
        trace: {
          ...(session.route?.trace.stats() ?? { total: 0 }),
          entries: (session.route?.trace.list() ?? []).map(entry => ({
            seq: entry.seq,
            method: entry.method,
            url: entry.url,
            state: entry.state,
            abortReason: entry.abortReason,
            httpStatus: entry.response?.httpStatus
          }))
        }
      })
      return
    }

    if (request.method === 'POST' && action === '/destroy') {
      void (async () => {
        if (!session) {
          send(404, { error: '会话不存在' })
          return
        }
        session.destroyed = true
        await session.route?.uninstall().catch(() => undefined)
        await manager?.close(sessionId, 'destroyed')
        sessions.delete(sessionId)
        send(200, { destroyed: true })
      })()
      return
    }

    if (request.method === 'POST' && action === '/load') {
      void (async () => {
        let envelope: MessageEnvelope
        try {
          envelope = validateMessage(await readJSON(request))
        } catch (error) {
          send(400, { error: String((error as Error)?.message ?? error) })
          return
        }
        if (!session) {
          send(404, { error: '会话不存在' })
          return
        }
        if (session.destroyed) {
          send(409, { error: '会话已关闭，不能装载' })
          return
        }
        try {
          assertForwardReady(session.channel)
          const payload = (envelope.payload ?? {}) as Record<string, unknown>
          const runtimeOrigin = new URL(pageOrigin.url).origin
          session.route = await installTargetRoute(session.page, {
            runtimeOrigins: [runtimeOrigin],
            toolFlowName: String(payload.toolFlowName ?? ''),
            forward: async requestPayload => {
              if (session.writeGate.isLocked && requestPayload.method === 'POST') {
                throw new ForwardFailure('动作已硬超时，拒绝新写请求')
              }
              if (requestPayload.url.includes('/flowInstancePostscript/save') && !session.transpondConfirmed) {
                throw new ForwardFailure('转发主请求未确认成功，禁止发送附言请求')
              }
              const outcome = await session.channel.send({
                sessionId: session.sessionId,
                runId: session.runId,
                pathRunId: session.pathRunId,
                stepId: session.stepId,
                payload: requestPayload
              })
              let parsedResponse: { isSuccess?: boolean, data?: { errorType?: string }, message?: string } | null = null
              if (outcome.body) {
                try {
                  const text = outcome.bodyEncoding === 'base64' ? Buffer.from(outcome.body, 'base64').toString('utf8') : outcome.body
                  parsedResponse = JSON.parse(text)
                } catch { parsedResponse = null }
              }
              if (requestPayload.url.includes('/web/flowInstanceApi/transpond') && outcome.httpStatus === 200 && parsedResponse?.isSuccess !== false) {
                session.transpondConfirmed = true
              }
              // 选择要求识别（S6/P1-6）：流程写响应 isSuccess=false 且 errorType 属于三种选择错误时，
              // 置位 choicePending 并向 Go 发 choice_required 事件；继续命令经 choice_continue 回来。
              if (requestPayload.method === 'POST' && outcome.httpStatus === 200 && outcome.body) {
                const parsed = parsedResponse
                const errorType = String(parsed?.data?.errorType ?? '')
                if (parsed && parsed.isSuccess === false && (CHOICE_ERRORS as readonly string[]).includes(errorType)) {
                  session.choicePending = true
                  session.choicePendingAt = Date.now()
                  session.lastChoiceError = parsed as unknown as Record<string, unknown>
                  const socket = session.channel.socketForEvents
                  if (socket) {
                    socket.send(JSON.stringify(encodeEnvelope('event', session, randomUUID(), {
                      type: 'choice_required',
                      data: parsed as unknown as Record<string, unknown>
                    })))
                  }
                }
              }
              return outcome
            }
          })
          await session.page.addInitScript(injectIdentityScript(payload))
          await session.page.goto(nativeURL(pageOrigin.url, payload), { waitUntil: 'domcontentloaded' })
          const mode = String(payload.mode ?? 'initiate')
          session.loadMode = mode
          session.instanceId = String(payload.instanceId ?? '')
          session.forwardPostscriptRequired = Array.isArray(payload.forwardCheckList) && payload.forwardCheckList.map(String).includes('fuyan')
          // 转发使用列表页自身组件；其他模式由 $initFlow/$flowDetail 捕获宿主。
          if (mode === 'forward') {
            const tab = String(payload.forwardTab ?? 'backlog')
            await prepareForwardList(session.page, {
              tab,
              instanceId: session.instanceId,
              jobTaskId: String(payload.jobTaskId ?? ''),
              receiverId: String(payload.receiverId ?? ''),
              receiverName: String(payload.receiverName ?? ''),
              checkList: Array.isArray(payload.forwardCheckList) ? payload.forwardCheckList.map(String) : []
            })
            session.nativeHost = { forwardList: true }
          } else {
            // 捕获失败直接抛错，绝不把“没捕到”当装载成功。
            session.nativeHost = await waitForExecutorNative(session.page)
          }
          const warnings: string[] = []
          let templateId = ''
          let templateCandidates: string[] = []
          let hostPath: string[] = []
          let detailSemantics: Record<string, unknown> | null = null

          if (mode === 'initiate') {
            // 发起模板精确选择（S2）：判据=trace 里 /web/flowTemplateApi/list 的真实响应。
            // 原生 mounted 只在 params.auditWay 存在时查询（InitFlowDialog.vue mounted），
            // 所以这里先等列表响应落进 trace，再做计划判定。
            await session.page.waitForFunction(
              'window.__executorNative && window.__executorNative.initFlow',
              undefined,
              { timeout: CAPTURE_TIMEOUT_MS }
            )
            const deadline = Date.now() + CAPTURE_TIMEOUT_MS
            let candidates: string[] | null = null
            while (Date.now() < deadline) {
              candidates = parseTemplateCandidateIds(session.route.trace)
              if (candidates !== null) break
              await new Promise(resolve => setTimeout(resolve, 150))
            }
            const plan = planTemplateSelection(candidates, String(payload.flowTemplateId ?? ''))
            if (plan.action === 'blocked') {
              throw new InitiateBlockedError(plan.reason)
            }
            templateCandidates = candidates ?? []
            if (plan.action === 'confirm') {
              // 多模板：调用原生 confirm(boundId)（走 chooseFlowDialog 的确认入口）。
              await session.page.evaluate(bound => {
                const instance = (window as unknown as { __executorNative: { initFlow?: { confirm?: (value: string) => void } } }).__executorNative.initFlow
                if (!instance || typeof instance.confirm !== 'function') {
                  throw new Error('原生缺少 confirm 选择入口')
                }
                instance.confirm(bound)
              }, plan.boundId).catch(error => { throw new InitiateBlockedError(String((error as Error)?.message ?? error)) })
            }
            // single：原生对唯一模板已自行 getFlowFindById，等装载完成即可。
            await waitForInitiateReady(session.page)
            templateId = await session.page.evaluate(() => {
              const native = (window as unknown as { __executorNative?: { initFlow?: { flowJson?: { id?: string } } } }).__executorNative
              return String(native?.initFlow?.flowJson?.id ?? '')
            })
            if (templateId !== plan.boundId) {
              // 唯一模板但装载的不是绑定模板：用原生 getFlowFindById 纠正后再等一次。
              await session.page.evaluate(bound => {
                const native = (window as unknown as { __executorNative?: { initFlow?: { getFlowFindById?: (id: string) => void } } }).__executorNative
                const instance = native?.initFlow
                if (!instance || typeof instance.getFlowFindById !== 'function') {
                  throw new Error('原生缺少 getFlowFindById 入口')
                }
                instance.getFlowFindById(bound)
              }, plan.boundId)
              await waitForInitiateReady(session.page)
              templateId = await session.page.evaluate(() => {
                const native = (window as unknown as { __executorNative?: { initFlow?: { flowJson?: { id?: string } | { id?: string }[] } } }).__executorNative
                const json = native?.initFlow?.flowJson
                const obj = Array.isArray(json) ? json[0] : json
                return String(obj?.id ?? '')
              })
              if (templateId !== plan.boundId) {
                throw new InitiateBlockedError(`装载完成但实际模板 ${templateId} != 计划绑定模板 ${plan.boundId}`)
              }
              warnings.push('原生自行装载的模板与绑定模板不一致，已用 getFlowFindById 纠正')
            }
            hostPath = await collectHostPath(session.page, ['InitFlowDialog', 'FlowDialog'])
          } else if (mode === 'forward') {
            hostPath = ['GroupApproveManage', String(payload.forwardTab ?? 'backlog')]
          } else {
            resolveDetailSemantics(mode === 'approve' ? 'audit' : mode, {
              jobTaskId: String(payload.jobTaskId ?? ''),
              flowInstanceId: String(payload.instanceId ?? '')
            })
            await waitForDetailReady(session.page)
            // 详情语义回读（S3 断言依据）：原生实例上的 operaType/isReInitiate/btnVisible 等
            // 是 mode 落到原生后的真实结果，不是我们传入的期望值。
            detailSemantics = await session.page.evaluate(() => {
              const native = (window as unknown as { __executorNative?: { flowDetail?: Record<string, unknown> } }).__executorNative
              const instance = native?.flowDetail
              if (!instance) return null
              return {
                operaType: instance.operaType ?? '',
                actionType: instance.actionType ?? '',
                isExamine: instance.isExamine === true,
                isReInitiate: instance.isReInitiate === true,
                btnVisible: instance.btnVisible === true,
                formExist: instance.formExist ?? '',
                formId: instance.formId ?? '',
                flowInstanceId: instance.flowInstanceId ?? ''
              }
            })
            // v-if 组件在标志翻转后的下一个 tick 才挂载：轮询等组件链出现，避免拿到空链。
            const hostDeadline = Date.now() + CAPTURE_TIMEOUT_MS
            while (Date.now() < hostDeadline) {
              hostPath = await collectHostPath(session.page, ['EnterpriseExamineDialog', 'ExamineOpinion'])
              if (hostPath.length > 0) break
              await new Promise(resolve => setTimeout(resolve, 200))
            }
            if (hostPath.length === 0) {
              warnings.push(await describeCapturedTree(session.page))
            }
          }

          // 数据补丁（S5 核心）：宿主按模式解析到真实表单 API；找不到=unsupported，绝不静默跳过。
          const allowed = Array.isArray(payload.changedFields) ? payload.changedFields.map(String) : []
          let patchReadback: Record<string, unknown> = {}
          if (allowed.length > 0) {
            // 表单宿主是异步渲染的（发起链 Steps1 监听 flowJson 自动 nextStep，
            // OtherSteps2 的 v-if 挂载与 FormMaking 初始化都在下一个 tick 之后）：
            // 轮询等待表单接口出现，超时才判 unsupported。
            const apiDeadline = Date.now() + CAPTURE_TIMEOUT_MS
            let available = false
            while (Date.now() < apiDeadline) {
              if (await formApiAvailable(session.page, mode)) { available = true; break }
              await new Promise(resolve => setTimeout(resolve, 200))
            }
            if (!available) {
              throw new UnsupportedFormError('未能在原生组件链上找到 setData/getValues 表单接口')
            }
            // 等待就绪：trace 无在途请求 + 稳定窗（远程选项/联动收敛），绝不硬编码就绪标志。
            await waitForOptionsReady(session.route.trace, 900)
            const readback = await applyNodePatchViaPage(session.page, mode, (payload.values ?? {}) as Record<string, unknown>, allowed, { userId: String(payload.userId ?? ''), companyId: String(payload.companyId ?? '') })
            if (readback === null) {
              throw new UnsupportedFormError('补丁应用时表单数据接口不可用')
            }
            patchReadback = readback
            // 回填后二次等待联动与远程请求收敛，并通用恢复被原生联动脚本误清空的非空意图值
            await waitForOptionsReady(session.route.trace, 600)
            const restoredReadback = await restoreIntendedPatchViaPage(session.page, mode, (payload.values ?? {}) as Record<string, unknown>, allowed, { userId: String(payload.userId ?? ''), companyId: String(payload.companyId ?? '') })
            if (restoredReadback !== null) {
              patchReadback = restoredReadback
            }
          }
          if (mode === 'initiate') {
            // 仅发起态:当前用户字段(申请人/部门/公司)应为计划发起账号。表单 refresh 初始化脚本的「仅空值填充」
            // 守卫可能被历史/补丁值挡住,这里用表单自身初始化脚本在空值探针下重算并权威覆盖(F-043 缺陷②);
            // 审批等模式下这些字段属真实发起人,绝不用当前操作人覆盖,故仅发起态执行。
            const identityReadback = await applyPlanIdentityViaPage(session.page, {
              userId: String(payload.userId ?? ''), accountName: String(payload.accountName ?? ''),
              companyId: String(payload.companyId ?? ''), companyName: String(payload.companyName ?? ''),
              departmentId: String(payload.departmentId ?? ''), departmentName: String(payload.departmentName ?? '')
            })
            if (identityReadback !== null) patchReadback = identityReadback
          }

          session.formBefore = await readNativeFormSnapshot(session.page)
          if (mode !== 'initiate' && mode !== 'forward' && Object.keys(session.formBefore).length === 0) {
            throw new UnsupportedFormError('实例表单正文尚未加载，拒绝让空表单进入写动作')
          }

          session.loaded = true
          session.identity = {
            mode,
            operator: payload.currentOperator ?? null,
            initiator: payload.initiator ?? null
          }
          session.route.setToolFlowName(String(payload.toolFlowName ?? ''))
          const pageErrors = await session.page.evaluate(() =>
            (window as unknown as { __executorPageErrors?: Array<unknown> }).__executorPageErrors ?? []
          )
          // traceUrls 是装载期全部目标请求的端点级清单（不含正文）：L1 断言与人工核对共用。
          const traceUrls = session.route.trace.list().map(entry => ({
            seq: entry.seq, method: entry.method, url: entry.url, state: entry.state, httpStatus: entry.response?.httpStatus
          }))
          send(200, {
            ready: true,
            mode,
            diagnostics: { mode, hostPath, templateId, templateCandidates, semantics: detailSemantics, patchReadback, warnings, pageErrors, traceUrls }
          })
        } catch (error) {
          // 装载期异常按七值语义映射：表单接口缺失=unsupported（阻塞+原因）；
          // 无权限/绑定缺失等发起阻塞=blocked；其余才是服务端故障（500）。
          console.error('[executor] 装载失败 mode=', String((envelope.payload as Record<string, unknown> | undefined)?.mode ?? ''),
            'error=', String((error as Error)?.message ?? error), (error as Error)?.stack ?? '')
          if (error instanceof UnsupportedFormError) {
            send(422, { status: 'unsupported', error: String((error as Error)?.message ?? error) })
            return
          }
          if (error instanceof InitiateBlockedError) {
            send(409, { status: 'blocked', error: String((error as Error)?.message ?? error) })
            return
          }
          send(500, { error: String((error as Error)?.message ?? error) })
        }
      })()
      return
    }
    send(404, { error: 'not found' })
  })

  const wss = new WebSocketServer({ noServer: true })
  server.on('upgrade', (request, socket, head) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`)
    const match = url.pathname.match(/^\/runtime\/session\/([^/]+)\/execute$/)
    if (!match) {
      socket.destroy()
      return
    }
    const session = sessions.get(match[1]!)
    if (!session || session.destroyed) {
      socket.destroy()
      return
    }
    wss.handleUpgrade(request, socket, head, ws => {
       if (!session.channel.attach(ws)) return
      ws.on('message', (data: Buffer) => {
        void (async () => {
          let envelope: MessageEnvelope
          try {
            envelope = validateMessage(JSON.parse(data.toString('utf8')))
          } catch (error) {
            sendEnvelope(ws, session, 'event', randomUUID(), { type: 'log', data: { level: 'error', message: String((error as Error)?.message ?? error) } })
            return
          }
          if (envelope.kind === 'request_response') {
            session.channel.resolve(envelope.requestId, (envelope.payload ?? {}) as { httpStatus?: number, headers?: Record<string, string>, body?: string, bodyEncoding?: 'utf8' | 'base64' })
            return
          }
          if (envelope.kind !== 'execute') {
            sendEnvelope(ws, session, 'event', envelope.requestId, { type: 'log', data: { level: 'error', message: `WS 不接受 ${envelope.kind} 消息` } })
            return
          }
          const action = String((envelope.payload as { action?: string } | undefined)?.action ?? '')
          if (action === 'choice_continue') {
            if (!session.actionInFlight || !session.choicePending) {
              sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: '当前没有等待选择的动作' })
              return
            }
            try {
              const choice = (envelope.payload as { choice?: ChoiceContinueChoice } | undefined)?.choice
              if (!choice) throw new Error('choice_continue 缺少 choice 载荷')
              const errorType = choice.kind === 'branch' ? 'custom_choose' : choice.kind === 'parallel' ? 'parallel_choose' : 'run_node_choose'
              canContinueChoice(errorType, session.choiceState, true)
              session.choiceState.continued = true
              session.choicePending = false
              // 只允许继续一次：继续后不再接受第二个 choice_continue。
              await dispatchChoiceContinueOnPage(session.page, errorType, choice, session.lastChoiceError)
              sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'success' })
            } catch (error) {
              session.choicePending = false
              sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: String((error as Error)?.message ?? error) })
            }
            return
          }
          if (session.actionInFlight) {
            sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: '同一会话同一时刻只允许一个动作' })
            return
          }
          const readOnly = action === 'capture' || action === 'validate'
          // 执行器真实计时起点：动作从此处装载分发直到收敛，墙钟差值即执行器处理耗时（F-042 选项 B）。
          const executorStart = performance.now()
          try {
            session.actionInFlight = true
            const baselineSeq = session.route!.trace.lastSeq
            // 页面异常只保留为证据，不参与终局；终局只看动作期望写端点响应。
            const errorsAtBaseline = await collectPageErrorEvidence(session.page)
            let dispatchFacts: { component: string, method: string } | null = null
            if (!readOnly) {
              const payload = (envelope.payload ?? {}) as { auditDesc?: string, loadMode?: string, nextNodeAuditors?: DispatchInput['nextNodeAuditors'] }
              dispatchFacts = await dispatchBusinessAction(session.page, {
                action,
                loadMode: payload.loadMode ?? session.loadMode,
                auditDesc: payload.auditDesc,
                nextNodeAuditors: payload.nextNodeAuditors
              })
              // 同意/不同意走原生 $confirm 弹窗：在页面内点击确认主按钮（已知差异，登记于设计文档）。
              if (action === 'approve' || action === 'reject') {
                await autoConfirmMessageBox(session.page)
              }
              // 提交/重新提交/审批时，等待渲染周期并捕获页面表单校验错误
              if (action === 'submit' || action === 'resubmit' || action === 'approve') {
                const validationErrors = await session.page.evaluate(() => {
                  const errors: string[] = []
                  const items = Array.from(document.querySelectorAll('.el-form-item.is-error, .sub-form-item.is-error'))
                  for (const item of items) {
                    const labelEl = item.querySelector('.el-form-item__label')
                    const errEl = item.querySelector('.el-form-item__error')
                    const label = (labelEl?.textContent || '').trim().replace(/[*：:]/g, '')
                    const msg = (errEl?.textContent || '').trim()
                    if (label && msg) errors.push(`${label}（${msg}）`)
                    else if (label) errors.push(`${label}未填写`)
                    else if (msg) errors.push(msg)
                  }
                  const tableCells = Array.from(document.querySelectorAll('.el-table__row td.is-error, .el-table__row .is-error'))
                  for (const cell of tableCells) {
                    const errEl = cell.querySelector('.el-form-item__error')
                    const td = cell.closest('td')
                    const table = cell.closest('table')
                    const colIdx = td?.cellIndex
                    const th = colIdx != null && table ? table.querySelectorAll('th')?.[colIdx] : null
                    const colName = (th?.textContent || '').trim().replace(/[*：:]/g, '')
                    const rowIdx = (cell.closest('tr')?.rowIndex ?? 0)
                    const msg = (errEl?.textContent || '').trim() || '未填写'
                    if (colName) errors.push(`明细第${rowIdx}行【${colName}】（${msg}）`)
                  }
                  return Array.from(new Set(errors))
                }).catch(() => [])
                if (validationErrors.length > 0) {
                  session.formValidationError = validationErrors.join('、')
                } else {
                  session.formValidationError = undefined
                }
              }
            }
            const verdict = readOnly
              ? { status: 'success' as const, requestTrace: [] }
              : await settleCompletion({
                action,
                trace: session.route!.trace,
                baselineSeq,
                hardTimeoutMs: Number(process.env.EXECUTOR_ACTION_TIMEOUT_MS || 180000),
                idleWindowMs: Number(process.env.EXECUTOR_IDLE_WINDOW_MS || 1200),
                forwardPostscriptRequired: session.forwardPostscriptRequired,
                formValidationError: session.formValidationError,
                gate: async () => {
                  if (!session.choicePending) return true
                  // 选择继续未在窗口内到达（Go 侧放弃闭环）：放行判定，让完成检测按目标拒绝收口。
                  return Date.now() - session.choicePendingAt > 15000
                },
                writeGate: session.writeGate
              })
            // 分发事实回传：调用了哪个组件哪个方法（L2 断言与运行详情的证据）。
            // debugWrites 记录动作窗口内每笔流程端点请求的判定输入（是否权限校验、响应摘要），
            // 供失败诊断与端点级证据核对。
            const debugWrites = session.route!.trace.list()
              .filter(e => e.seq > baselineSeq &&
                ['/web/flowInstanceApi/submit', '/flowInstanceApi/audit', '/web/flowInstanceApi/transpond',
                  '/web/flowInstanceApi/storageFormData', '/web/flowInstanceApi/reSubmit',
                  '/web/flowInstancePostscript/save'].some(p => e.url.includes(p)))
              .map(e => {
                let bodyText = e.body ?? ''
                if (e.bodyEncoding === 'base64') bodyText = Buffer.from(bodyText, 'base64').toString('utf8')
                let responseText = ''
                if (e.response?.body) {
                  responseText = e.response.bodyEncoding === 'base64'
                    ? Buffer.from(e.response.body, 'base64').toString('utf8')
                    : e.response.body
                }
                return {
                  seq: e.seq, url: e.url, state: e.state, httpStatus: e.response?.httpStatus ?? 0,
                  writeSequence: e.writeSequence,
                  isPermissionCheck: isPermissionCheckSubmit(e),
                  requestBody: bodyText,
                  responseBody: responseText,
                  requestBodyExcerpt: bodyText.slice(0, 300),
                  responseBodyExcerpt: responseText.slice(0, 600)
                }
              })
            const allPageErrors = await session.page.evaluate(() =>
              (window as unknown as { __executorPageErrors?: unknown[] }).__executorPageErrors ?? []
            ).catch(() => [])
            // 新实例 ID（发起链）：主实例写响应 data.id 是引擎核验与后续步骤的实例引用来源。
            let newInstanceId = ''
            for (const write of debugWrites) {
              if (write.isPermissionCheck || !write.url.includes('/web/flowInstanceApi/submit') || !write.responseBody) continue
              try {
                const parsed = JSON.parse(write.responseBody) as { data?: { id?: unknown } }
                const responseId = String(parsed?.data?.id ?? '').trim()
                if (responseId) { newInstanceId = responseId; break }
              } catch {}
            }
            let finalVerdict = verdict
            if ((action === 'save_draft' || action === 'submit') && session.instanceId && newInstanceId && session.instanceId !== newInstanceId) {
              finalVerdict = {
                status: 'failed' as const,
                message: `原实例 ${session.instanceId} 与写响应实例 ${newInstanceId} 不一致，已按重复实例缺陷停止`,
                requestTrace: verdict.requestTrace
              }
            }
            // 页面快照只作证据，不能冒充 Go 重读目标实例的最终事实；正文保护会把
            // 目标重读结果写入 formAfter/fieldCountAfter。保留独立字段避免两种来源混淆。
            const pageFormAfter = await readNativeFormSnapshot(session.page).catch(() => ({}))
            const verdictWithFacts = {
              ...finalVerdict,
              // 执行器墙钟真实耗时（毫秒）：与 status/requestTrace/businessSummary 平级上报给 Go 落库。
              executorMs: Math.round(performance.now() - executorStart),
              businessSummary: {
                dispatched: dispatchFacts,
                newInstanceId,
                debugWrites,
                formBefore: session.formBefore,
                pageFormAfter,
                pageFieldCountAfter: Object.keys(pageFormAfter).length,
                fieldCountBefore: Object.keys(session.formBefore).length,
                pageErrors: allPageErrors,
                relevantErrorsAtEnd: await collectPageErrorEvidence(session.page).catch(() => -1),
                relevantErrorsAtBaseline: errorsAtBaseline
              }
            }
            sendEnvelope(ws, session, 'result', envelope.requestId, verdictWithFacts)
          } catch (error) {
            sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: String((error as Error)?.message ?? error), executorMs: Math.round(performance.now() - executorStart) })
          } finally {
            session.actionInFlight = false
          }
        })().catch(error => {
          console.error('[executor] WS 处理失败：', error)
        })
      })
       ws.on('close', () => { session.channel.detach(ws, '执行通道连接中断') })
    })
  })

  server.listen(port, '0.0.0.0', () => {
    console.log(JSON.stringify({
      service: SERVICE_NAME,
      listening: `http://0.0.0.0:${port}`,
      health: '/health',
      pageOrigin: pageOrigin.url,
      launchArgs: HEADLESS_LAUNCH_ARGS,
      protocol: PROTOCOL_VERSION
    }))
  })

  async function shutdown (): Promise<void> {
    if (leaseReaper) clearInterval(leaseReaper)
    await manager?.closeAll().catch(() => undefined)
    wss.close()
    server.close()
    await browser?.close().catch(() => undefined)
    await pageOrigin.close().catch(() => undefined)
    fsRm(unpacked.root)
  }

  process.on('SIGINT', () => { void shutdown().finally(() => process.exit(0)) })
  process.on('SIGTERM', () => { void shutdown().finally(() => process.exit(0)) })
}

function fsRm (dir: string): void {
  import('node:fs').then(({ rmSync }) => { rmSync(dir, { recursive: true, force: true }) }).catch(() => undefined)
}

main().catch(error => {
  console.error(JSON.stringify({ ok: false, reason: String((error as Error)?.message ?? error) }))
  process.exitCode = 1
})
