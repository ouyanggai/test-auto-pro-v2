import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

import { clonePlain } from '../../form-runtime/src/runtime/formTemplate.js'
import { FORM_RUNTIME_VERSION, isRuntimeCommand, isRuntimeResponse } from '../../form-runtime/src/runtime/protocol.js'
import { classifyRequestWithManifest, classifyTargetRequest } from '../../form-runtime/src/runtime/requestPolicy.js'
import { classifyRuntimeMessage } from '../../web/src/features/path-configuration/runtimeProtocol.ts'

const fixture = JSON.parse(fs.readFileSync(new URL('../fixtures/f011_iframe_protocol_golden.json', import.meta.url), 'utf8'))

test('F011 iframe 全命令与响应 golden 保持 v1 契约', () => {
  assert.equal(fixture.commands.length, 7)
  for (const command of fixture.commands) assert.equal(isRuntimeCommand(command), true, command.type)
  for (const response of Object.values(fixture.responses)) assert.equal(isRuntimeResponse(response), true, response.type)
  assert.equal(isRuntimeCommand({ ...fixture.commands[0], version: 'f007-form-runtime/v0' }), false)
  assert.equal(isRuntimeResponse({ ...fixture.responses.result, type: 'unknown' }), false)
})

test('F011 validateAndGetValues golden 完整保留嵌套数组、JSON 字符串和虚拟字段', () => {
  const values = clonePlain(fixture.responses.result.payload.values)
  assert.deepEqual(values.region, ['east', 'shanghai'])
  assert.deepEqual(values.items, [{ name: '明细一' }])
  assert.equal(JSON.parse(values.project).id, 'project-1')
  assert.equal(values.project__virtualName, '项目一')
  assert.equal(fixture.responses.result.payload.validated, true)
  assert.equal(fixture.responses.result.payload.renderType, 'formmaking')
  assert.equal(fixture.responses.result.payload.ruleVersion, 'rule-v1')
  assert.equal(fixture.responses.result.payload.dirty, true)
  assert.deepEqual(fixture.responses.result.payload.issues, [])
})

test('F011 父页拒绝旧版本、旧会话、迟到请求并只消费当前请求', () => {
  const baseContext = {
    sessionId: 'session-current',
    pendingRequestIds: new Set(['request-current', 'request-error']),
    runtimeActive: true,
    disposed: false,
    bootPending: false,
  }
  assert.equal(classifyRuntimeMessage(fixture.responses.result, baseContext), 'result')
  assert.equal(classifyRuntimeMessage(fixture.responses.error, baseContext), 'error')
  assert.equal(classifyRuntimeMessage({ ...fixture.responses.result, version: 'old' }, baseContext), 'ignore')
  assert.equal(classifyRuntimeMessage({ ...fixture.responses.result, sessionId: 'session-old' }, baseContext), 'ignore')
  assert.equal(classifyRuntimeMessage({ ...fixture.responses.result, requestId: 'request-late' }, baseContext), 'ignore')
  assert.equal(classifyRuntimeMessage({ ...fixture.responses.result, type: 'unknown' }, baseContext), 'ignore')
  assert.equal(classifyRuntimeMessage(fixture.responses.result, { ...baseContext, pendingRequestIds: new Set() }), 'ignore')
  assert.equal(classifyRuntimeMessage(fixture.responses.boot, { ...baseContext, runtimeActive: false, bootPending: true }), 'boot')
  assert.equal(classifyRuntimeMessage(fixture.responses.state, { ...baseContext, disposed: true }), 'ignore')
})

test('F011 请求影子报告量化当前清单覆盖和启发式缺口', () => {
  let covered = 0
  let readMisses = 0
  let writeLeaks = 0
  for (const request of fixture.requestPolicy) {
    const decision = classifyTargetRequest(request.method, request.pathname)
    assert.equal(decision.allowed, request.allowed, request.pathname)
    if (request.manifest === 'read' && decision.allowed) covered++
    if (request.manifest === 'read' && !decision.allowed) readMisses++
    if (request.manifest === 'write' && decision.allowed) writeLeaks++
  }
  assert.deepEqual({ covered, readMisses, writeLeaks }, { covered: 2, readMisses: 1, writeLeaks: 1 })
  assert.equal(FORM_RUNTIME_VERSION, 'f007-form-runtime/v1')
})

test('P003 非空只读清单优先且未覆盖请求默认拒绝', () => {
  const manifest = fixture.requestPolicy
    .filter(request => request.manifest === 'read')
    .map(request => ({ method: request.method, path: request.pathname, source: 'golden_manifest' }))
  for (const request of fixture.requestPolicy) {
    const decision = classifyRequestWithManifest(request.method, request.pathname, manifest)
    assert.equal(decision.allowed, request.manifest === 'read', request.pathname)
  }
  assert.deepEqual(classifyRequestWithManifest('POST', '/api/web/order/detail/42', [
    { method: 'POST', path: '/web/order/detail/:id', source: 'vue_rule_catalog' },
  ]), { allowed: true, reason: 'manifest_allow', source: 'vue_rule_catalog', manifestStatus: 'present' })
  assert.equal(classifyRequestWithManifest('GET', '/web/unlisted', manifest).reason, 'manifest_miss')
  assert.equal(classifyRequestWithManifest('POST', '/web/flowInstanceApi/submit', manifest).source, 'hard_block')
  assert.equal(classifyRequestWithManifest('POST', '/web/flowProxy/findById', []).source, 'transition_heuristic')
})

test('F011 父页继续在协议判定前核对真实 origin 和 source', () => {
  const source = fs.readFileSync(new URL('../../web/src/features/path-configuration/FormRuntimeFrame.vue', import.meta.url), 'utf8')
  assert.match(source, /event\.origin !== runtimeOrigin\.value \|\| event\.source !== iframe\.value\?\.contentWindow/)
})
