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
} from '../../../executor-service/src/protocol.ts'

const fixturesDir = path.resolve('test/fixtures/runtimeexec/protocol')

function jsonFiles (dir: string): string[] {
  return fs.readdirSync(dir).filter(name => name.endsWith('.json')).sort()
}

test('全部合法夹具通过校验且版本为 v2', () => {
  const dir = path.join(fixturesDir, 'valid')
  assert.ok(jsonFiles(dir).length >= 8)
  for (const name of jsonFiles(dir)) {
    const raw = JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8'))
    const message = validateMessage(raw)
    assert.equal(message.version, PROTOCOL_VERSION)
  }
})

test('全部非法夹具被拒绝且错误码与文件名一致', () => {
  const dir = path.join(fixturesDir, 'invalid')
  for (const name of jsonFiles(dir)) {
    const raw = JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8'))
    assert.throws(() => validateMessage(raw), (error: unknown) => {
      assert.ok(error instanceof ProtocolError, `${name} 应抛 ProtocolError`)
      assert.equal(error.code, path.basename(name, '.json'))
      return true
    })
  }
})

test('旧协议 v1 被拒绝', () => {
  assert.throws(
    () => validateMessage({ version: 'exec-engine/v1', kind: 'load', requestId: 'r', sessionId: 's', runId: 'run', pathRunId: 'pr', stepId: 's1', payload: { mode: 'initiate' } }),
    (error: unknown) => error instanceof ProtocolError && error.code === 'version_rejected'
  )
})

test('load 携带 template 被拒绝', () => {
  assert.throws(
    () => validateMessage({ version: PROTOCOL_VERSION, kind: 'load', requestId: 'r', sessionId: 's', runId: 'run', pathRunId: 'pr', stepId: 's1', payload: { mode: 'initiate', template: { list: [] } } }),
    (error: unknown) => error instanceof ProtocolError && error.code === 'template_forbidden'
  )
})

test('结果状态固定七值', () => {
  assert.deepEqual([...RESULT_STATUSES].sort(), ['business_failed', 'failed', 'success', 'target_rejected', 'timeout', 'uncertain', 'unsupported'])
  assert.equal(EVENT_TYPES.includes('choice_required'), true)
  assert.equal(RUNTIME_ACTIONS.includes('forward'), true)
  assert.equal(RUNTIME_ACTIONS.includes('add_sign'), false)
})
