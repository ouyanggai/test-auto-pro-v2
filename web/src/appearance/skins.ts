// 颜色的唯一来源；默认值对照 Naive 公共主题，新皮肤按已批准视觉规格维护。
export type ThemeMode = 'light' | 'dark'
export type SkinId = 'default' | 'deep-space' | 'aurora' | 'titanium'

export interface SkinTokens {
  bg: string
  chrome: string
  surface: string
  surface2: string
  reading: string
  raised: string
  text1: string
  text2: string
  text3: string
  border: string
  borderStrong: string
  divider: string
  hover: string
  primary: string
  accent2: string
  success: string
  warning: string
  error: string
  info: string
  shadow1: string
  shadow2: string
  shadow3: string
  onPrimary: string
  onStatus: string
  glowA: string
  glowB: string
  glowC: string
  radiusControl: number
  radiusCard: number
  radiusNode: number
}

export const sansStack = 'Inter, "PingFang SC", "Microsoft YaHei", sans-serif'
export const monoStack = '"JetBrains Mono", "Cascadia Mono", "Cascadia Code", Consolas, "Sarasa Mono SC", monospace'

export const skinCatalog: ReadonlyArray<{ id: SkinId; name: string; description: string }> = [
  {
    "id": "default",
    "name": "默认",
    "description": "沿用现有配色"
  },
  {
    "id": "deep-space",
    "name": "深空指挥舱",
    "description": "冰蓝 · 网格 · 能量流"
  },
  {
    "id": "aurora",
    "name": "极光玻璃",
    "description": "折光 · 悬浮 · 轻盈"
  },
  {
    "id": "titanium",
    "name": "钛金精密",
    "description": "石墨 · 铂金 · 秩序"
  }
]

export const defaultTokens: Record<ThemeMode, SkinTokens> = {
  "light": {
    "bg": "#fff",
    "chrome": "#fff",
    "surface": "#fff",
    "surface2": "rgb(250, 250, 252)",
    "reading": "#fff",
    "raised": "#fff",
    "text1": "rgb(31, 34, 37)",
    "text2": "rgb(51, 54, 57)",
    "text3": "rgb(118, 124, 130)",
    "border": "rgb(224, 224, 230)",
    "borderStrong": "rgb(224, 224, 230)",
    "divider": "rgb(239, 239, 245)",
    "hover": "rgb(243, 243, 245)",
    "primary": "#18a058",
    "accent2": "#18a058",
    "success": "#18a058",
    "warning": "#f0a020",
    "error": "#d03050",
    "info": "#2080f0",
    "shadow1": "0 1px 2px -2px rgba(0, 0, 0, .08), 0 3px 6px 0 rgba(0, 0, 0, .06), 0 5px 12px 4px rgba(0, 0, 0, .04)",
    "shadow2": "0 3px 6px -4px rgba(0, 0, 0, .12), 0 6px 16px 0 rgba(0, 0, 0, .08), 0 9px 28px 8px rgba(0, 0, 0, .05)",
    "shadow3": "0 6px 16px -9px rgba(0, 0, 0, .08), 0 9px 28px 0 rgba(0, 0, 0, .05), 0 12px 48px 16px rgba(0, 0, 0, .03)",
    "onPrimary": "#ffffff",
    "onStatus": "#ffffff",
    "glowA": "transparent",
    "glowB": "transparent",
    "glowC": "transparent",
    "radiusControl": 3,
    "radiusCard": 3,
    "radiusNode": 8
  },
  "dark": {
    "bg": "rgb(16, 16, 20)",
    "chrome": "rgb(24, 24, 28)",
    "surface": "rgb(24, 24, 28)",
    "surface2": "rgba(255, 255, 255, 0.06)",
    "reading": "rgb(24, 24, 28)",
    "raised": "rgb(44, 44, 50)",
    "text1": "rgba(255, 255, 255, 0.9)",
    "text2": "rgba(255, 255, 255, 0.82)",
    "text3": "rgba(255, 255, 255, 0.52)",
    "border": "rgba(255, 255, 255, 0.24)",
    "borderStrong": "rgba(255, 255, 255, 0.24)",
    "divider": "rgba(255, 255, 255, 0.09)",
    "hover": "rgba(255, 255, 255, 0.09)",
    "primary": "#63e2b7",
    "accent2": "#63e2b7",
    "success": "#63e2b7",
    "warning": "#f2c97d",
    "error": "#e88080",
    "info": "#70c0e8",
    "shadow1": "0 1px 2px -2px rgba(0, 0, 0, .24), 0 3px 6px 0 rgba(0, 0, 0, .18), 0 5px 12px 4px rgba(0, 0, 0, .12)",
    "shadow2": "0 3px 6px -4px rgba(0, 0, 0, .24), 0 6px 12px 0 rgba(0, 0, 0, .16), 0 9px 18px 8px rgba(0, 0, 0, .10)",
    "shadow3": "0 6px 16px -9px rgba(0, 0, 0, .08), 0 9px 28px 0 rgba(0, 0, 0, .05), 0 12px 48px 16px rgba(0, 0, 0, .03)",
    "onPrimary": "#ffffff",
    "onStatus": "#ffffff",
    "glowA": "transparent",
    "glowB": "transparent",
    "glowC": "transparent",
    "radiusControl": 3,
    "radiusCard": 3,
    "radiusNode": 8
  }
}

