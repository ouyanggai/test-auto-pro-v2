// 诊断脚本：核对转发行到底出现在目标的哪个列表页（dueout / backlog），
// 定位「转发装载等待 tableData 命中实例」超时的真实原因。只读：不点击转发、不发写请求。
import { Browser } from './browser-driver.mjs'
import { delay, evalPage } from './page-actions.mjs'

const instanceId = process.argv[2]
if (!instanceId) {
  console.error('用法：node test/helpers/diagnose-forward-tab.mjs <instanceId>')
  process.exit(2)
}
const browser = await Browser.launch({ headless: true })
try {
  for (const tab of ['dueout', 'backlog']) {
    await browser.goto(`http://192.168.1.220:38081/#/flow/approve/${tab}?tab=${tab}`)
    await delay(6000)
    const state = await evalPage(browser, `
      const queue = [];
      const root = window.$vue;
      if (root) queue.push(root);
      let found = null;
      let tableCount = 0;
      const seen = new Set();
      while (queue.length > 0) {
        const node = queue.shift();
        if (!node || seen.has(node)) continue;
        seen.add(node);
        for (const key of Object.keys(node.$refs || {})) {
          const child = node.$refs[key];
          if (child && Array.isArray(child.tableData)) {
            tableCount++;
            const hit = child.tableData.find((row) => String(row.flowInstanceId ?? row.id ?? '') === ${JSON.stringify(instanceId)});
            if (hit && !found) found = { ref: key, jobTaskId: String(hit.jobTaskId ?? ''), title: String(hit.title ?? hit.flowName ?? ''), rowCount: child.tableData.length };
          }
        }
        queue.push(...(node.$children || []));
      }
      return {
        vueRoot: !!root,
        tableCount,
        found,
        bodyHint: textOf(document.body).slice(0, 200),
      };
    `)
    console.log(`tab=${tab}`, JSON.stringify(state))
  }
} finally {
  await browser.close()
}
