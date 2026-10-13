import assert from 'node:assert/strict'
import test from 'node:test'
import type { Page, Route } from 'playwright'

import { ExecutionBaselineRecorder, describeBaselineError, type BaselineRecord } from '../../../executor-service/src/execution-baseline.ts'
import { settleCompletion } from '../../../executor-service/src/completion.ts'
import { nativeDispatch } from '../../../executor-service/src/native-dispatch.ts'
import { installTargetRoute, type ForwardOutcome, type TargetRouteOptions } from '../../../executor-service/src/target-route.ts'

const UNKNOWN_BOUNDARIES = [
  'form_async_completion', 'business_callback_completion',
  'target_fact_verification', 'deployed_source_version'
]

// makeRecorder 使用内存日志和可控时钟，不启动页面、服务或真实目标请求。
function makeRecorder(enabled: boolean, options: { sink?: (record: BaselineRecord) => void, now?: () => number, maxActiveSpans?: number } = {}) {
  const records: BaselineRecord[] = []
  const recorder = new ExecutionBaselineRecorder({ runId: 'run-local', pathRunId: 'path-local', sid: 'fake-sid', platformCode: 'p1', projectId: 'project1' }, {
    enabled,
    now: options.now ?? (() => 10),
    maxActiveSpans: options.maxActiveSpans,
    sink: options.sink ?? (record => { records.push(record) })
  })
  return { recorder, records }
}

// deferred 把响应结束顺序交给测试控制，避免长等待和不稳定的计时断言。
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

test('关闭记录不取时钟、不写日志，原调用仍只发生一次', () => {
  let calls = 0
  const { recorder, records } = makeRecorder(false, {
    now: () => { throw new Error('关闭时不能取时钟') },
    sink: () => { throw new Error('关闭时不能写日志') }
  })
  const result = { saved: false }
  assert.strictEqual(recorder.invoke('原调用', () => { calls++; return result }), result)
  assert.equal(recorder.begin('未启用'), null)
  recorder.checkpoint('检查点')
  assert.equal(calls, 1)
  assert.equal(records.length, 0)
  assert.equal(recorder.stats().records, 0)
})

test('同步返回、this、参数和原异常对象在开关记录时保持一致', () => {
  for (const enabled of [false, true]) {
    const { recorder } = makeRecorder(enabled)
    const owner = { marker: '原组件' }
    const argument = { field: 'value' }
    const result = { retained: true }
    const calls: unknown[][] = []
    function original(this: typeof owner, value: typeof argument) {
      calls.push([this, value])
      return result
    }
    assert.strictEqual(recorder.invoke('同步调用', () => original.call(owner, argument), { method: 'original' }), result)
    assert.strictEqual(calls[0]![0], owner)
    assert.strictEqual(calls[0]![1], argument)
    assert.equal(calls.length, 1)
    const originalError = new Error('原业务错误')
    assert.throws(() => recorder.invoke('原业务失败', () => { throw originalError }), error => error === originalError)
  }
})

test('原Promise和thenable不被替换或追加then，异步错误原样交给调用方', async () => {
  for (const enabled of [false, true]) {
    const { recorder, records } = makeRecorder(enabled)
    const operation = deferred<object>()
    const returned = recorder.invoke('异步入口返回', () => operation.promise)
    assert.strictEqual(returned, operation.promise)
    const value = { completed: true }
    operation.resolve(value)
    assert.strictEqual(await returned, value)

    let subscriptions = 0
    const thenable = { then: () => { subscriptions++; throw new Error('记录不得订阅内部异步') } }
    assert.strictEqual(recorder.invoke('未知thenable', () => thenable), thenable)
    assert.equal(subscriptions, 0)

    const rejected = deferred<never>()
    const originalError = new Error('回调异步失败')
    const rejectedReturn = recorder.invoke('异步失败入口', () => rejected.promise)
    assert.strictEqual(rejectedReturn, rejected.promise)
    const rejectionCheck = assert.rejects(rejectedReturn, error => error === originalError)
    rejected.reject(originalError)
    await rejectionCheck
    if (enabled) {
      assert.equal(records.find(record => record.name === '异步入口返回' && record.phase === 'end')?.outcome, 'returned')
      assert.ok(records.every(record => UNKNOWN_BOUNDARIES.every(boundary => record.unknowns.includes(boundary))))
    }
  }
})

