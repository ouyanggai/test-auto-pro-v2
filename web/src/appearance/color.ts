// 只接受令牌表使用的颜色格式；错误配置直接暴露，避免静默生成不可读的界面。
export function parseColor(color: string): [number, number, number, number] {
  const hex = /^#([\da-f]{3}|[\da-f]{6})$/i.exec(color.trim())
  if (hex) {
    const digits = hex[1]!.length === 3 ? [...hex[1]!].map(c => c + c).join('') : hex[1]!
    return [0, 2, 4].map(i => parseInt(digits.slice(i, i + 2), 16)).concat(1) as [number, number, number, number]
  }
  const rgb = /^rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)(?:\s*,\s*([\d.]+))?\s*\)$/.exec(color)
  if (!rgb) throw new Error('主题颜色格式错误：' + color)
  return [Number(rgb[1]), Number(rgb[2]), Number(rgb[3]), rgb[4] === undefined ? 1 : Number(rgb[4])]
}

export function mix(a: string, b: string, ratio: number): string {
  const left = parseColor(a), right = parseColor(b)
  const weight = Math.min(1, Math.max(0, ratio))
  const channels = left.slice(0, 3).map((value, index) => Math.round(value * (1 - weight) + right[index]! * weight))
  return 'rgb(' + channels.join(', ') + ')'
}

export function lift(hex: string): string { return mix(hex, 'rgb(255, 255, 255)', .14) }
export function sink(hex: string): string { return mix(hex, 'rgb(0, 0, 0)', .16) }
export function alpha(hex: string, opacity: number): string {
  return 'rgba(' + parseColor(hex).slice(0, 3).join(', ') + ', ' + Math.min(1, Math.max(0, opacity)) + ')'
}
