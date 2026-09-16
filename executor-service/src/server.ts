// 独立执行服务：健康检查 + 会话生命周期。
// POST /runtime/session 只创建 BrowserContext，不导航；装载前必须先连转发通道。
import http from 'node:http'
import { randomUUID } from 'node:crypto'
import { WebSocketServer, type WebSocket } from 'ws'
import { launchHeadlessChromium, HEADLESS_LAUNCH_ARGS } from './browser.ts'
import { healthBody, listenPort, SERVICE_NAME } from './health.ts'
import { SessionManager } from './session-manager.ts'
import { ForwardChannel, ForwardFailure } from './forward-channel.ts'
import { installTargetRoute, type TargetRouteHandle } from './target-route.ts'
import { assertForwardReady } from './navigate.ts'
import { extractPageDist, serveFlowPage } from './page-origin.ts'
import { encodeEnvelope, PROTOCOL_VERSION, validateMessage, type MessageEnvelope } from './protocol.ts'
import { InitiateBlockedError, planTemplateSelection } from './native-initiate.ts'
import { resolveDetailSemantics } from './native-detail.ts'
import { applyNodePatch, UnsupportedFormError } from './data-patch.ts'
import { canContinueChoice, dispatchChoiceContinue, dispatchNativeAction } from './native-actions.ts'
import { settleCompletion, WriteGate } from './completion.ts'
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
  choiceState: { continued: boolean }
  identity: Record<string, unknown> | null
  nativeHost: Record<string, unknown> | null
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
      return Boolean(instance && instance.approveDialogVisible === true && instance.flowJson && instance.flowJson.id);
    })()`,
    undefined,
    { timeout: CAPTURE_TIMEOUT_MS }
  )
}

// waitForDetailReady 等待详情装载就绪：FormMaking 走 examineDialogVisible，
// 无表单固定页走 ExpensesClaimFormVisible（flowDetail.vue 的两条打开分支）。
async function waitForDetailReady (page: Page): Promise<void> {
  await page.waitForFunction(
    `(() => {
      const instance = window.__executorNative && window.__executorNative.flowDetail;
      if (!instance) return false;
      return instance.examineDialogVisible === true || instance.ExpensesClaimFormVisible === true;
    })()`,
    undefined,
    { timeout: CAPTURE_TIMEOUT_MS }
  )
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

// resolveFormApi 按模式解析真实表单数据接口（S5）：
// 发起=FlowDialog.$refs.OtherSteps2.$refs.generateForm（InitFlowDialog.vue/FlowDialog 结构）；
// 审批与重提=EnterpriseExamineDialog.$refs.generateForm（flowDetail.vue 结构）。
// 找不到返回 null，由调用方按 unsupported 阻塞，绝不静默跳过。
async function resolveFormApi (page: Page, mode: string): Promise<{ setData: (values: Record<string, unknown>) => Promise<void> | void, getValues: () => Promise<Record<string, unknown>> | Record<string, unknown> } | null> {
  const wantDialog = mode === 'initiate' ? 'FlowDialog' : 'EnterpriseExamineDialog'
  return page.evaluate(want => {
    const native = (window as unknown as { __executorNative?: { initFlow?: unknown, flowDetail?: unknown } }).__executorNative
    const container = (native?.initFlow || native?.flowDetail) as { $children?: unknown[] } | undefined
    if (!container) return null
    // 从捕获实例递归按 $options.name 找唯一宿主组件；找不到/多命中都视为缺失。
    const found: unknown[] = []
    const walk = (node: { $options?: { name?: string }, $children?: unknown[] }): void => {
      if (node?.$options?.name === want) { found.push(node); return }
      for (const child of (node.$children ?? []) as Array<{ $options?: { name?: string }, $children?: unknown[] }>) {
        walk(child)
      }
    }
    walk(container)
    if (found.length !== 1) return null
    const dialog = found[0] as { $refs?: Record<string, unknown> }
    const refs = dialog.$refs ?? {}
    const form = (want === 'FlowDialog'
      ? ((refs.OtherSteps2 as { $refs?: Record<string, unknown> } | undefined)?.$refs?.generateForm)
      : refs.generateForm) as { setData?: unknown, getValues?: unknown } | undefined
    if (!form || typeof form.setData !== 'function' || typeof form.getValues !== 'function') return null
    return {
      setData: (values: Record<string, unknown>) => (form.setData as (v: Record<string, unknown>) => void)(values),
      getValues: () => (form.getValues as () => Record<string, unknown>)()
    }
  }, wantDialog)
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
  const identity = {
    sid: String(payload.sid ?? ''),
    token: String(payload.sid ?? ''),
    customerCode: String(payload.customerCode ?? ''),
    userId: String(payload.userId ?? ''),
    userName: String(payload.accountName ?? ''),
    companyId: String(payload.companyId ?? ''),
    companyName: String(payload.companyName ?? ''),
    userDepartmentName: String(payload.departmentName ?? ''),
    topCompanyId: String(payload.companyId ?? ''),
    platformCode: String(payload.platformCode ?? '999999')
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
    // 页面异常采集（S7 完成判定与 S1 装载诊断共用）：任何 pageerror / 未处理 rejection
    // 都落进 __executorPageErrors，判定阶段「页面无异常」以它为准，不再使用幻影全局标志。
    window.__executorPageErrors = [];
    window.addEventListener('error', event => {
      window.__executorPageErrors.push({ kind: 'error', message: String(event.message ?? ''), source: String(event.filename ?? ''), lineno: event.lineno ?? 0 });
    });
    window.addEventListener('unhandledrejection', event => {
      let reason = '';
      try { reason = String(event.reason && event.reason.message ? event.reason.message : event.reason ?? ''); } catch (e) { reason = 'unhandledrejection'; }
      window.__executorPageErrors.push({ kind: 'unhandledrejection', message: reason });
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
  if (mode === 'initiate') {
    const params = new URLSearchParams()
    if (payload.selectFlowType) params.set('auditWay', String(payload.selectFlowType))
    if (payload.flowTemplateId) params.set('flowTemplateId', String(payload.flowTemplateId))
    if (payload.companyId) params.set('companyId', String(payload.companyId))
    if (payload.toolFlowName) params.set('title', String(payload.toolFlowName))
    return `${origin}#/flow/initiate?${params.toString()}`
  }
  const instanceId = String(payload.instanceId ?? '')
  const detailMode = mode === 'approve' || mode === 'forward' ? 'audit' : mode
  const params = new URLSearchParams()
  params.set('mode', detailMode)
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