test('日志或时钟失败不影响原调用、原返回及原错误', () => {
  for (const failure of ['sink', 'clock'] as const) {
    let calls = 0
    const { recorder } = makeRecorder(true, {
      sink: failure === 'sink' ? () => { throw new Error('本地日志写失败') } : undefined,
      now: failure === 'clock' ? () => { throw new Error('本地时钟失败') } : undefined
    })
    const result = { untouched: true }
    assert.strictEqual(recorder.invoke('日志失败仍执行', () => { calls++; return result }), result)
    const originalError = new Error('业务本身失败')
    assert.throws(() => recorder.invoke('日志失败不吞业务错误', () => { calls++; throw originalError }), error => error === originalError)
    assert.equal(calls, 2)
    if (failure === 'sink') assert.ok(recorder.stats().sinkFailures > 0)
  }
})

test('业务异常不能被诊断格式化异常替换', () => {
  const { recorder } = makeRecorder(true)
  const originalError = new Error('原始业务异常')
  originalError.toString = () => { throw new Error('异常文字转换失败') }
  assert.throws(() => recorder.invoke('异常保真', () => { throw originalError }), error => error === originalError)
})

test('开关记录都不读取业务错误的转换钩子、getter或代理属性', () => {
  for (const enabled of [false, true]) {
    const { recorder } = makeRecorder(enabled)
    let diagnosticReads = 0
    const originalError = new Error('原来的错误内容')
    originalError.toString = () => {
      diagnosticReads++
      originalError.message = '被诊断改写'
      return originalError.message
    }
    describeBaselineError(originalError)
    assert.throws(() => recorder.invoke('原始错误', () => { throw originalError }), error => error === originalError)
    assert.equal(originalError.message, '原来的错误内容')
    const proxyError = new Proxy({}, {
      get: () => { diagnosticReads++; throw new Error('不能读取原错误属性') },
      getOwnPropertyDescriptor: () => { diagnosticReads++; throw new Error('不能反射读取原错误') }
    })
    describeBaselineError(proxyError)
    assert.throws(() => recorder.invoke('代理错误', () => { throw proxyError }), error => error === proxyError)
    assert.equal(diagnosticReads, 0)
  }
})

test('时钟无效或倒退只记未知耗时，不以零耗时冒充', () => {
  for (const moments of [[NaN, NaN], [Infinity, Infinity], [10, 1]]) {
    let index = 0
    const { recorder, records } = makeRecorder(true, { now: () => moments[index++]! })
    recorder.invoke('未知耗时', () => '原值')
    assert.equal(records.findLast(record => record.phase === 'end')?.durationMs, null)
  }
})

test('未结束段在检查点保留未知，记录有界且不会取消后来完成的原Promise', async () => {
  let now = 1
  const { recorder, records } = makeRecorder(true, { now: () => now, maxActiveSpans: 1 })
  const span = recorder.begin('已有后置回调')
  assert.ok(span)
  const pending = deferred<string>()
  assert.strictEqual(recorder.invoke('原函数返回但回调未完', () => pending.promise), pending.promise)
  assert.ok(recorder.stats().droppedSpans > 0)
  recorder.checkpoint('观察结束')
  const checkpoint = records.findLast(record => record.phase === 'checkpoint')
  assert.ok(checkpoint?.pending?.some(item => item.name === '已有后置回调'))
  assert.ok(UNKNOWN_BOUNDARIES.every(boundary => checkpoint!.unknowns.includes(boundary)))
  assert.equal(recorder.stats().activeSpans, 1)
  now = 11
  pending.resolve('原业务晚到的结果')
  assert.equal(await pending.promise, '原业务晚到的结果')
  recorder.end(span, 'returned')
  assert.equal(recorder.stats().activeSpans, 0)
  assert.equal(records.findLast(record => record.phase === 'end')?.durationMs, 10)
})

