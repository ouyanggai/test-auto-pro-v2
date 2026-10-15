import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import { createRequire } from 'node:module'
import vm from 'node:vm'
import test from 'node:test'
import { defaultTokens, skinCatalog, skinTokens } from '../../../../web/src/appearance/skins.ts'
import { parseColor, lift, sink, alpha } from '../../../../web/src/appearance/color.ts'
import { readSkinId, readThemeMode, skinStorageKey, themeStorageKey } from '../../../../web/src/appearance/preferences.ts'
import { buildThemeOverrides } from '../../../../web/src/appearance/naiveTheme.ts'
import { applyAppearance, appearanceVariables } from '../../../../web/src/appearance/applyAppearance.ts'
import { chartBaseOption, statusPalette } from '../../../../web/src/features/reports/chartTheme.ts'

const read = path => readFileSync(new URL('../../../../' + path, import.meta.url), 'utf8')
const requireWeb = createRequire(new URL('../../../../web/package.json', import.meta.url))
const modes = ['light', 'dark']

// 独立算相对亮度，直接验证文字和按钮配色，而不是只检查令牌字段存在。
function contrast(left, right) {
  const luminance = color => {
    const channels = parseColor(color).slice(0, 3).map(value => {
      const c = value / 255
      return c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4
    })
    return channels[0] * .2126 + channels[1] * .7152 + channels[2] * .0722
  }
  const a = luminance(left), b = luminance(right)
  return (Math.max(a, b) + .05) / (Math.min(a, b) + .05)
}

