// 执行服务入口（T002 阶段）：提供健康检查并声明固定的运行参数。
// 会话管理（T003）、协议命令（T004）与请求转发（T006）在后续任务接入同一进程。
import http from 'node:http'
import { HEADLESS_LAUNCH_ARGS } from './browser.ts'

// EXECUTOR_PORT 默认 19011：与 form-runtime 开发端口 19001、主应用端口错开。
const port = Number(process.env.EXECUTOR_PORT || 19011)

const server = http.createServer((request, response) => {
  if (request.method === 'GET' && (request.url ?? '').split('?')[0] === '/healthz') {
    response.writeHead(200, { 'content-type': 'application/json; charset=utf-8' })
    response.end(JSON.stringify({
      service: 'form-runtime-executor',
      stage: 'T002',
      headless: true,
      launchArgs: HEADLESS_LAUNCH_ARGS
    }))
    return
  }
  response.writeHead(404, { 'content-type': 'application/json; charset=utf-8' })
  response.end(JSON.stringify({ error: 'not found' }))
})

server.listen(port, '127.0.0.1', () => {
  console.log(`[executor] listening on http://127.0.0.1:${port} (headless chromium service)`)
})
