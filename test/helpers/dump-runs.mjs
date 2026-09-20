// 只读诊断：汇总当前所有运行的状态，便于判断队列里是谁占用账号锁。
// 用法：node test/helpers/dump-runs.mjs [planId]
// 注意：/api/runs/{id}/paths 返回的行只有 statusName（中文名），没有 status 编码字段。
const ids = []
for (let id = 50; id <= 60; id += 1) ids.push(id)
for (const id of ids) {
  try {
    const response = await fetch(`http://127.0.0.1:19080/api/runs/${id}/paths`)
    if (!response.ok) continue
    const body = await response.json()
    const paths = (body.data || {}).paths || []
    if (!paths.length) continue
    const group = (names) => paths.filter((p) => names.includes(p.statusName)).length
    const terminal = group(['已完成', '失败', '已停止', '已取消'])
    const running = group(['运行中'])
    const waiting = group(['等待运行'])
    console.log(`run ${id}: 终态 ${terminal}/${paths.length} 运行中 ${running} 等待 ${waiting}`)
  } catch {
    // 忽略不存在的运行
  }
}
