// 诊断脚本：直连执行服务自带的原生 /flow 页面，核对每个列表标签页上
// 「转发」入口与实例行是否存在，定位转发装载超时的真实原因。只读：不点转发、不发写请求。
// 用法：node test/helpers/diagnose-forward-tabs.mjs <pageOrigin> <instanceId>
import { Browser } from './browser-driver.mjs'
import { delay, evalPage } from './page-actions.mjs'

const pageOrigin = (process.argv[2] || '').replace(/\/$/, '')
const instanceId = process.argv[3] || ''
if (!pageOrigin || !instanceId) {
  console.error('用法：node test/helpers/diagnose-forward-tabs.mjs <pageOrigin> <instanceId>')
  process.exit(2)
}

const browser = await Browser.launch({ headless: true })
try {
  for (const tab of ['backlog', 'dueout', 'submitted', 'finished']) {
    await browser.goto(`${pageOrigin}#/flow/approve/${tab}?tab=${tab}`)
    await delay(7000)
    const state = await evalPage(browser, `
      const queue = [];
      const root = window.$vue;
      if (root) queue.push(root);
      const seen = new Set();
      let refHit = null;
      let rowHit = null;
      while (queue.length > 0) {
        const node = queue.shift();
        if (!node || seen.has(node)) continue;
        seen.add(node);
        for (const key of Object.keys(node.$refs || {})) {
          const child = node.$refs[key];
          if (child && Array.isArray(child.tableData)) {
            if (key === ${JSON.stringify(tab)}) {
              refHit = { rows: child.tableData.length, hasTranspond: typeof child.transpond === 'function' };
              const row = child.tableData.find((r) => String(r.flowInstanceId ?? r.id ?? '') === ${JSON.stringify(instanceId)});
              if (row) rowHit = { jobTaskId: String(row.jobTaskId ?? ''), title: String(row.title ?? '') };
            }
          }
        }
        queue.push(...(node.$children || []));
      }
      return { vueRoot: !!root, refHit, rowHit };
    `)
    console.log(`tab=${tab}`, JSON.stringify(state))
  }
} finally {
  await browser.close()
}
