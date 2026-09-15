// T005 执行模式根桥接：把执行 App 根组件的方法挂到 window.__runtimeBridge__，
// 供无头执行服务以页面脚本注入的方式调用；不遍历组件层级，也不模拟按钮。
// 命令信封与执行服务协议（form-runtime/executor/src/protocol.ts、internal/runtimeexec/protocol.go）逐字同源：
// version 固定 exec-engine/v1，缺 run/pathRun/step/request 编号的命令一律拒绝。
import { EXECUTION_ACTIONS, EXECUTION_PROTOCOL_VERSION, isExecutionCommand } from './protocol.js'

// BRIDGE_METHODS 是对外暴露且仅有的七个方法；多余能力一律不挂到全局。
// loadForward 是转发复合动作的装载入口（评审 P1-4）：注入转发弹窗等价的行事实与接收人。
export const BRIDGE_METHODS = ['loadInitiate', 'loadApprove', 'loadForward', 'capture', 'validate', 'execute', 'destroy']

// BridgeInvalidatedError 表示桥接已销毁后仍被调用；调用方必须停止，不允许复活会话。
export class BridgeInvalidatedError extends Error {
  constructor () {
    super('执行桥接已销毁，不能继续执行命令')
    this.name = 'BridgeInvalidatedError'
    this.code = 'BRIDGE_DESTROYED'
  }
}

// createExecutionBridge 绑定执行模式 App 根组件，返回六个业务命令方法。
// app 必须提供：loadForExecution(mode, payload)、captureForExecution(validate)、
// executeBusinessAction(action, payload)、destroyExecutionSession()、executionSessionId()。
// 本模块只做信封校验、销毁失效与分发，不承载任何业务规则。
export function createExecutionBridge (app) {
  let invalidated = false

  // guard 在每个命令前统一校验：旧命令、错版本、缺编号、销毁后调用全部原地拒绝。
  function guard (command) {
    if (invalidated) throw new BridgeInvalidatedError()
    if (!isExecutionCommand(command)) {
      const error = new Error('执行命令信封不合法：版本、runId/pathRunId/stepId/requestId 必须齐全且为当前协议版本')
      error.code = 'BRIDGE_COMMAND_INVALID'
      throw error
    }
    const session = app.executionSessionId()
    if (session && command.sessionId && command.sessionId !== session) {
      const error = new Error(`命令会话 ${command.sessionId} 与当前会话 ${session} 不一致`)
      error.code = 'BRIDGE_SESSION_MISMATCH'
      throw error
    }
  }

  // wrap 为单个命令方法补上统一守卫；业务错误原样上抛，由执行服务转成协议结果。
  function wrap (handler) {
    return async command => {
      guard(command)
      return handler(command)
    }
  }

  const bridge = {
    version: EXECUTION_PROTOCOL_VERSION,
    loadInitiate: wrap(command => app.loadForExecution('initiate', command)),
    loadApprove: wrap(command => app.loadForExecution('approve', command)),
    loadForward: wrap(command => app.loadForExecution('forward', command)),
    capture: wrap(command => app.captureForExecution(false, command)),
    validate: wrap(command => app.captureForExecution(true, command)),
    execute: wrap(command => app.executeBusinessAction(command.payload.action, command)),
    destroy: wrap(async command => {
      invalidated = true
      await app.destroyExecutionSession(command)
      return { destroyed: true }
    })
  }

  // invalidate 供 App 在会话被替换/销毁时主动废弃桥接；之后任何方法调用都抛 BridgeInvalidatedError。
  function invalidate () {
    invalidated = true
  }
  bridge.invalidate = invalidate
  return bridge
}

// EXECUTION_ACTIONS 透传自协议模块，供页面侧与执行服务侧共用同一动作全集口径。
export { EXECUTION_ACTIONS, EXECUTION_PROTOCOL_VERSION }
