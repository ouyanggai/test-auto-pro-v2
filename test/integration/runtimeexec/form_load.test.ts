// T008 发起表单装载集成验证：拉起真实执行服务（无头 Chromium + 固定路由），
// ① 普通静态 FormMaking 表单装载成功，capture 返回 getValues 完整数据；
// ② 选项协调完成后值、显示值（虚拟字段）一致——错位历史值被协调回正确绑定值（F-035 同类缺陷回归）；
// ③ 未成功装载的会话拒绝业务动作（写动作门禁）。
import assert from 'node:assert/strict'
import fs from 'node:fs'
import net from 'node:net'
import path from 'node:path'
import test from 'node:test'
import { spawn } from 'node:child_process'

const executorDir = path.resolve('form-runtime/executor')

// environmentReady 确认执行服务运行条件；缺失时跳过（缺依赖不是测试结论）。
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

interface ExecutorProcess {
  base: string
  stop: () => Promise<void>
}

async function startExecutor (): Promise<ExecutorProcess> {
  const port = await freePort()
  const child = spawn('node', ['--experimental-strip-types', 'src/server.ts'], {
    cwd: executorDir,
    env: { ...process.env, EXECUTOR_PORT: String(port) },
    stdio: ['ignore', 'pipe', 'pipe']
  })
  child.stderr.on('data', data => process.stderr.write(`[executor] ${data}`))
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

// protocolEnvelope 构造协议信封。
function protocolEnvelope (kind: string, requestId: string, sessionId: string, payload?: unknown) {
  return {
    version: 'exec-engine/v1',
    kind,
    requestId,
    sessionId,
    runId: 'run-t008',
    pathRunId: 'pr-t008',
    stepId: 'step-t008',
    payload
  }
}

// 动作命令经 WS 发送并等待 result 信封。
function executeOverWS (base: string, sessionId: string, action: string): Promise<Record<string, unknown>> {
  return new Promise((resolve, reject) => {
    const wsURL = `${base.replace('http', 'ws')}/runtime/session/${sessionId}/execute`
    const ws = new WebSocket(wsURL)
    const timer = setTimeout(() => {
      ws.close()
      reject(new Error('WS 动作等待结果超时'))
    }, 60000)
    ws.onopen = () => ws.send(JSON.stringify(protocolEnvelope('execute', `req-${action}`, sessionId, { action })))
    ws.onmessage = event => {
      const message = JSON.parse(String(event.data))
      if (message.kind === 'result') {
        clearTimeout(timer)
        ws.close()
        resolve(message.payload)
      }
    }
    ws.onerror = () => {
      clearTimeout(timer)
      reject(new Error('WS 连接失败'))
    }
  })
}

async function createSession (base: string): Promise<string> {
  const response = await fetch(`${base}/runtime/session`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ accountKey: 'acct-t008', runId: 'run-t008', pathRunId: 'pr-t008', stepId: 'step-t008' })
  })
  assert.equal(response.status, 200, `创建会话应成功：${response.status}`)
  const body = await response.json() as { sessionId: string }
  return body.sessionId
}

// 员工请假单同构的最小模板：静态选项下拉（showLabel=false 时渲染层直接显示 value）+ 文本输入。
const TEMPLATE = {
  list: [
    {
      type: 'input',
      model: 'reason',
      label: '请假事由',
      options: { remote: false }
    },
    {
      type: 'select',
      model: 'vacateType',
      label: '请假类别',
      options: {
        remote: false,
        showLabel: true,
        options: [
          { value: '1', label: '产假' },
          { value: '2', label: '事假' }
        ]
      }
    }
  ],
  config: {}
}

test('T008 发起表单装载、选项协调与写动作门禁', { timeout: 240000 }, async t => {
  if (!environmentReady()) t.skip('执行服务运行条件缺失：先 pnpm install 并构建 form-runtime')
  const executor = await startExecutor()
  t.after(() => executor.stop())

  // ① 装载：历史值故意错位（绑定值 2=事假 / 补丁显示名 产假），补丁触及的虚拟名字段触发选项协调。
  const sessionId = await createSession(executor.base)
  const loadResponse = await fetch(`${executor.base}/runtime/session/${sessionId}/load`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(protocolEnvelope('load', 'req-load', sessionId, {
      mode: 'initiate',
      renderType: 'formmaking',
      template: TEMPLATE,
      values: { reason: '家庭事务', vacateType: '2', vacateType__virtualName: '产假' },
      permissions: ['reason', 'vacateType'],
      changedFields: ['vacateType__virtualName'],
      toolFlowName: 'T008 装载验证流程'
    }))
  })
  assert.equal(loadResponse.status, 200, '装载应成功')
  const loadBody = await loadResponse.json() as { businessSummary?: { ready?: boolean } }
  assert.equal(loadBody.businessSummary?.ready, true, `装载结果应为 ready：${JSON.stringify(loadBody)}`)

  // ② capture：走真实 FormMaking getValues；协调后值与虚拟显示字段一致（产假=1）。
  const capture = await executeOverWS(executor.base, sessionId, 'capture')
  assert.equal(capture.status, 'success', `capture 应成功：${JSON.stringify(capture)}`)
  const summary = capture.businessSummary as { values?: Record<string, unknown> }
  assert.equal(summary.values.reason, '家庭事务')
  assert.equal(summary.values.vacateType, '1', `错位绑定值应被协调回「产假」的绑定值 1：${JSON.stringify(summary.values)}`)
  assert.equal(summary.values.vacateType__virtualName, '产假', '虚拟显示字段必须与绑定值同口径')

  // ③ 装载成功的会话允许 validate（只读口径）。
  const validate = await executeOverWS(executor.base, sessionId, 'validate')
  assert.equal(validate.status, 'success', `validate 应成功：${JSON.stringify(validate)}`)
  await fetch(`${executor.base}/runtime/session/${sessionId}/destroy`, { method: 'POST' })

  // ④ 未装载的会话拒绝业务动作（写动作门禁）；capture 仍允许只读探查。
  const freshSession = await createSession(executor.base)
  const blocked = await executeOverWS(executor.base, freshSession, 'submit')
  assert.equal(blocked.status, 'failed', `未装载会话的业务动作必须被拒绝：${JSON.stringify(blocked)}`)
  assert.ok(String(blocked.message).includes('尚未成功装载'), '拒绝原因必须说明未装载')
  await fetch(`${executor.base}/runtime/session/${freshSession}/destroy`, { method: 'POST' })
})
