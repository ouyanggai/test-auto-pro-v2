// 诊断脚本：打开路径页 → 点「运行」→ 记录运行前检查弹窗的真实状态，定位检查卡住的位置。
// 只读：不启动运行、不发写请求。
import { Browser } from './browser-driver.mjs'
import { delay, evalPage } from './page-actions.mjs'

const planId = process.argv[2] || '21'
const browser = await Browser.launch({ headless: true })
try {
  await browser.goto(`http://127.0.0.1:19000/plans/${planId}/paths`)
  await delay(4000)
  // 记录拦截到的接口调用，确认前端到底请求了哪些只读接口、返回什么。
  await evalPage(browser, `
    window.__probe = [];
    const originalFetch = window.fetch;
    window.fetch = async (...args) => {
      const url = typeof args[0] === 'string' ? args[0] : args[0]?.url;
      const started = Date.now();
      try {
        const res = await originalFetch(...args);
        window.__probe.push({ url, status: res.status, ms: Date.now() - started });
        return res;
      } catch (error) {
        window.__probe.push({ url, error: String(error), ms: Date.now() - started });
        throw error;
      }
    };
    return true;
  `)
  const clicked = await evalPage(browser, `
    const button = $('[data-testid="plan-detail-run-button"]');
    if (!button) return false;
    fire(button);
    return true;
  `)
  console.log('clicked run button =', clicked)
  for (let i = 0; i < 10; i += 1) {
    await delay(3000)
    const state = await evalPage(browser, `
      return {
        hasSummary: !!$('[data-testid="run-preflight-summary"]'),
        modalText: textOf(document.querySelector('.n-modal') || document.body).slice(0, 400),
        probe: (window.__probe || []).filter((p) => String(p.url).includes('/api/')),
      };
    `)
    console.log(`--- t=${(i + 1) * 3}s hasSummary=${state.hasSummary}`)
    console.log('    api:', JSON.stringify(state.probe))
    if (state.hasSummary) { console.log('    modal:', state.modalText); break }
  }
  const finalState = await evalPage(browser, `
    return { bodyHasChecking: textOf(document.body).includes('稍等片刻'), modal: textOf(document.querySelector('.n-modal') || document.body).slice(0, 600) };
  `)
  console.log('final modal text:', finalState.modal)
  await browser.screenshot('logs/matrix/preflight-diagnose.png')
} finally {
  browser.close()
}
