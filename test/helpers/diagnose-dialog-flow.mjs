// 诊断脚本：严格按 run-single-path.mjs 的顺序走一遍「打开运行弹窗」，定位等待超时的真实原因。
// 只读：不点击「开始运行」，不创建运行。
import { Browser } from './browser-driver.mjs'
import { delay, evalPage } from './page-actions.mjs'
import { openRunDialog, openPlanPaths, setAllRunPathSelections, toggleRunPathSelection } from './tool-ui.mjs'

const planId = process.argv[2] || '21'
const sequence = Number(process.argv[3] || '1')
const browser = await Browser.launch({ headless: true })
try {
  console.log('step1: openRunDialog')
  const t0 = Date.now()
  await openRunDialog(browser, planId)
  console.log(`  ok in ${Date.now() - t0}ms`)

  console.log('step2: setAllRunPathSelections(false)')
  await setAllRunPathSelections(browser, false)
  console.log('  ok')

  console.log(`step3: toggleRunPathSelection(${sequence}, true)`)
  const toggled = await toggleRunPathSelection(browser, sequence, true)
  console.log('  result =', JSON.stringify(toggled))

  console.log('step4: probe dialog state without reopening')
  const state = await evalPage(browser, `
    return {
      hasSummary: !!$('[data-testid="run-preflight-summary"]'),
      dialogOpen: !!document.querySelector('.n-modal'),
      bodyHasCheckingHint: textOf(document.body).includes('稍等片刻'),
      selectedText: (textOf(document.body).match(/已选\\s*\\d+\\s*条/) || [''])[0],
    };
  `)
  console.log('  ', JSON.stringify(state))
} catch (error) {
  console.log('FAILED:', error.message.slice(0, 600))
} finally {
  await browser.close()
}
