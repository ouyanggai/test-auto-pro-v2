import { sansStack } from '../../appearance/skins.ts'
import type { SkinTokens } from '../../appearance/skins.ts'

// 只更新图表 option，不注册主题或重建实例；不传 legend.selected，保留用户的图例选择。
export function chartBaseOption(t: SkinTokens) {
  return {
    color: [t.primary, t.info, t.accent2, t.success, t.warning],
    textStyle: { color: t.text2, fontFamily: sansStack },
    legend: { textStyle: { color: t.text2 } },
    tooltip: { backgroundColor: t.raised, borderColor: t.border, textStyle: { color: t.text1 }, extraCssText: 'box-shadow:' + t.shadow2 + ';border-radius:' + t.radiusControl + 'px;' },
    xAxis: { axisLine: { lineStyle: { color: t.border } }, axisLabel: { color: t.text3 }, splitLine: { lineStyle: { color: t.divider } } },
    yAxis: { axisLine: { lineStyle: { color: t.border } }, axisLabel: { color: t.text3 }, splitLine: { lineStyle: { color: t.divider } } },
  }
}

export function statusPalette(t: SkinTokens): Record<string, string> {
  return { 成功: t.success, 失败: t.error, 已停止: t.warning, 已取消: t.text3, 结果待确认: t.info }
}
