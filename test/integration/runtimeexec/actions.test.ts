// T010/T010B 业务动作集成验证：以“假 Go”WS 对端驱动真实执行服务与真实 FlowDialog 提交链路。
// 场景（员工请假单同构的静态表单）：
// ① 装载：flowId 触发目标模板自装载（flowTemplateFindById + formTemplateApi/findById 经转发由假 Go 回答）；
// ② 提交：真实 sumbitFlow → formMakingFormBusiness → enterpriseHandleSubmit 链路发出 /web/flowInstanceApi/submit；
//    名称必须是工具流程名称（协议层恢复）；
// ③ 选择闭环：第一次提交被目标以 errorType=custom_choose 明确拒绝（未推进流程）→ 服务端广播 choice_required →
//    注入已配置分支/人员后调用真实 handleSaveBranchChooseNode 继续同一提交入口，第二次提交携带完整 nextAuditorList；
// ④ 未推进事实：两次提交均为目标受理前的选择要求，闭环只继续一次。
import assert from 'node:assert/strict'
import fs from 'node:fs'
import net from 'node:net'
import path from 'node:path'
import test from 'node:test'
import { spawn } from 'node:child_process'

const executorDir = path.resolve('form-runtime/executor')

function environmentReady (): boolean {
  return fs.existsSync(path.join(executorDir, 'node_modules'))
    && fs.existsSync(path.resolve('web/dist/form-runtime/index.html'))
}

async function freePort (): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = net.createServer()
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      server.close(() => address && typeof address !== 'string' ? resolve(address.port) : reject(new Error('端口分配失败')))
    })
    server.on('error', reject)
  })
}

interface ExecutorProcess { base: string; stop: () => Promise<void> }

async function startExecutor (): Promise<ExecutorProcess> {
  const port = await freePort()
  const child = spawn('node', ['--experimental-strip-types', 'src/server.ts'], {
    cwd: executorDir,
    env: { ...process.env, EXECUTOR_PORT: String(port) },
    stdio: ['ignore', 'pipe', 'pipe']
  })
  child.stderr.on('data', data => process.stderr.write(`[executor-err] ${data}`))
  child.stdout.on('data', data => process.stderr.write(`[executor-out] ${data}`))
  const base = `http://127.0.0.1:${port}`
  const deadline = Date.now() + 30000
  while (Date.now() < deadline) {
    try {
      await fetch(`${base}/runtime/session/none/status`)
      return {
        base,
        stop: () => new Promise((resolve, reject) => {
          child.once('exit', () => resolve())
          child.kill('SIGTERM')
          setTimeout(() => reject(new Error('执行服务退出超时')), 15000).unref()
        })
      }
    } catch {
      await new Promise(resolve => setTimeout(resolve, 200))
    }
  }
  child.kill('SIGKILL')
  throw new Error('执行服务 30 秒内未就绪')
}

// ===== 假 Go：作为 WS 对端应答转发请求并驱动命令 =====
interface ForwardRecord { url: string; body: Record<string, any>; raw: string }

