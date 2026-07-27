#!/usr/bin/env bash

set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

node - "${project_root}" <<'NODE'
const fs = require('node:fs')
const path = require('node:path')

const projectRoot = process.argv[2]
const read = (relativePath) => fs.readFileSync(path.join(projectRoot, relativePath), 'utf8')
const requireText = (source, expected, label) => {
  if (!source.includes(expected)) {
    throw new Error(`${label}缺少约定内容：${expected}`)
  }
}

const store = read('web/src/stores/app.ts')
requireText(store, "export type ThemeMode = 'light' | 'dark'", '主题状态')
requireText(store, "const themeStorageKey = 'test-auto-pro-theme'", '主题持久化')
requireText(store, "getItem(themeStorageKey) === 'dark' ? 'dark' : 'light'", '主题默认值')
requireText(store, "themeMode: loadThemeMode() as ThemeMode", '主题初始化')
requireText(store, "toggleThemeMode()", '主题切换')
requireText(store, "setItem(themeStorageKey, this.themeMode)", '主题持久化')

const app = read('web/src/App.vue')
requireText(app, 'NConfigProvider, NGlobalStyle, NLayout, NLayoutContent, NLayoutHeader, NLayoutSider', '主题与布局组件')
requireText(app, 'darkTheme', '深色主题')
requireText(app, '<n-global-style />', '全局主题样式')
requireText(app, ':theme="naiveTheme"', '主题注入')
requireText(app, '@click="appStore.toggleThemeMode"', '主题切换入口')
requireText(app, '<n-layout-header class="app-header" bordered>', '官方顶栏与底部分隔线')
requireText(app, '<div class="header-toolbar">', '右侧主题工具栏')
requireText(app, '<n-layout-sider', '官方侧栏组件')
requireText(app, '          bordered', '侧栏分隔线')
requireText(app, 'v-model:collapsed="sidebarCollapsed"', '侧栏收缩状态')
requireText(app, 'collapse-mode="width"', '侧栏收缩模式')
requireText(app, 'show-trigger="arrow-circle"', '侧栏圆形收缩触发器')
requireText(app, ':collapsed-width="0"', '侧栏收缩宽度')
requireText(app, ':show-collapsed-content="false"', '收缩菜单内容')
requireText(app, '<n-layout-content class="app-main" native-scrollbar>', '官方主内容布局')
if (app.includes('项目初始化')) {
  throw new Error('应用壳不应出现“项目初始化”状态文字')
}
if (app.includes(' embedded')) {
  throw new Error('工作区不应使用嵌入色 embedded')
}
if (app.indexOf('<div class="header-toolbar">') < app.indexOf('<span class="product-name">')) {
  throw new Error('主题工具栏必须位于产品名称右侧')
}

const styles = read('web/src/styles.css')
for (const expected of [
  'height: 100dvh;',
  'overflow: hidden;',
  'grid-template-rows: 64px minmax(0, 1fr);',
  '.app-sidebar',
  '.app-sidebar .n-layout-toggle-button',
  'width: 32px;',
  'height: 32px;',
  '.app-sidebar.n-layout-sider--collapsed .n-layout-toggle-button',
  'left: 4px;',
  'transform: translateY(-50%);',
  'z-index: 2;',
  'pointer-events: auto;',
  '.app-main',
  '.app-main > .n-layout-scroll-container',
  'overflow-y: auto;',
]) {
  requireText(styles, expected, '应用壳布局')
}
if (styles.includes('background: var(--n-color);')) {
  throw new Error('普通布局样式不应依赖 Naive 组件局部背景变量')
}
NODE
