// 夜间自动化第一步：真实打开工具前端，以计划账号新建一个计划，供后续路径与运行阶段复用。
// 用法：node test/helpers/night-create-plan.mjs "员工请假单（智慧斯能）"
// 输出：logs/night-state/plans.json 追加一条 {name, planId, flowName, account, createdAt}
import { mkdirSync, readFileSync, writeFileSync, existsSync } from 'node:fs'
import { Browser } from './browser-driver.mjs'
import { createPlan, readState } from './tool-ui.mjs'

const ACCOUNT = process.env.TEST_AUTO_PRO_TARGET_ACCOUNT || '骆蒙恩'
const STATE_DIR = 'logs/night-state'
const STATE_FILE = `${STATE_DIR}/plans.json`

const flowName = process.argv[2]
if (!flowName) {
  console.error('用法：node test/helpers/night-create-plan.mjs "<流程模板名称>"')
  process.exit(2)
}

// 计划名称按任务书命名规范：夜测_[流程名]_[时间戳]
const stamp = new Date().toISOString().replace(/[-:T]/g, '').slice(0, 14)
const name = `夜测_${flowName}_${stamp}`

const browser = await Browser.launch({ headless: true })
try {
  const planId = await createPlan(browser, { name, account: ACCOUNT, flowName })
  const state = await readState(browser, planId)
  mkdirSync(STATE_DIR, { recursive: true })
  const records = existsSync(STATE_FILE) ? JSON.parse(readFileSync(STATE_FILE, 'utf8')) : []
  records.push({ name, planId, flowName, account: ACCOUNT, createdAt: new Date().toISOString(), paths: state.paths })
  writeFileSync(STATE_FILE, JSON.stringify(records, null, 2), 'utf8')
  console.log(JSON.stringify({ ok: true, name, planId, flowName, account: ACCOUNT }, null, 2))
} finally {
  browser.close()
}
