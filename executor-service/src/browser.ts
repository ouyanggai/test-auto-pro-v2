// 无头 Chromium 启动的唯一入口：版本由 package.json 固定的 playwright 1.63.0 决定，
// 启动参数在此集中维护，探针与会话管理器都必须经过本模块启动浏览器。
import { chromium, type Browser } from 'playwright'

// 固定启动参数：关闭后台联网与首跑向导，保证探针与会话行为只依赖执行服务页面本身。
export const HEADLESS_LAUNCH_ARGS = [
  '--disable-dev-shm-usage',
  '--no-first-run',
  '--no-default-browser-check',
  '--disable-background-networking',
  '--disable-sync',
  '--disable-translate',
  '--mute-audio'
] as const

// launchHeadlessChromium 启动 Chromium：默认无头；调试需要人工观察页面时
// 置 EXECUTOR_HEADFUL=1 以有头模式启动，显示真实浏览器窗口。
//
// 浏览器来源按顺序解析，且只在「真的启动失败」时才换下一个：
//   ① EXECUTOR_BROWSER_CHANNEL 指定的 channel（显式配置优先）；
//   ② playwright 自带 chromium（`playwright install chromium` 的产物）；
//   ③ 本机已安装的 Chrome / Edge（channel=chrome|msedge）。
// 第 ③ 步是 Windows 迁移后必须的兜底：本机 Chrome 与 playwright 固定版本同为主版本 153
// （实测 153.0.8010.48 vs 期望 153.0.8010.12），驱动与页面行为一致；
// 若坚持只认下载的 chromium，在无外网或下载受阻的环境下执行器将完全无法启动。
export async function launchHeadlessChromium (): Promise<Browser> {
  const headful = process.env.EXECUTOR_HEADFUL === '1'
  const options = { headless: !headful, args: [...HEADLESS_LAUNCH_ARGS] }
  const configured = process.env.EXECUTOR_BROWSER_CHANNEL
  const candidates = configured ? [configured] : [undefined, 'chrome', 'msedge']
  const failures: string[] = []

  for (const channel of candidates) {
    try {
      return await chromium.launch(channel ? { ...options, channel } : options)
    } catch (error) {
      failures.push(`${channel ?? 'bundled-chromium'}: ${String((error as Error)?.message ?? error)}`)
    }
  }
  throw new Error(`没有可用的浏览器，已尝试 ${failures.length} 种来源：\n${failures.join('\n')}`)
}
