// T009 审批装载与身份注入验证：真实执行服务 + 无头 Chromium，
// ① 审批装载必须携带原发起人事实，缺失时被拒绝（当前人不得顶替发起人）；
// ② 身份分开保存：状态端点返回的发起人与操作人各自稳定；
// ③ 两个审批人依次/并行执行：表单值、身份事实互不串线，旧会话销毁后无残留。
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

function protocolEnvelope (kind: string, requestId: string, sessionId: string, payload?: unknown) {
  return {
    version: 'exec-engine/v1', kind, requestId, sessionId,
    runId: 'run-t009', pathRunId: 'pr-t009', stepId: 'step-t009', payload
  }
}

const TEMPLATE = {
  list: [
    { type: 'input', model: 'initiatorName', label: '发起人', options: { remote: false } },
    { type: 'input', model: 'auditNote', label: '审批备注', options: { remote: false } }
  ],
  config: {}
}

async function createSession (base: string, accountKey: string): Promise<string> {
  const response = await fetch(`${base}/runtime/session`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ accountKey, runId: 'run-t009', pathRunId: 'pr-t009', stepId: 'step-t009' })
  })
  assert.equal(response.status, 200)
  const body = await response.json() as { sessionId: string }
  return body.sessionId
}

interface LoadFailure { status: number; body: { error?: string; message?: string } }

async function loadApprove (base: string, sessionId: string, operator: { userId: string; accountName: string }, initiator: unknown, auditNote: string): Promise<LoadFailure | null> {
  const response = await fetch(`${base}/runtime/session/${sessionId}/load`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(protocolEnvelope('load', `req-${sessionId.slice(0, 8)}`, sessionId, {
      mode: 'approve',
      renderType: 'formmaking',
      template: TEMPLATE,
      values: { initiatorName: '张发起', auditNote },
      permissions: ['auditNote'],
      changedFields: [],
      toolFlowName: 'T009 审批身份验证流程',
      initiator,
      currentOperator: operator,
      instanceId: 'instance-t009',
      jobTaskId: `task-${operator.userId}`,
      nodeProxyId: 'node-t009'
    }))
  })
  const body = await response.json() as { error?: string; message?: string }
  if (response.status === 200) return null
  return { status: response.status, body }
}

interface StatusBody {
  loaded: boolean
  identity: {
    mode: string
    operatorAccount: string
    initiator: { userId: string; name: string } | null
    instanceId: string
    jobTaskId: string
    nodeProxyId: string
  } | null
}

async function statusOf (base: string, sessionId: string): Promise<StatusBody> {
  const response = await fetch(`${base}/runtime/session/${sessionId}/status`)
  assert.equal(response.status, 200)
  return await response.json() as StatusBody
}

test('T009 审批装载身份注入与双审批人隔离', { timeout: 300000 }, async t => {
  if (!environmentReady()) t.skip('执行服务运行条件缺失：先 pnpm install 并构建 form-runtime')
  const executor = await startExecutor()
  t.after(() => executor.stop())

  // ① 缺少发起人事实的审批装载必须失败。
  const rejectSession = await createSession(executor.base, 'acct-auditor')
  const failure = await loadApprove(executor.base, rejectSession, { userId: 'u-auditor', accountName: 'acct-auditor' }, null, 'x')
  assert.ok(failure, '缺发起人的审批装载应失败')
  assert.ok(JSON.stringify(failure.body).includes('发起人'), `拒绝原因必须点名发起人：${JSON.stringify(failure.body)}`)
  const rejectStatus = await statusOf(executor.base, rejectSession)
  assert.equal(rejectStatus.loaded, false, '失败装载不能置为已装载')
  await fetch(`${executor.base}/runtime/session/${rejectSession}/destroy`, { method: 'POST' })

  // ② 两个审批人各自装载自己的审批现场：操作人不同、发起人相同（同一主实例）。
  const sessionA = await createSession(executor.base, 'acct-auditor-a')
  const sessionB = await createSession(executor.base, 'acct-auditor-b')
  const loadA = loadApprove(executor.base, sessionA, { userId: 'u-auditor-a', accountName: 'acct-auditor-a' }, { userId: 'u-initiator', name: '张发起' }, 'A 的意见')
  const loadB = loadApprove(executor.base, sessionB, { userId: 'u-auditor-b', accountName: 'acct-auditor-b' }, { userId: 'u-initiator', name: '张发起' }, 'B 的意见')
  assert.equal(await loadA, null, '审批人 A 装载应成功')
  assert.equal(await loadB, null, '审批人 B 装载应成功')

  // ③ 身份分开保存：操作人各自独立、发起人一致、任务/节点事实各归各会话。
  const statusA = await statusOf(executor.base, sessionA)
  const statusB = await statusOf(executor.base, sessionB)
  assert.equal(statusA.identity!.mode, 'approve')
  assert.equal(statusA.identity!.operatorAccount, 'acct-auditor-a')
  assert.equal(statusB.identity!.operatorAccount, 'acct-auditor-b')
  assert.deepEqual(statusA.identity!.initiator, { userId: 'u-initiator', name: '张发起' })
  assert.deepEqual(statusB.identity!.initiator, { userId: 'u-initiator', name: '张发起' })
  assert.equal(statusA.identity!.jobTaskId, 'task-u-auditor-a')
  assert.equal(statusB.identity!.jobTaskId, 'task-u-auditor-b')
  assert.equal(statusA.identity!.nodeProxyId, 'node-t009')

  // ④ 销毁 A 后：A 的状态事实消失，B 的审批现场不受影响。
  await fetch(`${executor.base}/runtime/session/${sessionA}/destroy`, { method: 'POST' })
  const destroyedStatus = await fetch(`${executor.base}/runtime/session/${sessionA}/status`)
  assert.equal(destroyedStatus.status, 404, '销毁后的会话状态应不可见')
  const statusBAfter = await statusOf(executor.base, sessionB)
  assert.equal(statusBAfter.identity!.operatorAccount, 'acct-auditor-b', 'B 的现场不能被 A 销毁影响')
  assert.equal(statusBAfter.loaded, true)
  await fetch(`${executor.base}/runtime/session/${sessionB}/destroy`, { method: 'POST' })
})
