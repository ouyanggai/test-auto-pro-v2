import assert from 'node:assert/strict'
import test from 'node:test'
import type { RunRequestItem, RunStep, RunStepAttempt } from '../../../web/src/features/runs/api.ts'
import { actionRequestEmptyMessage, mergeActionRequests } from '../../../web/src/features/runs/actionRequests.ts'

// 固定夹具只有本地事实，不请求目标平台，不启动浏览器。
function request(seq: number, patch: Partial<RunRequestItem> = {}): RunRequestItem {
  return { method: 'POST', requestSeq: seq, endpoint: '/api/web/flowInstanceApi/transpond', requestClass: 'write', durationKnown: true, durationMs: 12, statusCode: 200, result: 'success', retryAttempt: 1, traceId: `trace-${seq}`, ...patch }
}
function attempt(patch: Partial<RunStepAttempt> = {}): RunStepAttempt {
  return { attemptNo: 1, verdictName: '执行成功', reason: '', basis: '', traceId: '', durationMs: 30, logPath: 'run/step.log', logLine: 1, ...patch }
}
function step(attempts: RunStepAttempt[], stepNo = 1): RunStep {
  return { stepNo, nodeKey: 'same-node', nodeName: '审批节点', actorName: '处理人', actionName: '转发', statusName: '执行成功', startedAt: '', finishedAt: '', durationMs: 30, attempts }
}
function runtime(requests: NonNullable<RunStepAttempt['runtimeDetail']>['requests']) {
  return { stages: [], status: 'success', capabilityVersion: 'v2', requests }
}

test('详情只消费指定动作，同节点其他动作不会混入', () => {
  const forward = step([attempt({ requests: [request(1)] })])
  const approve = step([attempt({ requests: [request(2, { endpoint: '/api/web/flowInstanceApi/audit' })] })], 2)
  assert.equal(forward.nodeKey, approve.nodeKey)
  assert.deepEqual(mergeActionRequests(forward).map(row => row.endpoint), ['/api/web/flowInstanceApi/transpond'])
  assert.equal(mergeActionRequests(approve).length, 1)
  assert.deepEqual(mergeActionRequests(null), [])
})

test('同一笔网络记录与页面轨迹合成一行，真实传输耗时和返回结果保留', () => {
  const original = '  目标 错误\n' + '详细原因'.repeat(100)
  const action = step([attempt({
    requests: [request(7, { result: 'failure', targetMessage: original, targetCode: 'ERROR', durationMs: 61 })],
    runtimeDetail: runtime([{ seq: 7, method: 'POST', path: '/web/flowInstanceApi/transpond', requestClass: 'write', state: 'fulfilled', httpStatus: 200 }]),
  })])
  const rows = mergeActionRequests(action)
  assert.equal(rows.length, 1)
  assert.equal(rows[0]!.durationMs, 61)
  assert.equal(rows[0]!.targetMessage, original)
  assert.equal(rows[0]!.traceId, 'trace-7')
  assert.equal(rows[0]!.evidence, 'network')
})

test('真实 SID 刷新产生两笔物理传输，同一页面序号不能吞掉重试', () => {
  const rows = mergeActionRequests(step([attempt({
    requests: [request(7, { statusCode: 401, retryAttempt: 1 }), request(7, { retryAttempt: 2, traceId: 'retry-trace' })],
    runtimeDetail: runtime([{ seq: 7, method: 'POST', path: '/web/flowInstanceApi/transpond', requestClass: 'write', state: 'fulfilled', httpStatus: 200 }]),
  })]))
  assert.equal(rows.length, 2)
  assert.deepEqual(rows.map(row => row.retryAttempt), [1, 2])
})

test('轨迹缺少传输日志时显示未知耗时，并区分本地拦截和转发失败', () => {
  const rows = mergeActionRequests(step([attempt({ runtimeDetail: runtime([
    { seq: 1, method: 'POST', path: '/web/formProxy/findById', requestClass: 'read', state: 'fulfilled', httpStatus: 200 },
    { seq: 2, method: 'POST', path: '/web/flowInstanceApi/transpond', requestClass: 'write', state: 'blocked', abortReason: '已登记，禁止重发' },
    { seq: 3, method: 'POST', path: '/web/flowInstancePostscript/save', requestClass: 'write', state: 'aborted' },
  ]) })]))
  assert.equal(rows.length, 3)
  assert.ok(rows.every(row => row.durationKnown === false))
  assert.match(rows[0]!.evidenceNote!, /请求已发生，暂无真实请求记录/)
  assert.match(rows[1]!.evidenceNote!, /本地拦截，未发送/)
  assert.match(rows[2]!.evidenceNote!, /无法确认目标是否收到/)
  assert.equal(rows[0]!.requestClass, 'read', 'POST 读取必须仍显示读取')
})

test('序号相同但方法、端点或尝试不同不能互相吞掉', () => {
  const rows = mergeActionRequests(step([
    attempt({ attemptNo: 1, requests: [request(1)] }),
    attempt({ attemptNo: 2, requests: [request(1)], runtimeDetail: runtime([
      { seq: 1, method: 'GET', path: '/web/flowInstanceApi/transpond', requestClass: 'read', state: 'fulfilled' },
      { seq: 2, method: 'POST', path: '/web/flowInstancePostscript/save', requestClass: 'write', state: 'fulfilled' },
    ]) }),
  ]))
  assert.equal(rows.length, 4)
  assert.deepEqual(rows.map(row => row.attemptNo), [1, 2, 2, 2])
})

test('读取与写入保持发生顺序，补缺轨迹放入对应时刻', () => {
  const rows = mergeActionRequests(step([attempt({
    requests: [request(3, { at: '2026-10-15T10:00:00.030+08:00' }), request(1, { at: '2026-10-15T10:00:00.010+08:00', endpoint: '/web/formProxy/findById', requestClass: 'read' })],
    runtimeDetail: runtime([{ seq: 2, method: 'POST', path: '/web/flowInstancePostscript/save', requestClass: 'write', state: 'fulfilled', requestAt: '2026-10-15T10:00:00.020+08:00' }]),
  })]))
  assert.deepEqual(rows.map(row => row.requestSeq), [1, 2, 3])
  assert.deepEqual(rows.map(row => row.requestClass), ['read', 'write', 'write'])
})

test('可读空日志和缺失日志使用不同文案，不能以缺失冒充未发送', () => {
  const action = step([attempt()])
  assert.match(actionRequestEmptyMessage(action, true), /没有发出/)
  assert.match(actionRequestEmptyMessage(action, false), /无法判断/)
  assert.doesNotMatch(actionRequestEmptyMessage(action, false), /没有发出/)
})

test('同一毫秒内仍按原始序号插入缺失轨迹', () => {
  const rows = mergeActionRequests(step([attempt({
    requests: [request(1, { at: '2026-10-15T10:00:00.010001+08:00' }), request(3, { at: '2026-10-15T10:00:00.010003+08:00' })],
    runtimeDetail: runtime([{ seq: 2, method: 'POST', path: '/web/flowInstancePostscript/save', requestClass: 'write', state: 'fulfilled', requestAt: '2026-10-15T10:00:00.010+08:00' }]),
  })]))
  assert.deepEqual(rows.map(row => row.requestSeq), [1, 2, 3])
})
