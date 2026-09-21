// 执行服务会话相关的共享类型：只描述内部事实，BrowserContext/Page 句柄不进入对外协议。
import type { BrowserContext, Page } from 'playwright'

// SessionStatus 是会话生命周期状态：
//   creating：Context/页面创建中，还不能接收命令；
//   ready：页面就绪，可以接收业务命令；
//   closed：正常关闭（主动销毁或动作结束）；
//   crashed：页面崩溃后被释放，必须通知 Go。
export type SessionStatus = 'creating' | 'ready' | 'closed' | 'crashed'

// ManagedSession 是一次节点动作的完整浏览器现场。
// 一次执行一个 BrowserContext；Context 内包含当前账号的 cookies、storage 与请求监听器。
export interface ManagedSession {
  sessionId: string
  accountKey: string
  createdAt: string
  lastHeartbeatAt: number
  leaseExpiresAt: number
  status: SessionStatus
  closedAt?: string
  closeReason?: string
  context: BrowserContext | null
  page: Page | null
}

export type CloseReason = 'destroyed' | 'superseded' | 'crashed' | 'shutdown' | 'lease_expired'
