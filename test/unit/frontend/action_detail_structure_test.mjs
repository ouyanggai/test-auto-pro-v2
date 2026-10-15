import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const panel = fs.readFileSync(new URL('../../../web/src/features/runs/RunNodePanel.vue', import.meta.url), 'utf8')
const view = fs.readFileSync(new URL('../../../web/src/views/RunDetailView.vue', import.meta.url), 'utf8')
const layout = fs.readFileSync(new URL('../../../web/src/styles/layout.css', import.meta.url), 'utf8')

test('动作弹窗只渲染一次有序请求清单，不列同节点其他操作或第二份轨迹', () => {
  assert.match(panel, /mergeActionRequests\(stepDialog.value\)/)
  assert.match(panel, /v-for="\(req, index\) in stepRequests"/)
  assert.doesNotMatch(panel, /requestGroupsByOutcome|nodeOperations|ret\.requests/)
  assert.match(panel, /req\.targetMessage/)
  assert.match(panel, /req\.retryAttempt > 1/)
  assert.match(panel, /copyTraceID\(req\.traceId\)/)
})

test('长错误和日志内容可滚动，窄窗口能换行，不渲染 HTTP 0', () => {
  assert.match(panel, /run-panel__dialog app-dialog app-dialog--lg/)
  assert.match(layout, /max-width: calc\(100vw - 48px\)/)
  assert.match(layout, /max-height: calc\(100dvh - 96px\)/)
  assert.match(layout, /\.app-dialog\.n-card > \.n-card__content[^}]*overflow: auto/s)
  assert.match(panel, /white-space: pre-wrap/)
  assert.match(panel, /overflow-wrap: anywhere/)
  assert.match(panel, /req\.statusCode > 0/)
  assert.match(panel, /stepRequestEmptyText/)
  assert.match(panel, /stepDialog.receiverName/)
})

test('事件流保持总览，原文和节点流转可见，收尾失败不会误标成功', () => {
  assert.match(view, /各节点的流转、执行结果与错误/)
  assert.match(view, /event\.detail/)
  assert.match(view, /path_run_step_settled/)
  assert.match(view, /kind === 'path_run_finished'/)
  assert.match(view, /eventKindClass\(event.kind, event.label\)/)
  assert.match(view, /white-space: pre-wrap/)
})
