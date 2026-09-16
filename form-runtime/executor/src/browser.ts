// 无头 Chromium 启动的唯一入口：版本由 package.json 固定的 playwright 版本决定，
// 启动参数在此集中维护，探针与会话管理器都必须经过本模块启动浏览器。
import { chromium, type Browser } from 'playwright'

// 固定启动参数：关闭后台联网与首跑向导，保证探针与会话行为只依赖 runtime 页面本身。
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
export async function launchHeadlessChromium(): Promise<Browser> {
  const headful = process.env.EXECUTOR_HEADFUL === '1'
  return chromium.launch({ headless: !headful, args: [...HEADLESS_LAUNCH_ARGS] })
}
