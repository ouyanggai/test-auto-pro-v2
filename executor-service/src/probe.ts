// T001 探针：验证无头 Chromium 可以启动。本阶段不装载业务页面，也不访问目标平台。
// 只做：启动浏览器、打开 about:blank、读取浏览器版本。禁止 locator/click/fill。
import { launchHeadlessChromium, HEADLESS_LAUNCH_ARGS } from './browser.ts'

async function main (): Promise<void> {
  const browser = await launchHeadlessChromium()
  try {
    const context = await browser.newContext()
    const page = await context.newPage()
    await page.goto('about:blank', { waitUntil: 'load' })
    const result = {
      ok: true,
      service: 'executor-service',
      browserVersion: browser.version(),
      launchArgs: HEADLESS_LAUNCH_ARGS,
      url: page.url()
    }
    console.log(JSON.stringify(result, null, 2))
    await context.close()
  } finally {
    await browser.close().catch(() => undefined)
  }
}

main().catch(error => {
  console.error(JSON.stringify({ ok: false, reason: String((error as Error)?.message ?? error) }))
  process.exitCode = 1
})
