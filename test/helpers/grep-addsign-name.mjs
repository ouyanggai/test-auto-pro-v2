// 只读诊断：在运行日志里查找「加签插入节点」的审批配置原始响应，判断目标是否返回了中文姓名。
// 用法：node test/helpers/grep-addsign-name.mjs [运行目录名的序号，默认最后一个]
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

const runsRoot = 'logs/runs'
const runDirs = readdirSync(runsRoot).filter((name) => statSync(join(runsRoot, name)).isDirectory())
console.log(`运行目录共 ${runDirs.length} 个，检查最近 3 个：`)
const targets = runDirs.slice(-3)
for (const runDir of targets) {
  console.log(`\n===== ${runDir} =====`)
  const pathsRoot = join(runsRoot, runDir, 'paths')
  let pathDirs = []
  try {
    pathDirs = readdirSync(pathsRoot).filter((name) => statSync(join(pathsRoot, name)).isDirectory())
  } catch {
    console.log('  （无 paths 目录）')
    continue
  }
  for (const pathDir of pathDirs) {
    const curlPath = join(pathsRoot, pathDir, 'curl.log')
    let content = ''
    try {
      content = readFileSync(curlPath, 'utf8')
    } catch {
      continue
    }
    // 目标代理树响应里带 flowNodeDetailConfigList 的片段，检查其 name 字段是否有中文。
    const lines = content.split(/\r?\n/).filter((line) => line.includes('flowNodeDetailConfigList'))
    if (!lines.length) continue
    console.log(`  ${pathDir}: 含 flowNodeDetailConfigList 的行 ${lines.length} 条`)
    for (const line of lines.slice(0, 2)) {
      // 抽取 personnel 明细的 name 字段，判断是否为中文姓名。
      const names = [...line.matchAll(/"name"\s*:\s*"([^"]*)"/g)].map((m) => m[1])
      const chinese = names.filter((n) => /[\u4e00-\u9fa5]/.test(n))
      console.log(`    name 字段 ${names.length} 个，中文 ${chinese.length} 个：${JSON.stringify(chinese.slice(0, 8))}`)
    }
  }
}
