// 只读诊断：打印某个图节点的完整原始数据（含 auditPeople），用于定位审批人不显示的问题。
// 用法：node test/helpers/dump-node-detail.mjs <runId> <路径序号> <节点ID或名称关键字>
const [runId, pathIndexRaw, needle] = process.argv.slice(2)
if (!runId || !needle) {
  console.error('用法：node test/helpers/dump-node-detail.mjs <runId> <路径序号> <节点ID或名称关键字>')
  process.exit(2)
}
const pathsEnvelope = await (await fetch(`http://127.0.0.1:19080/api/runs/${runId}/paths`)).json()
const paths = (pathsEnvelope.data || {}).paths || []
const target = paths[Number(pathIndexRaw || '1') - 1]
const envelope = await (await fetch(`http://127.0.0.1:19080/api/runs/${runId}?pathRunId=${target.pathRunId}`)).json()
const data = envelope.data || {}
const graph = data.dynamicGraph
const dynamic = new Set(data.dynamicNodeIds || [])

console.log('=== 计划图（runFlowGraph 的节点） ===')
for (const node of ((data.dynamicGraph || {}).nodes || [])) {
  if (!String(node.id).includes(needle) && !String(node.name).includes(needle)) continue
  console.log(JSON.stringify(node, null, 1))
}

console.log('\n=== 静态计划图的同名节点（用于对比） ===')
const staticEnvelope = await (await fetch(`http://127.0.0.1:19080/api/plans/${data.planId}/flow-graph`)).json().catch(() => null)
const staticGraph = staticEnvelope && staticEnvelope.data
if (staticGraph && staticGraph.nodes) {
  for (const node of staticGraph.nodes) {
    if (!String(node.id).includes(needle) && !String(node.name).includes(needle)) continue
    console.log(JSON.stringify(node, null, 1))
  }
} else {
  console.log('（静态图接口不可用或返回空）')
}

console.log('\n=== 运行期插入节点集合 ===')
console.log([...dynamic].join('\n'))