async function main (): Promise<void> {
  const port = listenPort()
  const unpacked = extractPageDist()
  const pageOrigin = await serveFlowPage(unpacked.distDir)
  let browser: Awaited<ReturnType<typeof launchHeadlessChromium>> | null = null
  let manager: SessionManager | null = null
  const sessions = new Map<string, RuntimeSession>()

  async function ensureManager (): Promise<SessionManager> {
    if (manager) return manager
    browser = await launchHeadlessChromium()
    manager = new SessionManager(browser)
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
          choiceState: { continued: false },
          identity: null,
          nativeHost: null
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

    if (request.method === 'GET' && action === '/status') {
      if (!session) {
        send(404, { error: '会话不存在' })
        return
      }
      // trace.entries 是验证台的端点级观测口（L1/L2 断言写端点与只读事实）；不含请求/响应正文。
      send(200, {
        sessionId,
        loaded: session.loaded,
        channelConnected: session.channel.connected,
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
              if (requestPayload.url.includes('/web/flowInstanceApi/transpond') && outcome.httpStatus === 200) {
                session.transpondConfirmed = true
              }
              return outcome
            }
          })
          await session.page.addInitScript(injectIdentityScript(payload))
          await session.page.goto(nativeURL(pageOrigin.url, payload), { waitUntil: 'domcontentloaded' })
          // 捕获失败直接抛错 → 500 门禁保留；绝不把「没捕到」当装载成功。
          session.nativeHost = await waitForExecutorNative(session.page)
          const mode = String(payload.mode ?? 'initiate')
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
                const native = (window as unknown as { __executorNative?: { initFlow?: { flowJson?: { id?: string } } } }).__executorNative
                return String(native?.initFlow?.flowJson?.id ?? '')
              })
              if (templateId !== plan.boundId) {
                throw new InitiateBlockedError(`装载完成但实际模板 ${templateId} != 计划绑定模板 ${plan.boundId}`)
              }
              warnings.push('原生自行装载的模板与绑定模板不一致，已用 getFlowFindById 纠正')
            }
            hostPath = await collectHostPath(session.page, ['InitFlowDialog', 'FlowDialog'])
          } else {
            resolveDetailSemantics(mode === 'approve' || mode === 'forward' ? 'audit' : mode, {
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
            const api = await resolveFormApi(session.page, mode)
            if (!api) {
              throw new UnsupportedFormError('未能在原生组件链上找到 setData/getValues 表单接口')
            }
            // 等待就绪：trace 无在途请求 + 稳定窗（远程选项/联动收敛），绝不硬编码就绪标志。
            await waitForOptionsReady(session.route.trace, 900)
            patchReadback = await applyNodePatch(api, (payload.values ?? {}) as Record<string, unknown>, { allowedKeys: allowed })
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
      session.channel.attach(ws)
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
            session.choicePending = false
            try {
              const host = (session.nativeHost ?? {}) as Parameters<typeof dispatchChoiceContinue>[0]
              const errorType = String((envelope.payload as { choice?: { kind?: string } } | undefined)?.choice?.kind ?? '')
              canContinueChoice(errorType === 'branch' ? 'custom_choose' : errorType === 'parallel' ? 'parallel_choose' : 'run_node_choose', session.choiceState, true)
              dispatchChoiceContinue(host, errorType === 'branch' ? 'custom_choose' : errorType === 'parallel' ? 'parallel_choose' : 'run_node_choose')
              sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'success' })
            } catch (error) {
              sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: String((error as Error)?.message ?? error) })
            }
            return
          }
          if (session.actionInFlight) {
            sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: '同一会话同一时刻只允许一个动作' })
            return
          }
          const readOnly = action === 'capture' || action === 'validate'
          try {
            session.actionInFlight = true
            const baselineSeq = session.route!.trace.lastSeq
            if (!readOnly) {
              await session.page.evaluate((action) => {
                const native = (window as unknown as { __executorNative?: { initFlow?: unknown, flowDetail?: unknown } }).__executorNative
                const container = native?.initFlow || native?.flowDetail
                if (!container) return
                const typed = container as { $children?: Array<{ $options?: { name?: string }, sumbitFlow?: unknown, temporaryStorage?: unknown, formMakingFormBusiness?: unknown, saveSubmit?: unknown, confirmTranspond?: unknown }> }
                const flowDialog = typed.$children?.find(c => c.$options?.name === 'FlowDialog')
                const opinion = typed.$children?.find(c => c.$options?.name === 'EnterpriseExamineOpinion')
                switch (action) {
                  case 'save_draft':
                    if (flowDialog && typeof flowDialog.sumbitFlow === 'function') {
                      (flowDialog.sumbitFlow as (kind: string) => unknown)('draft')
                    }
                    break
                  case 'submit':
                    if (flowDialog && typeof flowDialog.sumbitFlow === 'function') {
                      (flowDialog.sumbitFlow as (kind: string) => unknown)('submit')
                    }
                    break
                  case 'storage_form_data':
                    if (opinion && typeof opinion.temporaryStorage === 'function') {
                      (opinion.temporaryStorage as () => unknown)()
                    }
                    break
                  case 'resubmit':
                    if (opinion && typeof opinion.saveSubmit === 'function') {
                      (opinion.saveSubmit as (type: string) => unknown)('submit')
                    }
                    break
                  case 'approve':
                    if (opinion && typeof opinion.formMakingFormBusiness === 'function') {
                      (opinion.formMakingFormBusiness as (flag: boolean, type: string) => unknown)(true, 'pass')
                    }
                    break
                  case 'reject':
                    if (opinion && typeof opinion.formMakingFormBusiness === 'function') {
                      (opinion.formMakingFormBusiness as (flag: boolean, type: string) => unknown)(false, 'no_pass')
                    }
                    break
                  case 'forward':
                    if (opinion && typeof opinion.confirmTranspond === 'function') {
                      (opinion.confirmTranspond as () => unknown)()
                    }
                    break
                }
              }, action)
            }
            const verdict = readOnly
              ? { status: 'success' as const, requestTrace: [] }
              : await settleCompletion({
                action,
                trace: session.route!.trace,
                baselineSeq,
                businessDone: async () => {
                  const entries = session.route!.trace.list().filter(e => e.seq > baselineSeq)
                  const flowWrites = entries.filter(e => 
                    e.method === 'POST' && 
                    e.state === 'fulfilled' &&
                    e.response?.httpStatus === 200 &&
                    (e.url.includes('/flowInstanceApi/submit') || 
                     e.url.includes('/flowInstanceApi/audit') ||
                     e.url.includes('/flowInstanceApi/transpond') ||
                     e.url.includes('/flowInstanceApi/temporaryStorage') ||
                     e.url.includes('/flowInstanceApi/reEdit'))
                  )
                  if (flowWrites.length === 0) return false
                  const hasBusinessError = await session.page.evaluate(() => {
                    return (window as unknown as { __businessError?: boolean }).__businessError === true
                  }).catch(() => false)
                  return flowWrites.length > 0 && !hasBusinessError
                },
                hardTimeoutMs: Number(process.env.EXECUTOR_ACTION_TIMEOUT_MS || 180000),
                idleWindowMs: Number(process.env.EXECUTOR_IDLE_WINDOW_MS || 1200),
                gate: async () => !session.choicePending,
                writeGate: session.writeGate
              })
            sendEnvelope(ws, session, 'result', envelope.requestId, verdict)
          } catch (error) {
            sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: String((error as Error)?.message ?? error) })
          } finally {
            session.actionInFlight = false
          }
        })().catch(error => {
          console.error('[executor] WS 处理失败：', error)
        })
      })
      ws.on('close', () => { session.channel.detachAll('Go 通道已断开') })
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
