// 只读诊断：在最新一次运行的 curl.log 中定位「加签」写入请求（updateFlowProxy）的请求体，
// 直接检查加签人员明细里到底有没有中文姓名。用法：node test/helpers/dump-addsign-body.mjs [runDirName]
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

const runsRoot = 'logs/runs'
const dirs = readdirSync(runsRoot).filter((name) => statSync(join(runsRoot, name)).isDirectory())
const runDir = process.argv[2] || dirs.sort((a, b) => statSync(join(runsRoot, b)).mtimeMs - statSync(join(runsRoot, a)).mtimeMs)[0]
console.log(`运行目录：${runDir}`)

const pathsRoot = join(runsRoot, runDir, 'paths')
const pathDirs = readdirSync(pathsRoot).filter((name) => statSync(join(pathsRoot, name)).isDirectory())
for (const pathDir of pathDirs) {
  const curlPath = join(pathsRoot, pathDir, 'curl.log')
  let content = ''
  try {
    content = readFileSync(curlPath, 'utf8')
  } catch {
    continue
  }
  console.log(`\n===== ${pathDir} =====`)
  const blocks = content.split(/^--- begin curl /m)
  for (const block of blocks) {
    if (!block.includes('updateFlowProxy')) continue
    const head = block.split(/\r?\n/)[0]
    console.log(`  ${head.slice(0, 160)}`)
    // 定位加签人员明细里的 name 字段
    const detailIdx = block.indexOf('flowNodeDetailConfigList')
    if (detailIdx < 0) {
      console.log('    （该请求体不含 flowNodeDetailConfigList）')
      continue
    }
    const slice = block.slice(detailIdx, detailIdx + 700)
    console.log(`    明细片段：${slice.replace(/\s+/g, ' ').slice(0, 400)}`)
    const names = [...slice.matchAll(/"name"\s*:\s*"([^"]*)"/g)].map((m) => m[1])
    console.log(`    name 字段：${JSON.stringify(names)}`)
    const nodeName = block.match(/"nodeName"\s*:\s*"([^"]*)"/)
    console.log(`    首个 nodeName：${nodeName ? nodeName[1] : '(未找到)'}`)
  }
}
