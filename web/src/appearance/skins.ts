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
    "description": "青绿 · 玻璃 · 流光"
  },
  {
    "id": "titanium",
    "name": "钛金精密",
    "description": "银蓝 · 拉丝 · 掠光"
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
    "bg": "#061513",
    "chrome": "rgba(10,30,28,.55)",
    "surface": "rgba(18,52,48,.46)",
    "surface2": "#0A1C1A",
    "reading": "#0B1F1D",
    "raised": "#123230",
    "text1": "#E9FFF8",
    "text2": "#B5D6CD",
    "text3": "#85ABA0",
    "border": "rgba(150,255,230,.20)",
    "borderStrong": "rgba(150,255,230,.40)",
    "divider": "rgba(150,255,230,.11)",
    "hover": "rgba(47,212,217,.09)",
    "primary": "#2FD4D9",
    "onPrimary": "#03140F",
    "accent2": "#FF6BD6",
    "success": "#5BE38F",
    "warning": "#FFC857",
    "error": "#FF6F91",
    "info": "#8AB4FF",
    "onStatus": "#03140F",
    "glowA": "rgba(47,212,217,.30)",
    "glowB": "rgba(255,107,214,.22)",
    "glowC": "rgba(143,123,255,.22)",
    "radiusControl": 12,
    "radiusCard": 18,
    "radiusNode": 10,
    "shadow1": "inset 0 1px 0 rgba(255,255,255,.14), 0 8px 28px -12px rgba(0,0,0,.55)",
    "shadow2": "inset 0 1px 0 rgba(255,255,255,.2), 0 18px 44px -16px rgba(0,0,0,.6)",
    "shadow3": "inset 0 1px 0 rgba(255,255,255,.24), 0 30px 70px -22px rgba(0,0,0,.7)"
  },
  "light": {
    "bg": "#EAF8F4",
    "chrome": "rgba(255,255,255,.60)",
    "surface": "rgba(255,255,255,.58)",
    "surface2": "#F2FBF8",
    "reading": "#FFFFFF",
    "raised": "#FFFFFF",
    "text1": "#08211D",
    "text2": "#34504A",
    "text3": "#587369",
    "border": "#BFE0D8",
    "borderStrong": "#8EC4B8",
    "divider": "#D6ECE6",
    "hover": "rgba(11,127,124,.07)",
    "primary": "#0B7F7C",
    "onPrimary": "#FFFFFF",
    "accent2": "#D13FA6",
    "success": "#197A44",
    "warning": "#9E5F00",
    "error": "#CC2F55",
    "info": "#3F6BD9",
    "onStatus": "#FFFFFF",
    "glowA": "rgba(60,200,190,.40)",
    "glowB": "rgba(255,150,210,.34)",
    "glowC": "rgba(150,160,255,.30)",
    "radiusControl": 12,
    "radiusCard": 18,
    "radiusNode": 10,
    "shadow1": "inset 0 1px 0 rgba(255,255,255,.9), 0 8px 28px -12px rgba(11,127,124,.22)",
    "shadow2": "inset 0 1px 0 rgba(255,255,255,.95), 0 18px 44px -16px rgba(11,127,124,.28)",
    "shadow3": "inset 0 1px 0 #fff, 0 30px 70px -22px rgba(11,127,124,.34)"
  }
},
  "titanium": {
  "dark": {
    "bg": "#0E1013",
    "chrome": "#14171B",
    "surface": "#1A1E23",
    "surface2": "#15181C",
    "reading": "#171A1F",
    "raised": "#20252B",
    "text1": "#EEF1F5",
    "text2": "#B9C0CA",
    "text3": "#8A93A0",
    "border": "#2C333B",
    "borderStrong": "#44505C",
    "divider": "#232930",
    "hover": "rgba(141,180,232,.08)",
    "primary": "#8DB4E8",
    "onPrimary": "#0B121C",
    "accent2": "#C9A66B",
    "success": "#4FD18B",
    "warning": "#FFB020",
    "error": "#F0627A",
    "info": "#7BA7FF",
    "onStatus": "#0B121C",
    "glowA": "rgba(190,210,235,.07)",
    "glowB": "rgba(190,210,235,.045)",
    "glowC": "rgba(141,180,232,.12)",
    "radiusControl": 6,
    "radiusCard": 8,
    "radiusNode": 4,
    "shadow1": "0 1px 0 rgba(255,255,255,.04) inset, 0 1px 2px rgba(0,0,0,.5)",
    "shadow2": "0 1px 0 rgba(255,255,255,.06) inset, 0 8px 20px -8px rgba(0,0,0,.7)",
    "shadow3": "0 1px 0 rgba(255,255,255,.08) inset, 0 20px 44px -14px rgba(0,0,0,.8)"
  },
  "light": {
    "bg": "#E6E9ED",
    "chrome": "#F2F4F6",
    "surface": "#F8F9FB",
    "surface2": "#EEF0F3",
    "reading": "#FFFFFF",
    "raised": "#FFFFFF",
    "text1": "#12161B",
    "text2": "#3C4550",
    "text3": "#58626F",
    "border": "#C5CBD3",
    "borderStrong": "#98A3B0",
    "divider": "#D8DDE3",
    "hover": "rgba(47,94,148,.07)",
    "primary": "#2F5E94",
    "onPrimary": "#FFFFFF",
    "accent2": "#A8843F",
    "success": "#17855A",
    "warning": "#A66300",
    "error": "#C62F4A",
    "info": "#2E64C0",
    "onStatus": "#FFFFFF",
    "glowA": "rgba(255,255,255,.60)",
    "glowB": "rgba(255,255,255,.40)",
    "glowC": "rgba(47,94,148,.10)",
    "radiusControl": 6,
    "radiusCard": 8,
    "radiusNode": 4,
    "shadow1": "0 1px 0 #fff inset, 0 1px 2px rgba(18,22,27,.14)",
    "shadow2": "0 1px 0 #fff inset, 0 8px 20px -10px rgba(18,22,27,.28)",
    "shadow3": "0 1px 0 #fff inset, 0 20px 44px -16px rgba(18,22,27,.36)"
  }
}
}
