import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import test from 'node:test'
import { studioSpring, startStudioMotion } from '../../../../web/src/appearance/studioMotion.ts'
import { navigationItems } from '../../../../web/src/appearance/navigation.ts'

const requireWeb = createRequire(new URL('../../../../web/package.json', import.meta.url))

test('弹簧从原点起步，轻微越过目标后收敛，不产生无穷或剧烈抖动', () => {
  const values = studioSpring.slice(7, -1).split(',').map(Number)
  assert.equal(values.length, 61)
  assert.equal(values[0], 0)
  assert.equal(values.at(-1), 1)
  assert.ok(values.every(Number.isFinite))
  assert.ok(Math.max(...values) > 1 && Math.max(...values) < 1.15)
  assert.ok(values.slice(-8).every(value => Math.abs(value - 1) < .015))
})

test('真实导航组件在两套皮肤和嵌套路由中都保留四个链接与当前页语义', async () => {
  const { parse, compileScript } = requireWeb('vue/compiler-sfc')
  const ts = requireWeb('typescript')
  const { createSSRApp, h } = requireWeb('vue')
  const { renderToString } = requireWeb('vue/server-renderer')
  const { createRouter, createMemoryHistory } = requireWeb('vue-router')
  const source = readFileSync(new URL('../../../../web/src/components/StudioNavigation.vue', import.meta.url), 'utf8')
  const script = compileScript(parse(source).descriptor, { id: 'studio-navigation-test', inlineTemplate: true })
  const code = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  const exports = {}
  vm.runInNewContext(code, { exports, require: name => name === '../appearance/navigation'
    ? { navigationItems } : name === '../appearance/studioMotion' ? { studioSpring } : requireWeb(name) })
  for (const skin of ['aurora', 'titanium']) for (const item of navigationItems) {
    const router = createRouter({ history: createMemoryHistory(), routes: navigationItems.map(entry => ({ path: entry.path + '/:nested(.*)*', component: { render: () => null } })) })
    await router.push(item.path + '/1')
    const app = createSSRApp({ render: () => h(exports.default, { skin, activePath: item.path }) })
    app.use(router)
    const html = await renderToString(app)
    const links = [...html.matchAll(/<a\b([^>]*)>([\s\S]*?)<\/a>/g)]
    assert.equal(links.length, 4)
    assert.equal(links.filter(link => link[1].includes('aria-current="page"')).length, 1)
    for (const entry of navigationItems) {
      const link = links.find(candidate => candidate[1].includes('href="' + entry.path + '"'))
      assert.ok(link, entry.path)
      assert.ok(link[2].includes(entry.label))
      assert.equal(link[1].includes('aria-current="page"'), entry.path === item.path)
    }
  }
})

// 有界 DOM 替身只提供动画层消费的能力，记录真正的事件、帧、写入和取消行为。
function environment() {
  const listeners = new Map(), frames = new Map(), animations = [], observers = []
  let sequence = 0
  class Element {
    constructor() {
      this.isConnected = true
      this.values = new Map()
      this.classes = new Set()
      this.style = { setProperty: (key, value) => this.values.set(key, value), removeProperty: key => this.values.delete(key) }
      this.classList = { add: key => this.classes.add(key), remove: key => this.classes.delete(key), contains: key => this.classes.has(key) }
      this.dataset = {}
    }
    closest() { return this }
    getBoundingClientRect() { return { left: 10, top: 20, width: 300, height: 200 } }
    querySelectorAll() { return [] }
    matches(selector) { return selector === '.app-dialog' && this.dialog === true }
    animate(keyframes, options) {
      const value = { element: this, keyframes, options, finished: new Promise(() => {}), paused: false, cancelled: false,
        pause() { this.paused = true }, play() { this.paused = false }, cancel() { this.cancelled = true } }
      animations.push(value)
      return value
    }
  }
  class Observer {
    constructor(callback) { this.callback = callback; observers.push(this) }
    observe() {}
    disconnect() { this.disconnected = true }
  }
  const doc = { hidden: false, body: {}, addEventListener: (name, fn) => listeners.set(name, fn), removeEventListener: name => listeners.delete(name),
    defaultView: { requestAnimationFrame: fn => { frames.set(++sequence, fn); return sequence }, cancelAnimationFrame: id => frames.delete(id) } }
  const root = new Element(), content = new Element()
  root.ownerDocument = doc
  content.querySelectorAll = () => Array.from({ length: 5 }, () => new Element())
  const previous = { Element: globalThis.Element, HTMLElement: globalThis.HTMLElement, MutationObserver: globalThis.MutationObserver }
  Object.assign(globalThis, { Element, HTMLElement: Element, MutationObserver: Observer })
  return { root, content, doc, listeners, frames, animations, observers, Element, restore() {
    for (const [key, value] of Object.entries(previous)) if (value === undefined) delete globalThis[key]; else globalThis[key] = value
  } }
}

test('指针反馈合并同帧事件、只持有一个表面，切走后释放监听和在途动画', () => {
  const e = environment()
  try {
    const controller = startStudioMotion(e.root, e.content)
    const first = new e.Element(), second = new e.Element()
    for (let index = 0; index < 100; index++) e.listeners.get('pointermove')({ target: first, pointerType: 'mouse', clientX: index + 10, clientY: index + 20 })
    assert.equal(e.frames.size, 1)
    const paint = [...e.frames.values()][0]; e.frames.clear(); paint()
    assert.equal(first.values.get('--studio-x'), '99px')
    assert.equal(first.values.get('--studio-y'), '99px')
    e.listeners.get('pointermove')({ target: second, pointerType: 'mouse', clientX: 100, clientY: 100 })
    assert.equal(first.classes.size, 0)
    assert.equal(first.values.size, 0)
    const stale = [...e.frames.values()][0]
    controller.stop(); stale()
    assert.equal(e.frames.size, 0)
    assert.equal(e.listeners.size, 0)
    assert.equal(second.classes.size, 0)
    assert.equal(second.values.size, 0)
    assert.ok(e.animations.every(animation => animation.cancelled))
    assert.ok(e.observers.every(observer => observer.disconnected))
    assert.equal(e.root.values.size, 0)
  } finally { e.restore() }
})

test('弹窗入场不因内容更新重播，动画并发有上限，隐藏页面暂停，画布页不入场', () => {
  const e = environment()
  try {
    e.content.classes.add('app-main--flush')
    const controller = startStudioMotion(e.root, e.content)
    assert.equal(e.animations.length, 0)
    const dialog = new e.Element(); dialog.dialog = true
    const notify = node => e.observers[0].callback([{ addedNodes: [node] }])
    notify(dialog); notify(dialog)
    assert.equal(e.animations.length, 1)
    for (let index = 0; index < 30; index++) { const node = new e.Element(); node.dialog = true; notify(node) }
    assert.equal(e.animations.length, 12)
    e.doc.hidden = true; e.listeners.get('visibilitychange')()
    assert.equal(e.root.dataset.studioMotion, 'paused')
    assert.ok(e.animations.every(animation => animation.paused))
    e.doc.hidden = false; e.listeners.get('visibilitychange')()
    assert.ok(e.animations.every(animation => !animation.paused))
    controller.stop()
  } finally { e.restore() }
})
