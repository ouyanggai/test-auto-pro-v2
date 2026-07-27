<script setup lang="ts">
import { computed, h } from 'vue'
import { NConfigProvider, NLayout, NLayoutContent, NLayoutHeader, NLayoutSider, NMenu, zhCN, dateZhCN } from 'naive-ui'
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
</script>

<template>
  <n-config-provider :locale="zhCN" :date-locale="dateZhCN">
    <n-layout class="app-shell" has-sider>
      <n-layout-sider class="app-sidebar" :width="224" bordered>
        <div class="product-name">{{ appStore.productName }}</div>
        <n-menu :options="menuOptions" :value="selectedKey" />
      </n-layout-sider>
      <n-layout>
        <n-layout-header class="app-header" bordered>项目初始化</n-layout-header>
        <n-layout-content class="app-content">
          <router-view />
        </n-layout-content>
      </n-layout>
    </n-layout>
  </n-config-provider>
</template>
