import assert from 'node:assert/strict'
import test from 'node:test'
import type { WebSocket } from 'ws'

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

test('已有活动通道时拒绝新连接且不清理旧连接', () => {
  const channel = new ForwardChannel()
  let rejected = false
  const oldSocket = { OPEN: 1, readyState: 1, close: () => undefined } as unknown as WebSocket
  const newSocket = { OPEN: 1, readyState: 1, close: () => { rejected = true } } as unknown as WebSocket
  assert.equal(channel.attach(oldSocket), true)
  assert.equal(channel.attach(newSocket), false)
  assert.equal(rejected, true)
  assert.equal(channel.socketForEvents, oldSocket)
})
