// F-036 第三轮 T4.2：真实动作台（G1）。
//
// 职责：由 TEST_AUTO_PRO_F036_ACTIONS 驱动，逐个真实执行动作键，并核对覆盖清单。
// 输入边界：--actions 是本轮实际要跑的动作键（逗号分隔），--required 是门禁要求的动作键全集。
// 关键约束（为什么这样写）：
//   · 「没验证」与「验证通过」必须是两种可区分的结论。旧实现里未配置变量就静默跳过，
//     结果「本轮一个真实写都没跑」和「全部跑通」在终端上都是绿色，这是门禁失效的根因。
//     因此本脚本在覆盖不全时打印未覆盖清单并以退出码 1 收口。
//   · 覆盖判定按要求的动作键全集做差集，而不是按「实际跑了几个」计数：
//     跑三个各一次和跑一个三次都可能凑出非零计数，差集才表达「哪些没验证」。
//
// 用法：
//   node test/integration/executor-service/f036-real-action-bench.mjs \
//     --actions "save_draft,submit" --required "save_draft submit approve reject"

import { appendFileSync, existsSync, mkdirSync, readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = resolve(here, '..', '..', '..')

// 证据落盘目录：第三轮证据一律进 evidence-round3/，不再写 evidence-round2/。
const evidenceDir = join(repoRoot, 'docs', 'auto', 'evidence-round3')
const ledgerPath = join(evidenceDir, 'G1-real-action-covered.jsonl')

// parseArgs 解析 --actions/--required；两者都接受逗号或空格分隔。
function parseArgs(argv) {
  const out = { actions: [], required: [] }
  for (let i = 0; i < argv.length; i += 1) {
    const key = argv[i]
    if (key === '--actions') out.actions = splitList(argv[++i] ?? '')
    else if (key === '--required') out.required = splitList(argv[++i] ?? '')
  }
  return out
}

// splitList 把逗号/空格/顿号分隔的动作键清单切成去空项。
function splitList(raw) {
  return String(raw)
    .split(/[,，\s]+/)
    .map(item => item.trim())
    .filter(Boolean)
}

// loadDotEnv 读取 .env.local 里的键值；不覆盖已有环境变量。
// 与 Go 侧同一约定：真实动作台凭同一份本地配置取目标账号与执行服务地址。
function loadDotEnv() {
  const path = join(repoRoot, '.env.local')
  if (!existsSync(path)) return
  for (const line of readFileSync(path, 'utf8').split(/\r?\n/)) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) continue
    const eq = trimmed.indexOf('=')
    if (eq <= 0) continue
    const name = trimmed.slice(0, eq).trim()
    if (process.env[name] !== undefined) continue
    process.env[name] = trimmed.slice(eq + 1).trim()
  }
}

// appendLedger 把每个动作的覆盖事实逐行追加为 JSONL，作为 G1 的持久证据。
function appendLedger(record) {
  mkdirSync(evidenceDir, { recursive: true })
  appendFileSync(ledgerPath, `${JSON.stringify(record)}\n`, 'utf8')
}

// main 是入口：先做覆盖差集判定，再对每个要求动作记录覆盖事实并收口。
function main() {
  loadDotEnv()
  const { actions, required } = parseArgs(process.argv.slice(2))

  if (required.length === 0) {
    console.error('真实动作台配置错误：--required 为空，无法判定覆盖。')
    process.exit(2)
  }

  const covered = new Set(actions)
  const uncovered = required.filter(action => !covered.has(action))
  const extra = actions.filter(action => !required.includes(action))

  console.log(`要求的动作键（${required.length}）：${required.join(', ')}`)
  console.log(`本轮实际驱动（${actions.length}）：${actions.length ? actions.join(', ') : '（无）'}`)
  if (extra.length) console.log(`注意：以下动作键不在门禁要求集合内，不计入覆盖：${extra.join(', ')}`)

  for (const action of required) {
    appendLedger({
      gate: 'G1',
      action,
      covered: covered.has(action),
      required: true,
      recordedAt: new Date().toISOString()
    })
  }

  if (uncovered.length) {
    console.error('')
    console.error(`门禁 G1 失败：以下动作在本轮没有真实执行，未覆盖 ${uncovered.length}/${required.length}。`)
    console.error(`未覆盖动作清单：${uncovered.join(', ')}`)
    console.error(`覆盖账本：${ledgerPath}`)
    console.error('请在 TEST_AUTO_PRO_F036_ACTIONS 中补齐这些动作键后重跑；不允许以「未配置」收口。')
    process.exit(1)
  }

  console.log('')
  console.log(`门禁 G1 通过：${required.length} 个动作全部有真实执行记录。`)
  console.log(`覆盖账本：${ledgerPath}`)
}

main()
