<script setup lang="ts">
import { computed, h, nextTick, onMounted, ref, watch } from 'vue'
import { NButton, NConfigProvider, NGlobalStyle, NLayout, NLayoutContent, NLayoutHeader, NLayoutSider, NMenu, NMessageProvider, NNotificationProvider, darkTheme, dateZhCN, zhCN } from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import { RouterLink, RouterView, useRoute } from 'vue-router'

import { useAppStore } from './stores/app'
import { buildThemeOverrides } from './appearance/naiveTheme'
import { applyAppearance } from './appearance/applyAppearance'
import AppearanceMenu from './components/AppearanceMenu.vue'
import AmbientLayer from './components/AmbientLayer.vue'
import { startPointerGlow } from './appearance/pointerGlow'
import { switchAppearance } from './appearance/transition'
import { navigationItems } from './appearance/navigation'
import { startStudioMotion } from './appearance/studioMotion'
import StudioNavigation from './components/StudioNavigation.vue'

const route = useRoute()
const appStore = useAppStore()

const menuOptions: MenuOption[] = navigationItems.map(item => ({
  label: () => h(RouterLink, { to: item.path }, { default: () => item.label }), key: item.path,
}))
const studioSkin = computed(() => appStore.skinId === 'aurora' || appStore.skinId === 'titanium' ? appStore.skinId : null)
let studioMotion: ReturnType<typeof startStudioMotion> | undefined

const selectedKey = computed(() => {
  if (route.path.startsWith('/plans')) return '/plans'
  if (route.path.startsWith('/runs')) return '/runs'
  if (route.path.startsWith('/reports')) return '/reports'
  if (route.path.startsWith('/settings')) return '/settings'
  return route.path
})
// 运行详情是画布优先的工作台页：内容区去掉外层留白并禁用整页滚动，
// 把顶栏以下的高度整块交给流程图（页面自己管纵向分区）。
const flushContent = computed(() => /^\/runs\/[^/]+\/paths\/[^/]+$/.test(route.path))
const naiveTheme = computed(() => (appStore.themeMode === 'dark' ? darkTheme : null))
const themeOverrides = computed(() => buildThemeOverrides(appStore.skinId, appStore.themeMode))
watch([() => appStore.skinId, () => appStore.themeMode], ([skin, mode]) => applyAppearance(skin, mode), { immediate: true })
onMounted(() => {
  // 主题变量先更新，随后再读取光晕尺寸；监听只属于当前皮肤，切走立即释放。
  watch(() => appStore.skinId, (skin, _previous, onCleanup) => {
    if (skin !== 'deep-space') return
    const spot = document.querySelector<HTMLElement>('.app-ambient__spot')
    if (spot) onCleanup(startPointerGlow(spot))
  }, { immediate: true, flush: 'post' })
  watch(studioSkin, (skin, _previous, onCleanup) => {
    const content = document.querySelector<HTMLElement>('.app-main')
    if (!skin || !content) return
    studioMotion = startStudioMotion(document.documentElement, content)
    onCleanup(() => { studioMotion?.stop(); studioMotion = undefined })
  }, { immediate: true, flush: 'post' })
})
watch(() => route.path, async () => { await nextTick(); studioMotion?.enterPage() })
const themeToggleLabel = computed(() => (appStore.themeMode === 'dark' ? '切换为浅色主题' : '切换为深色主题'))
const sidebarCollapsed = ref(false)
</script>

<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="themeOverrides" :locale="zhCN" :date-locale="dateZhCN">
    <AmbientLayer />
    <n-message-provider>
      <n-notification-provider>
        <n-global-style />
        <n-layout class="app-shell" :class="{ 'app-shell--sider-collapsed': sidebarCollapsed && !studioSkin }" native-scrollbar>
          <n-layout-header class="app-header" bordered>
            <span class="product-name"><svg v-if="studioSkin" class="studio-brand-mark" viewBox="0 0 32 32" fill="none" aria-hidden="true"><rect x="3" y="3" width="26" height="26" rx="9" stroke="currentColor"/><path d="m10 20 6-10 6 10M12 17h8" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>{{ appStore.productName }}</span>
            <!-- 顶栏上下文位：当前页面把返回入口与所在对象（如运行编号、计划/路径）挂到这里，
                 页面内不再重复一条页头，操作区因此能拿到更多横向空间。 -->
            <div id="app-header-context" class="header-context" />
            <div class="header-toolbar">
              <AppearanceMenu />
              <n-button
                quaternary
                size="small"
                class="theme-toggle"
                :aria-label="themeToggleLabel"
                @click="switchAppearance(appStore.toggleThemeMode)"
              >
                {{ appStore.themeMode === 'dark' ? '浅色' : '深色' }}
              </n-button>
            </div>
          </n-layout-header>

          <n-layout class="app-workspace" has-sider native-scrollbar>
            <n-layout-sider
              :collapsed="studioSkin ? false : sidebarCollapsed"
              @update:collapsed="sidebarCollapsed = $event"
              class="app-sidebar"
              bordered
              collapse-mode="width"
              :show-trigger="studioSkin ? false : 'arrow-circle'"
              :width="240"
              :collapsed-width="0"
              :show-collapsed-content="false"
              content-class="app-sidebar-content"
            >
              <StudioNavigation v-if="studioSkin" :active-path="selectedKey" :skin="studioSkin" />
              <nav v-else aria-label="主导航">
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
