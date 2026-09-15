// 执行引擎会话相关的共享类型：只描述 executor 内部事实，
// BrowserContext/Page 句柄绝不允许进入对外协议消息（T004 起只传 JSON 数据）。
import type { BrowserContext, Page } from 'playwright'

// SessionStatus 是会话生命周期状态：
//   creating：Context/页面创建中，还不能接收命令；
//   ready：页面就绪，可以接收业务命令；
//   closed：正常关闭（主动销毁或动作结束）；
//   crashed：页面崩溃后被释放，必须通知 Go。
export type SessionStatus = 'creating' | 'ready' | 'closed' | 'crashed'

// ManagedSession 是一次节点动作的完整浏览器现场。
// 纪律（docs/s/01-执行引擎设计.md 第 3 节）：一次执行一个 BrowserContext；
// Context 内包含当前账号的 cookies、localStorage、sessionStorage、SID、Vuex、表单组件树与请求监听器。
export interface ManagedSession {
  // sessionId 是执行服务内部生成的稳定会话编号，所有协议消息都携带它。
  sessionId: string
  // accountKey 是目标账号唯一键；同一账号同时至多存在一个未关闭会话（同账号串行由 Go 排队保证）。
  accountKey: string
  createdAt: string
  status: SessionStatus
  closedAt?: string
  closeReason?: string
  // context/page 是 runtime 内部句柄：关闭后置空，任何代码不得把它们写入协议或日志正文之外的位置。
  context: BrowserContext | null
  page: Page | null
}

// CloseReason 区分正常释放与异常释放，供运行日志与 Go 侧判定使用。
export type CloseReason = 'destroyed' | 'superseded' | 'crashed' | 'shutdown'
