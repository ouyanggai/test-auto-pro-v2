// 诊断脚本：把路径 1 的编译场景按控制层注入规则演算一遍，定位「回退恢复链缺少加签节点同意」的位置。
// 只读：不发写请求、不启动运行。
const ADD_SIGN_PREFIX = 'add-sign-inserted-'

// rollbackPrecedes 与 internal/engine/control/control.go 的判定保持一致。
function rollbackPrecedes (steps, i, anchorKey) {
  if (i <= 0 || i >= steps.length) return false
  for (let j = i - 1; j >= 0; j -= 1) {
    if (steps[j].source === 'system_navigation') continue
    if (steps[j].action === 'approve' && steps[j].nodeKey === anchorKey) return false
    return steps[j].action === 'rollback_previous'
  }
  return false
}

// 模拟 injectInsertedNodeApprovals 的拼装规则（与 internal/engine/control/control.go 保持一致）。
function inject (steps, anchorKey, insertedKeys) {
  const insertedSet = new Set(insertedKeys)
  const out = []
  let injected = 0
  for (let i = 0; i < steps.length; i += 1) {
    const rollbackBefore = steps[i].action === 'approve' && steps[i].nodeKey === anchorKey && rollbackPrecedes(steps, i, anchorKey)
    if (rollbackBefore) {
      // 替换语义：用插入节点同意替换这条多余的锚点恢复同意。
      for (const key of insertedKeys) out.push({ source: 'system_default', action: 'approve', nodeKey: key, injected: true })
      injected += insertedKeys.length
      continue
    }
    out.push(steps[i])
    const isAnchorApprove = steps[i].action === 'approve' && steps[i].nodeKey === anchorKey
    const nextIsAnchorApprove = i + 1 < steps.length && steps[i + 1].action === 'approve' && steps[i + 1].nodeKey === anchorKey
    const nextIsInserted = i + 1 < steps.length && insertedSet.has(steps[i + 1].nodeKey)
    if (isAnchorApprove && !nextIsAnchorApprove && !nextIsInserted) {
      for (const key of insertedKeys) out.push({ source: 'system_default', action: 'approve', nodeKey: key, injected: true })
      injected += insertedKeys.length
    }
  }
  out.forEach((s, i) => { s.sequence = i + 1 })
  return { out, injected }
}

const planId = process.argv[2] || '21'
const pathId = process.argv[3] || '212'
const anchorKey = process.argv[4] || '7b9c5652562de0bb8c272c080676d488'
const insertedId = process.argv[5] || '3f353005c1674370b1b707c26711b0b1'

const response = await fetch(`http://127.0.0.1:19080/api/plans/${planId}/execution-paths/${pathId}/configuration/compiled-scenario`)
const envelope = await response.json()
const steps = envelope.data.compiledScenario

const before = steps.map((s) => ({ source: s.source, action: s.action, nodeKey: s.nodeKey, sequence: s.sequence }))
const { out, injected } = inject(before, anchorKey, [ADD_SIGN_PREFIX + insertedId])
console.log(`注入步骤数 = ${injected}`)
console.log('--- 注入后的步骤表（锚点附近） ---')
for (const s of out) {
  const near = s.nodeKey === anchorKey || String(s.nodeKey).includes('add-sign-inserted')
  if (!near) continue
  console.log(`  #${s.sequence} src=${s.source} action=${s.action} node=${s.nodeKey}${s.injected ? '  <== 注入' : ''}`)
}
