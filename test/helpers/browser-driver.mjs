// 夜间真实浏览器自治测试的页面驱动库：只做「像人一样操作页面」需要的通用能力，
// 不包含任何写死的计划 ID、路径 ID 或流程模板 ID（任务书硬性纪律 2）。
//
// 定位说明：本文件是测试脚本，不是产品运行时代码；产品运行时仍是 executor-service。
// 之所以用 CDP 驱动真实 Chrome，是环境现实决定的：本机没有 playwright 下载的 chromium
// （ms-playwright 目录下没有浏览器），执行服务靠 browser.ts 的 channel 兜底使用本机 Chrome。
// 这里复用同一台本机 Chrome（独立无头实例），页面行为与执行服务完全一致。
import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import net from 'node:net'
import os from 'node:os'
import path from 'node:path'

const CHROME_CANDIDATES = [
  process.env.TEST_AUTO_PRO_CHROME || '',
  'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
  'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
  path.join(process.env.LOCALAPPDATA || '', 'Google\\Chrome\\Application\\chrome.exe'),
  'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
  'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe'
].filter(Boolean)

// resolveChromePath 返回第一个真实存在的浏览器可执行文件路径，找不到直接抛错停止本次执行。
export function resolveChromePath () {
  for (const candidate of CHROME_CANDIDATES) {
    if (candidate && existsSync(candidate)) return candidate
  }
  throw new Error('未找到可用的 Chrome/Edge 可执行文件，请用 TEST_AUTO_PRO_CHROME 指定')
}

// freePort 向系统申请一个空闲端口，避免多次运行时固定端口冲突。
async function freePort () {
  return await new Promise((resolve, reject) => {
    const server = net.createServer()
    server.on('error', reject)
    server.listen(0, '127.0.0.1', () => {
      const { port } = server.address()
      server.close(() => resolve(port))
    })
  })
}

// delay 是页面轮询等待的基础原语；所有等待都用它做退避，避免忙等。
export function delay (ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

// Cdp 是最小 CDP 客户端：只用 Runtime.evaluate 在页面里执行函数，
// 不引入第三方依赖，也不对页面做任何注入式改写（不破坏前端真实交互逻辑）。
class Cdp {
  constructor (wsUrl) {
    this.wsUrl = wsUrl
    this.id = 0
    this.pending = new Map()
    this.socket = null
  }

  async connect () {
    // 复用全局 fetch 自带的 WebSocket 实现（Node 22+），不额外引入依赖
    const { WebSocket } = globalThis
    if (typeof WebSocket !== 'function') throw new Error('当前 Node 没有内置 WebSocket，请使用 Node 22 以上')
    this.socket = new WebSocket(this.wsUrl)
    await new Promise((resolve, reject) => {
      this.socket.addEventListener('open', resolve, { once: true })
      this.socket.addEventListener('error', () => reject(new Error(`无法连接 CDP：${this.wsUrl}`)), { once: true })
    })
    this.socket.addEventListener('message', (event) => {
      let message
      try {
        message = JSON.parse(typeof event.data === 'string' ? event.data : Buffer.from(event.data).toString())
      } catch {
        return
      }
      if (!message.id) return
      const entry = this.pending.get(message.id)
      if (!entry) return
      this.pending.delete(message.id)
      if (message.error) entry.reject(new Error(JSON.stringify(message.error)))
      else entry.resolve(message.result)
    })
  }

  send (method, params = {}, sessionId) {
    const id = ++this.id
    const payload = { id, method, params }
    if (sessionId) payload.sessionId = sessionId
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject })
      this.socket.send(JSON.stringify(payload))
    })
  }

  close () {
    try {
      this.socket?.close()
    } catch {
      // 关闭失败不影响进程退出
    }
  }
}

// Browser 是一次真实浏览器会话：一个无头 Chrome + 一个页面标签。
export class Browser {
  constructor (child, cdp, sessionId, targetId) {
    this.child = child
    this.cdp = cdp
    this.sessionId = sessionId
    this.targetId = targetId
    this.frames = new Map()
  }

