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
requireText(app, 'NConfigProvider, NGlobalStyle', '主题组件')
requireText(app, 'darkTheme', '深色主题')
requireText(app, '<n-global-style />', '全局主题样式')
requireText(app, ':theme="naiveTheme"', '主题注入')
requireText(app, '@click="appStore.toggleThemeMode"', '主题切换入口')
requireText(app, '<header class="app-header">', '顶栏布局')
requireText(app, '<aside class="app-sidebar"', '侧栏布局')
requireText(app, '<main class="app-main">', '主内容布局')

const styles = read('web/src/styles.css')
for (const expected of [
  'height: 100dvh;',
  'overflow: hidden;',
  'grid-template-rows: 64px minmax(0, 1fr);',
  'grid-template-columns: 240px minmax(0, 1fr);',
  '.app-sidebar',
  '.app-main',
  'overflow-y: auto;',
  'border-bottom: 1px solid var(--n-border-color);',
  'border-right: 1px solid var(--n-border-color);',
  'background: var(--n-color);',
]) {
  requireText(styles, expected, '应用壳布局')
}
NODE
