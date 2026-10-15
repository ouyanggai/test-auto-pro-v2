<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { navigationItems } from '../appearance/navigation'
import { studioSpring } from '../appearance/studioMotion'

const props = defineProps<{ activePath: string; skin: 'aurora' | 'titanium' }>()
const element = ref<HTMLElement>()
const indicator = ref<HTMLElement>()
let observer: ResizeObserver | undefined
let animation: Animation | undefined
let ready = false

// 连续定位同一个指示面，尺寸变化时重测；快速切换从当前画面接续，不叠加动画。
async function align(animate = true) {
  await nextTick()
  const nav = element.value, pill = indicator.value
  const selected = nav?.querySelector<HTMLElement>('[aria-current="page"]')
  if (!nav || !pill || !selected || !nav.isConnected) return
  const bounds = nav.getBoundingClientRect(), next = selected.getBoundingClientRect(), previous = pill.getBoundingClientRect()
  animation?.cancel()
  pill.style.width = next.width + 'px'
  pill.style.height = next.height + 'px'
  pill.style.left = next.left - bounds.left + 'px'
  pill.style.top = next.top - bounds.top + 'px'
  pill.style.opacity = '1'
  if (animate && ready && previous.width && previous.height) {
    animation = pill.animate([
      { transform: `translate(${previous.left - next.left}px, ${previous.top - next.top}px) scale(${previous.width / next.width}, ${previous.height / next.height})` },
      { transform: 'translate(0, 0) scale(1, 1)' },
    ], { duration: 600, easing: studioSpring })
    // 快速换页和卸载都会取消旧动画，取消属于正常接续，不能冒出未处理的 Promise 错误。
    void animation.finished.catch(() => {})
  }
  ready = true
}

// 方向键只移动焦点，回车仍由真正的 RouterLink 导航，保留浏览器链接行为。
function handleKey(event: KeyboardEvent) {
  const offset = ['ArrowDown', 'ArrowRight'].includes(event.key) ? 1 : ['ArrowUp', 'ArrowLeft'].includes(event.key) ? -1 : 0
  if (!offset) return
  const links = Array.from(element.value?.querySelectorAll<HTMLAnchorElement>('a') ?? [])
  const index = links.indexOf(event.target as HTMLAnchorElement)
  if (index < 0) return
  event.preventDefault()
  links[(index + offset + links.length) % links.length]?.focus()
}
watch(() => [props.activePath, props.skin], () => void align())
onMounted(() => {
  observer = new ResizeObserver(() => void align(false))
  if (element.value) observer.observe(element.value)
  void align(false)
})
onBeforeUnmount(() => { observer?.disconnect(); animation?.cancel() })
</script>

<template>
  <nav ref="element" class="studio-navigation" aria-label="主导航" @keydown="handleKey">
    <span ref="indicator" class="studio-navigation__indicator" aria-hidden="true"></span>
    <RouterLink v-for="(item, index) in navigationItems" :key="item.path" :to="item.path"
      class="studio-navigation__link" :aria-current="activePath === item.path ? 'page' : undefined">
      <svg class="studio-navigation__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <template v-if="item.symbol === 'plans'"><rect x="5" y="4" width="14" height="17" rx="3"/><path d="M9 3h6v4H9zM9 12h6M9 16h4"/></template>
        <template v-else-if="item.symbol === 'runs'"><circle cx="12" cy="12" r="9"/><path d="m10 8 6 4-6 4z"/></template>
        <template v-else-if="item.symbol === 'reports'"><path d="M4 4v16h16M8 15v-4M13 15V7M18 15v-6"/></template>
        <template v-else><path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3"/><circle cx="15" cy="17" r="3"/></template>
      </svg>
      <span>{{ item.label }}</span><small class="studio-navigation__index" aria-hidden="true">0{{ index + 1 }}</small>
    </RouterLink>
  </nav>
</template>