test('四套皮肤各有两版，字段完整且配置不依赖组件库', () => {
  assert.deepEqual(skinCatalog.map(item => item.id), ['default', 'deep-space', 'aurora', 'titanium'])
  assert.deepEqual(skinCatalog.map(item => item.name), ['默认', '深空指挥舱', '极光玻璃', '钛金精密'])
  const fields = Object.keys(defaultTokens.light).sort()
  for (const skin of skinCatalog) for (const mode of modes) {
    assert.deepEqual(Object.keys(skinTokens[skin.id][mode]).sort(), fields)
    assert.equal(readSkinId({ getItem: key => key === skinStorageKey ? skin.id : mode }), skin.id)
    assert.equal(readThemeMode({ getItem: key => key === themeStorageKey ? mode : skin.id }), mode)
  }
  assert.doesNotMatch(read('web/src/appearance/skins.ts'), /from ['"]naive-ui/)
})

test('默认皮肤与当前 Naive 配色逐字段一致且不覆盖组件库主题', () => {
  const naive = requireWeb('naive-ui')
  const mapping = {
    bg: 'bodyColor', chrome: 'cardColor', surface: 'cardColor', reading: 'tableColor', raised: 'modalColor',
    surface2: 'actionColor', hover: 'hoverColor', text1: 'textColor1', text2: 'textColor2', text3: 'textColor3',
    primary: 'primaryColor', success: 'successColor', warning: 'warningColor', error: 'errorColor', info: 'infoColor',
    border: 'borderColor', borderStrong: 'borderColor', divider: 'dividerColor',
    shadow1: 'boxShadow1', shadow2: 'boxShadow2', shadow3: 'boxShadow3',
  }
  for (const mode of modes) {
    for (const [token, field] of Object.entries(mapping)) assert.equal(defaultTokens[mode][token], naive[mode + 'Theme'].common[field])
    assert.equal(buildThemeOverrides('default', mode), undefined)
  }
})

test('六套新配色保证正文、辅助文字、状态与实心按钮都能读清', () => {
  for (const skin of skinCatalog.slice(1)) for (const mode of modes) {
    const t = skinTokens[skin.id][mode]
    const pairs = [['text1', 'bg', 10], ['text2', 'bg', 6.5], ['text3', 'bg', 4.5], ['text3', 'reading', 4.5], ['primary', 'reading', 4.5], ['onPrimary', 'primary', 4.5]]
    for (const status of ['success', 'warning', 'error', 'info']) pairs.push([status, 'reading', 4.5], ['onStatus', status, 4.5])
    for (const [a, b, minimum] of pairs) assert.ok(contrast(t[a], t[b]) >= minimum, skin.id + '/' + mode + ' ' + a + '/' + b + ': ' + contrast(t[a], t[b]))
  }
})

test('非法偏好、拒绝存储与首屏脚本都落到同一默认值', () => {
  const script = read('web/index.html').match(/<script>([\s\S]*?)<\/script>/)[1]
  for (const value of [null, '', 'x', 'AURORA', 'constructor', '__proto__', ...skinCatalog.map(s => s.id)]) {
    const storage = { getItem: () => value }
    const root = { dataset: {}, style: {} }
    vm.runInNewContext(script, { localStorage: storage, document: { documentElement: root } })
    assert.equal(root.dataset.skin, readSkinId(storage))
    assert.equal(root.dataset.mode, readThemeMode(storage))
  }
  const rejected = { getItem() { throw new Error('storage denied') } }
  assert.equal(readSkinId(rejected), 'default')
  assert.equal(readThemeMode(rejected), 'light')
  const root = { dataset: {}, style: {} }
  vm.runInNewContext(script, { localStorage: rejected, document: { documentElement: root } })
  assert.equal(root.dataset.skin, 'default')
  assert.equal(root.dataset.mode, 'light')
})

test('新主题覆盖悬停按下色、阅读面和实心按钮文字，不混用透明阅读面', () => {
  for (const skin of skinCatalog.slice(1)) for (const mode of modes) {
    const t = skinTokens[skin.id][mode], overrides = buildThemeOverrides(skin.id, mode)
    for (const name of ['primary', 'info', 'success', 'warning', 'error']) {
      for (const state of ['Hover', 'Pressed', 'Suppl']) assert.ok(overrides.common[name + 'Color' + state])
      assert.notEqual(overrides.common[name + 'ColorHover'], t[name])
      const suffix = name[0].toUpperCase() + name.slice(1)
      assert.equal(overrides.Button['textColor' + suffix], name === 'primary' ? t.onPrimary : t.onStatus)
    }
    assert.equal(parseColor(overrides.Layout.color)[3], 0)
    assert.equal(parseColor(overrides.common.cardColor)[3], 1)
    assert.equal(parseColor(overrides.common.bodyColor)[3], 1)
    assert.equal(overrides.Card.color, t.surface)
  }
})

test('全局变量作用在同一个根元素，不改动业务对象或重建页面', () => {
  const writes = new Map()
  const root = { dataset: {}, style: { setProperty: (key, value) => writes.set(key, value) } }
  for (const skin of skinCatalog) for (const mode of modes) {
    applyAppearance(skin.id, mode, root)
    assert.equal(root.dataset.skin, skin.id)
    assert.equal(root.style.colorScheme, mode)
    assert.equal(writes.get('--primary-color'), skinTokens[skin.id][mode].primary)
    for (const key of ['--text-color-1', '--text-color-2', '--text-color-3', '--divider-color', '--card-color', '--font-family-mono', '--ta-on-status']) assert.ok(appearanceVariables(skin.id, mode)[key])
  }
  for (const file of ['web/src/App.vue', 'web/src/router/index.ts', 'web/src/features/path-configuration/FormRuntimeFrame.vue', 'web/src/features/reports/ReportChart.vue', 'web/src/features/flow-graph/FlowGraphCanvas.vue']) {
    assert.doesNotMatch(read(file), /:key=["'][^"']*(?:skinId|themeMode)/)
  }
  assert.doesNotMatch(read('web/src/features/path-configuration/FormRuntimeFrame.vue'), /appearance\//)
  assert.doesNotMatch(read('web/src/appearance/applyAppearance.ts'), /fetch\(|iframe|location\.|router\./)
})

test('颜色派生支持表内格式，明确拒绝错误值', () => {
  assert.deepEqual(parseColor('#abc'), [170, 187, 204, 1])
  assert.deepEqual(parseColor('rgba(10, 20, 30, .5)'), [10, 20, 30, .5])
  assert.equal(lift('#000'), 'rgb(36, 36, 36)')
  assert.equal(sink('#fff'), 'rgb(214, 214, 214)')
  assert.equal(alpha('#abc', .2), 'rgba(170, 187, 204, 0.2)')
  assert.throws(() => parseColor('invalid'))
})

test('八种首屏底色与实际令牌一致，切换入口保持快捷键与分组键盘操作', () => {
  const css = read('web/src/styles/layout.css')
  const rules = [...css.matchAll(/html(?:\[data-skin="([^"]+)"\])?\[data-mode="([^"]+)"\]\s*\{\s*background:\s*([^;]+);/g)]
  assert.equal(rules.length, 8)
  for (const [, id, mode, color] of rules) assert.deepEqual(parseColor(color), parseColor(skinTokens[id ?? 'default'][mode].bg))
  const app = read('web/src/App.vue'), menu = read('web/src/components/AppearanceMenu.vue')
  assert.match(app, /@click="appStore.toggleThemeMode"/)
  assert.match(menu, /role="radiogroup"/)
  assert.match(menu, /role="radio"/)
  assert.match(menu, /aria-checked/)
  assert.match(menu, /ArrowRight/)
  assert.match(menu, /ArrowLeft/)
  assert.doesNotMatch(menu, /fetch\(|defineEmits/)
})

test('11 处现有弹窗都有统一尺寸，尺寸和滚动只由全局规则管理', () => {
  const { parse } = requireWeb('vue/compiler-sfc')
  const targets = {
    'views/PlanPathsView.vue': ['sm', 'sm'],
    'views/PlanPathConfigurationView.vue': ['md'],
    'features/history-replay/BaseFormDataPicker.vue': ['lg'],
    'features/path-configuration/ActionOrchestrationEditor.vue': ['md', 'sm'],
    'features/path-configuration/ActionFlowDialog.vue': ['md'],
    'features/run-readiness/RunPreflightDialog.vue': ['md'],
    'features/runs/RunNodePanel.vue': ['md', 'lg', 'lg'],
  }
  const attr = (node, name) => node.props?.find(prop => prop.type === 6 && prop.name === name)?.value?.content
  let count = 0
  for (const [file, sizes] of Object.entries(targets)) {
    const source = read('web/src/' + file)
    const result = parse(source)
    assert.deepEqual(result.errors, [])
    const modals = []
    const walk = node => {
      if (node.tag === 'n-modal') modals.push(node)
      for (const child of node.children ?? []) walk(child)
    }
    walk(result.descriptor.template.ast)
    assert.equal(modals.length, sizes.length, file)
    for (const [index, modal] of modals.entries()) {
      const card = attr(modal, 'preset') ? modal : modal.children.find(child => child.tag === 'n-card')
      assert.ok(card, file)
      assert.ok(attr(card, 'class').includes('app-dialog app-dialog--' + sizes[index]), file)
      assert.ok(!attr(card, 'style')?.includes('width'), file)
      count++
    }
    assert.doesNotMatch(source, /dialogStyle|pickerStyle/)
  }
  assert.equal(count, 11)
  const layout = read('web/src/styles/layout.css')
  assert.match(layout, /max-height: calc\(100dvh - 96px\)/)
  assert.match(layout, /\.n-card__content[^}]*min-height: 0;[^}]*overflow: auto/)
  assert.match(layout, /\.n-card__footer[^}]*flex: none/)
})

test('颜色只来自令牌，唯一例外是八条首屏底色与原生表单白底', () => {
  const sources = []
  const walk = (directory, prefix) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const file = new URL(entry.name + (entry.isDirectory() ? '/' : ''), directory)
      const path = prefix + entry.name
      if (entry.isDirectory()) walk(file, path + '/')
      else if (/\.(vue|ts|css)$/.test(path)) sources.push([path, readFileSync(file, 'utf8')])
    }
  }
  walk(new URL('../../../../web/src/', import.meta.url), '')
  for (const [file, source] of sources) {
    if (file === 'appearance/skins.ts') continue
    const colors = source.match(/#[0-9a-fA-F]{3,8}\b/g) ?? []
    if (file === 'styles/layout.css') assert.equal(colors.length, 8)
    else if (file === 'features/path-configuration/FormRuntimeFrame.vue') assert.deepEqual(colors, ['#fff'])
    else assert.deepEqual(colors, [], file)
  }
  const joined = sources.map(([, source]) => source).join('\n')
  const references = new Set([...joined.matchAll(/var\((--[\w-]+)/g)].map(match => match[1]))
  const definitions = new Set([...joined.matchAll(/(--[\w-]+)['"]?\s*:/g)].map(match => match[1]))
  assert.deepEqual([...references].filter(key => !definitions.has(key)), [], '不能留下未定义变量造成部分换肤失效')
})

test('图表颜色随八种组合更新，保持数据和图例状态的接口边界', () => {
  for (const skin of skinCatalog) for (const mode of modes) {
    const t = skinTokens[skin.id][mode], option = chartBaseOption(t), palette = statusPalette(t)
    assert.equal(option.tooltip.backgroundColor, t.raised)
    assert.equal(option.xAxis.axisLabel.color, t.text3)
    assert.equal(option.yAxis.splitLine.lineStyle.color, t.divider)
    assert.equal(palette.成功, t.success)
    assert.equal(palette.失败, t.error)
    assert.equal(palette.已停止, t.warning)
    assert.ok(!Object.hasOwn(option.legend, 'selected'))
  }
  const chart = read('web/src/features/reports/ReportChart.vue')
  assert.match(chart, /:option="option" autoresize/)
  assert.doesNotMatch(chart, /registerTheme|:key=/)
})
