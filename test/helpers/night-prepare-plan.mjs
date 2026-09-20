// 夜间自动化第二步：在已建计划上生成执行路径、按序号截断到上限、做一键配置（随机动作 + 历史数据注入）。
// 用法：node test/helpers/night-prepare-plan.mjs <planId> [--max-paths 12] [--mode random|approve_only] [--history 关键词]
import { Browser } from './browser-driver.mjs'
import { configureAllPaths, generatePaths, keepFirstPaths, openPlanPaths, readState } from './tool-ui.mjs'

const planId = process.argv[2]
if (!planId) {
  console.error('用法：node test/helpers/night-prepare-plan.mjs <planId> [--max-paths 12] [--mode random] [--history 关键词]')
  process.exit(2)
}
const flag = (name, fallback) => {
  const index = process.argv.indexOf(`--${name}`)
  return index >= 0 ? process.argv[index + 1] : fallback
}
const maxPaths = Number(flag('max-paths', '12'))
const mode = flag('mode', 'random')
const history = flag('history', '')

const browser = await Browser.launch({ headless: true })
try {
  await openPlanPaths(browser, planId)
  const before = await readState(browser, planId)
  console.log(`起始路径数 = ${before.paths.length}`)
  if (!before.paths.length) {
    const count = await generatePaths(browser, planId)
    console.log(`生成路径数 = ${count}`)
  }
  const trimmed = await keepFirstPaths(browser, planId, maxPaths)
  console.log(`截断结果 = ${JSON.stringify(trimmed)}`)
  const configured = await configureAllPaths(browser, planId, { actionMode: mode, historyKeyword: history })
  console.log(`一键配置 = ${JSON.stringify(configured)}`)
  const after = await readState(browser, planId)
  const summary = {
    planId,
    pathCount: after.paths.length,
    configured: after.paths.filter((p) => p.configurationStatus === 'configured').length,
    ready: after.paths.filter((p) => p.dataStatus === 'ready').length,
    paths: after.paths,
  }
  console.log(JSON.stringify(summary, null, 2))
} finally {
  browser.close()
}
