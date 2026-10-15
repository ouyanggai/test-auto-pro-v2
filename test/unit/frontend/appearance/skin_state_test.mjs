import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import test from 'node:test'
import { applyAppearance } from '../../../../web/src/appearance/applyAppearance.ts'
import { buildThemeOverrides } from '../../../../web/src/appearance/naiveTheme.ts'
import { skinCatalog } from '../../../../web/src/appearance/skins.ts'
import { useAppStore } from '../../../../web/src/stores/app.ts'
import { switchAppearance } from '../../../../web/src/appearance/transition.ts'

const requireWeb = createRequire(new URL('../../../../web/package.json', import.meta.url))
const vue = requireWeb('vue')

// 用 Vue 的实际渲染与路由更新运行 App；内存节点只替代 DOM，不启动浏览器或加载 iframe。
function memoryRenderer() {
  const node = (type, text = '') => ({ type, text, props: {}, children: [], parent: null })
  const remove = element => {
    if (element.parent) element.parent.children.splice(element.parent.children.indexOf(element), 1)
    element.parent = null
  }
  const renderer = vue.createRenderer({
    createElement: type => node(type), createText: text => node('#text', text), createComment: text => node('#comment', text),
    setText: (element, text) => { element.text = text },
    setElementText: (element, text) => { element.children = []; element.text = text },
    parentNode: element => element.parent,
    nextSibling: element => element.parent?.children[element.parent.children.indexOf(element) + 1] ?? null,
    patchProp: (element, key, _old, value) => { element.props[key] = value },
    insert(element, parent, anchor = null) {
      remove(element)
      const index = anchor ? parent.children.indexOf(anchor) : -1
      parent.children.splice(index < 0 ? parent.children.length : index, 0, element)
      element.parent = parent
    },
    remove,
  })
  const find = (root, type) => root.type === type ? root : root.children.map(child => find(child, type)).find(Boolean)
  return { renderer, root: node('root'), find }
}

test('实际应用壳在八种外观之间切换时保留路由实例、未保存输入和子节点身份', async () => {
  const { parse, compileScript } = requireWeb('vue/compiler-sfc')
  const ts = requireWeb('typescript')
  const pinia = requireWeb('pinia')
  const routerApi = requireWeb('vue-router')
  const documentBefore = Object.getOwnPropertyDescriptor(globalThis, 'document')
  const windowBefore = Object.getOwnPropertyDescriptor(globalThis, 'window')
  const values = new Map()
  let transitions = 0
  const fakeDocument = {
    documentElement: { dataset: {}, style: { setProperty: (key, value) => values.set(key, value) }, animate: () => { transitions++; return {} } },
    querySelector: () => null,
    // 由异步回调执行真正的换肤逻辑，确保不是绕过切换入口直接改 store 的静态检查。
    startViewTransition(update) {
      const ready = Promise.resolve().then(update)
      return { ready, finished: ready.then(() => {}) }
    },
  }
  Object.defineProperty(globalThis, 'document', { configurable: true, value: fakeDocument })
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { innerWidth: 1280, innerHeight: 800 } })
  let mounted = 0, unmounted = 0, app
  try {
    const source = readFileSync(new URL('../../../../web/src/App.vue', import.meta.url), 'utf8')
    const script = compileScript(parse(source).descriptor, { id: 'skin-state-audit', inlineTemplate: true })
    const code = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
    // Naive 外壳只透传插槽；业务页通过真实 RouterView 挂载，生命周期和状态由真实 Vue 管理。
    const wrapper = { setup: (_props, { slots }) => () => vue.h('section', slots.default?.()) }
    const empty = { render: () => null }
    const naive = Object.fromEntries(['NButton', 'NConfigProvider', 'NGlobalStyle', 'NLayout', 'NLayoutContent', 'NLayoutHeader', 'NLayoutSider', 'NMenu', 'NMessageProvider', 'NNotificationProvider'].map(name => [name, wrapper]))
    const modules = {
      './stores/app': { useAppStore }, './appearance/naiveTheme': { buildThemeOverrides },
      './appearance/applyAppearance': { applyAppearance }, './appearance/navigation': { navigationItems: [] },
      './appearance/pointerGlow': { startPointerGlow: () => assert.fail('内存验证不创建光晕节点') },
      './appearance/studioMotion': { startStudioMotion: () => assert.fail('内存验证不测量工作区') },
      './appearance/transition': { switchAppearance },
    }
    const exports = {}
    vm.runInNewContext(code, { exports, document: fakeDocument, require: name => name === 'naive-ui' ? naive
      : modules[name] ?? (name.endsWith('.vue') ? { default: empty } : requireWeb(name)) })
    const probe = {
      setup() {
        const draft = vue.ref(''), selected = vue.ref('节点 3')
        vue.onMounted(() => mounted++)
        vue.onBeforeUnmount(() => unmounted++)
        return () => vue.h('article', [
          vue.h('input', { value: draft.value, onInput: event => { draft.value = event.target.value } }),
          vue.h('iframe', { src: '/native-form-probe' }), vue.h('canvas', { 'data-selected': selected.value }),
          vue.h('dialog', { open: true }, draft.value),
        ])
      },
    }
    const router = routerApi.createRouter({ history: routerApi.createMemoryHistory(), routes: [{ path: '/plans', component: probe }] })
    await router.push('/plans')
    const e = memoryRenderer(), storePlugin = pinia.createPinia()
    app = e.renderer.createApp(exports.default)
    app.use(storePlugin).use(router).mount(e.root)
    await vue.nextTick()
    const store = useAppStore(storePlugin)
    const input = e.find(e.root, 'input'), iframe = e.find(e.root, 'iframe'), canvas = e.find(e.root, 'canvas'), dialog = e.find(e.root, 'dialog')
    input.props.onInput({ target: { value: '未保存：表单和节点配置' } })
    for (const skin of [...skinCatalog, ...skinCatalog.toReversed()]) for (const mode of ['light', 'dark']) {
      await switchAppearance(() => { store.setSkinId(skin.id); store.setThemeMode(mode) })
      await vue.nextTick()
      assert.equal(fakeDocument.documentElement.dataset.skin, skin.id)
      assert.equal(fakeDocument.documentElement.dataset.mode, mode)
      assert.equal(router.currentRoute.value.fullPath, '/plans')
      assert.equal(e.find(e.root, 'input'), input)
      assert.equal(input.props.value, '未保存：表单和节点配置')
      assert.equal(e.find(e.root, 'iframe'), iframe)
      assert.equal(e.find(e.root, 'canvas'), canvas)
      assert.equal(canvas.props['data-selected'], '节点 3')
      assert.equal(e.find(e.root, 'dialog'), dialog)
      assert.equal(dialog.text, '未保存：表单和节点配置')
      assert.equal(dialog.props.open, true)
      assert.equal(mounted, 1)
      assert.equal(unmounted, 0)
    }
    assert.equal(transitions, 16)
  } finally {
    app?.unmount()
    if (documentBefore) Object.defineProperty(globalThis, 'document', documentBefore)
    else delete globalThis.document
    if (windowBefore) Object.defineProperty(globalThis, 'window', windowBefore)
    else delete globalThis.window
  }
  assert.equal(unmounted, 1)
})
