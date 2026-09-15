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
          forward: async payload => channel.send({
            sessionId: record.sessionId,
            runId,
            pathRunId,
            stepId,
            payload
          })
        })
        // 页面异常必须通知 Go（设计 3.1/6）：在此监听并经通道推送事件。
        record.page!.on('pageerror', error => {
          const runtimeSession = sessions.get(record.sessionId)
          if (!channel.connected || !runtimeSession) return
          sendEnvelope(channel.socketForEvents!, runtimeSession, 'event', randomUUID(), {
            type: 'page_error',
            data: { message: String((error as Error)?.message ?? error) }
          })
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
          identity: null
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
        trace: session.route.trace.stats()
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
        try {
          const command = { ...envelope, payload: { action: mode === 'approve' ? 'load_approve' : 'load_initiate', ...(envelope.payload as Record<string, unknown>) } } as Record<string, unknown>
          const result = await runBridgeAction(session, mode === 'approve' ? 'loadApprove' : 'loadInitiate', command)
          session.loaded = true
          // 身份事实快照（T009）：操作人是会话账号本身；发起人来自装载载荷，二者分开保存。
          const payload = (envelope.payload ?? {}) as Record<string, any>
          session.identity = {
            mode,
            operatorAccount: session.accountKey,
            initiator: payload.initiator ?? null,
            instanceId: String(payload.instanceId ?? ''),
            jobTaskId: String(payload.jobTaskId ?? ''),
            nodeProxyId: String(payload.nodeProxyId ?? ''),
            toolFlowName: String(payload.toolFlowName ?? '')
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
          if (session.actionInFlight) {
            sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: '同一会话同一时刻只允许一个动作' })
            return
          }
          const action = String((envelope.payload as any)?.action ?? '')
          // 装载门禁（T008）：业务动作必须发生在成功装载之后；装载失败/未装载绝不放行写动作。
          const readOnlyCommand = action === 'capture' || action === 'validate'
          if (!readOnlyCommand && !session.loaded) {
            sendEnvelope(ws, session, 'result', envelope.requestId, { status: 'failed', message: '表单尚未成功装载，不允许执行业务动作' })
            return
          }
          session.actionInFlight = true
          try {
            // 硬超时（设计 7）：超时后不再等待页面回调，返回 timeout；T011 会接入更完整的完成检测。
            const result = await Promise.race([
              runBridgeAction(session, bridgeMethodFor(action), envelope as unknown as Record<string, unknown>),
              new Promise((_, reject) => setTimeout(() => {
                const error: BridgeError = new Error("动作执行超时")
                error.code = 'BRIDGE_COMMAND_TIMEOUT'
                reject(error)
              }, Number(process.env.EXECUTOR_ACTION_TIMEOUT_MS || 180000)))
            ])
            sendEnvelope(ws, session, 'result', envelope.requestId, settleResult(result))
          } catch (error) {
            sendEnvelope(ws, session, 'result', envelope.requestId, settleError(error))
          } finally {
            session.actionInFlight = false
          }
        })().catch(error => {
          console.error('[executor] WS 处理失败：', error)
        })
      })
      ws.on('close', () => session.channel.detachAll('Go 通道已断开'))
    })
  })

  server.listen(port, '127.0.0.1', () => {
    console.log(JSON.stringify({
      service: 'form-runtime-executor',
      listening: `http://127.0.0.1:${port}`,
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
