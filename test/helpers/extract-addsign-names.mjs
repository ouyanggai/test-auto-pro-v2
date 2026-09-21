// 只读诊断：从指定 curl.log 的 updateFlowProxy 写入请求里，抽出加签人员明细的 name 字段，
// 判断姓名到底有没有随写请求落盘。用法：node test/helpers/extract-addsign-names.mjs <curlLogPath>
import { readFileSync } from 'node:fs'

const file = process.argv[2]
if (!file) {
  console.error('用法：node test/helpers/extract-addsign-names.mjs <curlLogPath>')
  process.exit(2)
}

const content = readFileSync(file, 'utf8')
const blockStart = content.indexOf('updateFlowProxy')
if (blockStart < 0) {
  console.log('未找到 updateFlowProxy 请求')
  process.exit(0)
}
// 取该请求之后足够长的片段（整行即整个请求体）。
const lineEnd = content.indexOf('\n', blockStart)
const block = content.slice(blockStart, lineEnd < 0 ? undefined : lineEnd)
console.log(`请求体长度：${block.length}`)

// 抽出所有人员明细里的 bizId / name 成对出现的位置。
const detailIdx = block.indexOf('flowNodeDetailConfigList')
console.log(`flowNodeDetailConfigList 首次出现位置：${detailIdx}`)

// 逐个 nodeName 与 personnel 明细配对输出，便于确认加签节点是否带姓名。
const nodeNameMatches = [...block.matchAll(/"nodeName"\s*:\s*"([^"]*)"/g)].map((m) => m[1])
console.log(`nodeName 共 ${nodeNameMatches.length} 个：${JSON.stringify(nodeNameMatches.slice(0, 20))}`)

const names = [...block.matchAll(/"name"\s*:\s*"([^"]*)"/g)].map((m) => m[1])
const chinese = [...new Set(names.filter((n) => /[\u4e00-\u9fa5]/.test(n)))]
console.log(`name 字段共 ${names.length} 个（去重）${new Set(names).size} 个，其中中文名 ${chinese.length} 个`)
console.log(`中文名样本：${JSON.stringify(chinese.slice(0, 20))}`)

// 直接看加签节点的审批配置片段：定位 assign 之后的一段。
const assignIdx = block.indexOf('"auditType":"assign"')
if (assignIdx >= 0) {
  const slice = block.slice(Math.max(0, assignIdx - 400), assignIdx + 900)
  console.log(`\n=== 加签节点片段（auditType=assign 附近）===`)
  console.log(slice)
} else {
  console.log('\n未找到 auditType=assign 片段')
  const idx2 = block.indexOf('flowNodeDetailConfigList')
  if (idx2 >= 0) console.log(block.slice(idx2, idx2 + 900))
}
