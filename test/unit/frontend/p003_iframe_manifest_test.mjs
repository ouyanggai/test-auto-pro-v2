import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

import { buildValuesEnvelope } from '../../../form-runtime/src/runtime/formTemplate.js'
import { installReadOnlyRequestPolicy } from '../../../form-runtime/src/runtime/requestPolicy.js'

test('P003 两类渲染器共享完整诊断回传外壳', () => {
  const payload = buildValuesEnvelope({
    renderType: 'vue_custom', ruleVersion: 'rule-p003', values: { rows: [{ id: 1 }], virtual__name: '名称' },
    validated: true, dirty: true, unsupported: [], generatedFieldPaths: ['rows'], manualOverridePaths: ['virtual__name'],
    issues: [{ code: 'field_partial', status: 'partial', source: 'vue_bridge', canRetry: true }], stats: { autoFilled: 1, manualPending: 0 },
  })
  assert.equal(payload.renderType, 'vue_custom')
  assert.equal(payload.ruleVersion, 'rule-p003')
  assert.equal(payload.validated, true)
  assert.equal(payload.dirty, true)
  assert.deepEqual(payload.values.rows, [{ id: 1 }])
  assert.deepEqual(payload.generatedFieldPaths, ['rows'])
  assert.deepEqual(payload.manualOverridePaths, ['virtual__name'])
  assert.equal(payload.issues[0].code, 'field_partial')
})

test('P003 空清单显式告警并保留过渡判定，SID 继续进入内网请求 URL', () => {
  class FakeXHR {
    open(method, url) { this.opened = [method, url] }
    setRequestHeader() {}
    send() {}
  }
  const originalWindow = globalThis.window
  const originalXHR = globalThis.XMLHttpRequest
  globalThis.XMLHttpRequest = FakeXHR
  globalThis.window = { location: { href: 'http://runtime.test/' }, fetch: async () => new Response('{}') }
  try {
    const issues = []
    const restore = installReadOnlyRequestPolicy({ sid: 'diagnostic-sid', baseURL: 'http://target.test/api', readRequestManifest: [], onIssue: issue => issues.push(issue) })
    const request = new XMLHttpRequest()
    request.open('POST', '/web/flowProxy/findById')
    assert.equal(request.opened[1], 'http://target.test/api/web/flowProxy/findById?sid=diagnostic-sid')
    assert.equal(issues[0].code, 'request_manifest_empty')
    assert.equal(issues[0].status, 'partial')
    restore()
  } finally {
    globalThis.window = originalWindow
    globalThis.XMLHttpRequest = originalXHR
  }
})

test('P003 父页下发规则版本和只读清单，运行时回传结构化问题', () => {
  const frame = fs.readFileSync(new URL('../../../web/src/features/path-configuration/FormRuntimeFrame.vue', import.meta.url), 'utf8')
  const runtime = fs.readFileSync(new URL('../../../form-runtime/src/App.vue', import.meta.url), 'utf8')
  assert.match(frame, /ruleVersion: props\.form\.ruleVersion/)
  assert.match(frame, /readRequestManifest: props\.form\.readRequests/)
  assert.match(runtime, /renderType: this\.renderType, ruleVersion: this\.ruleVersion/)
  assert.match(runtime, /issues: this\.combinedIssues\(\)/)
})
