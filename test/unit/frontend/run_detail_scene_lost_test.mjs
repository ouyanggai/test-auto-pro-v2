import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const viewPath = new URL('../../../web/src/views/RunDetailView.vue', import.meta.url)
const source = fs.readFileSync(viewPath, 'utf8')

test('现场丢失或结果待确认时详情页保持可查看，不自动跳回运行列表', () => {
  assert.match(source, /sceneLostNote/)
  assert.doesNotMatch(source, /sceneLostBouncing/)
  assert.doesNotMatch(source, /handleSceneLost/)
  assert.doesNotMatch(source, /router\.push\(['"]\/runs['"]\)/)
})
