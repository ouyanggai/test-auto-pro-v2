// 一次性诊断脚本：抓取「路径页」加载失败时的前端真实报错。
// 背景：后端重启后浏览器直接访问 /plans/17/paths 稳定显示「计划 ID 不正确」，
// 但同一计划经 /api/plans/17 读取正常，需要区分是前端路由参数问题还是接口失败。
// 用法：node test/helpers/inspect-plan-load.mjs
import { Browser } from './browser-driver.mjs'
import { evalPage } from './page-actions.mjs'

const browser = await Browser.launch({ headless: true })
try {
  // 先注入失败请求收集器（必须用表达式形式，浏览器端不接受裸 return 语句）。
  await browser.eval(`
    (() => {
      window.__apiFailures = [];
      const originalFetch = window.fetch;
      window.fetch = async (...args) => {
        const response = await originalFetch(...args);
        if (!response.ok) window.__apiFailures.push({ url: String(args[0]), status: response.status });
        return response;
      };
      return true;
    })()
  `)

  await browser.goto('http://127.0.0.1:19000/plans/17/paths')
  await new Promise((resolve) => setTimeout(resolve, 8000))

  const snapshot = await evalPage(browser, `
    return {
      url: location.href,
      bodyText: document.body.innerText.slice(0, 500),
      apiFailures: window.__apiFailures || []
    };
  `)
  console.log(JSON.stringify(snapshot, null, 1))

  // 从页面上下文调同一个只读接口，确认后端是否真的可用。
  const probe = await evalPage(browser, `
    const response = await fetch('/api/plans/17', { method: 'GET' });
    const envelope = await response.json();
    return { status: response.status, name: (envelope.data || {}).name || null, error: envelope.error || null };
  `)
  console.log('直接调用 /api/plans/17 →', JSON.stringify(probe))
} finally {
  await browser.close()
}
