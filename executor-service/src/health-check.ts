// 独立健康检查：只请求本服务 /health，不访问目标平台，不启动 Chromium。
import { DEFAULT_PORT, type HealthBody } from './health.ts'

const baseURL = (process.env.RUNTIME_EXECUTOR_URL || `http://127.0.0.1:${process.env.EXECUTOR_PORT || DEFAULT_PORT}`).replace(/\/+$/, '')

async function main (): Promise<void> {
  const response = await fetch(`${baseURL}/health`)
  const text = await response.text()
  let body: HealthBody | null = null
  try {
    body = JSON.parse(text) as HealthBody
  } catch {
    throw new Error(`健康检查响应不是 JSON：${text.slice(0, 200)}`)
  }
  if (response.status !== 200 || body?.ok !== true || body.service !== 'executor-service') {
    throw new Error(`健康检查失败：HTTP ${response.status} ${text}`)
  }
  console.log(JSON.stringify({ ok: true, url: `${baseURL}/health`, body }, null, 2))
}

main().catch(error => {
  console.error(JSON.stringify({ ok: false, reason: String((error as Error)?.message ?? error) }))
  process.exitCode = 1
})
