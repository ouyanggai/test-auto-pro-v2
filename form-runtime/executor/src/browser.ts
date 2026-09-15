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

// launchHeadlessChromium 启动无头 Chromium；本项目不使用可见窗口。
export async function launchHeadlessChromium(): Promise<Browser> {
  return chromium.launch({ headless: true, args: [...HEADLESS_LAUNCH_ARGS] })
}