test('修改后续上下文不会串改已输出的身份记录', () => {
  const { recorder, records } = makeRecorder(true)
  recorder.invoke('A上下文', () => undefined)
  recorder.setContext({ sid: 'fake-sid-B', projectId: 'project-B' })
  recorder.invoke('B上下文', () => undefined)
  assert.equal(records.find(record => record.name === 'A上下文')?.context.sid, 'fake-sid')
  assert.equal(records.find(record => record.name === 'B上下文')?.context.sid, 'fake-sid-B')
  assert.equal(records.find(record => record.name === 'B上下文')?.context.projectId, 'project-B')
})

test('A请求晚于B上下文返回时，结束仍归属A请求开始的身份', () => {
  const { recorder, records } = makeRecorder(true)
  const requestA = recorder.begin('A请求')
  assert.ok(requestA)
  recorder.setContext({ sid: 'fake-sid-B', projectId: 'project-B', stepId: 'step-B' })
  recorder.invoke('B请求', () => 'B结果')
  recorder.end(requestA, 'returned', { response: 'A晚到结果' })
  const beganA = records.find(record => record.name === 'A请求' && record.phase === 'begin')
  const endedA = records.find(record => record.name === 'A请求' && record.phase === 'end')
  assert.deepEqual(endedA?.context, beganA?.context)
  assert.equal(endedA?.context.sid, 'fake-sid')
  assert.equal(endedA?.context.projectId, 'project1')
  assert.equal(endedA?.context.stepId, undefined)
  assert.equal(records.find(record => record.name === 'B请求')?.context.sid, 'fake-sid-B')
})

test('无效容量不会令未结束记录突破256条保护上限', () => {
  for (const maxActiveSpans of [NaN, Infinity, -Infinity, -1, 0]) {
    const { recorder } = makeRecorder(true, { maxActiveSpans })
    for (let index = 0; index < 257; index++) recorder.begin(`挂起${index}`)
    assert.ok(recorder.stats().activeSpans <= 256)
    assert.ok(recorder.stats().droppedSpans > 0)
  }
})

// nativeScenario 执行生产原生分发，所有表单和请求出口均为可计数的内存方法。
function nativeScenario(enabled: boolean, action: 'submit' | 'approve' | 'forward') {
  const { recorder, records } = makeRecorder(enabled)
  const events: Array<{ kind: string, value?: unknown }> = []
  const counts = { getData: 0, getValues: 0, setData: 0, requests: 0, mounts: 0 }
  const form = {
    getData() { counts.getData++; events.push({ kind: 'validate' }); return { person: 'p1', person__virtualName: '姓名', bigNumber: '9007199254740993' } },
    getValues() { counts.getValues++; return { shouldNotRead: true } },
    setData() { counts.setData++; throw new Error('观察不得回填表单') }
  }
  const send = (endpoint: string, value: unknown) => { counts.requests++; events.push({ kind: endpoint, value }) }
  const component = {
    $options: { name: action === 'submit' ? 'FlowDialog' : 'ExamineOpinion' },
    approveForm: { approveMessage: '' },
    sumbitFlow(kind: unknown) { send('pre_save', { kind }); send('submit', form.getData()); send('post_bind', { instanceId: 'i1' }) },
    submitCheckBefore(kind: unknown) { send('audit', { kind, form: form.getData(), message: this.approveForm.approveMessage }) }
  }
  const forward = {
    $options: { name: 'Backlog' },
    confirmTranspond() { send('transpond', { instanceId: 'i1' }); send('postscript', { instanceId: 'i2', desc: '', fuyan: false }) }
  }
  const native = action === 'submit'
    ? { initFlow: { $options: { name: 'InitFlowDialog' }, $children: [component] } }
    : action === 'approve'
      ? { flowDetail: { $options: { name: 'flowDetail' }, $children: [component] } }
      : { forwardList: forward }
  const result = recorder.invoke('native_dispatch', () => nativeDispatch(native, { action, auditDesc: '同意意见' }))
  return { events, counts, result, records }
}

