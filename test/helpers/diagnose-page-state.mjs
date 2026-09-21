// 诊断脚本：打开执行服务自带原生页面，记录其真实运行时状态（是否挂载 $vue、正文内容）。
// 只读：不发写请求。
import { Browser } from './browser-driver.mjs'
import { delay, evalPage } from './page-actions.mjs'

const pageOrigin = (process.argv[2] || 'http://127.0.0.1:56857').replace(/\/$/, '')
const tab = process.argv[3] || 'backlog'

const browser = await Browser.launch({ headless: true })
try {
  await browser.goto(`${pageOrigin}/#/flow/approve/${tab}?tab=${tab}`)
  await delay(8000)
  const state = await evalPage(browser, `
    return {
      title: document.title,
      hasVueRoot: !!window.$vue,
      url: location.href,
      body: (document.body ? document.body.innerText : '').slice(0, 300),
      appHtml: document.querySelector('#app') ? document.querySelector('#app').innerHTML.slice(0, 300) : 'NO_APP',
      executorNative: !!window.__executorNative,
    };
  `)
  console.log(JSON.stringify(state, null, 2))
  await browser.screenshot('logs/matrix/forward-page.png')
} finally {
  await browser.close()
}
