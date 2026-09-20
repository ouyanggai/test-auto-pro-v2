// 夜间自动化第三步：在页面上真实启动一次运行（串行或并发），并持续记录运行事实直到终态。
// 用法：node test/helpers/night-run.mjs <planId> --dispatch serial|parallel --concurrency 3 --mode auto|manual_control
// 输出：logs/night-state/runs.json 追加一条运行记录；标准输出持续打印进度。
import { mkdirSync, readFileSync, writeFileSync, existsSync } from 'node:fs'
import { Browser } from './browser-driver.mjs'
import { currentView, delay, evalPage } from './page-actions.mjs'
import { openPlanPaths, openRunDialog, readState, selectRunnablePaths, startRun } from './tool-ui.mjs'

const planId = process.argv[2]
if (!planId) {
  console.error('用法：node test/helpers/night-run.mjs <planId> [--dispatch serial] [--concurrency 3] [--mode auto]')
  process.exit(2)
}
const flag = (name, fallback) => {
  const index = process.argv.indexOf(`--${name}`)
  return index >= 0 ? process.argv[index + 1] : fallback
}
const dispatch = flag('dispatch', 'serial')
const concurrency = Number(flag('concurrency', '3'))
const mode = flag('mode', 'auto')
const timeoutMinutes = Number(flag('timeout-minutes', '90'))

const STATE_DIR = 'logs/night-state'
const STATE_FILE = `${STATE_DIR}/runs.json`

// readRunFacts 通过运行详情接口读取本次运行每条路径的真实状态与步数（只读）。
async function readRunFacts (browser, runId) {
  return await evalPage(browser, `
    const response = await fetch('/api/runs/${runId}/paths', { method: 'GET' });
    const envelope = await response.json();
    const data = envelope.data || {};
    const paths = data.paths || data.items || [];
    return {
      finished: paths.length > 0 && paths.every((p) => ['completed', 'failed', 'stopped', 'cancelled'].includes(p.status)),
      paths: paths.map((p) => ({
        pathRunId: p.pathRunId, pathId: p.pathId, pathName: p.pathName,
        status: p.status, statusName: p.statusName, resultName: p.resultName,
        executedSteps: p.executedSteps, skippedSteps: p.skippedSteps,
        totalSteps: p.totalSteps, stopStepNo: p.stopStepNo,
      })),
    };
  `)
}

const browser = await Browser.launch({ headless: true })
try {
  await openPlanPaths(browser, planId)
  await openRunDialog(browser, planId)
  const selection = await selectRunnablePaths(browser, planId)
  console.log(`可运行路径 ${selection.runnableCount} 条，本次勾选 ${selection.checkedCount} 条`)
  if (selection.blocked.length) {
    console.log(`被阻塞路径：${JSON.stringify(selection.blocked)}`)
  }
  await openRunDialog(browser, planId)
  const runId = await startRun(browser, { mode, dispatch, concurrency })
  console.log(`运行已启动 runId=${runId}`)

  const deadline = Date.now() + timeoutMinutes * 60 * 1000
  let facts = { paths: [], finished: false }
  let unchanged = 0
  let previous = ''
  while (Date.now() < deadline) {
    await delay(15000)
    facts = await readRunFacts(browser, runId)
    const snapshot = JSON.stringify(facts.paths.map((p) => `${p.pathName}:${p.status}`))
    unchanged = snapshot === previous ? unchanged + 1 : 0
    previous = snapshot
    const done = facts.paths.filter((p) => ['completed', 'failed', 'stopped', 'cancelled'].includes(p.status)).length
    console.log(`[${new Date().toLocaleTimeString('zh-CN')}] 终态 ${done}/${facts.paths.length}`)
    if (facts.finished) break
    // 自动模式下长时间无进展：读一次页面上的当前状态，便于事后定位卡点
    if (unchanged > 0 && unchanged % 8 === 0) {
      const view = await currentView(browser)
      console.log(`（长时间无进展，页面状态：${view.slice(0, 400)}）`)
    }
  }

  const record = {
    planId, runId, dispatch, concurrency, mode,
    startedAt: new Date(Date.now() - (Date.now() - (deadline - timeoutMinutes * 60 * 1000))).toISOString(),
    recordedAt: new Date().toISOString(),
    finished: facts.finished,
    paths: facts.paths,
  }
  mkdirSync(STATE_DIR, { recursive: true })
  const records = existsSync(STATE_FILE) ? JSON.parse(readFileSync(STATE_FILE, 'utf8')) : []
  records.push(record)
  writeFileSync(STATE_FILE, JSON.stringify(records, null, 2), 'utf8')
  if (runId) await browser.screenshot(`logs/night-evidence-3/运行_${planId}_${runId}_终局.png`)
  console.log(JSON.stringify(record, null, 2))
  process.exitCode = facts.finished ? 0 : 3
} finally {
  browser.close()
}
