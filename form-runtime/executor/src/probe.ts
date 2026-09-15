// T002 探针：验证无头 Chromium 可以启动并加载本项目 runtime 固定路由。
// 只做四件事：启动浏览器、提供静态构建产物、打开 /form-runtime/index.html#/test-auto-form、
// 读取页面真实的 runtime 版本；不做任何界面操作，也不访问目标平台。
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { launchHeadlessChromium } from './browser.ts'
import { serveDirectory } from './static-origin.ts'

// resolveDistRoot 返回 web 构建产物根目录：form-runtime 的 publicPath 固定为 /form-runtime/，
// 因此静态源根目录是其上级 web/dist；允许用 FORM_RUNTIME_DIST_ROOT 覆盖。
function resolveDistRoot(): string {
  if (process.env.FORM_RUNTIME_DIST_ROOT) return path.resolve(process.env.FORM_RUNTIME_DIST_ROOT)
  const here = path.dirname(fileURLToPath(import.meta.url))
  return path.resolve(here, '../../../web/dist')
}

async function main(): Promise<void> {
  const distRoot = resolveDistRoot()
  const entryFile = path.join(distRoot, 'form-runtime', 'index.html')
  if (!fs.existsSync(entryFile)) {
    console.error(JSON.stringify({ ok: false, reason: `缺少 form-runtime 构建产物：${entryFile}`, hint: '先执行 pnpm --dir form-runtime build' }))
    process.exitCode = 1
    return
  }
  const origin = await serveDirectory(distRoot)
  const browser = await launchHeadlessChromium()
  const pageErrors: string[] = []
  try {
    const context = await browser.newContext()
    const page = await context.newPage()
    page.on('pageerror', (error: Error) => { pageErrors.push(String(error?.message ?? error)) })
    const entryURL = new URL('form-runtime/index.html#/test-auto-form', origin.url).toString()
    await page.goto(entryURL, { waitUntil: 'load' })
    // 等待真实 runtime 代码执行：版本全局由 form-runtime/src/main.js 在真实入口脚本里写入。
    await page.waitForFunction('window.__formRuntimeVersion__ !== undefined', undefined, { timeout: 30000 })
    // 等待隔离路由生效：main.js 会把真实宿主路由替换到固定执行路由。
    await page.waitForFunction(
      "Boolean(window.$router) && window.$router.currentRoute && window.$router.currentRoute.path === '/test-auto-form'",
      undefined,
      { timeout: 30000 }
    )
    // 等待执行桥接注册：App 组件树装载完成后必须主动挂上 window.__runtimeBridge__（T005）。
    await page.waitForFunction('Boolean(window.__runtimeBridge__)', undefined, { timeout: 30000 })
    const bridgeMethods = await page.evaluate(
      "Object.keys(window.__runtimeBridge__).filter(name => typeof window.__runtimeBridge__[name] === 'function').sort()"
    )
    const runtimeVersion = await page.evaluate('String(window.__formRuntimeVersion__)')
    const browserVersion = browser.version()
    const result = {
      ok: pageErrors.length === 0,
      runtimeVersion,
      browserVersion,
      url: entryURL,
      bridgeMethods,
      pageErrors
    }
    console.log(JSON.stringify(result, null, 2))
    if (!result.ok) process.exitCode = 1
    await context.close()
  } finally {
    await browser.close().catch(() => undefined)
    await origin.close().catch(() => undefined)
  }
}

main().catch(error => {
  console.error(JSON.stringify({ ok: false, reason: String(error?.message ?? error) }))
  process.exitCode = 1
})