function startFakeGo (base: string, sessionId: string) {
  const ws = new WebSocket(`${base.replace('http', 'ws')}/runtime/session/${sessionId}/execute`)
  const forwards: ForwardRecord[] = []
  const events: Array<Record<string, any>> = []
  const results: Array<Record<string, any>> = []
  const waiters: Array<() => void> = []
  // 目标响应脚本：按 /web/flowInstanceApi/submit 的第几次调用返回。
  let submitCalls = 0
  ws.onmessage = event => {
    const message = JSON.parse(String(event.data))
    if (message.kind === 'request_forward') {
      const payload = message.payload as { method: string; url: string; body?: string; bodyEncoding?: string }
      const rawBody = payload.body && payload.bodyEncoding === 'base64'
        ? Buffer.from(payload.body, 'base64').toString('utf8')
        : ''
      let parsedBody: Record<string, any> = {}
      try { parsedBody = JSON.parse(rawBody || '{}') } catch { parsedBody = {} }
      forwards.push({ url: payload.url, body: parsedBody, raw: rawBody })
      let responseText: string
      if (payload.url.includes('/web/flowTemplateApi/findById')) {
        // 目标流程模板读取（字段权限与下一节点树）。
        responseText = JSON.stringify({ message: 'success', isSuccess: true, data: FLOW_FIND_BY_ID })
      } else if (payload.url.includes('/web/formTemplateApi/findById')) {
        // 目标表单模板读取：templateData 为 FormMaking JSON 字符串。
        responseText = JSON.stringify({
          message: 'success',
          isSuccess: true,
          data: { fieldsTemplateList: [], templateData: JSON.stringify(TEMPLATE) }
        })
      } else if (payload.url.includes('/flowGroup/list') || payload.url.includes('/type/list')) {
        // 列表类读取：data 必须是数组（页面直接 filter/map）。
        responseText = JSON.stringify({ message: 'success', isSuccess: true, data: [] })
      } else if (payload.url.endsWith('/web/flowInstanceApi/submit') && parsedBody.formDataMongoVo) {
        // 只有携带表单正文的才是真实提交；checkFlowPermission 同端点但无正文。
        submitCalls++
        if (submitCalls === 1) {
          // 第一次：明确的选择要求（流程未推进）。
          responseText = JSON.stringify({
            message: '手动条件分支,请选择',
            isSuccess: false,
            data: {
              errorType: 'custom_choose',
              branchNodes: [
                {
                  id: 'node-b1',
                  nodeName: '产假分支',
                  type: 'manual',
                  flowNodeAuditConfig: { auditType: 'assign', type: 'assign' }
                },
                {
                  id: 'node-b2',
                  nodeName: '事假分支',
                  type: 'manual',
                  flowNodeAuditConfig: { auditType: 'assign', type: 'assign' }
                }
              ]
            }
          })
        } else {
          responseText = JSON.stringify({ message: 'success', isSuccess: true, data: { id: 'instance-new', status: 'run' } })
        }
      } else {
        responseText = JSON.stringify({ message: 'success', isSuccess: true, data: {} })
      }
      ws.send(JSON.stringify({
        version: 'exec-engine/v1', kind: 'request_response', requestId: message.requestId,
        sessionId, runId: message.runId, pathRunId: message.pathRunId, stepId: message.stepId,
        payload: { httpStatus: 200, headers: { 'content-type': 'application/json' }, body: Buffer.from(responseText).toString('base64'), bodyEncoding: 'base64' }
      }))
    } else if (message.kind === 'event') {
      events.push(message.payload)
    } else if (message.kind === 'result') {
      results.push(message.payload)
    }
    waiters.splice(0).forEach(run => run())
  }
  const send = (payloadBody: unknown) => ws.send(JSON.stringify({
    version: 'exec-engine/v1', kind: 'execute', requestId: `req-${Math.random().toString(36).slice(2)}`,
    sessionId, runId: 'run-t010', pathRunId: 'pr-t010', stepId: 'step-t010', payload: payloadBody
  }))
  const waitUntil = (predicate: () => boolean, timeoutMs = 90000) => new Promise<void>((resolve, reject) => {
    const check = () => {
      if (predicate()) { resolve(); return true }
      return false
    }
    if (check()) return
    const timer = setTimeout(() => reject(new Error(
      `等待条件超时：forwards=${JSON.stringify(forwards.map(f => ({ url: f.url.split('/api/')[1], keys: Object.keys(f.body || {}), dataKeys: f.body && f.body.data ? Object.keys(f.body.data) : null })))} events=${JSON.stringify(events.map(e => e.type))} results=${JSON.stringify(results)}`
    )), timeoutMs)
    const poll = setInterval(() => { if (check()) { clearInterval(poll); clearTimeout(timer) } }, 200)
    waiters.push(() => { if (check()) { clearInterval(poll); clearTimeout(timer) } })
  })
  return { ws, forwards, events, results, send, waitUntil }
}

function protocolEnvelope (sessionId: string, payload: unknown) {
  return {
    version: 'exec-engine/v1', kind: 'load', requestId: 'req-load', sessionId,
    runId: 'run-t010', pathRunId: 'pr-t010', stepId: 'step-t010', payload
  }
}

const TEMPLATE = {
  list: [
    { type: 'input', model: 'reason', label: '请假事由', options: { remote: false, required: true } },
    {
      type: 'select', model: 'vacateType', label: '请假类别',
      options: { remote: false, showLabel: true, options: [{ value: '1', label: '产假' }, { value: '2', label: '事假' }] }
    }
  ],
  config: {}
}

// 目标流程/表单详情的最小真实形状（与 OtherSteps2 的解析逻辑对齐）。
const FLOW_FIND_BY_ID = {
  flowNodeTemplate: {
    type: 'start',
    childFlowNodeTemplate: {
      type: 'approve',
      flowNodeAuditConfig: { auditType: 'assign', type: 'assign' }
    },
    flowNodeFieldPowerTemplateList: []
  },
  formTemplateBizRelevanceVoList: []
}

