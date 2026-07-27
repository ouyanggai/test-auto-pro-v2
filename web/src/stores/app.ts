import { defineStore } from 'pinia'

export type ThemeMode = 'light' | 'dark'

const themeStorageKey = 'test-auto-pro-theme'

function loadThemeMode(): ThemeMode {
  if (typeof window === 'undefined') {
    return 'light'
  }

  return window.localStorage.getItem(themeStorageKey) === 'dark' ? 'dark' : 'light'
}

export const useAppStore = defineStore('app', {
  state: () => ({
    productName: '流程自动化测试平台',
    themeMode: loadThemeMode() as ThemeMode,
  }),
  actions: {
    toggleThemeMode() {
      this.themeMode = this.themeMode === 'light' ? 'dark' : 'light'
      window.localStorage.setItem(themeStorageKey, this.themeMode)
    },
  },
})
