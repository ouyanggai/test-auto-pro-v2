// 执行服务（T007 起）：HTTP 会话端点 + WebSocket 命令/转发通道 + 静态 runtime 产物，同端口复用。
// 边界（docs/s/01-执行引擎设计.md 第 5 节）：
//   - POST /runtime/session                创建执行会话（账号唯一键 + 运行编号）
//   - POST /runtime/session/{id}/load      装载身份、流程和表单（调用页面桥接）
//   - WS   /runtime/session/{id}/execute   执行业务命令并推送事件/转发请求/结果
//   - GET  /runtime/session/{id}/status    查询当前状态（不执行动作、不触发目标请求）
//   - POST /runtime/session/{id}/destroy   关闭会话
// runtime 不做任何调度；同账号排队由 Go 负责。
import fs from 'node:fs'
import http from 'node:http'
import path from 'node:path'
import { randomUUID } from 'node:crypto'
import { fileURLToPath } from 'node:url'
import { WebSocketServer, type WebSocket } from 'ws'
import { launchHeadlessChromium, HEADLESS_LAUNCH_ARGS } from './browser.ts'
import { SESSION_EVENTS, SessionManager } from './session-manager.ts'
import { installTargetRoute, type TargetRouteHandle } from './target-route.ts'
import { ForwardChannel } from './forward-channel.ts'
import { settleCompletion } from './completion.ts'
import { PROTOCOL_VERSION, validateMessage, type MessageEnvelope } from './protocol.ts'
import { serveDirectory } from './static-origin.ts'

// EXECUTOR_PORT 默认 19011：与 form-runtime 开发端口 19001、主应用端口错开。
const port = Number(process.env.EXECUTOR_PORT || 19011)

// resolveDistRoot 与探针一致：form-runtime 产物在 web/dist/form-runtime，publicPath /form-runtime/。
function resolveDistRoot(): string {
  if (process.env.FORM_RUNTIME_DIST_ROOT) return path.resolve(process.env.FORM_RUNTIME_DIST_ROOT)
  return path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../web/dist')
}

interface RuntimeSession {
  sessionId: string
  accountKey: string
  runId: string
  pathRunId: string
  stepId: string
  page: import('playwright').Page
  route: TargetRouteHandle
  channel: ForwardChannel
  loaded: boolean
  actionInFlight: boolean
  destroyed: boolean
  identity: Record<string, unknown> | null
  // 复合动作纪律（T011）：转发主请求确认成功后才放行附言请求；硬超时后拒绝新写请求。
  transpondConfirmed: boolean
  writeBlocked: boolean
  // 选择闭环（T010B）：目标等待选择期间动作未结束，choice_continue 是原动作的一部分。
  choicePending: boolean
}

// BridgeError 是页面内错误对象（浏览器上下文），允许携带稳定 code。
type BridgeError = Error & { code?: string }

interface SessionPayload {
  accountKey?: string
  requestId?: string
  runId?: string
  pathRunId?: string
  stepId?: string
}

// CHOICE_ERROR_TYPES 是目标“尚未提交、等待选择”的三种明确业务返回（设计 5.2 / T010B）。
const CHOICE_ERROR_TYPES = ['custom_choose', 'parallel_choose', 'run_node_choose']

// detectChoiceError 解析转发响应正文；命中三种选择错误时返回事件数据，其余返回 null。
function detectChoiceError(payload: { body?: string; bodyEncoding?: string }): Record<string, unknown> | null {
  if (!payload.body || payload.bodyEncoding !== 'base64') return null
  try {
    const parsed = JSON.parse(Buffer.from(payload.body, 'base64').toString('utf8'))
    const errorType = parsed && parsed.data && parsed.data.errorType
    if (parsed.isSuccess === false && typeof errorType === 'string' && (CHOICE_ERROR_TYPES as string[]).includes(errorType)) {
      return { errorType, data: parsed.data }
    }
  } catch {
    // 非 JSON 响应不参与选择闭环判定。
  }
  return null
}

