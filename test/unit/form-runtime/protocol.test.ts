// T004 协议单测（TS 侧）：与 Go 契约测试消费同一份夹具 test/fixtures/runtimeexec/protocol，
// 并补充 ProtocolError 稳定错误码与版本/动作全集的直查断言。
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'

import {
  EVENT_TYPES,
  PROTOCOL_VERSION,
  ProtocolError,
  RESULT_STATUSES,
  RUNTIME_ACTIONS,
  validateMessage
} from '../../../form-runtime/executor/src/protocol.ts'

const fixturesDir = path.resolve('test/fixtures/runtimeexec/protocol')

function jsonFiles (dir: string): string[] {
  return fs.readdirSync(dir).filter(name => name.endsWith('.json')).sort()
}

test('T004 全部合法夹具通过校验且版本回读一致', () => {
  const dir = path.join(fixturesDir, 'valid')
  assert.ok(jsonFiles(dir).length >= 8)
  for (const name of jsonFiles(dir)) {
    const raw = JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8'))
    const message = validateMessage(raw)
    assert.equal(message.version, PROTOCOL_VERSION)
  }
})

test('T004 全部非法夹具被拒绝且错误码与文件名一致', () => {
  const dir = path.join(fixturesDir, 'invalid')
  assert.ok(jsonFiles(dir).length >= 6)
  for (const name of jsonFiles(dir)) {
    const raw = JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8'))
    assert.throws(() => validateMessage(raw), (error: unknown) => {
      assert.ok(error instanceof ProtocolError, `${name} 应抛 ProtocolError`)
      assert.equal((error as ProtocolError).code, path.basename(name, '.json'))
      return true
    })
  }
})

test('T004 旧协议版本被拒绝，不静默兼容', () => {
  assert.throws(() => validateMessage({ version: 'exec-engine/v0', kind: 'load', requestId: 'r', sessionId: 's', runId: 'run', pathRunId: 'pr', stepId: 's1' }),
    (error: unknown) => error instanceof ProtocolError && (error as ProtocolError).code === 'version_rejected')
})

test('T004 未知动作被拒绝：Go 直连动作不允许进入 runtime 协议', () => {
  for (const action of ['add_sign', 'transfer', 'withdraw', 'urge', 'follow', 'unfollow', 'click', '']) {
    assert.throws(() => validateMessage({ version: PROTOCOL_VERSION, kind: 'execute', requestId: 'r', sessionId: 's', runId: 'run', pathRunId: 'pr', stepId: 's1', payload: { action } }),
      (error: unknown) => error instanceof ProtocolError && (error as ProtocolError).code === 'action_unknown',
      `动作 ${action} 应被拒绝`)
  }
})

test('T004 结果状态固定七值、事件子类型固定五种', () => {
  assert.deepEqual([...RESULT_STATUSES].sort(), ['business_failed', 'failed', 'success', 'target_rejected', 'timeout', 'uncertain', 'unsupported'])
  assert.equal(EVENT_TYPES.includes('choice_required' as any), true)
  assert.equal(RUNTIME_ACTIONS.includes('forward' as any), true)
  assert.equal(RUNTIME_ACTIONS.includes('add_sign' as any), false)
})
