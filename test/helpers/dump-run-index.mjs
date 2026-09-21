// 只读诊断：列出各运行是否仍有可读数据，便于挑选可用于排查的样本。
// 用法：node test/helpers/dump-run-index.mjs [起始id] [结束id]
const from = Number(process.argv[2] || '50')
const to = Number(process.argv[3] || '60')
for (let id = from; id <= to; id += 1) {
  try {
    const response = await fetch(`http://127.0.0.1:19080/api/runs/${id}/paths`)
    if (!response.ok) continue
    const body = await response.json()
    const paths = (body.data || {}).paths || []
    if (!paths.length) {
      console.log(`run ${id}: 无路径（已清理或不存在）`)
      continue
    }
    const done = paths.filter((p) => p.statusName === '已完成').length
    const failed = paths.filter((p) => p.statusName === '失败').length
    console.log(`run ${id}: ${paths.length} 条路径，已完成 ${done}，失败 ${failed}`)
  } catch (error) {
    console.log(`run ${id}: 读取失败 ${error.message}`)
  }
}
