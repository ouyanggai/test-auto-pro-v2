import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import test from 'node:test'
import { skinCatalog } from '../../../../web/src/appearance/skins.ts'
import { buildThemeOverrides } from '../../../../web/src/appearance/naiveTheme.ts'

const requireWeb = createRequire(new URL('../../../../web/package.json', import.meta.url))

test('八种外观下真实懒加载图表有自己的有限高度，渲染器不能反过来撑高容器', async () => {
  const { parse, compileScript } = requireWeb('vue/compiler-sfc')
  const { createSSRApp, h } = requireWeb('vue')
  const { renderToString } = requireWeb('vue/server-renderer')
  const { NConfigProvider, darkTheme } = requireWeb('naive-ui')
  const ts = requireWeb('typescript')
  const postcss = createRequire(requireWeb.resolve('vite'))('postcss')
  const source = readFileSync(new URL('../../../../web/src/features/reports/ReportChart.vue', import.meta.url), 'utf8')
  const descriptor = parse(source).descriptor
  const script = compileScript(descriptor, { id: 'report-chart-layout-test', inlineTemplate: true })
  const code = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
  const exports = {}
  // 编译真实 ReportChart，异步包入口接真实 vue-echarts；不手写一个自认为正确的图表 DOM 替身。
  vm.runInNewContext(code, { exports, require: name => name === './echarts' ? { VChart: requireWeb('vue-echarts').default } : requireWeb(name) })
  const css = postcss.parse(descriptor.styles.map(style => style.content).join('\n'))
  const attr = (node, name) => node.props?.find(prop => prop.type === 6 && prop.name === name)?.value?.content
  const classes = node => (attr(node, 'class') ?? '').split(/\s+/)
  const find = (node, className) => classes(node).includes(className) ? node : (node.children ?? []).map(child => find(child, className)).find(Boolean)
  const stylesFor = node => {
    const values = {}
    css.walkRules(rule => {
      if (!rule.selectors.some(selector => /^\.[\w-]+$/.test(selector) && classes(node).includes(selector.slice(1)))) return
      rule.walkDecls(declaration => { values[declaration.prop] = declaration.value })
    })
    return values
  }
  for (const skin of skinCatalog) for (const mode of ['light', 'dark']) {
    const warnings = []
    const app = createSSRApp({ render: () => h(NConfigProvider, { theme: mode === 'dark' ? darkTheme : null, themeOverrides: buildThemeOverrides(skin.id, mode) }, {
      default: () => h(exports.default, { option: { series: [{ type: 'bar', data: [6750, 20000] }] }, class: 'parent-chart-probe' }),
    }) })
    app.config.warnHandler = message => warnings.push(message)
    const html = await renderToString(app)
    const frame = find(parse('<template>' + html + '</template>').descriptor.template.ast, 'report-chart')
    const label = skin.id + '/' + mode
    assert.ok(frame, label)
    assert.equal(frame.tag, 'div', label + ' 必须是独立的尺寸容器')
    assert.ok(classes(frame).includes('parent-chart-probe'), label + ' 父级属性必须能正常继承')
    const bounds = stylesFor(frame)
    assert.match(bounds.height, /^\d+(?:\.\d+)?px$/, label + ' 根容器不能依赖未定高父级的百分比高度')
    assert.ok(parseFloat(bounds.height) >= 160 && parseFloat(bounds.height) <= 320, label + ' 绘图区必须有合理且有限的高度')
    assert.equal(bounds.position, 'relative')
    const canvas = frame.children.find(node => node.tag === 'x-vue-echarts')
    assert.ok(canvas, label + ' 必须继续使用真实图表组件')
    const drawing = stylesFor(canvas)
    assert.equal(drawing.position, 'absolute', label + ' 渲染器尺寸不能参与父容器高度计算')
    assert.equal(drawing.inset, '0')
    assert.equal(drawing.height, '100%')
    assert.deepEqual(warnings, [], label + ' 不得再丢失父级样式类')
  }
})
