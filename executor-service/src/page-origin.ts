import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import http from 'node:http'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const CONTENT_TYPES: Record<string, string> = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2'
}

export interface PageOrigin {
  url: string
  close: () => Promise<void>
}

function serviceRoot (): string {
  return path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
}

// extractPageDist 把 flow-source/dist.zip 解到临时目录的 flow/ 前缀下，对齐 publicPath=/flow/。
export function extractPageDist (): { root: string, distDir: string } {
  const zipPath = path.join(serviceRoot(), 'flow-source', 'dist.zip')
  if (!fs.existsSync(zipPath)) {
    throw new Error('缺少 executor-service/flow-source/dist.zip，无法装载原生 /flow 页面')
  }
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'executor-page-'))
  execFileSync('unzip', ['-o', '-q', zipPath, '-d', root])
  const unpacked = path.join(root, 'dist')
  const distDir = path.join(root, 'flow')
  fs.renameSync(unpacked, distDir)
  return { root, distDir }
}

// serveFlowPage 在独立端口提供 /flow/ 静态页，不代理目标平台。
export function serveFlowPage (distDir: string): Promise<PageOrigin> {
  const root = path.resolve(distDir)
  const server = http.createServer((request, response) => {
    if (request.method !== 'GET') {
      response.writeHead(405, { 'content-type': 'text/plain; charset=utf-8' })
      response.end('method not allowed')
      return
    }
    const requestPath = decodeURIComponent((request.url ?? '/').split('?')[0] ?? '/')
    let relative = requestPath.replace(/^\/flow\/?/, '')
    if (relative === '' || relative.endsWith('/')) relative += 'index.html'
    const target = path.resolve(root, relative)
    if (target !== root && !target.startsWith(root + path.sep)) {
      response.writeHead(403, { 'content-type': 'text/plain; charset=utf-8' })
      response.end('forbidden')
      return
    }
    const filePath = fs.existsSync(target) && fs.statSync(target).isFile() ? target : path.join(root, 'index.html')
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
  const { promise, resolve, reject } = Promise.withResolvers<PageOrigin>()
  server.once('error', reject)
  server.listen(0, '127.0.0.1', () => {
    const address = server.address()
    if (!address || typeof address === 'string') {
      reject(new Error('原生页面静态源端口分配失败'))
      return
    }
    resolve({
      url: `http://127.0.0.1:${address.port}/flow/`,
      close: () => {
        const closed = Promise.withResolvers<void>()
        server.close(error => (error ? closed.reject(error) : closed.resolve()))
        return closed.promise
      }
    })
  })
  return promise
}
