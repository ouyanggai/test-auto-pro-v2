<script setup lang="ts">
import { computed, h, ref } from 'vue'
import { NButton, NConfigProvider, NGlobalStyle, NLayout, NLayoutSider, NMenu, darkTheme, dateZhCN, zhCN } from 'naive-ui'
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
const sidebarCollapsed = ref(false)
</script>

<template>
  <n-config-provider :theme="naiveTheme" :locale="zhCN" :date-locale="dateZhCN">
    <n-global-style />
    <div class="app-shell">
      <header class="app-header">
        <span class="product-name">{{ appStore.productName }}</span>
        <div class="header-toolbar">
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
      </header>

      <n-layout class="app-workspace" embedded has-sider native-scrollbar>
        <n-layout-sider
          v-model:collapsed="sidebarCollapsed"
          class="app-sidebar"
          bordered
          collapse-mode="width"
          show-trigger="arrow-circle"
          :width="240"
          :collapsed-width="0"
          :show-collapsed-content="false"
          content-class="app-sidebar-content"
        >
          <nav aria-label="主导航">
            <n-menu :options="menuOptions" :value="selectedKey" />
          </nav>
        </n-layout-sider>
        <main class="app-main">
          <router-view />
        </main>
      </n-layout>
    </div>
  </n-config-provider>
</template>
