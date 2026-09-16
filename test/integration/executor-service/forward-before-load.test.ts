import assert from 'node:assert/strict'
import test from 'node:test'

import { ForwardChannel, ForwardFailure } from '../../../executor-service/src/forward-channel.ts'
import { assertForwardReady } from '../../../executor-service/src/navigate.ts'

test('装载前断开转发通道则禁止导航', () => {
  const channel = new ForwardChannel()
  assert.equal(channel.connected, false)
  assert.throws(() => assertForwardReady(channel), (error: unknown) => {
    assert.ok(error instanceof ForwardFailure)
    assert.equal(String(error.message).includes('禁止导航'), true)
    return true
  })
})