test('实际普通发起、同意和转发分发开关记录时不增加请求、表单调用或组件', () => {
  for (const action of ['submit', 'approve', 'forward'] as const) {
    const off = nativeScenario(false, action)
    const on = nativeScenario(true, action)
    assert.deepEqual(on.events, off.events)
    assert.deepEqual(on.counts, off.counts)
    assert.deepEqual(on.result, off.result)
    assert.equal(on.counts.getValues, 0)
    assert.equal(on.counts.setData, 0)
    assert.equal(on.counts.mounts, 0)
    assert.ok(on.records.length > 0)
  }
})

type FakeRequest = { url: string, body?: string, method?: string }

// makeFakeRouteFixture 直接捕获生产page.route处理器，所有请求/回填均在内存完成。
async function makeFakeRouteFixture(enabled: boolean, forward: TargetRouteOptions['forward'], options: { sink?: (record: BaselineRecord) => void } = {}) {
  const { recorder, records } = makeRecorder(enabled, options)
  const events: Array<{ kind: string, value?: unknown }> = []
  let handler: ((route: Route) => Promise<void>) | undefined
  const page = {
    async route(_pattern: string, callback: (route: Route) => Promise<void>) { handler = callback },
    async unroute() { events.push({ kind: 'unroute' }) }
  } as unknown as Page
  const handle = await installTargetRoute(page, {
    runtimeOrigins: ['https://runtime.invalid'], baseline: recorder,
    forward: payload => {
      events.push({ kind: 'forward', value: { ...payload, headers: { ...payload.headers } } })
      return forward(payload)
    }
  })
  async function request(input: FakeRequest) {
    const fakeRequest = {
      url: () => input.url,
      method: () => input.method ?? 'POST',
      allHeaders: async () => ({ SID: 'fake-sid', 'Content-Type': 'application/json' }),
      postDataBuffer: () => input.body === undefined ? null : Buffer.from(input.body)
    }
    const route = {
      request: () => fakeRequest,
      async continue() { events.push({ kind: 'continue' }) },
      async abort(reason: string) { events.push({ kind: 'abort', value: reason }) },
      async fulfill(value: { status: number, headers: Record<string, string>, body?: string | Buffer, contentType?: string }) {
        events.push({ kind: 'fulfill', value: { ...value, body: Buffer.isBuffer(value.body) ? value.body.toString('base64') : value.body } })
      }
    } as unknown as Route
    assert.ok(handler)
    await handler(route)
  }
  return { recorder, records, events, handle, request }
}

// outcome 构造完整本地响应，不读取真实服务，也不对业务信封补猜测字段。
function outcome(value: unknown, httpStatus = 200): ForwardOutcome {
  return { httpStatus, headers: { 'content-type': 'application/json' }, bodyEncoding: 'utf8', body: JSON.stringify(value) }
}

// stableTrace 排除墙钟字段，只比较业务可见的请求、响应顺序、内容和终态。
function stableTrace(fixture: Awaited<ReturnType<typeof makeFakeRouteFixture>>) {
  return fixture.handle.trace.list().map(entry => ({
    seq: entry.seq, method: entry.method, url: entry.url, headers: entry.headers,
    body: entry.body, bodyEncoding: entry.bodyEncoding, state: entry.state, writeSequence: entry.writeSequence,
    abortReason: entry.abortReason,
    response: entry.response && { responseSeq: entry.response.responseSeq, httpStatus: entry.response.httpStatus, headers: entry.response.headers, body: entry.response.body, bodyEncoding: entry.response.bodyEncoding }
  }))
}

