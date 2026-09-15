// 本地静态源：把已构建的 form-runtime 产物按原 publicPath（/form-runtime/）提供服务，
// 供无头浏览器加载固定 runtime 路由；只服务构建目录内的文件，不代理任何目标平台请求。
import http from 'node:http'
import fs from 'node:fs'
import path from 'node:path'

// CONTENT_TYPES 覆盖 Vue 构建产物中出现的静态资源类型，避免浏览器按二进制猜测。
const CONTENT_TYPES: Record<string, string> = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.ttf': 'font/ttf',
  '.eot': 'application/vnd.ms-fontobject',
  '.map': 'application/json; charset=utf-8'
}

export interface StaticOrigin {
  url: string
  close: () => Promise<void>
}

// serveDirectory 在 127.0.0.1 随机端口提供 rootDir 的静态服务；返回的 url 以斜杠结尾。
// 只接受 GET；路径规范化后必须仍位于 rootDir 内，防止目录穿越。
export function serveDirectory(rootDir: string): Promise<StaticOrigin> {
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
  return new Promise((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      if (!address || typeof address === 'string') {
        reject(new Error('静态源端口分配失败'))
        return
      }
      resolve({
        url: `http://127.0.0.1:${address.port}/`,
        close: () => new Promise<void>((done, fail) => server.close(error => (error ? fail(error) : done())))
      })
    })
  })
}
