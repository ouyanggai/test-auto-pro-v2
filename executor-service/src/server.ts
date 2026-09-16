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
import { selectBoundTemplate } from './native-initiate.ts'
import { resolveDetailSemantics } from './native-detail.ts'
import { applyNodePatch } from './data-patch.ts'
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
    window.__executorNative = { initFlow: null, flowDetail: null };
    const wrap = (name, original) => {
      return function wrapped () {
        const created = original.apply(this, arguments);
        if (created && typeof created === 'object') window.__executorNative[name] = created.instance || created;
        return created;
      };
    };
    const waitVue = () => {
      const Vue = window.Vue || (window.app && window.app.constructor);
      if (!Vue || !Vue.prototype) return false;
      if (Vue.prototype.$initFlow) Vue.prototype.$initFlow = wrap('initFlow', Vue.prototype.$initFlow);
      if (Vue.prototype.$flowDetail) Vue.prototype.$flowDetail = wrap('flowDetail', Vue.prototype.$flowDetail);
      return true;
    };
    if (!waitVue()) {
      const timer = setInterval(() => { if (waitVue()) clearInterval(timer); }, 20);
      setTimeout(() => clearInterval(timer), 10000);
    }
  })()`
}

function nativeURL (origin: string, payload: Record<string, unknown>): string {
  const mode = String(payload.mode ?? 'initiate')
  if (mode === 'initiate') {
    const params = new URLSearchParams()
    if (payload.selectFlowType) params.set('auditWay', String(payload.selectFlowType))
    if (payload.formProxyId) params.set('flowTemplateId', String(payload.formProxyId))
    if (payload.companyId) params.set('companyId', String(payload.companyId))
    if (payload.toolFlowName) params.set('title', String(payload.toolFlowName))
    return `${origin}#/flow/initiate?${params.toString()}`
  }
  const instanceId = String(payload.instanceId ?? '')
  const detailMode = mode === 'approve' || mode === 'forward' ? 'audit' : mode
  const params = new URLSearchParams()
  params.set('mode', detailMode)
  if (payload.jobTaskId) params.set('jobTaskId', String(payload.jobTaskId))
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
      send(200, {
        sessionId,
        loaded: session.loaded,
        channelConnected: session.channel.connected,
        identity: session.identity,
        trace: session.route?.trace.stats() ?? { total: 0 }
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
          const host = await session.page.waitForFunction(
            'window.__executorNative && (window.__executorNative.initFlow || window.__executorNative.flowDetail)',
            undefined,
            { timeout: 30000 }
          ).then(async () => {
            const captured = await session.page.evaluate(() => (window as unknown as { __executorNative: Record<string, unknown> }).__executorNative)
            if (!captured || (!captured.initFlow && !captured.flowDetail)) {
              throw new Error('原生宿主捕获失败')
            }
            return captured
          }).catch(() => null)
          session.nativeHost = host
          const mode = String(payload.mode ?? 'initiate')
          if (mode === 'initiate') {
            const handle = await session.page.evaluate(() => {
              const instance = (window as unknown as { __executorNative: { initFlow?: Record<string, unknown> } }).__executorNative.initFlow
              if (!instance) return null
              return {
                params: (instance as { params?: Record<string, unknown> }).params ?? {},
                flowList: (instance as { flowList?: Array<{ id: string }> }).flowList ?? []
              }
            })
            if (handle) {
              selectBoundTemplate({
                params: handle.params,
                flowList: handle.flowList,
                confirm: id => { void session.page.evaluate(bound => {
                  const instance = (window as unknown as { __executorNative: { initFlow?: { confirm?: (value: string) => void } } }).__executorNative.initFlow
                  instance?.confirm?.(bound)
                }, id) },
                getFlowFindById: id => { void session.page.evaluate(bound => {
                  const instance = (window as unknown as { __executorNative: { initFlow?: { getFlowFindById?: (value: string) => void } } }).__executorNative.initFlow
                  instance?.getFlowFindById?.(bound)
                }, id) }
              }, String(payload.flowProxyId ?? ''))
            }
          } else {
            resolveDetailSemantics(mode === 'approve' || mode === 'forward' ? 'audit' : mode, {
              jobTaskId: String(payload.jobTaskId ?? ''),
              flowInstanceId: String(payload.instanceId ?? '')
            })
          }
          const allowed = Array.isArray(payload.changedFields) ? payload.changedFields.map(String) : []
          if (allowed.length > 0) {
            const api = await session.page.evaluate(() => {
              const native = (window as unknown as { __executorNative: { initFlow?: unknown, flowDetail?: unknown } }).__executorNative
              const container = native.initFlow || native.flowDetail
              if (!container) return false
              const typed = container as { $children?: Array<{ $options?: { name?: string }, $refs?: { generateForm?: { setData?: unknown, getValues?: unknown } } }> }
              const dialog = typed.$children?.find(c => c.$options?.name === 'FlowDialog' || c.$options?.name === 'FlowDetail')
              const form = dialog?.$refs?.generateForm
              return Boolean(form && typeof form.setData === 'function' && typeof form.getValues === 'function')
            })
            if (api) {
              await applyNodePatch({
                setData: next => session.page.evaluate(values => {
                  const native = (window as unknown as { __executorNative: { initFlow?: unknown, flowDetail?: unknown } }).__executorNative
                  const container = native.initFlow || native.flowDetail
                  const typed = container as { $children?: Array<{ $options?: { name?: string }, $refs?: { generateForm?: { setData: (v: unknown) => void } } }> }
                  const dialog = typed.$children?.find(c => c.$options?.name === 'FlowDialog' || c.$options?.name === 'FlowDetail')
                  dialog?.$refs?.generateForm?.setData(values)
                }, next),
                getValues: () => session.page.evaluate(() => {
                  const native = (window as unknown as { __executorNative: { initFlow?: unknown, flowDetail?: unknown } }).__executorNative
                  const container = native.initFlow || native.flowDetail
                  const typed = container as { $children?: Array<{ $options?: { name?: string }, $refs?: { generateForm?: { getValues: () => Record<string, unknown> } } }> }
                  const dialog = typed.$children?.find(c => c.$options?.name === 'FlowDialog' || c.$options?.name === 'FlowDetail')
                  return dialog?.$refs?.generateForm?.getValues() ?? {}
                })
              }, values, { allowedKeys: allowed, optionsReady: false })
            }
          }
          if (!session.nativeHost) {
            send(500, { error: '原生宿主未捕获，装载失败' })
            return
          }
          session.loaded = true
          session.identity = {
            mode,
            operator: payload.currentOperator ?? null,
            initiator: payload.initiator ?? null
          }
          session.route.setToolFlowName(String(payload.toolFlowName ?? ''))
          send(200, { ready: true, mode })
        } catch (error) {
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
            const baselineSeq = session.route!.trace.nextSeq()
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
