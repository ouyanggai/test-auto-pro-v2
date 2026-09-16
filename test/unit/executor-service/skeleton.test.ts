import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { spawn } from 'node:child_process'
import { healthBody, listenPort, SERVICE_NAME } from '../../../executor-service/src/health.ts'

const srcDir = path.resolve('executor-service/src')

function readSourceTree (dir: string): string {
  const parts: string[] = []
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      parts.push(readSourceTree(full))
      continue
    }
    if (entry.name.endsWith('.ts')) parts.push(fs.readFileSync(full, 'utf8'))
  }
  return parts.join('\n')
}

test('健康检查载荷固定且不访问目标', () => {
  const body = healthBody()
  assert.equal(body.ok, true)
  assert.equal(body.service, SERVICE_NAME)
  assert.equal(body.protocol, 'exec-engine/v2')
  assert.equal(listenPort(), 19011)
})

test('执行服务源码禁止界面操作 API', () => {
  const source = readSourceTree(srcDir)
  for (const forbidden of ['locator(', 'page.click', 'page.fill', 'page.$', 'mouse.click', 'keyboard.type']) {
    assert.equal(source.includes(forbidden), false, `不得出现 ${forbidden}`)
  }
})

test('骨架 HTTP 服务提供 /health', async () => {
  const port = 19111
  const child = spawn(process.execPath, ['--experimental-strip-types', 'src/server.ts'], {
    cwd: path.resolve('executor-service'),
    env: { ...process.env, EXECUTOR_PORT: String(port) },
    stdio: ['ignore', 'pipe', 'pipe']
  })
  // 等待真实子进程打印 listening：无法用假时钟推进外部 Node 进程。
  const { promise: ready, resolve, reject } = Promise.withResolvers<void>()
  const abort = AbortSignal.timeout(10_000)
  abort.addEventListener('abort', () => reject(new Error('执行服务骨架未在 10s 内就绪')))
  child.stdout?.on('data', chunk => {
    if (String(chunk).includes('"listening"')) resolve()
  })
  child.stderr?.on('data', chunk => {
    const text = String(chunk)
    if (text.includes('ok":false')) reject(new Error(text))
  })
  child.on('exit', code => reject(new Error(`骨架进程提前退出：${code}`)))
  await ready
  try {
    const response = await fetch(`http://127.0.0.1:${port}/health`)
    assert.equal(response.status, 200)
    const body = await response.json() as { ok: boolean, service: string, protocol: string }
    assert.equal(body.ok, true)
    assert.equal(body.service, 'executor-service')
    assert.equal(body.protocol, 'exec-engine/v2')
  } finally {
    child.kill('SIGTERM')
    const { promise: exited, resolve: done } = Promise.withResolvers<void>()
    child.once('exit', () => done())
    await exited
  }
})
