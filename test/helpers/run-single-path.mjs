// 一次性验证脚本：只勾选指定序号的路径并真实启动一次自动运行，然后等待终态。
// 用于定向验证「加签节点显示审批人姓名」这类需要真实执行加签动作的修复。
// 用法：node test/helpers/run-single-path.mjs <planId> <sequenceNo> [--mode auto] [--timeout-minutes 40]
import { Browser } from './browser-driver.mjs'
import { delay, evalPage } from './page-actions.mjs'
import { openRunDialog, setAllRunPathSelections, startRun, toggleRunPathSelection } from './tool-ui.mjs'

const planId = process.argv[2]
const sequence = process.argv[3]
if (!planId || !sequence) {
  console.error('用法：node test/helpers/run-single-path.mjs <planId> <sequenceNo> [--mode auto] [--timeout-minutes 40]')
  process.exit(2)
}
const flag = (name, fallback) => {
  const index = process.argv.indexOf(`--${name}`)
  return index >= 0 ? process.argv[index + 1] : fallback
}
const mode = flag('mode', 'auto')
const timeoutMinutes = Number(flag('timeout-minutes', '40'))

// readRunFacts 读取本次运行每条路径的真实状态（只读接口，不在页面上猜测）。
async function readRunFacts (browser, runId) {
  return await evalPage(browser, `
    const response = await fetch('/api/runs/${runId}/paths', { method: 'GET' });
    const envelope = await response.json();
    const data = envelope.data || {};
    const paths = data.paths || data.items || [];
    return {
      finished: paths.length > 0 && paths.every((p) => ['completed', 'failed', 'stopped', 'cancelled'].includes(p.status)),
      paths: paths.map((p) => ({ pathRunId: p.pathRunId, pathName: p.pathName, status: p.status, statusName: p.statusName, stopStepNo: p.stopStepNo })),
    };
  `)
}

const browser = await Browser.launch({ headless: true })
try {
  await openRunDialog(browser, planId)
  await setAllRunPathSelections(browser, false)
  // toggleRunPathSelection 成功时返回布尔 true；失败会自己抛错，这里只做结果确认。
  const toggled = await toggleRunPathSelection(browser, Number(sequence), true)
  if (toggled !== true) throw new Error(`勾选路径 #${sequence} 未生效：${JSON.stringify(toggled)}`)
  console.log(`已勾选路径 #${sequence}`)

  const runId = await startRun(browser, { mode, dispatch: 'serial', concurrency: 1 })
  console.log(`运行已启动 runId=${runId}`)

  const deadline = Date.now() + timeoutMinutes * 60 * 1000
  let facts = { paths: [], finished: false }
  while (Date.now() < deadline) {
    await delay(15000)
    facts = await readRunFacts(browser, runId)
    const done = facts.paths.filter((p) => ['completed', 'failed', 'stopped', 'cancelled'].includes(p.status)).length
    console.log(`[${new Date().toLocaleTimeString('zh-CN')}] 终态 ${done}/${facts.paths.length} ${facts.paths.map((p) => `${p.pathName}:${p.statusName}`).join(', ')}`)
    if (facts.finished) break
  }
  console.log(JSON.stringify({ planId, runId, finished: facts.finished, paths: facts.paths }, null, 2))
  process.exitCode = facts.finished ? 0 : 3
} finally {
  await browser.close()
}
