import { defineStore } from 'pinia'
import { browserStorage, readSkinId, readThemeMode, skinStorageKey, themeStorageKey, writePreference } from '../appearance/preferences.ts'
import type { SkinId, ThemeMode } from '../appearance/skins'
export type { SkinId, ThemeMode } from '../appearance/skins'

export const useAppStore = defineStore('app', {
  state: () => ({
    productName: '流程自动化测试平台',
    themeMode: readThemeMode(browserStorage()),
    skinId: readSkinId(browserStorage()),
  }),
  actions: {
    toggleThemeMode() {
      this.setThemeMode(this.themeMode === 'light' ? 'dark' : 'light')
    },
    setThemeMode(mode: ThemeMode) {
      this.themeMode = mode
      writePreference(themeStorageKey, mode)
    },
    setSkinId(id: SkinId) {
      this.skinId = id
      writePreference(skinStorageKey, id)
    },
  },
})
