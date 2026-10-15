<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NIcon, NPopover, NRadioButton, NRadioGroup } from 'naive-ui'
import { CheckmarkOutline, ColorPaletteOutline } from '@vicons/ionicons5'
import { skinCatalog, skinTokens } from '../appearance/skins'
import type { SkinId } from '../appearance/skins'
import { useAppStore } from '../stores/app'

const store = useAppStore()
const previews = computed(() => Object.fromEntries(skinCatalog.map(({ id }) => {
  const t = skinTokens[id][store.themeMode]
  return [id, {
    '--preview-bg': t.bg, '--preview-chrome': t.chrome, '--preview-surface': t.surface,
    '--preview-primary': t.primary, '--preview-border': t.border,
    '--preview-a': t.glowA, '--preview-b': t.glowB, '--preview-radius': Math.ceil(t.radiusCard / 3) + 'px',
  }]
})))

// 方向键在同一组皮肤中循环并移动焦点；不关闭弹层，便于连续比较八种组合。
function handleKey(event: KeyboardEvent, id: SkinId) {
  const offset = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 0
  if (!offset) return
  event.preventDefault()
  const next = skinCatalog[(skinCatalog.findIndex(skin => skin.id === id) + offset + skinCatalog.length) % skinCatalog.length]!
  store.setSkinId(next.id)
  const group = (event.currentTarget as HTMLElement).parentElement
  group?.querySelector<HTMLButtonElement>('[data-skin-choice="' + next.id + '"]')?.focus()
}
</script>

<template>
  <n-popover trigger="click" placement="bottom-end" :show-arrow="false" :width="392" class="appearance-popover">
    <template #trigger>
      <n-button quaternary size="small" aria-label="选择外观">
        <template #icon><n-icon :component="ColorPaletteOutline" /></template>
        外观
      </n-button>
    </template>
    <div class="appearance-menu">
      <div class="appearance-menu__label">皮肤</div>
      <div class="appearance-menu__grid" role="radiogroup" aria-label="皮肤">
        <button v-for="skin in skinCatalog" :key="skin.id" type="button" role="radio"
          class="appearance-menu__choice" :class="{ 'is-selected': store.skinId === skin.id }"
          :aria-checked="store.skinId === skin.id" :tabindex="store.skinId === skin.id ? 0 : -1"
          :data-skin-choice="skin.id" @click="store.setSkinId(skin.id)" @keydown="handleKey($event, skin.id)">
          <span class="appearance-preview" :style="previews[skin.id]">
            <i class="appearance-preview__header"></i><i class="appearance-preview__sidebar"></i>
            <i class="appearance-preview__card"></i><i class="appearance-preview__card appearance-preview__card--second"></i>
            <i class="appearance-preview__dot"></i>
          </span>
          <span class="appearance-menu__name">{{ skin.name }}</span>
          <span class="appearance-menu__description">{{ skin.description }}</span>
          <span v-if="store.skinId === skin.id" class="appearance-menu__check"><n-icon :component="CheckmarkOutline" /></span>
        </button>
      </div>
      <div class="appearance-menu__label appearance-menu__label--mode">明暗</div>
      <n-radio-group :value="store.themeMode" size="small" aria-label="明暗模式" @update:value="store.setThemeMode">
        <n-radio-button value="light">浅色</n-radio-button><n-radio-button value="dark">深色</n-radio-button>
      </n-radio-group>
    </div>
  </n-popover>
</template>

<style scoped>
.appearance-menu { padding: 4px; }
.appearance-menu__label { margin-bottom: 12px; color: var(--text-color-3); font-size: 12px; }
.appearance-menu__label--mode { margin-top: 24px; }
.appearance-menu__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.appearance-menu__choice { position: relative; display: flex; flex-direction: column; min-width: 0; padding: 0 0 8px; text-align: left; font: inherit; color: var(--text-color-1); background: var(--ta-reading); border: 1px solid var(--divider-color); border-radius: 8px; cursor: pointer; }
.appearance-menu__choice.is-selected { outline: 2px solid var(--primary-color); outline-offset: 1px; }
.appearance-menu__choice:focus-visible { outline: 2px solid var(--primary-color); outline-offset: 3px; }
.appearance-menu__name { margin: 8px 8px 0; font-size: 14px; font-weight: 600; }
.appearance-menu__description { margin: 0 8px; color: var(--text-color-3); font-size: 12px; line-height: 20px; white-space: nowrap; }
.appearance-menu__check { position: absolute; right: 8px; top: 8px; display: grid; place-items: center; width: 16px; height: 16px; border-radius: 50%; background: var(--primary-color); color: var(--ta-on-primary); }
.appearance-preview { position: relative; display: block; width: 100%; height: 76px; overflow: hidden; border-radius: 7px 7px 0 0; background: radial-gradient(ellipse at top right, var(--preview-a), transparent), radial-gradient(ellipse at bottom left, var(--preview-b), transparent), var(--preview-bg); }
.appearance-preview i { position: absolute; display: block; pointer-events: none; }
.appearance-preview__header { inset: 0 0 auto; height: 10px; background: var(--preview-chrome); border-bottom: 1px solid var(--preview-border); }
.appearance-preview__sidebar { inset: 10px auto 0 0; width: 28px; background: var(--preview-chrome); border-right: 1px solid var(--preview-border); }
.appearance-preview__card { left: 40px; top: 22px; width: 64%; height: 16px; border: 1px solid var(--preview-border); border-radius: var(--preview-radius); background: var(--preview-surface); }
.appearance-preview__card--second { top: 46px; width: 45%; }
.appearance-preview__dot { left: 10px; top: 24px; width: 8px; height: 8px; border-radius: 50%; background: var(--preview-primary); }
</style>
