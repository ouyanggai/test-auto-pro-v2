<script setup lang="ts">
import { computed, h } from 'vue'
import { NButton, NConfigProvider, NGlobalStyle, NMenu, darkTheme, dateZhCN, zhCN } from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import { RouterLink, RouterView, useRoute } from 'vue-router'

import { useAppStore } from './stores/app'

const route = useRoute()
const appStore = useAppStore()

const menuOptions: MenuOption[] = [
  { label: () => h(RouterLink, { to: '/plans' }, { default: () => '测试计划' }), key: '/plans' },
  { label: () => h(RouterLink, { to: '/runs' }, { default: () => '运行记录' }), key: '/runs' },
  { label: () => h(RouterLink, { to: '/settings' }, { default: () => '系统设置' }), key: '/settings' },
]

const selectedKey = computed(() => route.path)
const naiveTheme = computed(() => (appStore.themeMode === 'dark' ? darkTheme : null))
const themeToggleLabel = computed(() => (appStore.themeMode === 'dark' ? '切换为浅色主题' : '切换为深色主题'))
</script>

<template>
  <n-config-provider :theme="naiveTheme" :locale="zhCN" :date-locale="dateZhCN">
    <n-global-style />
    <div class="app-shell">
      <header class="app-header">
        <div class="product-brand">
          <span class="product-name">{{ appStore.productName }}</span>
          <n-button
            quaternary
            size="small"
            class="theme-toggle"
            :aria-label="themeToggleLabel"
            @click="appStore.toggleThemeMode"
          >
            {{ appStore.themeMode === 'dark' ? '浅色' : '深色' }}
          </n-button>
        </div>
        <span class="header-context">项目初始化</span>
      </header>

      <div class="app-workspace">
        <aside class="app-sidebar" aria-label="主导航">
          <n-menu :options="menuOptions" :value="selectedKey" />
        </aside>
        <main class="app-main">
          <router-view />
        </main>
      </div>
    </div>
  </n-config-provider>
</template>
