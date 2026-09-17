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

// extractPageDist 把 flow-source/dist 复制到临时目录的 flow/ 前缀下，对齐 publicPath=/flow/。
//
// 迁移修订：旧实现依赖 flow-source/dist.zip 并调用 `unzip`，两者都只在 macOS 上成立
// （zip 是 Mac 上手工产物，Windows 默认没有 unzip）。现在直接消费 flow-source 里
// 已构建好的 dist/ 目录：纯 Node 复制，不依赖任何外部命令，也不依赖 zip 中间产物。
export function extractPageDist (): { root: string, distDir: string } {
  const distSource = path.join(serviceRoot(), 'flow-source', 'dist')
  if (!fs.existsSync(distSource)) {
    throw new Error('缺少 executor-service/flow-source/dist，无法装载原生 /flow 页面（先执行 pnpm --dir executor-service build-native-page）')
  }
  if (!fs.existsSync(path.join(distSource, 'index.html'))) {
    throw new Error('executor-service/flow-source/dist 缺少 index.html，页面产物不完整')
  }
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'executor-page-'))
  const distDir = path.join(root, 'flow')
  fs.cpSync(distSource, distDir, { recursive: true })
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
    const ext = path.extname(target)
    const contentType = CONTENT_TYPES[ext] ?? 'application/octet-stream'
    fs.readFile(target, (error, content) => {
      if (error) {
        if (error.code === 'ENOENT') {
          response.writeHead(404, { 'content-type': 'text/plain; charset=utf-8' })
          response.end('not found')
        } else {
          response.writeHead(500, { 'content-type': 'text/plain; charset=utf-8' })
          response.end('internal error')
        }
        return
      }
      response.writeHead(200, { 'content-type': contentType })
      response.end(content)
    })
  })
  return new Promise(resolve => {
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      if (!address || typeof address === 'string') {
        throw new Error('页面服务未绑定端口')
      }
      resolve({
        url: `http://127.0.0.1:${address.port}/flow/`,
        close: async () => new Promise(done => { server.close(() => { done() }) })
      })
    })
  })
}

export function injectIdentityScript (payload: Record<string, unknown>): string {
  const sid = String(payload.sid ?? '')
  const platformCode = String(payload.platformCode ?? '200001')
  const userName = String(payload.currentOperator && typeof payload.currentOperator === 'object' && 'name' in payload.currentOperator ? payload.currentOperator.name : '')
  return `
    (function() {
      const vue = window.$vue;
      if (!vue) return;
      if (!window.__executorNative) window.__executorNative = {};
      window.__executorNative.initFlow = vue.$children && vue.$children.find(c => c.$options.name === 'InitFlowDialog');
      window.__executorNative.flowDetail = vue.$children && vue.$children.find(c => c.$options.name === 'FlowDetail');
      if (${JSON.stringify(sid)}) {
        localStorage.setItem('token', ${JSON.stringify(sid)});
      }
      if (${JSON.stringify(platformCode)}) {
        localStorage.setItem('platformCode', ${JSON.stringify(platformCode)});
      }
      if (${JSON.stringify(userName)}) {
        localStorage.setItem('userName', ${JSON.stringify(userName)});
      }
    })();
  `
}