async function main(): Promise<void> {
  const distRoot = resolveDistRoot()
  if (!fs.existsSync(path.join(distRoot, 'form-runtime', 'index.html'))) {
    console.error(JSON.stringify({ ok: false, reason: `缺少 form-runtime 构建产物：${distRoot}`, hint: '先执行 pnpm --dir form-runtime build' }))
    process.exitCode = 1
    return
  }
  const staticOrigin = await serveDirectory(distRoot)
  const entryURL = new URL('form-runtime/index.html#/test-auto-form', staticOrigin.url).toString()
  const browser = await launchHeadlessChromium()
  const manager = new SessionManager(browser)
  const sessions = new Map<string, RuntimeSession>()

  manager.events.on(SESSION_EVENTS.crashed, session => {
    const runtimeSession = sessions.get(session.sessionId)
    if (runtimeSession) {
      runtimeSession.destroyed = true
      runtimeSession.channel.detachAll('页面崩溃')
    }
  })
  manager.events.on(SESSION_EVENTS.closed, session => {
    const runtimeSession = sessions.get(session.sessionId)
    if (runtimeSession) {
      runtimeSession.destroyed = true
      runtimeSession.channel.detachAll('会话已关闭')
    }
  })

  // runBridgeAction 在页面内调用执行桥接的指定方法；桥接只接受 JSON 命令，返回 JSON 结果。
  async function runBridgeAction(session: RuntimeSession, method: string, command: Record<string, unknown>): Promise<unknown> {
    return session.page.evaluate(([bridgeMethod, bridgeCommand]) => {
      const bridge = (window as any).__runtimeBridge__
      if (!bridge || typeof bridge[bridgeMethod as string] !== 'function') {
        const error: BridgeError = new Error("执行桥接未注册或已销毁")
        error.code = 'BRIDGE_UNAVAILABLE'
        throw error
      }
      return bridge[bridgeMethod as string](bridgeCommand)
    }, [method, command] as const)
  }

  // bridgeMethodFor 按 execute 载荷里的动作分派到桥接方法；
  // load_initiate/load_approve/capture/validate 是独立桥接方法，其余（七个业务动作与 choice_continue）走 execute。
  function bridgeMethodFor(action: string): string {
    switch (action) {
      case 'load_initiate': return 'loadInitiate'
      case 'load_approve': return 'loadApprove'
      case 'capture': return 'capture'
      case 'validate': return 'validate'
      default: return 'execute'
    }
  }

  // settleResult 把桥接调用结果映射为协议结果载荷：
  //   已带 status 的结果原样使用（T011 完成检测会生成丰富结果）；
  //   普通返回包 success；错误按稳定 code 映射 unsupported/timeout/failed。
  function settleResult(value: unknown): Record<string, unknown> {
    if (value && typeof value === 'object' && typeof (value as any).status === 'string') {
      return value as Record<string, unknown>
    }
    return { status: 'success', businessSummary: value ?? null }
  }

  function settleError(error: unknown): Record<string, unknown> {
    const code = String((error as BridgeError)?.code ?? '')
    const message = String((error as Error)?.message ?? error)
    if (code === 'EXECUTION_ACTION_UNSUPPORTED') return { status: 'unsupported', message }
    if (code === 'BRIDGE_COMMAND_TIMEOUT') return { status: 'timeout', message }
    return { status: 'failed', message }
  }

  function sendEnvelope(socket: WebSocket, session: RuntimeSession, kind: string, requestId: string, payload: unknown): void {
    socket.send(JSON.stringify({
      version: PROTOCOL_VERSION,
      kind,
      requestId,
      sessionId: session.sessionId,
      runId: session.runId,
      pathRunId: session.pathRunId,
      stepId: session.stepId,
      payload
    }))
  }

  const server = http.createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`)
    const send = (status: number, body: unknown): void => {
      response.writeHead(status, { 'content-type': 'application/json; charset=utf-8' })
      response.end(JSON.stringify(body))
    }
    if (!url.pathname.startsWith('/runtime/')) {
      send(404, { error: 'not found' })
      return
    }

    if (request.method === 'POST' && url.pathname === '/runtime/session') {
      void (async () => {
        const chunks: Buffer[] = []
        for await (const chunk of request) chunks.push(chunk as Buffer)
        let body: SessionPayload
        try {
          body = JSON.parse(Buffer.concat(chunks).toString('utf8') || '{}')
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
        const record = await manager.create(accountKey).catch(error => {
          send(409, { error: String((error as Error)?.message ?? error) })
          return null
        })
        if (!record) return
        const channel = new ForwardChannel()
        const route = await installTargetRoute(record.page!, {
          runtimeOrigins: [new URL(entryURL).origin],
          forward: async payload => {
            const runtimeSessionNow = sessions.get(record.sessionId)
            // 复合动作纪律：附言只允许在转发主请求确认成功后发出；
            // 硬超时后不再放行任何新写请求（设计 7）。
            if (runtimeSessionNow) {
              if (payload.url.includes('/flowInstancePostscript/save') && !runtimeSessionNow.transpondConfirmed) {
                throw new Error('转发主请求未确认成功，禁止发送附言请求')
              }
              if (runtimeSessionNow.writeBlocked && payload.method === 'POST') {
                throw new Error('动作已硬超时，拒绝新写请求')
              }
            }
            const outcome = await channel.send({
              sessionId: record.sessionId,
              runId,
              pathRunId,
              stepId,
              payload
            })
            if (runtimeSessionNow && payload.url.includes('/web/flowInstanceApi/transpond') && outcome.httpStatus === 200) {
              runtimeSessionNow.transpondConfirmed = true
            }
            // 选择闭环检测（T010B）：目标返回 isSuccess=false 且 errorType 属于三种选择错误时，
            // 表示流程尚未推进、只在等待选择；以 choice_required 事件通知 Go，用已配置数据继续。
            const choiceErrorType = detectChoiceError({ body: outcome.body, bodyEncoding: outcome.bodyEncoding })
            if (choiceErrorType) {
              const runtimeSession = sessions.get(record.sessionId)
              const socket = channel.socketForEvents
              if (runtimeSession && socket) {
                runtimeSession.choicePending = true
                sendEnvelope(socket, runtimeSession, 'event', randomUUID(), {
                  type: 'choice_required',
                  data: { ...choiceErrorType, url: payload.url }
                })
              }
            }
            return outcome
          }
        })
        // 页面异常必须通知 Go（设计 3.1/6）：在此监听并经通道推送事件。
        // 装载阶段 WS 通道尚未连接，事件无处可去：同时落一行执行服务 stderr，
        // 保证装载期的页面异常在日志里可见（2026-09-16 验收排障：fmDone 链路异常无处可查）。
        record.page!.on('pageerror', error => {
          console.error('[executor] 页面异常:', String((error as Error)?.stack ?? error))
          const runtimeSession = sessions.get(record.sessionId)
          if (!channel.connected || !runtimeSession) return
          sendEnvelope(channel.socketForEvents!, runtimeSession, 'event', randomUUID(), {
            type: 'page_error',
            data: { message: String((error as Error)?.message ?? error) }
          })
        })
        // 页面侧请求失败与 console 错误同样落 stderr：网络层失败会让业务回调整条中断，
        // 而 Go 出口只记录转发成功的事实，两边对照才能定位「200 但页面没收到」的缺口。
        record.page!.on('requestfailed', request => {
          console.error('[executor] 请求失败:', new Date().toISOString(), request.method(), request.url(), request.failure()?.errorText ?? '')
        })
        record.page!.on('console', message => {
          if (message.type() === 'error') console.error('[executor:page-console]', message.text().slice(0, 400))
        })
        // 创建即装载固定执行路由：桥接就绪后会话才可用。
        try {
          await record.page!.goto(entryURL, { waitUntil: 'load' })
          await record.page!.waitForFunction('Boolean(window.__runtimeBridge__)', undefined, { timeout: 30000 })
        } catch (error) {
          await manager.close(record.sessionId, 'superseded')
          send(500, { error: `固定路由装载失败：${String((error as Error)?.message ?? error)}` })
          return
        }
        sessions.set(record.sessionId, {
          sessionId: record.sessionId,
          accountKey,
          runId,
          pathRunId,
          stepId,
          page: record.page!,
          route,
          channel,
          loaded: false,
          actionInFlight: false,
          destroyed: false,
          identity: null,
          transpondConfirmed: false,
          writeBlocked: false,
          choicePending: false
        })
        send(200, { sessionId: record.sessionId, status: record.status, entryURL })
      })()
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
      send(200, {
        sessionId,
        loaded: session.loaded,
        channelConnected: session.channel.connected,
        identity: session.identity,
        trace: session.route.trace.stats(),
        traceEntries: session.route.trace.list().map(entry => ({
          seq: entry.seq, method: entry.method, url: entry.url, state: entry.state,
          abortReason: entry.abortReason,
          httpStatus: entry.response?.httpStatus
        }))
      })
      return
    }
    if (request.method === 'POST' && action === '/destroy') {
      void (async () => {
        if (!session) {
          send(404, { error: '会话不存在' })
          return
        }
        // 先让页面桥接自行清理（storage facade、Vuex、请求策略），再关闭 Context。
        if (session.loaded) {
          await runBridgeAction(session, 'destroy', {
            version: PROTOCOL_VERSION, kind: 'destroy', requestId: randomUUID(),
            sessionId, runId: session.runId, pathRunId: session.pathRunId, stepId: session.stepId
          }).catch(() => undefined)
        }
        await manager.close(sessionId, 'destroyed')
        sessions.delete(sessionId)
        send(200, { destroyed: true })
      })()
      return
    }
    if (request.method === 'POST' && action === '/load') {
      void (async () => {
        const chunks: Buffer[] = []
        for await (const chunk of request) chunks.push(chunk as Buffer)
        let envelope: MessageEnvelope
        try {
          envelope = validateMessage(JSON.parse(Buffer.concat(chunks).toString('utf8')))
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
        const mode = String((envelope.payload as any)?.mode ?? 'initiate')
        // 装载模式 → 桥接方法（评审 P1-4）：转发走 loadForward，把转发弹窗等价状态注入审批宿主。
        const loadCommandByMode: Record<string, string> = { approve: 'load_approve', forward: 'load_forward' }
        const bridgeMethodByMode: Record<string, string> = { approve: 'loadApprove', forward: 'loadForward' }
        try {
          const command = { ...envelope, payload: { action: loadCommandByMode[mode] ?? 'load_initiate', ...(envelope.payload as Record<string, unknown>) } } as Record<string, unknown>
          const result = await runBridgeAction(session, bridgeMethodByMode[mode] ?? 'loadInitiate', command)
          session.loaded = true
          const loadPayload = (envelope.payload ?? {}) as Record<string, any>
          // 工具流程名称保护（设计 5.2）：主实例 submit 发出前由路由层恢复工具值。
          session.route.setToolFlowName(String(loadPayload.toolFlowName ?? ''))
          // 身份事实快照（T009）：操作人是会话账号本身；发起人来自装载载荷，二者分开保存。
          session.identity = {
            mode,
            operatorAccount: session.accountKey,
            initiator: loadPayload.initiator ?? null,
            instanceId: String(loadPayload.instanceId ?? ''),
            jobTaskId: String(loadPayload.jobTaskId ?? ''),
            nodeProxyId: String(loadPayload.nodeProxyId ?? ''),
            toolFlowName: String(loadPayload.toolFlowName ?? '')
          }
          send(200, settleResult(result))
        } catch (error) {
          send(500, settleError(error))
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
      console.error('[executor] 通道已连接:', session.sessionId, new Date().toISOString())
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
            session.channel.resolve(envelope.requestId, (envelope.payload ?? {}) as Record<string, any>)
            return
          }
          if (envelope.kind !== 'execute') {
            // 协议方向约束：Go 在 WS 上只发 execute 与 request_response。
            sendEnvelope(ws, session, 'event', envelope.requestId, { type: 'log', data: { level: 'error', message: `WS 不接受 ${envelope.kind} 消息` } })
            return
          }
          const action = String((envelope.payload as any)?.action ?? '')
          if (action === 'choice_continue') {
            // 选择闭环继续：属于正在进行的动作，不检查在途标志、不启动新的完成检测。
            if (!session.actionInFlight || !session.choicePending) {
              sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: '当前没有等待选择的动作' })
              return
            }
            session.choicePending = false
            try {
              const dispatched = await runBridgeAction(session, 'execute', envelope as unknown as Record<string, unknown>)
              sendEnvelope(ws, session, 'result', envelope.requestId, settleResult(dispatched))
            } catch (error) {
              sendEnvelope(ws, session, 'result', envelope.requestId, settleError(error))
            }
            return
          }
          if (session.actionInFlight) {
            sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: '同一会话同一时刻只允许一个动作' })
            return
          }
          // 装载门禁（T008）：业务动作必须发生在成功装载之后；装载失败/未装载绝不放行写动作。
          const readOnlyCommand = action === 'capture' || action === 'validate'
          if (!readOnlyCommand && !session.loaded) {
            sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: '表单尚未成功装载，不允许执行业务动作' })
            return
          }
          session.actionInFlight = true
          session.writeBlocked = false
          session.transpondConfirmed = false
          const baselineSeq = session.route.trace.list().length
          // 重置宿主业务完成标志（上一动作的信号不得带入本动作）。
          await session.page.evaluate(() => {
            const host = (window as any).__runtimeExecutionHost__
            if (host && typeof host.resetBusinessDone === 'function') host.resetBusinessDone()
          }).catch(() => undefined)
          try {
            // 分派阶段：调用真实目标入口（异步链路立即返回）。
            const dispatchTimeoutMs = Number(process.env.EXECUTOR_ACTION_TIMEOUT_MS || 180000)
            const dispatched = await Promise.race([
              runBridgeAction(session, bridgeMethodFor(action), envelope as unknown as Record<string, unknown>),
              new Promise((_, reject) => setTimeout(() => {
                const error: BridgeError = new Error('动作执行超时')
                error.code = 'BRIDGE_COMMAND_TIMEOUT'
                reject(error)
              }, dispatchTimeoutMs))
            ])
            // 完成检测（T011）：网络空闲稳定窗 + 页面异常 + 业务完成事件 + 复合动作纪律。
            const readOnlyCommand = action === 'capture' || action === 'validate'
            const verdict = readOnlyCommand
              ? settleResult(dispatched)
              : await settleCompletion({
                  action,
                  trace: session.route.trace,
                  baselineSeq,
                  businessDone: async () => session.page.evaluate(() => {
                    const host = (window as any).__runtimeExecutionHost__
                    return Boolean(host && typeof host.isBusinessDone === 'function' && host.isBusinessDone())
                  }).catch(() => false),
                  hardTimeoutMs: dispatchTimeoutMs,
                  idleWindowMs: Number(process.env.EXECUTOR_IDLE_WINDOW_MS || 1200),
                  gate: async () => !session.choicePending
                })
            if (verdict.status === 'timeout') session.writeBlocked = true
            sendEnvelope(ws, session, 'result', envelope.requestId, { ...verdict })
          } catch (error) {
            const settled = settleError(error)
            if (settled.status === 'timeout') session.writeBlocked = true
            sendEnvelope(ws, session, 'result', envelope.requestId, settled)
          } finally {
            session.actionInFlight = false
          }
        })().catch(error => {
          console.error('[executor] WS 处理失败：', error)
        })
      })
      ws.on('close', () => { console.error('[executor] 通道已断开:', session.sessionId, new Date().toISOString()); session.channel.detachAll('Go 通道已断开') })
    })
  })

  server.listen(port, '0.0.0.0', () => {
    console.log(JSON.stringify({
      service: 'form-runtime-executor',
      listening: `http://0.0.0.0:${port}`,
      entryURL,
      launchArgs: HEADLESS_LAUNCH_ARGS
    }))
  })

  async function shutdown(): Promise<void> {
    await manager.closeAll().catch(() => undefined)
    wss.close()
    server.close()
    await browser.close().catch(() => undefined)
    await staticOrigin.close().catch(() => undefined)
  }
  process.on('SIGINT', () => { void shutdown().finally(() => process.exit(0)) })
  process.on('SIGTERM', () => { void shutdown().finally(() => process.exit(0)) })
}

main().catch(error => {
  console.error(JSON.stringify({ ok: false, reason: String((error as Error)?.message ?? error) }))
  process.exitCode = 1
})
