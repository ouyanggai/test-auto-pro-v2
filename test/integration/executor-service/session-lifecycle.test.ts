import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import test from 'node:test'

async function startExecutor (port: number): Promise<{ stop: () => Promise<void> }> {
  const child = spawn(process.execPath, ['--experimental-strip-types', 'src/server.ts'], {
    cwd: 'executor-service',
    env: { ...process.env, EXECUTOR_PORT: String(port) },
    stdio: ['ignore', 'pipe', 'pipe']
  })
  const { promise: ready, resolve, reject } = Promise.withResolvers<void>()
  const abort = AbortSignal.timeout(60_000)
  abort.addEventListener('abort', () => reject(new Error('执行服务未在 60s 内就绪')))
  child.stdout?.on('data', chunk => {
    if (String(chunk).includes('"listening"')) resolve()
  })
  child.stderr?.on('data', chunk => {
    const text = String(chunk)
    if (text.includes('"ok":false')) reject(new Error(text))
  })
  child.on('exit', code => reject(new Error(`执行服务提前退出：${code}`)))
  await ready
  return {
    stop: async () => {
      child.kill('SIGTERM')
      const { promise, resolve: done } = Promise.withResolvers<void>()
      child.once('exit', () => done())
      await promise
    }
  }
}

test('POST /runtime/session 创建会话，缺字段 400，未知路径 404', async () => {
  const port = 19121
  const service = await startExecutor(port)
  try {
    const missing = await fetch(`http://127.0.0.1:${port}/runtime/session`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ accountKey: 'acct' })
    })
    assert.equal(missing.status, 400)

    const created = await fetch(`http://127.0.0.1:${port}/runtime/session`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ accountKey: 'acct-1', runId: '122', pathRunId: '396', stepId: '1' })
    })
    const createdText = await created.text()
    assert.equal(created.status, 200, createdText)
    const body = JSON.parse(createdText) as { sessionId: string, status: string }
    assert.ok(body.sessionId)
    assert.equal(body.status, 'ready')


    const status = await fetch(`http://127.0.0.1:${port}/runtime/session/${body.sessionId}/status`)
    assert.equal(status.status, 200)
    const statusBody = await status.json() as { loaded: boolean }
    assert.equal(statusBody.loaded, false)

    const destroyed = await fetch(`http://127.0.0.1:${port}/runtime/session/${body.sessionId}/destroy`, { method: 'POST' })
    assert.equal(destroyed.status, 200)

    const unknown = await fetch(`http://127.0.0.1:${port}/no-such`)
    assert.equal(unknown.status, 404)
  } finally {
    await service.stop()
  }
})
