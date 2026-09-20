// 只读诊断：列出指定运行里所有非终态路径，用于判断队列被谁占用。
// 用法：node test/helpers/dump-run-active.mjs <runId>
const runId = process.argv[2]
if (!runId) {
  console.error('用法：node test/helpers/dump-run-active.mjs <runId>')
  process.exit(2)
}
const envelope = await (await fetch(`http://127.0.0.1:19080/api/runs/${runId}/paths`)).json()
const paths = (envelope.data || {}).paths || []
console.log(`run ${runId} 共 ${paths.length} 条路径：`)
for (const p of paths) {
  const mark = ['已完成', '失败', '已停止', '已取消'].includes(p.statusName) ? ' ' : '*'
  console.log(`  ${mark} ${p.pathName} | ${p.statusName} | ${p.executedSteps}/${p.totalSteps}`)
}
