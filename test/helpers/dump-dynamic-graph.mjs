// 只读诊断：打印一次运行的动态图（加签等运行期插入的节点）与节点运行态事实。
// 用于定位「加签节点没有审批人信息、不显示执行信息」这类展示问题。
// 用法：node test/helpers/dump-dynamic-graph.mjs <runId> [路径序号，从 1 开始，默认 1]
// 注意：不要从 PowerShell 传中文参数（会被按 ANSI 重新编码导致匹配失败），故用序号定位。
const runId = process.argv[2]
const pathIndex = Number(process.argv[3] || '1')
if (!runId) {
  console.error('用法：node test/helpers/dump-dynamic-graph.mjs <runId> [路径序号]')
  process.exit(2)
}
const pathsEnvelope = await (await fetch(`http://127.0.0.1:19080/api/runs/${runId}/paths`)).json()
const paths = (pathsEnvelope.data || {}).paths || []
const target = paths[pathIndex - 1]
if (!target) {
  console.error(`未找到第 ${pathIndex} 条路径（共 ${paths.length} 条）`)
  process.exit(1)
}
const envelope = await (await fetch(`http://127.0.0.1:19080/api/runs/${runId}?pathRunId=${target.pathRunId}`)).json()
const data = envelope.data || {}
console.log(`路径=${data.pathName} 状态=${data.pathRunStatusName}`)
console.log(`dynamicGraphBindable=${data.dynamicGraphBindable}`)
console.log(`dynamicNodeIds=${JSON.stringify(data.dynamicNodeIds || [])}`)

const dynamic = new Set(data.dynamicNodeIds || [])
const graph = data.dynamicGraph
console.log(`dynamicGraph=${graph ? graph.nodes.length + ' 个节点' : 'null'}`)
if (graph) {
  for (const node of graph.nodes) {
    const mark = dynamic.has(node.id) ? ' [运行期插入]' : ''
    console.log(`  节点 ${node.id} name=${node.name} type=${node.type}${mark}`)
  }
}

console.log('\n节点运行态（nodeStates）：')
const states = data.nodeStates || {}
for (const [nodeId, state] of Object.entries(states)) {
  const name = graph ? (graph.nodes.find((n) => n.id === nodeId) || {}).name : ''
  const dyn = dynamic.has(nodeId) ? ' [运行期插入]' : ''
  console.log(`  ${nodeId}${dyn} name=${name || '(图中无此节点)'} status=${state.status} statusName=${state.statusName}`)
}

console.log('\n步骤（nodeId 映射）：')
for (const step of data.steps || []) {
  const dyn = dynamic.has(step.nodeId) ? ' [运行期插入]' : ''
  console.log(`  第 ${step.stepNo} 步 action=${step.action} nodeKey=${step.nodeKey} nodeId=${step.nodeId}${dyn} actor=${step.actorName || '-'} status=${step.statusName}`)
}
