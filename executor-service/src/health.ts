// 健康检查只证明本进程可听端口，不访问目标平台，不装载业务页面。
export const SERVICE_NAME = 'executor-service'
export const DEFAULT_PORT = 19011

export interface HealthBody {
  ok: true
  service: string
  protocol: 'exec-engine/v2'
}

// healthBody 是 GET /health 与 GET /runtime/health 的固定成功载荷。
export function healthBody (): HealthBody {
  return {
    ok: true,
    service: SERVICE_NAME,
    protocol: 'exec-engine/v2'
  }
}

// listenPort 读取 EXECUTOR_PORT，缺省 19011。
export function listenPort (): number {
  const raw = Number(process.env.EXECUTOR_PORT || DEFAULT_PORT)
  if (!Number.isInteger(raw) || raw <= 0 || raw > 65535) {
    throw new Error(`EXECUTOR_PORT 非法：${process.env.EXECUTOR_PORT ?? ''}`)
  }
  return raw
}
