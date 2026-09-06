<script setup lang="ts">
import { computed, h, ref } from 'vue'
import { NButton, NConfigProvider, NGlobalStyle, NLayout, NLayoutContent, NLayoutHeader, NLayoutSider, NMenu, NMessageProvider, NNotificationProvider, darkTheme, dateZhCN, zhCN } from 'naive-ui'
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

const selectedKey = computed(() => (route.path.startsWith('/plans') ? '/plans' : route.path))
// 运行详情是画布优先的工作台页：内容区去掉外层留白并禁用整页滚动，
// 把顶栏以下的高度整块交给流程图（页面自己管纵向分区）。
const flushContent = computed(() => /^\/runs\/[^/]+$/.test(route.path))
const naiveTheme = computed(() => (appStore.themeMode === 'dark' ? darkTheme : null))
const themeToggleLabel = computed(() => (appStore.themeMode === 'dark' ? '切换为浅色主题' : '切换为深色主题'))
const sidebarCollapsed = ref(false)
</script>

<template>
  <n-config-provider :theme="naiveTheme" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-notification-provider>
        <n-global-style />
        <n-layout class="app-shell" native-scrollbar>
          <n-layout-header class="app-header" bordered>
            <span class="product-name">{{ appStore.productName }}</span>
            <!-- 顶栏上下文位：当前页面把返回入口与所在对象（如运行编号、计划/路径）挂到这里，
                 页面内不再重复一条页头，操作区因此能拿到更多横向空间。 -->
            <div id="app-header-context" class="header-context" />
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
          </n-layout-header>

          <n-layout class="app-workspace" has-sider native-scrollbar>
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
            <n-layout-content class="app-main" :class="{ 'app-main--flush': flushContent }" native-scrollbar>
              <router-view />
            </n-layout-content>
          </n-layout>
        </n-layout>
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>