test('实际路由保留first准备、401和四类选择响应的完整载荷与原有回填', async () => {
  const scenarios = [
    { path: '/web/flowJobTaskLink/list', body: { data: { taskStatus: 'pending' }, flowInstanceIdList: ['i1'] }, response: outcome({ isSuccess: true, data: [{ jobTaskId: 'task1' }] }) },
    { path: '/web/flowInstanceApi/submit', body: { data: { checkPermissions: 'first', flowTemplateId: 'template1' } }, response: outcome({ isSuccess: true, data: { flowProxyId: 'prepared-proxy' } }) },
    { path: '/flowInstanceApi/audit', body: { data: { id: 'i1', jobTaskId: 'task1' } }, response: outcome({ isSuccess: false, code: 'AUTH_401', message: 'SID失效' }) },
    { path: '/flowInstanceApi/audit', body: { data: { id: 'i1' } }, response: outcome({ message: 'HTTP401' }, 401) },
    ...['custom_choose', 'parallel_choose', 'run_node_choose', 'extended_attribute_users'].map(errorType => ({
      path: '/flowInstanceApi/audit', body: { data: { id: 'i1', jobTaskId: 'task1' } }, response: outcome({ isSuccess: false, message: '请选择', data: { errorType, branchNodes: [{ id: 'node1' }] } })
    }))
  ]
  const run = async (enabled: boolean) => {
    let index = 0
    const fixture = await makeFakeRouteFixture(enabled, () => Promise.resolve(scenarios[index++]!.response))
    for (const scenario of scenarios) await fixture.request({ url: `https://target.invalid/api${scenario.path}`, body: JSON.stringify(scenario.body) })
    assert.equal(index, scenarios.length)
    assert.equal(fixture.events.filter(event => event.kind === 'forward').length, scenarios.length)
    assert.equal(fixture.events.filter(event => event.kind === 'fulfill').length, scenarios.length)
    return fixture
  }
  const off = await run(false)
  const on = await run(true)
  assert.deepEqual(on.events, off.events)
  assert.deepEqual(stableTrace(on), stableTrace(off))
  assert.equal(on.events.filter(event => event.kind === 'abort').length, 0)
  assert.ok(on.records.some(record => record.name === 'target_response' && record.phase === 'end'))
  assert.ok(on.records.some(record => record.name === 'page_fulfill' && record.phase === 'end'))
})

test('响应乱序和在途未完成不被记录改写，正文回填仍按原完成顺序', async () => {
  const run = async (enabled: boolean) => {
    const responses = [deferred<ForwardOutcome>(), deferred<ForwardOutcome>()]
    const entered = [deferred<void>(), deferred<void>()]
    let index = 0
    const fixture = await makeFakeRouteFixture(enabled, () => {
      const current = index++
      entered[current]!.resolve(undefined)
      return responses[current]!.promise
    })
    const first = fixture.request({ url: 'https://target.invalid/api/web/flowJobTaskLink/list', body: '{"data":{"id":"link1"}}' })
    const second = fixture.request({ url: 'https://target.invalid/api/web/flowJobTaskLink/list', body: '{"data":{"id":"link2"}}' })
    await Promise.all(entered.map(item => item.promise))
    fixture.recorder.checkpoint('两笔仍在途')
    assert.equal(fixture.events.filter(event => event.kind === 'fulfill').length, 0)
    if (enabled) {
      const checkpoint = fixture.records.findLast(record => record.phase === 'checkpoint')
      assert.equal(checkpoint?.pending?.filter(item => item.name === 'target_response').length, 2)
      assert.ok(UNKNOWN_BOUNDARIES.every(boundary => checkpoint!.unknowns.includes(boundary)))
    }
    responses[1]!.resolve(outcome({ isSuccess: true, data: { id: 'link2' } }))
    await second
    responses[0]!.resolve(outcome({ isSuccess: true, data: { id: 'link1' } }))
    await first
    return fixture
  }
  const off = await run(false)
  const on = await run(true)
  assert.deepEqual(on.events, off.events)
  assert.deepEqual(stableTrace(on), stableTrace(off))
  assert.deepEqual(on.handle.trace.list().map(entry => entry.response?.responseSeq), [2, 1])
})

