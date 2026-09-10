import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('../../../../web/src/views/PlanPathConfigurationView.vue', import.meta.url), 'utf8')

// functionBody 截取指定函数到下一段函数注释之间的源码，锁定成功提示必须接在真实保存流程上。
function functionBody(name) {
  const start = source.indexOf(`function ${name}`)
  const end = source.indexOf('\n// ', start)
  assert.notEqual(start, -1, `未找到函数 ${name}`)
  return source.slice(start, end === -1 ? source.length : end)
}

// TestCurrentNodeSaveShowsSuccess 验证正常响应和响应丢失后的服务端对账都经统一收尾显示成功提示。
test('保存当前节点后显示一次成功提示', () => {
  const finish = functionBody('finishConfirmedNodeSave')
  assert.match(source, /notification\.success\(\{\s*title: '节点配置已保存'/)
  assert.match(finish, /showNodeSaveNotice\('当前节点配置已保存'\)/)
  assert.equal(finish.match(/destination\.kind === 'next-node'/g)?.length, 1)
  assert.ok(finish.indexOf("showNodeSaveNotice('当前节点配置已保存')") < finish.indexOf("destination.kind === 'next-node'"))
})

// TestSaveAllNodesShowsSuccess 验证“保存全部节点”完整成功时给出包含保存数量的明确反馈。
test('保存全部节点后显示一次成功提示', () => {
  const saveAll = functionBody('saveAllNodes')
  assert.match(saveAll, /showNodeSaveNotice\(`已保存 \$\{savedCount\} 个节点`\)/)
})