test('T010 提交链路与手动分支选择闭环（真实 FlowDialog）', { timeout: 300000 }, async t => {
  if (!environmentReady()) t.skip('执行服务运行条件缺失：先 pnpm install 并构建 form-runtime')
  const executor = await startExecutor()
  t.after(() => executor.stop())

  const createResponse = await fetch(`${executor.base}/runtime/session`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ accountKey: 'acct-t010', runId: 'run-t010', pathRunId: 'pr-t010', stepId: 'step-t010' })
  })
  assert.equal(createResponse.status, 200)
  const { sessionId } = await createResponse.json() as { sessionId: string }

  // 假 Go 接管 WS 通道（创建会话后、装载前）。
  const fakeGo = startFakeGo(executor.base, sessionId)
  await new Promise<void>((resolve, reject) => {
    fakeGo.ws.onopen = () => resolve()
    fakeGo.ws.onerror = () => reject(new Error('假 Go 连接失败'))
  })
  // realSubmits 过滤出真实提交：端点相同但携带表单正文的才是提交动作（checkFlowPermission 同端点无正文）。
  function realSubmits () {
    return fakeGo.forwards.filter(item => item.url.endsWith('/web/flowInstanceApi/submit') && item.body.formDataMongoVo)
  }

  // ① 装载（发起模式，带 flowId）：模板由目标自装载，两次 findById 读取由假 Go 回答。
  const loadResponse = await fetch(`${executor.base}/runtime/session/${sessionId}/load`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(protocolEnvelope(sessionId, {
      mode: 'initiate',
      renderType: 'formmaking',
      flowId: 'flow-t010',
      formId: 'form-t010',
      formExist: '',
      selectFlowType: 'employee_leave',
      template: TEMPLATE,
      values: { reason: '产假事宜', vacateType: '1', vacateType__virtualName: '产假' },
      permissions: ['reason', 'vacateType'],
      changedFields: [],
      toolFlowName: 'T010 提交闭环验证'
    }))
  })
  const loadText = await loadResponse.text()
  assert.equal(loadResponse.status, 200, `装载应成功：${loadText}`)
  const loadBody = JSON.parse(loadText) as { businessSummary?: { ready?: boolean } }
  assert.equal(loadBody.businessSummary?.ready, true)

  // ② 提交：真实提交链路发出第一次 submit（checkFlowPermission 同端点但无表单正文，不算提交）。
  fakeGo.send({ action: 'submit' })
  const waitWithTrace = async (predicate: () => boolean) => {
    try {
      await fakeGo.waitUntil(predicate)
    } catch (error) {
      const status = await (await fetch(`${executor.base}/runtime/session/${sessionId}/status`)).json() as { traceEntries: unknown }
      throw new Error(`${String(error)}\ntrace=${JSON.stringify(status.traceEntries)}`)
    }
  }
  // ③ 目标返回明确选择要求：服务端必须广播 choice_required 事件（此时动作未结束，检测挂起）。
  await waitWithTrace(() => fakeGo.events.some(item => item.type === 'choice_required'))
  const choiceEvent = fakeGo.events.find(item => item.type === 'choice_required')!
  assert.equal(choiceEvent.data.errorType, 'custom_choose')
  assert.ok(Array.isArray(choiceEvent.data.data.branchNodes) && choiceEvent.data.data.branchNodes.length === 2)
  const firstSubmit = realSubmits()[0]!
  assert.equal(firstSubmit.body.data.name, 'T010 提交闭环验证', '主实例名称必须是工具流程名称')
  assert.equal(firstSubmit.body.formDataMongoVo.data.reason, '产假事宜', '提交正文必须携带表单数据')

  // ④ 注入已配置的分支与人员后继续：调用真实 handleSaveBranchChooseNode，第二次提交携带完整 nextAuditorList。
  fakeGo.send({
    action: 'choice_continue',
    choice: {
      kind: 'branch',
      branchEntryNodeProxyIds: ['node-b1'],
      nextAuditors: [
        { nodeProxyId: 'node-b1', bizId: 'u-auditor-1', name: '审核人一', auditDetailType: 'personnel' }
      ]
    }
  })
  await fakeGo.waitUntil(() => realSubmits().length >= 2)
  const secondSubmit = realSubmits()[1]!
  const auditors = secondSubmit.body.nextAuditorList
  assert.ok(Array.isArray(auditors), '第二次提交必须携带 nextAuditorList')
  assert.ok(
    auditors.some((item: Record<string, unknown>) => item.nodeProxyId === 'node-b1' && item.auditDetailType === undefined),
    '分支入口只传 nodeProxyId'
  )
  assert.ok(
    auditors.some((item: Record<string, unknown>) => item.nodeProxyId === 'node-b1' && item.bizId === 'u-auditor-1' && item.name === '审核人一' && item.auditDetailType === 'personnel'),
    `自选人员必须携带 bizId/name/auditDetailType：${JSON.stringify(auditors)}`
  )
  assert.equal(secondSubmit.body.data.name, 'T010 提交闭环验证', '继续提交同样保持工具流程名称')
  assert.equal(secondSubmit.body.formDataMongoVo.data.reason, '产假事宜')

  // ⑤ 目标受理：原动作的完成检测在闭环继续后落定——流程成功 + 业务完成事件 → success。
  await fakeGo.waitUntil(() => fakeGo.results.length >= 2)
  assert.equal(fakeGo.results[0]!.status, 'success', `原动作最终判定应为 success：${JSON.stringify(fakeGo.results[0])}`)
  await new Promise(resolve => setTimeout(resolve, 1500))
  assert.equal(realSubmits().length, 2, `闭环只允许继续一次，实际提交 ${realSubmits().length} 次`)
  await fetch(`${executor.base}/runtime/session/${sessionId}/destroy`, { method: 'POST' })
})
