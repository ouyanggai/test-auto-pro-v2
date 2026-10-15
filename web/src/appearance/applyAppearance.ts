import { monoStack, skinTokens } from './skins.ts'
import type { SkinId, ThemeMode } from './skins.ts'

export function appearanceVariables(skin: SkinId, mode: ThemeMode): Record<string, string> {
  const t = skinTokens[skin][mode]
  return {
    '--text-color-1': t.text1, '--text-color-2': t.text2, '--text-color-3': t.text3,
    '--divider-color': t.divider, '--border-color': t.border, '--card-color': t.reading, '--body-color': t.bg,
    '--hover-color': t.hover, '--table-color-hover': t.hover, '--primary-color': t.primary,
    '--success-color': t.success, '--warning-color': t.warning, '--error-color': t.error, '--info-color': t.info,
    '--font-family-mono': monoStack, '--border-radius': t.radiusControl + 'px',
    '--ta-bg': t.bg, '--ta-chrome': t.chrome, '--ta-surface': t.surface, '--ta-surface-2': t.surface2,
    '--ta-reading': t.reading, '--ta-raised': t.raised, '--ta-border-strong': t.borderStrong,
    '--ta-on-primary': t.onPrimary, '--ta-on-status': t.onStatus, '--ta-accent-2': t.accent2,
    '--ta-glow-a': t.glowA, '--ta-glow-b': t.glowB, '--ta-glow-c': t.glowC,
    '--ta-shadow-1': t.shadow1, '--ta-shadow-2': t.shadow2, '--ta-shadow-3': t.shadow3,
    '--ta-radius-control': t.radiusControl + 'px', '--ta-radius-card': t.radiusCard + 'px', '--ta-radius-node': t.radiusNode + 'px',
    '--ta-ease': 'cubic-bezier(.2,.8,.2,1)',
  }
}

// 根节点上的变量同时供路由内容和传送到 body 的浮层使用，不触碰业务组件生命周期。
export function applyAppearance(skin: SkinId, mode: ThemeMode, root: HTMLElement = document.documentElement): void {
  root.dataset.skin = skin
  root.dataset.mode = mode
  root.style.colorScheme = mode
  for (const [key, value] of Object.entries(appearanceVariables(skin, mode))) root.style.setProperty(key, value)
}
