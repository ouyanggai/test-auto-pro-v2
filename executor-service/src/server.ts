// T001 独立执行服务骨架：先提供端口监听与健康检查，不装载业务页面、不访问目标。
// 后续任务再接入会话、v2 协议、原生 /flow 路由和请求转发。
import http from 'node:http'
import { HEADLESS_LAUNCH_ARGS } from './browser.ts'
import { healthBody, listenPort, SERVICE_NAME } from './health.ts'

async function main (): Promise<void> {
  const port = listenPort()
  const server = http.createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`)
    const send = (status: number, body: unknown): void => {
      response.writeHead(status, { 'content-type': 'application/json; charset=utf-8' })
      response.end(JSON.stringify(body))
    }
    if (request.method === 'GET' && (url.pathname === '/health' || url.pathname === '/runtime/health')) {
      send(200, healthBody())
      return
    }
    send(404, { error: 'not found' })
  })

  server.listen(port, '0.0.0.0', () => {
    console.log(JSON.stringify({
      service: SERVICE_NAME,
      listening: `http://0.0.0.0:${port}`,
      health: `/health`,
      launchArgs: HEADLESS_LAUNCH_ARGS
    }))
  })

  function shutdown (): void {
    server.close(() => process.exit(0))
  }
  process.on('SIGINT', shutdown)
  process.on('SIGTERM', shutdown)
}

main().catch(error => {
  console.error(JSON.stringify({ ok: false, reason: String((error as Error)?.message ?? error) }))
  process.exitCode = 1
})
