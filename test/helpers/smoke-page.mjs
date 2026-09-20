// 冒烟：在新建计划页验证账号并读取流程模板列表，确认能真实选中目标流程。
import { Browser } from './browser-driver.mjs'
import { clickByText, currentView, delay, evalPage, fillByPlaceholder, waitFor } from './page-actions.mjs'

const ACCOUNT = process.env.TEST_AUTO_PRO_TARGET_ACCOUNT || '骆蒙恩'
const FLOW_NAME = process.argv[2] || '员工请假单（智慧斯能）'

const browser = await Browser.launch({ headless: true })
await browser.goto('http://127.0.0.1:19000/plans/new')
await fillByPlaceholder(browser, '例如：', '冒烟计划_勿用', { scope: 'document.body' })
await fillByPlaceholder(browser, '请输入账号', ACCOUNT, { scope: 'document.body' })
await clickByText(browser, '验证账号', { scope: 'document.body', tag: '.n-button' })
await waitFor(browser, 'return document.body.innerText.includes("账号已验证")', { label: '账号验证通过', timeoutMs: 60000 })
console.log('账号状态 =', await evalPage(browser, 'return textOf($$(".account-status").find(visible))'))
await fillByPlaceholder(browser, '搜索流程模板名称', FLOW_NAME, { scope: 'document.body' })
await delay(1500)
const rows = await evalPage(browser, `
  return $$('.candidate-row').filter(visible).map((el) => {
    const title = el.querySelector('strong');
    const spans = Array.from(el.querySelectorAll('span')).map(textOf);
    return { title: title ? textOf(title) : '', parts: spans };
  });
`)
console.log('候选行 =', JSON.stringify(rows, null, 1))
await clickByText(browser, FLOW_NAME, { scope: 'document.body', tag: '.candidate-row', exact: false })
console.log('选中后 =', JSON.stringify(await currentView(browser)))
await browser.screenshot('logs/night-evidence-3/冒烟_选中流程.png')
browser.close()
