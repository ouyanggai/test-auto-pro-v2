import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const viewPath = new URL('../../../../web/src/views/RunDetailView.vue', import.meta.url)
const apiPath = new URL('../../../../web/src/features/runs/api.ts', import.meta.url)
const panelPath = new URL('../../../../web/src/features/runs/RunNodePanel.vue', import.meta.url)
const view = fs.readFileSync(viewPath, 'utf8')
const api = fs.readFileSync(apiPath, 'utf8')
const panel = fs.readFileSync(panelPath, 'utf8')

test('路径切换递增请求代次并 abort 旧请求', () => {
  assert.match(view, /let requestVersion = 0/)
  assert.match(view, /controller.abort\(\)/)
  assert.match(view, /function isCurrent\(version: number, pathRunId: number\)/)
  assert.match(view, /closeNodePanel\(\)/)
  assert.match(view, /watch\(\(\) => route.params.pathRunId/)
})

test('Teleport 不再使用 defer，节点面板带路径稳定 key', () => {
  assert.match(view, /<Teleport v-if="detail" to="#app-header-context">/)
  assert.doesNotMatch(view, /Teleport v-if="detail" defer/)
  assert.match(view, /:key="`\$\{detail.pathRunId\}-\$\{selectedNodeKey\}`"/)
})

test('离开页面 abort 在途请求且不写状态', () => {
  assert.match(view, /disposed = true/)
  assert.match(view, /onBeforeUnmount/)
  assert.match(view, /if \(isAbortError\(error\) \|\| !isCurrent/)
  assert.match(api, /function isAbortError/)
  assert.match(api, /fetchRunEvents\(runId: string, afterEventId: number, pathRunId\?: number, signal\?: AbortSignal\)/)
})

test('事件游标按 pathRunId 绑定，旧路径不串流', () => {
  assert.match(view, /let eventPathRunID = 0/)
  assert.match(view, /if \(eventPathRunID !== pathRunId\)/)
  assert.match(view, /runEvents.value = \[\]/)
})

test('新响应没有 currentPreview 时保留同代次有效节点', () => {
  assert.match(view, /lastValidCurrentNodeByVersion/)
  assert.match(view, /lastValidCurrentNodeByVersion.set\(requestVersion, previewKey\)/)
})

test('放行、停止和失败重试都按请求代次丢弃旧响应', () => {
  assert.match(view, /async function retryFailed/)
  assert.match(view, /if \(!isCurrent\(version, pathRunId\)\) return/)
  assert.match(view, /watch\(activeTab, \(tab\) =>/)
  assert.match(view, /void loadDetail\(\)\.catch\(\(\) => \{\}\)/)
})

test('流程图读取中止时不得改写成格式异常', () => {
  const graphApi = fs.readFileSync(new URL('../../../../web/src/features/flow-graph/api.ts', import.meta.url), 'utf8')
  assert.match(graphApi, /if \(signal\.aborted\) throw error/)
})


test('节点面板仍有三个 Modal，切换路径时父级会卸载旧实例', () => {
  const modalCount = (panel.match(/<n-modal/g) || []).length
  assert.equal(modalCount, 3)
  assert.match(view, /v-if="selectedNodeKey && detail"/)
})