test('转发主写成功而附言失败，开关记录不改变原判定且不重发主写', async () => {
  const run = async (enabled: boolean) => {
    const fixture = await makeFakeRouteFixture(enabled, payload => Promise.resolve(payload.writeSequence === 1
      ? outcome({ isSuccess: true, data: { id: 'new-instance' } })
      : outcome({ isSuccess: false, message: '附言保存失败' })))
    await fixture.request({ url: 'https://target.invalid/api/web/flowInstanceApi/transpond', body: '{"data":{"id":"old-instance"},"fuyan":false}' })
    await fixture.request({ url: 'https://target.invalid/api/web/flowInstancePostscript/save', body: '{"data":{"flowInstanceId":"new-instance","desc":""}}' })
    const verdict = await settleCompletion({ action: 'forward', trace: fixture.handle.trace, baselineSeq: 0, hardTimeoutMs: 1000, idleWindowMs: 1, forwardPostscriptRequired: true })
    return { fixture, verdict: { status: verdict.status, message: verdict.message } }
  }
  const off = await run(false)
  const on = await run(true)
  assert.deepEqual(on.fixture.events, off.fixture.events)
  assert.deepEqual(stableTrace(on.fixture), stableTrace(off.fixture))
  assert.deepEqual(on.verdict, off.verdict)
  assert.equal(on.verdict.status, 'business_failed')
  assert.equal(on.fixture.events.filter(event => event.kind === 'forward').length, 2)
})

test('正文转发失败仍保持原中止行为，不新发请求', async () => {
  const run = async (enabled: boolean) => {
    let calls = 0
    const fixture = await makeFakeRouteFixture(enabled, () => { calls++; return Promise.reject(new Error('模拟正文传输中断')) })
    await fixture.request({ url: 'https://target.invalid/api/web/flowInstanceApi/submit', body: '{"data":{"id":"i1"}}' })
    return { fixture, calls }
  }
  const off = await run(false)
  const on = await run(true)
  assert.equal(off.calls, 1)
  assert.equal(on.calls, 1)
  assert.deepEqual(on.fixture.events, off.fixture.events)
  assert.deepEqual(stableTrace(on.fixture), stableTrace(off.fixture))
  assert.equal(on.fixture.events.filter(event => event.kind === 'abort').length, 1)
  assert.equal(on.fixture.events.filter(event => event.kind === 'fulfill').length, 0)
})

test('实际路由写记录失败不把原成功响应变成请求中止', async () => {
  const response = outcome({ isSuccess: true, data: { id: 'i1' } })
  const off = await makeFakeRouteFixture(false, () => Promise.resolve(response))
  const failedLogging = await makeFakeRouteFixture(true, () => Promise.resolve(response), { sink: () => { throw new Error('记录输出失败') } })
  const request = { url: 'https://target.invalid/api/web/flowInstanceApi/submit', body: '{"data":{"id":"i1"}}' }
  await off.request(request)
  await failedLogging.request(request)
  assert.deepEqual(failedLogging.events, off.events)
  assert.deepEqual(stableTrace(failedLogging), stableTrace(off))
  assert.equal(failedLogging.events.filter(event => event.kind === 'forward').length, 1)
  assert.equal(failedLogging.events.filter(event => event.kind === 'fulfill').length, 1)
  assert.equal(failedLogging.events.filter(event => event.kind === 'abort').length, 0)
  assert.ok(failedLogging.recorder.stats().sinkFailures > 0)
})