export const skinTokens: Record<SkinId, Record<ThemeMode, SkinTokens>> = {
  default: defaultTokens,
  "deep-space": {
  "dark": {
    "bg": "#060B1A",
    "chrome": "rgba(10,18,40,.78)",
    "surface": "rgba(16,28,56,.72)",
    "surface2": "#0B1530",
    "reading": "#0C1731",
    "raised": "#121F42",
    "text1": "#E8F0FF",
    "text2": "#B3C1E2",
    "text3": "#8191B8",
    "border": "rgba(122,162,255,.20)",
    "borderStrong": "rgba(122,190,255,.42)",
    "divider": "rgba(122,162,255,.12)",
    "hover": "rgba(92,200,255,.08)",
    "primary": "#5CC8FF",
    "onPrimary": "#04111F",
    "accent2": "#8B7BFF",
    "success": "#3DDC97",
    "warning": "#FFB84D",
    "error": "#FF6B7A",
    "info": "#8FA8FF",
    "onStatus": "#04111F",
    "glowA": "rgba(92,200,255,.16)",
    "glowB": "rgba(139,123,255,.12)",
    "glowC": "rgba(92,200,255,.08)",
    "radiusControl": 8,
    "radiusCard": 12,
    "radiusNode": 6,
    "shadow1": "0 0 0 1px rgba(122,162,255,.14), 0 8px 24px -12px rgba(0,0,0,.6)",
    "shadow2": "0 0 0 1px rgba(122,190,255,.28), 0 0 24px -6px rgba(92,200,255,.35), 0 16px 40px -16px rgba(0,0,0,.7)",
    "shadow3": "0 0 0 1px rgba(122,190,255,.36), 0 0 40px -8px rgba(92,200,255,.45), 0 28px 64px -20px rgba(0,0,0,.8)"
  },
  "light": {
    "bg": "#EDF2FA",
    "chrome": "rgba(248,250,254,.82)",
    "surface": "rgba(255,255,255,.82)",
    "surface2": "#F3F6FC",
    "reading": "#FFFFFF",
    "raised": "#FFFFFF",
    "text1": "#0B1630",
    "text2": "#38486A",
    "text3": "#5B6B8C",
    "border": "#CBD6EA",
    "borderStrong": "#9FB2D6",
    "divider": "#DEE6F4",
    "hover": "rgba(20,88,230,.06)",
    "primary": "#1458E6",
    "onPrimary": "#FFFFFF",
    "accent2": "#6D5BF0",
    "success": "#0B7F52",
    "warning": "#9E5F00",
    "error": "#D12C45",
    "info": "#5A4FD6",
    "onStatus": "#FFFFFF",
    "glowA": "rgba(20,88,230,.10)",
    "glowB": "rgba(109,91,240,.08)",
    "glowC": "rgba(20,88,230,.05)",
    "radiusControl": 8,
    "radiusCard": 12,
    "radiusNode": 6,
    "shadow1": "0 1px 2px rgba(11,22,48,.06), 0 8px 24px -12px rgba(20,88,230,.18)",
    "shadow2": "0 2px 4px rgba(11,22,48,.06), 0 14px 32px -14px rgba(20,88,230,.26)",
    "shadow3": "0 4px 8px rgba(11,22,48,.06), 0 24px 56px -18px rgba(20,88,230,.32)"
  }
},
  "aurora": {
  "dark": {
    "bg": "#0C1120",
    "chrome": "rgba(22,29,47,.78)",
    "surface": "#151D2E",
    "surface2": "#1B2639",
    "reading": "#151D2E",
    "raised": "#1D273C",
    "text1": "#F1F5FF",
    "text2": "#C4CCE0",
    "text3": "#9BA9C6",
    "border": "#2C3A54",
    "borderStrong": "#536583",
    "divider": "#253149",
    "hover": "rgba(174,191,255,.09)",
    "primary": "#AEBFFF",
    "onPrimary": "#101932",
    "accent2": "#62DDD3",
    "success": "#5BE38F",
    "warning": "#FFC857",
    "error": "#FF6F91",
    "info": "#8AB4FF",
    "onStatus": "#03140F",
    "glowA": "rgba(129,153,255,.20)",
    "glowB": "rgba(66,214,210,.14)",
    "glowC": "rgba(188,161,255,.10)",
    "radiusControl": 11,
    "radiusCard": 22,
    "radiusNode": 10,
    "shadow1": "0 2px 4px rgba(0,0,0,.12), 0 14px 40px -24px rgba(0,0,0,.55)",
    "shadow2": "inset 0 1px 0 rgba(255,255,255,.06), 0 20px 48px -20px rgba(0,0,0,.60)",
    "shadow3": "inset 0 1px 0 rgba(255,255,255,.10), 0 32px 100px -24px rgba(0,0,0,.75)"
  },
  "light": {
    "bg": "#EEF1F8",
    "chrome": "rgba(255,255,255,.78)",
    "surface": "#FFFFFF",
    "surface2": "#F3F5FC",
    "reading": "#FFFFFF",
    "raised": "#FFFFFF",
    "text1": "#15213B",
    "text2": "#384862",
    "text3": "#596984",
    "border": "#D4DCEA",
    "borderStrong": "#A3B1CC",
    "divider": "#E5EAF4",
    "hover": "rgba(64,85,184,.05)",
    "primary": "#4055B8",
    "onPrimary": "#FFFFFF",
    "accent2": "#0B7F7C",
    "success": "#197A44",
    "warning": "#9E5F00",
    "error": "#CC2F55",
    "info": "#3F6BD9",
    "onStatus": "#FFFFFF",
    "glowA": "rgba(142,161,234,.27)",
    "glowB": "rgba(95,207,201,.18)",
    "glowC": "rgba(185,165,226,.15)",
    "radiusControl": 11,
    "radiusCard": 22,
    "radiusNode": 10,
    "shadow1": "0 2px 4px rgba(29,45,87,.025), 0 14px 36px -24px rgba(54,76,136,.22)",
    "shadow2": "inset 0 1px 0 rgba(255,255,255,.95), 0 18px 44px -20px rgba(54,76,136,.24)",
    "shadow3": "inset 0 1px 0 rgba(255,255,255,1), 0 32px 100px -30px rgba(34,55,110,.34)"
  }
},
  "titanium": {
  "dark": {
    "bg": "#111211",
    "chrome": "#191B19",
    "surface": "#1B1D1A",
    "surface2": "#20231F",
    "reading": "#1B1D1A",
    "raised": "#262923",
    "text1": "#F3F1E9",
    "text2": "#C5C6B9",
    "text3": "#A0A596",
    "border": "#393D34",
    "borderStrong": "#69705F",
    "divider": "#30342C",
    "hover": "rgba(220,199,161,.07)",
    "primary": "#DCC7A1",
    "onPrimary": "#1B2018",
    "accent2": "#DCC7A1",
    "success": "#89CDA3",
    "warning": "#E6BE79",
    "error": "#F3998D",
    "info": "#A3C3E5",
    "onStatus": "#131911",
    "glowA": "rgba(235,231,216,.05)",
    "glowB": "rgba(220,199,161,.06)",
    "glowC": "rgba(235,231,216,.03)",
    "radiusControl": 6,
    "radiusCard": 14,
    "radiusNode": 4,
    "shadow1": "0 1px 0 rgba(255,255,255,.035), 0 4px 16px -12px rgba(0,0,0,.4)",
    "shadow2": "0 1px 0 rgba(255,255,255,.06) inset, 0 12px 32px -18px rgba(0,0,0,.6)",
    "shadow3": "0 1px 0 rgba(255,255,255,.08) inset, 0 32px 90px -24px rgba(0,0,0,.8)"
  },
  "light": {
    "bg": "#EBEAE5",
    "chrome": "#F4F3EF",
    "surface": "#FAFAF6",
    "surface2": "#F0F0E9",
    "reading": "#FAFAF6",
    "raised": "#FFFFFF",
    "text1": "#1B211B",
    "text2": "#414B40",
    "text3": "#606B5C",
    "border": "#D2D4C8",
    "borderStrong": "#959B8B",
    "divider": "#E1E3D8",
    "hover": "rgba(105,82,50,.05)",
    "primary": "#695232",
    "onPrimary": "#FFFFFF",
    "accent2": "#8B5F28",
    "success": "#1F6E43",
    "warning": "#8D520B",
    "error": "#AD3830",
    "info": "#365F85",
    "onStatus": "#FFFFFF",
    "glowA": "rgba(255,255,255,.48)",
    "glowB": "rgba(160,146,113,.10)",
    "glowC": "rgba(255,255,255,.18)",
    "radiusControl": 6,
    "radiusCard": 14,
    "radiusNode": 4,
    "shadow1": "0 1px 2px rgba(35,42,29,.03), 0 4px 16px -12px rgba(35,42,29,.14)",
    "shadow2": "inset 0 1px 0 rgba(255,255,255,.9), 0 12px 32px -18px rgba(35,42,29,.22)",
    "shadow3": "inset 0 1px 0 rgba(255,255,255,1), 0 32px 90px -24px rgba(35,42,29,.28)"
  }
}
}
