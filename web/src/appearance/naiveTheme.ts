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
  // 浅底按钮按文字色再叠半透明底色，组件库默认的 28% 按下底色会吃掉自定义状态色的对比度。
  const button: NonNullable<GlobalThemeOverrides['Button']> = {
    colorOpacitySecondary: '.10', colorOpacitySecondaryHover: '.14', colorOpacitySecondaryPressed: '.18',
    colorSecondary: alpha(t.text1, .04), colorSecondaryHover: alpha(t.text1, .06), colorSecondaryPressed: alpha(t.text1, .08),
    colorTertiary: alpha(t.text1, .04), colorTertiaryHover: alpha(t.text1, .06), colorTertiaryPressed: alpha(t.text1, .08),
    colorQuaternary: 'transparent', colorQuaternaryHover: alpha(t.text1, .06), colorQuaternaryPressed: alpha(t.text1, .08),
  }
  for (const name of ['primary', 'info', 'success', 'warning', 'error'] as const) {
    // 浅色的白字实心按钮和状态文字都不能在悬停时变淡；深色仍以提亮保持反馈。
    // 输入框和选择器沿用公共状态色；按钮还需按实际叠底单独处理。
    const hover = mode === 'light' ? mix(t[name], 'rgb(0, 0, 0)', .08) : lift(t[name])
    Object.assign(common, {
      [name + 'Color']: t[name], [name + 'ColorHover']: hover,
      [name + 'ColorPressed']: sink(t[name]), [name + 'ColorSuppl']: hover,
    })
    const suffix = name[0]!.toUpperCase() + name.slice(1)
    // Naive 的浅底、三级和四级按钮取 colorX 当文字，无法单独覆盖 textColorX；
    // 在按钮主题内增强该颜色，并从同一基色派生实心按钮各状态，不改变图表、节点和全局令牌。
    const buttonColor = mode === 'light' ? sink(t[name]) : mix(t[name], 'rgb(255, 255, 255)', .10)
    const buttonHover = mode === 'light' ? sink(buttonColor) : lift(buttonColor)
    const buttonPressed = mix(buttonColor, 'rgb(0, 0, 0)', mode === 'light' ? .24 : .08)
    Object.assign(button, {
      ['color' + suffix]: buttonColor, ['colorHover' + suffix]: buttonHover,
      ['colorFocus' + suffix]: buttonHover,
      ['colorPressed' + suffix]: buttonPressed,
    })
    // 弹窗抬高面和工具条底色也参与对比；六种按钮共用这组文字色，不能只在白底验证。
    for (const kind of ['Ghost', 'Text']) for (const [state, color] of [
      ['', buttonColor], ['Hover', buttonHover], ['Focus', buttonHover], ['Pressed', buttonPressed],
    ]) Object.assign(button, { ['textColor' + kind + state + suffix]: color })
    // 深色的高亮实心按钮必须使用深色文字。
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
