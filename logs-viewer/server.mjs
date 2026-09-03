// 本机日志浏览服务：零依赖，只读展示 F-013 分层日志。
// 用 `pnpm dev:l` 在本目录启动，默认端口 19002，日志根默认取上一级目录的 logs/。
// 只读打开文件、只在日志根内解析路径，绝不写入或删除日志。
import { createServer } from 'node:http'
import { readdir, readFile, stat } from 'node:fs/promises'
import { createReadStream } from 'node:fs'
import path from 'node:path'
import process from 'node:process'

const PORT = Number(process.env.LOGS_VIEWER_PORT ?? 19002)
const LOG_ROOT = path.resolve(process.env.TEST_AUTO_PRO_LOG_ROOT ?? path.join(process.cwd(), '..', 'logs'))
const MAX_TAIL_BYTES = 512 * 1024

// listLogFiles 递归列出日志根下的所有日志文件，按修改时间倒序，最新的排在前面。
async function listLogFiles(dir = LOG_ROOT, prefix = '') {
  let entries = []
  try {
    entries = await readdir(dir, { withFileTypes: true })
  }
  catch {
    return []
  }
  const files = []
  for (const entry of entries) {
    const absolute = path.join(dir, entry.name)
    const relative = prefix ? `${prefix}/${entry.name}` : entry.name
    if (entry.isDirectory()) {
      files.push(...await listLogFiles(absolute, relative))
      continue
    }
    const info = await stat(absolute).catch(() => null)
    if (!info) continue
    files.push({ path: relative, size: info.size, modified: info.mtime.toISOString() })
  }
  return files.sort((left, right) => right.modified.localeCompare(left.modified))
}

// resolveInsideRoot 只接受日志根内部的相对路径，拒绝任何穿越尝试。
function resolveInsideRoot(relative) {
  const absolute = path.resolve(LOG_ROOT, relative)
  if (absolute !== LOG_ROOT && !absolute.startsWith(LOG_ROOT + path.sep)) return null
  return absolute
}

// readTail 读取文件尾部有界内容，避免打开超大日志时把内存顶满。
async function readTail(absolute, maxBytes = MAX_TAIL_BYTES) {
  const info = await stat(absolute)
  if (info.size <= maxBytes) return { text: await readFile(absolute, 'utf8'), truncated: false, size: info.size }
  const stream = createReadStream(absolute, { start: info.size - maxBytes, encoding: 'utf8' })
  let text = ''
  for await (const chunk of stream) text += chunk
  return { text: text.slice(text.indexOf('\n') + 1), truncated: true, size: info.size }
}

// sendJSON 输出统一 JSON 响应。
function sendJSON(response, status, value) {
  const body = JSON.stringify(value)
  response.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8', 'Content-Length': Buffer.byteLength(body) })
  response.end(body)
}

const server = createServer(async (request, response) => {
  const url = new URL(request.url ?? '/', `http://127.0.0.1:${PORT}`)
  try {
    if (url.pathname === '/' || url.pathname === '/index.html') {
      const page = await readFile(path.join(import.meta.dirname, 'index.html'))
      response.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' })
      response.end(page)
      return
    }
    if (url.pathname === '/api/files') {
      sendJSON(response, 200, { root: LOG_ROOT, files: await listLogFiles() })
      return
    }
    if (url.pathname === '/api/file') {
      const relative = url.searchParams.get('path') ?? ''
      const absolute = resolveInsideRoot(relative)
      if (!absolute) {
        sendJSON(response, 400, { error: '路径超出日志根' })
        return
      }
      const keyword = (url.searchParams.get('q') ?? '').trim()
      const tail = await readTail(absolute)
      const lines = tail.text.split('\n')
      const filtered = keyword ? lines.filter(line => line.includes(keyword)) : lines
      sendJSON(response, 200, {
        path: relative, size: tail.size, truncated: tail.truncated,
        total: lines.length, matched: filtered.length,
        text: filtered.slice(-3000).join('\n'),
      })
      return
    }
    sendJSON(response, 404, { error: '未知路径' })
  }
  catch (error) {
    sendJSON(response, 500, { error: `读取日志失败：${error instanceof Error ? error.message : '未知错误'}` })
  }
})

server.listen(PORT, '127.0.0.1', () => {
  process.stdout.write(`日志浏览服务已启动：http://127.0.0.1:${PORT}\n日志根：${LOG_ROOT}\n`)
})
