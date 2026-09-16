// 本地静态源：只服务构建目录内的文件，不代理目标平台请求。
import http from 'node:http'
import fs from 'node:fs'
import path from 'node:path'

const CONTENT_TYPES: Record<string, string> = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8'
}

export interface StaticOrigin {
  url: string
  close: () => Promise<void>
}

export function serveDirectory (rootDir: string): Promise<StaticOrigin> {
  const root = path.resolve(rootDir)
  const server = http.createServer((request, response) => {
    if (request.method !== 'GET') {
      response.writeHead(405, { 'content-type': 'text/plain; charset=utf-8' })
      response.end('method not allowed')
      return
    }
    const requestPath = decodeURIComponent((request.url ?? '/').split('?')[0] ?? '/')
    const relative = requestPath.replace(/^\/+/, '')
    const target = path.resolve(root, relative)
    if (target !== root && !target.startsWith(root + path.sep)) {
      response.writeHead(403, { 'content-type': 'text/plain; charset=utf-8' })
      response.end('forbidden')
      return
    }
    fs.stat(target, (statError, stats) => {
      const filePath = !statError && stats.isFile() ? target : path.join(root, 'index.html')
      fs.readFile(filePath, (readError, data) => {
        if (readError) {
          response.writeHead(404, { 'content-type': 'text/plain; charset=utf-8' })
          response.end('not found')
          return
        }
        const type = CONTENT_TYPES[path.extname(filePath).toLowerCase()] ?? 'application/octet-stream'
        response.writeHead(200, { 'content-type': type })
        response.end(data)
      })
    })
  })
  const { promise, resolve, reject } = Promise.withResolvers<StaticOrigin>()
  server.once('error', reject)
  server.listen(0, '127.0.0.1', () => {
    const address = server.address()
    if (!address || typeof address === 'string') {
      reject(new Error('静态源端口分配失败'))
      return
    }
    resolve({
      url: `http://127.0.0.1:${address.port}/`,
      close: () => {
        const closed = Promise.withResolvers<void>()
        server.close(error => (error ? closed.reject(error) : closed.resolve()))
        return closed.promise
      }
    })
  })
  return promise
}
