import type { SkinId, ThemeMode } from './skins.ts'

export const themeStorageKey = 'test-auto-pro-theme'
export const skinStorageKey = 'test-auto-pro-skin'
type AppearanceStorage = Pick<Storage, 'getItem'>

export function browserStorage(): Storage | undefined {
  try { return typeof window === 'undefined' ? undefined : window.localStorage } catch { return undefined }
}

export function readSkinId(storage?: AppearanceStorage): SkinId {
  try {
    const id = storage?.getItem(skinStorageKey)
    return id === 'deep-space' || id === 'aurora' || id === 'titanium' ? id : 'default'
  } catch { return 'default' }
}

export function readThemeMode(storage?: AppearanceStorage): ThemeMode {
  try { return storage?.getItem(themeStorageKey) === 'dark' ? 'dark' : 'light' } catch { return 'light' }
}

// 浏览器拒绝存储时仍允许当次切换；外观存储失败不能阻断编辑和执行。
export function writePreference(key: string, value: string): void {
  try { browserStorage()?.setItem(key, value) } catch { /* 当次选择继续保留在 store。 */ }
}
