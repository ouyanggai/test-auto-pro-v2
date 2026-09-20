// 一次性运维脚本：用真实页面操作停掉指定运行（夜间脚本重启后端后遗留的活动运行）。
// 背景：POST /api/runs/{id}/stop 在自动模式下返回 409；运行必须经界面上的真实按钮停止，
// 这符合任务书「真实点击页面」的要求，也避免直接改数据库。
// 用法：node test/helpers/stop-run-via-ui.mjs <runId>
import { Browser } from './browser-driver.mjs'
import { evalPage, clickByText, listVisibleTexts } from './page-actions.mjs'
import { TOOL_ORIGIN } from './tool-ui.mjs'

const runId = process.argv[2]
if (!runId) {
  console.error('用法：node test/helpers/stop-run-via-ui.mjs <runId>')
  process.exit(2)
}

const browser = await Browser.launch({ headless: true })
try {
  await browser.goto(`${TOOL_ORIGIN}/runs/${runId}`)
  await new Promise((resolve) => setTimeout(resolve, 5000))
  console.log('页面文本：', (await listVisibleTexts(browser)).slice(0, 40).join(' / '))

  // 运行详情页的「停止」按钮文案在不同状态下可能是「停止」「停止运行」「终止运行」。
  for (const label of ['停止运行', '停止', '终止运行', '取消运行']) {
    const clicked = await clickByText(browser, label, { tag: '.n-button', exact: true })
    if (clicked) {
      console.log(`已点击「${label}」`)
      break
    }
  }
  await new Promise((resolve) => setTimeout(resolve, 2000))

  // 停止通常有二次确认弹窗，确认按钮文案「确定」「确认停止」等。
  for (const label of ['确定', '确认', '确认停止', '停止运行']) {
    const clicked = await clickByText(browser, label, { tag: '.n-button', exact: true })
    if (clicked) {
      console.log(`已点击确认「${label}」`)
      break
    }
  }
  await new Promise((resolve) => setTimeout(resolve, 3000))

  const after = await evalPage(browser, `
    const response = await fetch('/api/runs/${runId}', { method: 'GET' });
    const envelope = await response.json();
    return { statusName: (envelope.data || {}).runStatusName };
  `)
  console.log('停止后运行状态：', JSON.stringify(after))
} finally {
  await browser.close()
}
