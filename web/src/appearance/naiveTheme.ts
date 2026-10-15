import type { GlobalThemeOverrides } from 'naive-ui'
import { alpha, lift, mix, sink } from './color.ts'
import { monoStack, skinTokens } from './skins.ts'
import type { SkinId, ThemeMode } from './skins.ts'

export function buildThemeOverrides(skin: SkinId, mode: ThemeMode): GlobalThemeOverrides | undefined {
  if (skin === 'default') return undefined
  const t = skinTokens[skin][mode]
  const common: NonNullable<GlobalThemeOverrides['common']> = {
    bodyColor: t.bg, cardColor: t.reading, modalColor: t.raised, popoverColor: t.raised, tableColor: t.reading,
    actionColor: t.surface2, hoverColor: t.hover, inputColor: mode === 'dark' ? mix(t.reading, 'rgb(255, 255, 255)', .05) : 'rgb(255, 255, 255)',
    textColorBase: t.text1, textColor1: t.text1, textColor2: t.text2, textColor3: t.text3,
    textColorDisabled: alpha(t.text3, .6), placeholderColor: t.text3,
    borderColor: t.border, dividerColor: t.divider,
    borderRadius: t.radiusControl + 'px', borderRadiusSmall: Math.max(t.radiusControl - 2, 2) + 'px',
    boxShadow1: t.shadow1, boxShadow2: t.shadow2, boxShadow3: t.shadow3, fontFamilyMono: monoStack,
  }
  const button: NonNullable<GlobalThemeOverrides['Button']> = {}
  for (const name of ['primary', 'info', 'success', 'warning', 'error'] as const) {
    Object.assign(common, {
      [name + 'Color']: t[name], [name + 'ColorHover']: lift(t[name]),
      [name + 'ColorPressed']: sink(t[name]), [name + 'ColorSuppl']: lift(t[name]),
    })
    const suffix = name[0]!.toUpperCase() + name.slice(1)
    // 深色的高亮按钮必须使用深色文字；幽灵和文字按钮仍由 Naive 按状态色生成。
    for (const state of ['', 'Hover', 'Pressed', 'Focus', 'Disabled']) {
      Object.assign(button, { ['textColor' + state + suffix]: name === 'primary' ? t.onPrimary : t.onStatus })
    }
  }
  return {
    common, Button: button,
    Layout: { color: 'rgba(0, 0, 0, 0)', colorEmbedded: 'rgba(0, 0, 0, 0)', headerColor: t.chrome, siderColor: t.chrome, headerBorderColor: t.divider, siderBorderColor: t.divider },
    Card: { color: t.surface, colorModal: t.raised, colorPopover: t.raised, colorEmbedded: t.surface2, borderColor: t.divider, borderRadius: t.radiusCard + 'px' },
    Dialog: { color: t.raised, borderRadius: t.radiusCard + 'px' },
    DataTable: { thColor: t.surface2, thTextColor: t.text2, tdColor: t.reading, tdColorHover: t.hover, borderColor: t.divider, thColorHover: t.hover },
    Menu: { itemColorActive: alpha(t.primary, .14), itemColorActiveHover: alpha(t.primary, .20), itemColorActiveCollapsed: alpha(t.primary, .14), itemTextColorActive: t.primary, itemTextColorActiveHover: t.primary, itemIconColorActive: t.primary, itemIconColorActiveHover: t.primary, arrowColorActive: t.primary },
    Tabs: { tabTextColorActiveLine: t.primary, tabTextColorHoverLine: t.primary, barColor: t.primary },
  }
}
