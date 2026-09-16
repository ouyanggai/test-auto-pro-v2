import type { Page } from 'playwright'
import { ForwardFailure, type ForwardChannel } from './forward-channel.ts'
import { installTargetRoute, type TargetRouteHandle } from './target-route.ts'

// assertForwardReady 在导航前检查转发通道：未连接则禁止 page.goto。
export function assertForwardReady (channel: ForwardChannel): void {
  if (!channel.connected) {
    throw new ForwardFailure('转发通道未连接，禁止导航')
  }
}

// installRouteThenNavigate 先装拦截、再确认通道、最后导航。
// 通道失败时页面不得发出目标请求。
export async function installRouteThenNavigate (
  page: Page,
  channel: ForwardChannel,
  url: string,
  runtimeOrigins: string[]
): Promise<TargetRouteHandle> {
  const handle = await installTargetRoute(page, {
    runtimeOrigins,
    forward: async payload => channel.send({
      sessionId: '',
      runId: '',
      pathRunId: '',
      stepId: '',
      payload
    })
  })
  try {
    assertForwardReady(channel)
    await page.goto(url, { waitUntil: 'domcontentloaded' })
    return handle
  } catch (error) {
    await handle.uninstall().catch(() => undefined)
    throw error
  }
}