  // launch 启动无头本机 Chrome 并连上 CDP，返回可直接执行页面脚本的 Browser。
  static async launch ({ headless = true } = {}) {
    const port = await freePort()
    const profile = path.join(os.tmpdir(), `test-auto-pro-night-${process.pid}-${port}`)
    const args = [
      headless ? '--headless=new' : '--headless=false',
      `--remote-debugging-port=${port}`,
      `--user-data-dir=${profile}`,
      '--no-first-run',
      '--no-default-browser-check',
      '--disable-background-networking',
      '--disable-sync',
      '--disable-translate',
      '--mute-audio',
      '--window-size=1600,1000',
      'about:blank'
    ]
    const child = spawn(resolveChromePath(), args, { stdio: 'ignore', windowsHide: true })
    let version = null
    for (let attempt = 0; attempt < 100; attempt += 1) {
      await delay(200)
      try {
        const response = await fetch(`http://127.0.0.1:${port}/json/version`)
        if (response.ok) {
          version = await response.json()
          break
        }
      } catch {
        // 浏览器还在启动，继续等待
      }
    }
    if (!version) {
      child.kill()
      throw new Error('无头 Chrome 启动超时')
    }
    const cdp = new Cdp(version.webSocketDebuggerUrl)
    await cdp.connect()
    const { targetId } = await cdp.send('Target.createTarget', { url: 'about:blank' })
    const { sessionId } = await cdp.send('Target.attachToTarget', { targetId, flatten: true })
    const browser = new Browser(child, cdp, sessionId, targetId)
    await cdp.send('Page.enable', {}, sessionId)
    await cdp.send('Runtime.enable', {}, sessionId)
    await cdp.send('Page.setDownloadBehavior', { behavior: 'deny' }, sessionId).catch(() => {})
    return browser
  }

  // eval 在页面里执行一段函数体表达式字符串；表达式必须是可序列化的同步/异步代码。
  async eval (expression) {
    const result = await this.cdp.send('Runtime.evaluate', {
      expression,
      awaitPromise: true,
      returnByValue: true,
      userGesture: true
    }, this.sessionId)
    if (result.exceptionDetails) {
      const text = result.exceptionDetails.exception?.description || result.exceptionDetails.text
      throw new Error(`页面脚本异常：${text}`)
    }
    return result.result?.value
  }

  // goto 在顶层标签打开地址并等待文档就绪。
  async goto (url, { timeoutMs = 60000 } = {}) {
    await this.cdp.send('Page.navigate', { url }, this.sessionId)
    const deadline = Date.now() + timeoutMs
    while (Date.now() < deadline) {
      await delay(300)
      const state = await this.eval('document.readyState').catch(() => null)
      if (state === 'complete' || state === 'interactive') {
        await delay(400)
        return
      }
    }
    throw new Error(`打开页面超时：${url}`)
  }

  // screenshot 保存页面截图，用于留下人工可核对的证据。
  async screenshot (filePath) {
    const { data } = await this.cdp.send('Page.captureScreenshot', { format: 'png' }, this.sessionId)
    const { writeFileSync, mkdirSync } = await import('node:fs')
    mkdirSync(path.dirname(filePath), { recursive: true })
    writeFileSync(filePath, Buffer.from(data, 'base64'))
    return filePath
  }

  // toFrame 返回注入到目标 iframe 的 CDP 会话对象，用于在目标平台页面里执行脚本。
  // 目标平台界面在 iframe 中，必须用独立 session 才能做真实点击。
  async frameSession (frameUrlPart) {
    const tree = await this.cdp.send('Page.getFrameTree', {}, this.sessionId)
    const found = findFrame(tree.frameTree, frameUrlPart)
    if (!found) throw new Error(`页面里找不到 iframe：${frameUrlPart}`)
    const { sessionId } = await this.cdp.send('Target.attachToTarget', {
      targetId: this.targetId,
      flatten: true
    })
    return { cdp: this.cdp, sessionId, frameId: found.id, url: found.url }
  }

  close () {
    this.cdp.close()
    try {
      this.child.kill()
    } catch {
      // 进程已退出
    }
  }
}

// findFrame 深度优先查找 url 中包含指定片段的 frame。
function findFrame (node, urlPart) {
  if (node.frame && node.frame.url && node.frame.url.includes(urlPart)) return node.frame
  for (const child of node.childFrames || []) {
    const hit = findFrame(child, urlPart)
    if (hit) return hit
  }
  return null
}

// evalInSession 在指定 CDP session（通常是 iframe）里执行脚本。
export async function evalInSession (cdp, sessionId, expression) {
  const result = await cdp.send('Runtime.evaluate', {
    expression,
    awaitPromise: true,
    returnByValue: true,
    userGesture: true
  }, sessionId)
  if (result.exceptionDetails) {
    const text = result.exceptionDetails.exception?.description || result.exceptionDetails.text
    throw new Error(`页面脚本异常：${text}`)
  }
  return result.result?.value
}

export { freePort }
