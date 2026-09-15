// T005 执行桥接单测：用最小 App 假件核对桥接的六个方法、信封校验、会话匹配与销毁失效；
// 并断言桥接源码不包含任何按钮/DOM 操作痕迹（只调用业务方法）。
import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

import { BRIDGE_METHODS, BridgeInvalidatedError, createExecutionBridge } from '../../../form-runtime/src/runtime/executionBridge.js'
import { EXECUTION_ACTIONS, EXECUTION_PROTOCOL_VERSION } from '../../../form-runtime/src/runtime/protocol.js'

// fakeApp 模拟执行模式 App 根组件对桥接暴露的最小接口，并镜像真实的销毁失效接线。
function fakeApp () {
  const app = {
    session: '',
    destroyed: 0,
    loads: [],
    captures: [],
    actions: [],
    executionSessionId () {
      return app.session
    },
    async loadForExecution (mode, command) {
      app.loads.push({ mode, requestId: command.requestId })
      app.session = command.sessionId
      return { ready: true, mode }
    },
    async captureForExecution (validate, command) {
      app.assertExecutionSession(command)
      app.captures.push(validate)
      return { values: { a: 1 }, validated: validate }
    },
    async executeBusinessAction (action, command) {
      app.assertExecutionSession(command)
      app.actions.push(action)
      return { action, done: true }
    },
    async destroyExecutionSession () {
      app.destroyed++
      app.session = ''
      // 真实接线里 destroySession 会触发 invalidateExecutionBridge。
      if (app.executionBridge && typeof app.executionBridge.invalidate === 'function') app.executionBridge.invalidate()
      return { destroyed: true }
    },
    assertExecutionSession (command) {
      if (!app.session || (command.sessionId && command.sessionId !== app.session)) {
        const error = new Error('会话不一致')
        error.code = 'BRIDGE_SESSION_MISMATCH'
        throw error
      }
    }
  }
  return app
}

function command (overrides = {}) {
  return {
    version: EXECUTION_PROTOCOL_VERSION,
    requestId: 'req-1',
    sessionId: 'sess-1',
    runId: 'run-1',
    pathRunId: 'pr-1',
    stepId: 'step-1',
    payload: { action: 'submit' },
    ...overrides
  }
}

test('T005 桥接只暴露六个方法且版本一致', () => {
  const app = fakeApp()
  const bridge = createExecutionBridge(app)
  app.executionBridge = bridge
  assert.equal(bridge.version, EXECUTION_PROTOCOL_VERSION)
  const methods = BRIDGE_METHODS.filter(name => typeof bridge[name] === 'function').sort()
  assert.deepEqual(methods, ['capture', 'destroy', 'execute', 'loadApprove', 'loadInitiate', 'validate'])
  // 除 version/invalidate 外不允许出现其他可调用属性，防止桥接变成万能入口。
  const extra = Object.keys(bridge).filter(name => !BRIDGE_METHODS.includes(name) && name !== 'version' && name !== 'invalidate')
  assert.deepEqual(extra, [])
})

test('T005 合法命令按方法分发并返回 JSON 结果', async () => {
  const app = fakeApp()
  const bridge = createExecutionBridge(app)
  // 执行服务在创建会话时已生成 sessionId，load 命令总是携带它。
  const loadResult = await bridge.loadInitiate(command({ payload: { template: {} } }))
  assert.deepEqual(loadResult, { ready: true, mode: 'initiate' })
  assert.equal(app.session, 'sess-1')
  const captureResult = await bridge.capture(command())
  assert.deepEqual(captureResult, { values: { a: 1 }, validated: false })
  const validateResult = await bridge.validate(command())
  assert.equal(validateResult.validated, true)
  const executeResult = await bridge.execute(command({ payload: { action: 'save_draft' } }))
  assert.deepEqual(executeResult, { action: 'save_draft', done: true })
  const approveLoad = await bridge.loadApprove(command({ requestId: 'req-2' }))
  assert.equal(approveLoad.mode, 'approve')
})

test('T005 信封不合法或缺编号的命令被拒绝', async () => {
  const app = fakeApp()
  const bridge = createExecutionBridge(app)
  const invalids = [
    command({ version: 'f007-form-runtime/v1' }),
    command({ runId: '' }),
    command({ stepId: undefined }),
    command({ requestId: 42 })
  ]
  for (const item of invalids) {
    await assert.rejects(() => bridge.capture(item), error => error.code === 'BRIDGE_COMMAND_INVALID')
  }
})

test('T005 会话不匹配的命令被拒绝', async () => {
  const app = fakeApp()
  const bridge = createExecutionBridge(app)
  await bridge.loadInitiate(command({ sessionId: '' }))
  await assert.rejects(() => bridge.capture(command({ sessionId: 'sess-other' })), error => error.code === 'BRIDGE_SESSION_MISMATCH')
})

test('T005 destroy 后桥接对象不可执行', async () => {
  const app = fakeApp()
  const bridge = createExecutionBridge(app)
  await bridge.loadInitiate(command({ sessionId: '' }))
  const destroyResult = await bridge.destroy(command())
  assert.deepEqual(destroyResult, { destroyed: true })
  assert.equal(app.destroyed, 1)
  // 保留的旧引用也必须不可执行：任何方法都抛 BridgeInvalidatedError。
  for (const name of BRIDGE_METHODS) {
    if (name === 'destroy') continue
    await assert.rejects(() => bridge[name](command()), error => error instanceof BridgeInvalidatedError)
  }
})

test('T005 业务动作全集与 Go 直连动作边界', () => {
  // runtime 协议动作不包含 Go 直连动作，防止同一动作两条写出口。
  for (const goOnlyAction of ['add_sign', 'transfer', 'rollback_previous', 'retrieve', 'withdraw', 'urge', 'follow', 'unfollow', 'system_automatic']) {
    assert.equal(EXECUTION_ACTIONS.includes(goOnlyAction), false, `${goOnlyAction} 不应进入 runtime 协议`)
  }
})

test('T005 桥接源码没有按钮点击、DOM 定位或 $parent 痕迹', () => {
  const source = fs.readFileSync(new URL('../../../form-runtime/src/runtime/executionBridge.js', import.meta.url), 'utf8')
  for (const forbidden of ['.click', 'dispatchEvent', '$parent', 'querySelector', 'getByText', 'locator']) {
    assert.equal(source.includes(forbidden), false, `桥接源码不应包含 ${forbidden}`)
  }
})
