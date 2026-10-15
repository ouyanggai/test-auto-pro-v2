# F-049 视觉与实现规格（可直接照做）

- 状态：随 F-049 为 awaiting_approval，尚未实施。
- 作用：[F-049](F-049-futuristic-skins-and-ui-polish.md) 说"做什么、怎么验收"，本文件说"每个像素、每个文件怎么做"。数值、变量名、文件名都已定死；实施者不需要再做审美判断，只需照表落地并按文末测试验证。
- 与 ARCHITECTURE 的分工：技术边界以 [ARCHITECTURE.md F-049 章节](../ARCHITECTURE.md#f-049-主题与界面边界待批准) 为准，本文是其具体化；冲突时以 ARCHITECTURE 为准并回头改本文。

## 0. 方案评审结论

### 0.1 对原方案的判断

原方案方向正确、边界严谨（不丢状态、不碰业务、原生表单不换肤）。要补的是下面几处会在实施中翻车的点：

| # | 问题 | 后果 | 本文的处理 |
| --- | --- | --- | --- |
| 1 | 只有配色形容词，没有色值、变量名、层级 | 不同实施者做出完全不同的东西，无法验收"是否还原" | 第 2 节给全部 8 套色值与变量表 |
| 2 | 源码里约 70 处使用了**从未定义**的 CSS 变量：`--text-color-3`(31)、`--primary-color`(10)、`--divider-color`(9)、`--text-color-1`(8)、`--card-color`(6)、`--font-family-mono`(2)、`--table-color-hover`/`--hover-color`/`--border-radius` 等，集中在报告中心、空态、计划候选列表等 10 个文件 | 这些颜色今天等于"没写"（继承正文色），换肤后它们不会变，页面会出现"一块没换的灰" | 第 3.3 节：在根节点补齐这批变量（顺带把今天的隐性缺陷修掉） |
| 3 | 约 209 处硬编码十六进制色分布在 19 个文件（`NodeConfigurationPanel` 的 `#e5e7eb/#64748b`、`FlowTreeEdge` 的 `#d03050`、`RunPreflightDialog` 的 rgba、报告图表整张调色板、节点状态角标的 `#fff`） | 深色/新皮肤下直接穿帮；尤其 `#fff` 字压在深色皮肤的亮绿/亮橙状态底上，对比度只有 1.7:1，看不清 | 第 7 节逐文件清单；新增 `--ta-on-status` |
| 4 | 新皮肤要"透明玻璃/渐变"，但 NLayout、NDataTable、NCard 默认是不透明底色 | 氛围层被 NLayout 底色整块盖住，什么都看不到 | 第 3.2 节：Layout 底色清空，表格/日志用不透明"阅读面" |
| 5 | 没写**首屏闪白**：Naive 的 body 底色要等 Vue 挂载后才写 | 选了深色皮肤，刷新瞬间先白一下 | 第 3.4 节：`index.html` 内联 5 行脚本 + 6 行首屏底色 |
| 6 | "指针跟随统一一个回调"没说怎么跟 | 若写成改 `<html>` 上的 CSS 变量，每帧都会触发整页样式重算，长表格会卡 | 第 4.5 节：只写**一个**元素的 `transform`，纯合成 |
| 7 | 浅色版没有视觉方案 | 发光、霓虹在白底上是脏的，浅色版会很难看 | 第 4 节：浅色版把"发光"换成"柔光晕 + 投影"，规则写明 |
| 8 | 持续动效 + 全屏 `backdrop-filter` 的性能边界缺失；"不联动减少动效"是用户决定，那更要给性能预算 | 内网办公机集显长期占用 | 第 4.6 节性能预算（可测试的硬规则） |
| 9 | 弹窗里 4 处是裸 `n-card`、4 处 `preset=card`、3 处 `preset=dialog`，宽度写在行内样式 | 无法统一 | 第 6 节 11 处逐个指定尺寸与改法，行内宽度/颜色注入全部删除 |
| 10 | 状态色 `info` 与新皮肤主色同属蓝系，易混 | "运行中"与"可点击"分不清 | 第 2 节已把 info 与 primary 在色相或明度上拉开，并有测试 |

### 0.2 三套新皮肤是不是最好看的？

结论：**保留这三个方向，但原稿各有一个会让它显得廉价的写法，已改。** 不建议换掉任何一套，理由是它们占据三个互不重叠的轴：

| 皮肤 | 轴 | 原稿的风险 | 改后的招牌（识别度来源） |
| --- | --- | --- | --- |
| 深空指挥舱 | 冷、精确、仪表 | "藏蓝 + 紫光 + 网格"是最常见的 AI 仪表盘模板，没有记忆点；紫色与极光的粉紫撞色 | **去掉紫色主导**：冰蓝为唯一主色，紫只做极小点缀；招牌改为**卡片对角 L 形刻度角标 + 轨道环上的运行光点 + 顶栏流光**，像飞行仪表而不是"发光卡片" |
| 极光玻璃 | 柔、有机、通透 | 全界面毛玻璃会让表格、日志的可读性变差，而且最吃性能；浅色版半透明叠白底会发灰 | **玻璃只用在"外壳"**（顶栏、侧栏、弹窗、KPI 卡）；表格、日志、代码一律**不透明阅读面**；主色改为青（偏蓝）以避免与成功绿混淆；浅色版用糖果色柔光团而不是发光 |
| 钛金精密 | 中性、工业、克制 | 原稿"扫描光带"若全屏常驻会像老式杀毒软件，廉价；暖金容易滥用 | **金色只出现 3 处**（侧栏当前项竖条、页签下划线、顶栏居中短线）；拉丝纹理常驻但极淡；掠光带低频（14s 一次）；这是最适合长时间盯日志的一套 |

补充判断：

1. 三套都以深色为主视觉，浅色版最容易"弱"。本文要求浅色版各有独立的氛围方案（第 4 节），不是深色版直接反色。
2. 若以后想再加第四套，我推荐**暖色的"琥珀工坊"（暖纸白底 + 墨色 + 琥珀强调）**，与现有三套的冷色系互补；不放进本切片。
3. 默认皮肤不动配色，但补齐第 0.1 节第 2 点的未定义变量后，报告中心等页面的次要文字会第一次真正变成"次要灰"，这是修缺陷，不是配色改动；人工核对时需知晓。

## 1. 文件清单

### 新增

| 文件 | 内容 |
| --- | --- |
| `web/src/appearance/skins.ts` | `SkinId`、`SkinTokens`、`skinCatalog`、8 套令牌表（第 2 节）。**不 import naive-ui**，让 Node 测试可直接加载 |
| `web/src/appearance/preferences.ts` | `readSkinId(storage)`、`readThemeMode(storage)`、存储键常量；纯函数，便于用假 storage 测试 |
| `web/src/appearance/color.ts` | `lift(hex)`、`sink(hex)`、`mix(a, b, ratio)`、`alpha(hex, a)`；只处理 `#rgb/#rrggbb/rgb()/rgba()` |
| `web/src/appearance/naiveTheme.ts` | `buildThemeOverrides(skin, mode)`（第 3.2 节）。默认皮肤返回 `undefined` |
| `web/src/appearance/applyAppearance.ts` | `applyAppearance(skin, mode)`：写 `<html>` 的 `data-skin`/`data-mode`/`color-scheme` 与全部 CSS 变量（第 3.3 节） |
| `web/src/appearance/pointerGlow.ts` | `startPointerGlow(el)`，返回停止函数（第 4.5 节） |
| `web/src/components/AppearanceMenu.vue` | 顶栏"外观"入口与弹层（第 5 节） |
| `web/src/components/AmbientLayer.vue` | 氛围层 DOM，无业务数据（第 4.1 节） |
| `web/src/styles/skins.css` | 三套新皮肤的装饰、氛围、关键帧、组件级造型（第 4、8 节） |
| `web/src/styles/layout.css` | 间距、字阶、页面骨架、弹窗统一类（第 6、8.1 节），对所有皮肤（含默认）生效 |
| `web/src/features/reports/chartTheme.ts` | `chartBaseOption(tokens)`、`statusPalette(tokens)`（第 7.3 节） |
| `test/unit/frontend/appearance/appearance_test.mjs` | 第 10 节 |
| `test/run-f049.ps1` | 仿 `test/run-f048.ps1`，UTF-8 BOM |

### 修改

| 文件 | 改动 |
| --- | --- |
| `web/index.html` | 首屏内联脚本（第 3.4 节） |
| `web/src/main.ts` | 在 `./styles.css` 之后依次 `import './styles/layout.css'`、`import './styles/skins.css'` |
| `web/src/stores/app.ts` | 增加 `skinId`；`setSkinId`；存储读取改走 `preferences.ts` |
| `web/src/App.vue` | `NConfigProvider` 加 `theme-overrides`；挂 `AmbientLayer`、`AppearanceMenu`；`watch` 调用 `applyAppearance` 与 `startPointerGlow` |
| `web/src/styles.css` | `--app-content-pad-x` 改为 `clamp(24px, 3.2vw, 48px)`；删去已被 `layout.css` 接管的页面骨架样式（不要两处各写一份） |
| 第 6、7 节列出的 11 个弹窗文件与 10 余个含硬编码色的文件 | 按清单逐个 |

不得修改：`form-runtime/**`、`FormRuntimeFrame.vue` 内部的 `background: #fff`、任何 `api.ts`、后端与执行器。

## 2. 设计令牌

### 2.1 令牌字段（`SkinTokens`）

| 字段 | 用途 | 可否半透明 |
| --- | --- | --- |
| `bg` | 页面底色（也是 Naive `bodyColor`、画布底） | 否 |
| `chrome` | 顶栏、侧栏底 | 可（玻璃） |
| `surface` | 普通卡片底（KPI、摘要、面板） | 可（玻璃） |
| `surface2` | 嵌套/内凹底：表头、分组底、行 hover 底 | 否 |
| `reading` | **阅读面**：表格、日志、代码、长文本、表单外框内侧。要求完全不透明 | **否** |
| `raised` | 弹窗、下拉、气泡底 | 否 |
| `text1/2/3` | 主文字 / 次文字 / 辅助文字 | — |
| `border` `borderStrong` `divider` | 控件边框 / 强调或聚焦边框 / 内部分隔线 | — |
| `hover` | 行、菜单项 hover 底 | 可 |
| `primary` `onPrimary` | 主色 / 主色上的文字 | — |
| `accent2` | **仅装饰**（不得表达状态、不得做文字） | — |
| `success` `warning` `error` `info` `onStatus` | 状态色 / 状态色底上的文字 | — |
| `glowA/B/C` | 氛围层三种光的颜色（含透明度） | 含 |
| `shadow1/2/3` | 阴影三档 | — |
| `radiusControl` `radiusCard` `radiusNode` | 控件 / 卡片 / 流程节点圆角（px 数值） | — |

派生（代码里算，不进表）：`primaryHover = lift(primary)`、`primaryPressed = sink(primary)`；status 同理；输入框底：深色 `mix(reading, '#fff', .05)`，浅色 `#ffffff`。`lift` = 向白混 14%，`sink` = 向黑混 16%（与 Naive 默认的悬停/按下方向一致）。

### 2.2 默认皮肤

默认皮肤**不写任何 `themeOverrides`**；令牌取自 Naive 自己的 `lightTheme.common` / `darkTheme.common`，以保证"保留当前配色"可证明。实现为在 `skins.ts` 中手抄下表，由测试对照 `naive-ui` 实际值断言相等（第 10 节）。

| 字段 | default·light | default·dark |
| --- | --- | --- |
| bg / reading / raised | `#ffffff` / `#ffffff` / `#ffffff` | `rgb(16, 16, 20)` / `rgb(24, 24, 28)` / `rgb(44, 44, 50)` |
| chrome / surface | `#ffffff` / `#ffffff` | `rgb(24, 24, 28)` / `rgb(24, 24, 28)` |
| surface2 | `rgb(250, 250, 252)` | `rgba(255, 255, 255, 0.06)` |
| hover | `rgb(243, 243, 245)` | `rgba(255, 255, 255, 0.09)` |
| text1 / 2 / 3 | `rgb(31, 34, 37)` / `rgb(51, 54, 57)` / `rgb(118, 124, 130)` | `rgba(255, 255, 255, 0.9)` / `0.82` / `0.52` |
| border / divider | `rgb(224, 224, 230)` / `rgb(239, 239, 245)` | `rgba(255, 255, 255, 0.24)` / `0.09` |
| borderStrong | 同 border | 同 border |
| primary / success | `#18a058` | `#63e2b7` |
| warning / error / info | `#f0a020` / `#d03050` / `#2080f0` | `#f2c97d` / `#e88080` / `#70c0e8` |
| onPrimary / onStatus | `#ffffff` | `#ffffff`（保持现状，见第 0.1 节第 3 点说明） |
| accent2 | 同 primary | 同 primary |
| glow、shadow | 无氛围；shadow 取 Naive `boxShadow1/2/3` | 同 |
| radiusControl / radiusCard / radiusNode | `3` / `3` / `8` | 同 |

### 2.3 新皮肤色值（六套，逐字照抄）

对比度已用脚本核算：`text1/bg ≥ 14.9`、`text2/bg ≥ 8.0`、`text3/阅读面 ≥ 5.2`、`primary/阅读面 ≥ 4.8`、`onPrimary/primary ≥ 4.8`、四种状态色在阅读面上均 ≥ 4.5。测试以这些阈值断言，改色必须同时过测试。

**深空指挥舱**

| 字段 | dark | light |
| --- | --- | --- |
| bg | `#060B1A` | `#EDF2FA` |
| chrome | `rgba(10,18,40,.78)` | `rgba(248,250,254,.82)` |
| surface | `rgba(16,28,56,.72)` | `rgba(255,255,255,.82)` |
| surface2 | `#0B1530` | `#F3F6FC` |
| reading / raised | `#0C1731` / `#121F42` | `#FFFFFF` / `#FFFFFF` |
| text1 / 2 / 3 | `#E8F0FF` / `#B3C1E2` / `#8191B8` | `#0B1630` / `#38486A` / `#5B6B8C` |
| border / borderStrong / divider | `rgba(122,162,255,.20)` / `rgba(122,190,255,.42)` / `rgba(122,162,255,.12)` | `#CBD6EA` / `#9FB2D6` / `#DEE6F4` |
| hover | `rgba(92,200,255,.08)` | `rgba(20,88,230,.06)` |
| primary / onPrimary | `#5CC8FF` / `#04111F` | `#1458E6` / `#FFFFFF` |
| accent2 | `#8B7BFF` | `#6D5BF0` |
| success / warning / error / info | `#3DDC97` / `#FFB84D` / `#FF6B7A` / `#8FA8FF` | `#0B7F52` / `#9E5F00` / `#D12C45` / `#5A4FD6` |
| onStatus | `#04111F` | `#FFFFFF` |
| glowA / B / C | `rgba(92,200,255,.16)` / `rgba(139,123,255,.12)` / `rgba(92,200,255,.08)` | `rgba(20,88,230,.10)` / `rgba(109,91,240,.08)` / `rgba(20,88,230,.05)` |
| radiusControl / Card / Node | 8 / 12 / 6 | 8 / 12 / 6 |

**极光玻璃**

| 字段 | dark | light |
| --- | --- | --- |
| bg | `#061513` | `#EAF8F4` |
| chrome | `rgba(10,30,28,.55)` | `rgba(255,255,255,.60)` |
| surface | `rgba(18,52,48,.46)` | `rgba(255,255,255,.58)` |
| surface2 | `#0A1C1A` | `#F2FBF8` |
| reading / raised | `#0B1F1D` / `#123230` | `#FFFFFF` / `#FFFFFF` |
| text1 / 2 / 3 | `#E9FFF8` / `#B5D6CD` / `#85ABA0` | `#08211D` / `#34504A` / `#587369` |
| border / borderStrong / divider | `rgba(150,255,230,.20)` / `rgba(150,255,230,.40)` / `rgba(150,255,230,.11)` | `#BFE0D8` / `#8EC4B8` / `#D6ECE6` |
| hover | `rgba(47,212,217,.09)` | `rgba(11,127,124,.07)` |
| primary / onPrimary | `#2FD4D9` / `#03140F` | `#0B7F7C` / `#FFFFFF` |
| accent2 | `#FF6BD6` | `#D13FA6` |
| success / warning / error / info | `#5BE38F` / `#FFC857` / `#FF6F91` / `#8AB4FF` | `#197A44` / `#9E5F00` / `#CC2F55` / `#3F6BD9` |
| onStatus | `#03140F` | `#FFFFFF` |
| glowA / B / C | `rgba(47,212,217,.30)` / `rgba(255,107,214,.22)` / `rgba(143,123,255,.22)` | `rgba(60,200,190,.40)` / `rgba(255,150,210,.34)` / `rgba(150,160,255,.30)` |
| radiusControl / Card / Node | 12 / 18 / 10 | 12 / 18 / 10 |

**钛金精密**

| 字段 | dark | light |
| --- | --- | --- |
| bg | `#0E1013` | `#E6E9ED` |
| chrome | `#14171B` | `#F2F4F6` |
| surface | `#1A1E23` | `#F8F9FB` |
| surface2 | `#15181C` | `#EEF0F3` |
| reading / raised | `#171A1F` / `#20252B` | `#FFFFFF` / `#FFFFFF` |
| text1 / 2 / 3 | `#EEF1F5` / `#B9C0CA` / `#8A93A0` | `#12161B` / `#3C4550` / `#58626F` |
| border / borderStrong / divider | `#2C333B` / `#44505C` / `#232930` | `#C5CBD3` / `#98A3B0` / `#D8DDE3` |
| hover | `rgba(141,180,232,.08)` | `rgba(47,94,148,.07)` |
| primary / onPrimary | `#8DB4E8` / `#0B121C` | `#2F5E94` / `#FFFFFF` |
| accent2（暖金） | `#C9A66B` | `#A8843F` |
| success / warning / error / info | `#4FD18B` / `#FFB020` / `#F0627A` / `#7BA7FF` | `#17855A` / `#A66300` / `#C62F4A` / `#2E64C0` |
| onStatus | `#0B121C` | `#FFFFFF` |
| glowA / B / C | `rgba(190,210,235,.07)` / `rgba(190,210,235,.045)` / `rgba(141,180,232,.12)` | `rgba(255,255,255,.60)` / `rgba(255,255,255,.40)` / `rgba(47,94,148,.10)` |
| radiusControl / Card / Node | 6 / 8 / 4 | 6 / 8 / 4 |

### 2.4 阴影

| 皮肤·模式 | shadow1 | shadow2 | shadow3 |
| --- | --- | --- | --- |
| 深空·dark | `0 0 0 1px rgba(122,162,255,.14), 0 8px 24px -12px rgba(0,0,0,.6)` | `0 0 0 1px rgba(122,190,255,.28), 0 0 24px -6px rgba(92,200,255,.35), 0 16px 40px -16px rgba(0,0,0,.7)` | `0 0 0 1px rgba(122,190,255,.36), 0 0 40px -8px rgba(92,200,255,.45), 0 28px 64px -20px rgba(0,0,0,.8)` |
| 深空·light | `0 1px 2px rgba(11,22,48,.06), 0 8px 24px -12px rgba(20,88,230,.18)` | `0 2px 4px rgba(11,22,48,.06), 0 14px 32px -14px rgba(20,88,230,.26)` | `0 4px 8px rgba(11,22,48,.06), 0 24px 56px -18px rgba(20,88,230,.32)` |
| 极光·dark | `inset 0 1px 0 rgba(255,255,255,.14), 0 8px 28px -12px rgba(0,0,0,.55)` | `inset 0 1px 0 rgba(255,255,255,.2), 0 18px 44px -16px rgba(0,0,0,.6)` | `inset 0 1px 0 rgba(255,255,255,.24), 0 30px 70px -22px rgba(0,0,0,.7)` |
| 极光·light | `inset 0 1px 0 rgba(255,255,255,.9), 0 8px 28px -12px rgba(11,127,124,.22)` | `inset 0 1px 0 rgba(255,255,255,.95), 0 18px 44px -16px rgba(11,127,124,.28)` | `inset 0 1px 0 #fff, 0 30px 70px -22px rgba(11,127,124,.34)` |
| 钛金·dark | `0 1px 0 rgba(255,255,255,.04) inset, 0 1px 2px rgba(0,0,0,.5)` | `0 1px 0 rgba(255,255,255,.06) inset, 0 8px 20px -8px rgba(0,0,0,.7)` | `0 1px 0 rgba(255,255,255,.08) inset, 0 20px 44px -14px rgba(0,0,0,.8)` |
| 钛金·light | `0 1px 0 #fff inset, 0 1px 2px rgba(18,22,27,.14)` | `0 1px 0 #fff inset, 0 8px 20px -10px rgba(18,22,27,.28)` | `0 1px 0 #fff inset, 0 20px 44px -16px rgba(18,22,27,.36)` |

### 2.5 字体与字阶

不引入网络字体（内网）。字体栈沿用 `Inter, "PingFang SC", "Microsoft YaHei", sans-serif`；等宽为 `"JetBrains Mono", "Cascadia Mono", "Cascadia Code", Consolas, "Sarasa Mono SC", monospace`（新增变量 `--font-family-mono`，默认皮肤也使用，修第 0.1 节第 2 点）。

| 层级 | 字号/行高 | 字重 | 用法 |
| --- | --- | --- | --- |
| 页面标题 | 24 / 32 | 600 | `h1` |
| 分组标题 | 16 / 24 | 600 | 卡片/分组标题 |
| 正文 | 14 / 22 | 400 | 默认 |
| 次要说明 | 13 / 20 | 400 | `text3` 或 `text2` |
| 注释 | 12 / 18 | 400 | 表头、标签、角标 |
| KPI 数字 | 28 / 36 | 600 | `font-variant-numeric: tabular-nums; letter-spacing: -0.01em` |
| 等宽 | 12 / 18 | 400 | 接口、追踪号、日志 |

所有数字列、耗时、计数一律加 `font-variant-numeric: tabular-nums`。皮肤差异：深空的"页面标题上方小标"用 11px 等宽、`letter-spacing: .08em`、大写、`text3`（仅当页面已有该信息位时加，不新增文案）；其余皮肤不加。

### 2.6 间距与节奏

只允许 `4 8 12 16 24 32 48`。`--app-content-pad-x: clamp(24px, 3.2vw, 48px)`（1024px 宽≈33px，≥1500px 为 48px）。页面标题与首个内容块间距 24；卡片内边距 16（size small）/24（medium）；表格行高 44（单元格 `padding: 10px 12px`）；相邻卡片间距 16；分组之间 24。

## 3. 换肤管线

### 3.1 数据流

```
store.skinId + store.themeMode
   ├─► naiveTheme（darkTheme | null） + buildThemeOverrides(skin, mode) ─► NConfigProvider（Naive 组件与 useThemeVars）
   ├─► applyAppearance(skin, mode) ─► <html data-skin data-mode style="--*"> ─► skins.css / layout.css / 所有弹窗
   └─► chartBaseOption(tokens) ─► 各图表 option（computed）
```

令牌表是唯一的颜色来源；CSS 文件里**不得出现色值**（除 `skins.css` 内装饰用的 rgba，且必须只引用 `--ta-glow-*` 或 `color-mix` 到令牌）。

### 3.2 `buildThemeOverrides`

`skin === 'default'` 返回 `undefined`。其余返回下列覆盖（`GlobalThemeOverrides`）。`px(n)` 表示 `${n}px`，`clear = 'rgba(0, 0, 0, 0)'`。

```ts
common: {
  primaryColor: t.primary, primaryColorHover: lift(t.primary), primaryColorPressed: sink(t.primary), primaryColorSuppl: lift(t.primary),
  // info / success / warning / error 同上四件套，必须带 Hover/Pressed/Suppl，否则悬停会跳回 Naive 默认色
  bodyColor: t.bg, cardColor: t.reading, modalColor: t.raised, popoverColor: t.raised, tableColor: t.reading,
  actionColor: t.surface2, hoverColor: t.hover, inputColor: <见 2.1 派生>,
  textColorBase: t.text1, textColor1: t.text1, textColor2: t.text2, textColor3: t.text3,
  textColorDisabled: alpha(t.text3, .6), placeholderColor: t.text3,
  borderColor: t.border, dividerColor: t.divider,
  borderRadius: px(t.radiusControl), borderRadiusSmall: px(Math.max(t.radiusControl - 2, 2)),
  boxShadow1: t.shadow1, boxShadow2: t.shadow2, boxShadow3: t.shadow3,
  fontFamilyMono: monoStack,
},
Layout:   { color: clear, colorEmbedded: clear, headerColor: t.chrome, siderColor: t.chrome,
            headerBorderColor: t.divider, siderBorderColor: t.divider },
Card:     { color: t.surface, colorModal: t.raised, colorPopover: t.raised, colorEmbedded: t.surface2,
            borderColor: t.divider, borderRadius: px(t.radiusCard) },
Dialog:   { color: t.raised, borderRadius: px(t.radiusCard) },
DataTable:{ thColor: t.surface2, thTextColor: t.text2, tdColor: t.reading, tdColorHover: t.hover,
            borderColor: t.divider, thColorHover: t.hover },
Menu:     { itemColorActive: alpha(t.primary, .14), itemColorActiveHover: alpha(t.primary, .20),
            itemColorActiveCollapsed: alpha(t.primary, .14), itemTextColorActive: t.primary,
            itemTextColorActiveHover: t.primary, itemIconColorActive: t.primary, itemIconColorActiveHover: t.primary,
            arrowColorActive: t.primary },
Tabs:     { tabTextColorActiveLine: t.primary, tabTextColorHoverLine: t.primary, barColor: t.primary },
```

注意：`cardColor` 必须是不透明（`reading`），因为 Select、Popover 等内部组件取它；只有 `Card.color` 允许半透明（玻璃）。`Layout.color` 必须清空，否则氛围层被整块盖住。

### 3.3 `applyAppearance`：写到 `<html>` 的变量

```ts
// 属性
html.dataset.skin = skin          // 'default' | 'deep-space' | 'aurora' | 'titanium'
html.dataset.mode = mode          // 'light' | 'dark'
html.style.colorScheme = mode     // 让滚动条与原生控件跟随
```

变量（逐个 `html.style.setProperty`）：

| 变量 | 值 | 说明 |
| --- | --- | --- |
| `--text-color-1` `--text-color-2` `--text-color-3` | `text1/2/3` | **补齐源码已在用但从未定义的变量** |
| `--divider-color` `--border-color` `--card-color` `--body-color` `--hover-color` `--table-color-hover` | `divider / border / reading / bg / hover / hover` | 同上 |
| `--primary-color` `--success-color` `--warning-color` `--error-color` `--info-color` | 对应令牌 | 弹窗传送到 body 后靠它们取色，替代各弹窗里的 `dialogStyle` 变量注入 |
| `--font-family-mono` `--border-radius` | 等宽栈 / `radiusControl` | 同上 |
| `--ta-bg` `--ta-chrome` `--ta-surface` `--ta-surface-2` `--ta-reading` `--ta-raised` | 同名令牌 | |
| `--ta-border-strong` `--ta-on-primary` `--ta-on-status` `--ta-accent-2` | 同名令牌 | |
| `--ta-glow-a` `--ta-glow-b` `--ta-glow-c` | glow | |
| `--ta-shadow-1` `--ta-shadow-2` `--ta-shadow-3` | shadow | |
| `--ta-radius-control` `--ta-radius-card` `--ta-radius-node` | `Npx` | |
| `--ta-ease` | `cubic-bezier(.2,.8,.2,1)` | 全部过渡使用同一曲线 |

默认皮肤同样写入以上变量（取 2.2 节），不存在"默认皮肤不走令牌"的分支。

### 3.4 首屏不闪白

`web/index.html` 的 `<head>` 内、`<title>` 之后插入：

```html
<script>
  (function () {
    try {
      var skin = localStorage.getItem('test-auto-pro-skin')
      var d = document.documentElement
      d.dataset.skin = { 'deep-space': 1, aurora: 1, titanium: 1 }[skin] ? skin : 'default'
      d.dataset.mode = localStorage.getItem('test-auto-pro-theme') === 'dark' ? 'dark' : 'light'
      d.style.colorScheme = d.dataset.mode
    } catch (e) {}
  })()
</script>
```

`layout.css` 顶部加首屏底色（Vue 挂载前生效，挂载后被 Naive 与 `applyAppearance` 接管）：

```css
html[data-mode="dark"]  { background: #101014; }
html[data-mode="light"] { background: #ffffff; }
html[data-skin="deep-space"][data-mode="dark"]  { background: #060b1a; }
html[data-skin="deep-space"][data-mode="light"] { background: #edf2fa; }
html[data-skin="aurora"][data-mode="dark"]      { background: #061513; }
html[data-skin="aurora"][data-mode="light"]     { background: #eaf8f4; }
html[data-skin="titanium"][data-mode="dark"]    { background: #0e1013; }
html[data-skin="titanium"][data-mode="light"]   { background: #e6e9ed; }
```

（此处的色值是首屏兜底，是第 3.1 节"CSS 不出色值"唯一的例外，测试里加白名单并断言它们与令牌表的 `bg` 一致。）

### 3.5 Store

```ts
export type SkinId = 'default' | 'deep-space' | 'aurora' | 'titanium'
// state: productName、themeMode、skinId（新增）
// actions: toggleThemeMode（保留）、setThemeMode(mode)、setSkinId(id)
// 存储键：test-auto-pro-theme（沿用）、test-auto-pro-skin（新增）。读写都 try/catch，失败静默回退默认值。
// 非法的存储值（包括旧版本残留的未知字符串）一律回退 default / light，不做迁移。
```

`App.vue`：

```ts
const naiveTheme = computed(() => (appStore.themeMode === 'dark' ? darkTheme : null))
const themeOverrides = computed(() => buildThemeOverrides(appStore.skinId, appStore.themeMode))
watch([() => appStore.skinId, () => appStore.themeMode], ([skin, mode]) => applyAppearance(skin, mode), { immediate: true })
```

`<n-config-provider :theme="naiveTheme" :theme-overrides="themeOverrides" ...>`。**`RouterView`、`n-layout`、`AmbientLayer` 上不得出现以 skin/mode 为值的 `:key`。**

## 4. 氛围层与三套动效语言

### 4.1 DOM

`AmbientLayer.vue`（常驻，不随皮肤增删节点，只靠 CSS 显隐）：

```html
<div class="app-ambient" aria-hidden="true">
  <i class="app-ambient__a"></i>
  <i class="app-ambient__b"></i>
  <i class="app-ambient__c"></i>
  <i class="app-ambient__grid"></i>
  <i class="app-ambient__spot"></i>
</div>
```

放在 `NConfigProvider` 内、`n-layout.app-shell` 之前（兄弟节点）。基础样式：

```css
.app-ambient { position: fixed; inset: 0; z-index: 0; overflow: hidden; pointer-events: none; contain: strict; }
.app-ambient > i { position: absolute; display: block; pointer-events: none; }
html[data-skin="default"] .app-ambient { display: none; }
.app-shell { position: relative; z-index: 1; }
```

弹窗、下拉、提示由 Naive 传送到 body，其 z-index 远高于 1，不受影响。

### 4.2 深空指挥舱

```css
/* 色底：两团冷光 */
html[data-skin="deep-space"] body { /* 由 Naive 写 bodyColor，这里不覆盖 */ }
[data-skin="deep-space"] .app-ambient::before {
  content: ""; position: absolute; inset: 0;
  background:
    radial-gradient(1200px 700px at 78% -10%, var(--ta-glow-a), transparent 60%),
    radial-gradient(900px 600px at 10% 110%, var(--ta-glow-b), transparent 60%);
}
/* 细网格：64px 周期，向右下漂移，漂满一个周期无缝回绕 */
[data-skin="deep-space"] .app-ambient__grid {
  inset: -64px;
  background-image:
    linear-gradient(color-mix(in srgb, var(--primary-color) 9%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--primary-color) 9%, transparent) 1px, transparent 1px);
  background-size: 64px 64px;
  -webkit-mask-image: radial-gradient(ellipse at 50% 30%, #000 20%, transparent 75%);
          mask-image: radial-gradient(ellipse at 50% 30%, #000 20%, transparent 75%);
  animation: ta-grid-drift 60s linear infinite;
}
@keyframes ta-grid-drift { to { transform: translate3d(64px, 64px, 0); } }
/* 星轨：右上角两道大圆环，缓慢旋转，环上带一枚运行光点 */
[data-skin="deep-space"] .app-ambient__a {
  width: 1500px; height: 1500px; left: 55%; top: -45%; border-radius: 50%;
  border: 1px solid color-mix(in srgb, var(--primary-color) 14%, transparent);
  animation: ta-orbit 240s linear infinite;
}
[data-skin="deep-space"] .app-ambient__a::before { /* 内环 */
  content: ""; position: absolute; inset: 180px; border-radius: 50%;
  border: 1px dashed color-mix(in srgb, var(--primary-color) 10%, transparent);
}
[data-skin="deep-space"] .app-ambient__a::after { /* 光点 */
  content: ""; position: absolute; left: 50%; top: -4px; width: 7px; height: 7px; margin-left: -3px; border-radius: 50%;
  background: var(--primary-color);
}
[data-skin="deep-space"][data-mode="dark"] .app-ambient__a::after { box-shadow: 0 0 18px 4px color-mix(in srgb, var(--primary-color) 70%, transparent); }
[data-skin="deep-space"][data-mode="light"] .app-ambient__a::after { box-shadow: 0 0 0 4px color-mix(in srgb, var(--primary-color) 16%, transparent); }
@keyframes ta-orbit { to { transform: rotate(360deg); } }
/* 光晕跟随：见 4.5 */
[data-skin="deep-space"] .app-ambient__spot {
  width: 520px; height: 520px; left: 0; top: 0; border-radius: 50%;
  background: radial-gradient(circle, var(--ta-glow-a), transparent 65%);
  will-change: transform;
}
[data-skin="deep-space"] .app-ambient__b,
[data-skin="deep-space"] .app-ambient__c { display: none; }
```

浅色版同一套结构，差异只靠令牌（光变淡、光点无发光、网格线变成淡蓝）；浅色版**不得**出现 `box-shadow` 发光和高饱和渐变底。

组件级招牌（`skins.css`）：

```css
/* 卡片对角 L 形刻度角标：左上与右下各一对 14×1 / 1×14 的线，仅用 background-image，不动 background-color */
[data-skin="deep-space"] .n-card {
  background-image:
    linear-gradient(var(--primary-color), var(--primary-color)),
    linear-gradient(var(--primary-color), var(--primary-color)),
    linear-gradient(var(--primary-color), var(--primary-color)),
    linear-gradient(var(--primary-color), var(--primary-color));
  background-size: 14px 1px, 1px 14px, 14px 1px, 1px 14px;
  background-position: left top, left top, right bottom, right bottom;
  background-repeat: no-repeat;
}
/* 顶栏底边流光：一条 240px 的亮线沿底边来回流过 */
[data-skin="deep-space"] .app-header { overflow: hidden; }
[data-skin="deep-space"] .app-header::after {
  content: ""; position: absolute; left: 0; bottom: 0; width: 240px; height: 1px;
  background: linear-gradient(90deg, transparent, var(--primary-color), transparent);
  animation: ta-header-flow 7s linear infinite;
}
@keyframes ta-header-flow { from { transform: translateX(-240px); } to { transform: translateX(100vw); } }
```

交互：按钮悬停＝描边变 `--ta-border-strong` + `box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary-color) 22%, transparent)`；主按钮悬停多一道 `inset 0 0 0 1px` 冰蓝高光。**不使用位移。**

### 4.3 极光玻璃

```css
[data-skin="aurora"] .app-ambient__a,
[data-skin="aurora"] .app-ambient__b,
[data-skin="aurora"] .app-ambient__c {
  width: 720px; height: 720px; border-radius: 50%; filter: blur(90px); will-change: transform;
}
[data-skin="aurora"] .app-ambient__a { left: -8%;  top: -12%; background: var(--ta-glow-a); animation: ta-aurora-a 38s ease-in-out infinite alternate; }
[data-skin="aurora"] .app-ambient__b { right: -10%; top: 8%;  background: var(--ta-glow-b); animation: ta-aurora-b 52s ease-in-out infinite alternate; }
[data-skin="aurora"] .app-ambient__c { left: 28%;  bottom: -30%; background: var(--ta-glow-c); animation: ta-aurora-c 45s ease-in-out infinite alternate; }
@keyframes ta-aurora-a { from { transform: translate3d(-4vw, -3vh, 0) scale(1); }    to { transform: translate3d(22vw, 14vh, 0) scale(1.18); } }
@keyframes ta-aurora-b { from { transform: translate3d(4vw, 6vh, 0) scale(1.1); }     to { transform: translate3d(-24vw, 18vh, 0) scale(.92); } }
@keyframes ta-aurora-c { from { transform: translate3d(-10vw, 4vh, 0) scale(.95); }  to { transform: translate3d(18vw, -16vh, 0) scale(1.15); } }
/* 极光帘幕：竖向光带缓慢横移，顶部强、向下淡出 */
[data-skin="aurora"] .app-ambient__grid {
  inset: 0 -200px 40% -200px; opacity: .55;
  background: repeating-linear-gradient(100deg, transparent 0 70px, color-mix(in srgb, var(--primary-color) 10%, transparent) 100px, transparent 150px);
  -webkit-mask-image: linear-gradient(to bottom, #000, transparent); mask-image: linear-gradient(to bottom, #000, transparent);
  animation: ta-curtain 80s linear infinite;
}
@keyframes ta-curtain { to { transform: translate3d(200px, 0, 0); } }
[data-skin="aurora"] .app-ambient__spot {
  width: 460px; height: 460px; left: 0; top: 0; border-radius: 50%;
  background: radial-gradient(circle, color-mix(in srgb, var(--ta-accent-2) 28%, transparent), transparent 65%); will-change: transform;
}
```

玻璃面（**只**用于下列元素，同屏 `backdrop-filter` 元素不超过 6 个）：`.app-header`、`.app-sidebar`、`.n-modal .n-card`/`.n-dialog`（含 `.n-modal-mask`）、KPI/摘要类卡片（`.kpi`、`.n-card` 在非表格场景）：

```css
[data-skin="aurora"] .app-header, [data-skin="aurora"] .app-sidebar, [data-skin="aurora"] .n-card {
  -webkit-backdrop-filter: blur(18px) saturate(140%); backdrop-filter: blur(18px) saturate(140%);
  border: 1px solid color-mix(in srgb, var(--primary-color) 22%, transparent);
}
/* 含表格、日志、代码的卡片内部一律回到阅读面，禁止玻璃 */
[data-skin="aurora"] .n-card .n-data-table, [data-skin="aurora"] .n-card pre, [data-skin="aurora"] .n-card .n-code { background: var(--ta-reading); }
```

悬停折光：卡片和主按钮悬停时一道高光自左向右扫过，**仅悬停触发，0.8s 一次**：

```css
[data-skin="aurora"] .n-card:hover, [data-skin="aurora"] .n-button--primary-type:hover {
  background-image: linear-gradient(115deg, transparent 40%, rgba(255,255,255,.14) 50%, transparent 60%);
  background-size: 250% 100%; background-position: -20% 0; transition: background-position .8s var(--ta-ease);
}
```

（`.n-card` 初始需 `background-position: 120% 0` 才有"扫过"效果；实施时在同一选择器上补默认态。）

### 4.4 钛金精密

```css
/* 拉丝纹：常驻静态极淡竖纹 */
[data-skin="titanium"] .app-ambient__grid {
  inset: 0; opacity: 1;
  background-image: repeating-linear-gradient(90deg, rgba(255,255,255,.018) 0 1px, transparent 1px 3px);
}
[data-skin="titanium"][data-mode="light"] .app-ambient__grid {
  background-image: repeating-linear-gradient(90deg, rgba(18,22,27,.022) 0 1px, transparent 1px 3px);
}
/* 掠光：两条斜向亮带，低频横扫 */
[data-skin="titanium"] .app-ambient__a, [data-skin="titanium"] .app-ambient__b {
  top: 0; bottom: 0; width: 60vw; left: -60vw; will-change: transform;
}
[data-skin="titanium"] .app-ambient__a { background: linear-gradient(100deg, transparent 30%, var(--ta-glow-a) 50%, transparent 70%); animation: ta-sheen 14s ease-in-out 2s infinite; }
[data-skin="titanium"] .app-ambient__b { background: linear-gradient(100deg, transparent 40%, var(--ta-glow-b) 50%, transparent 60%); animation: ta-sheen 22s ease-in-out 9s infinite reverse; }
@keyframes ta-sheen { 0% { transform: translate3d(0,0,0); } 60%, 100% { transform: translate3d(180vw,0,0); } }
/* 扫描光带：160px 高，自上而下，16s 一轮 */
[data-skin="titanium"] .app-ambient__c {
  left: 0; right: 0; top: -160px; height: 160px; will-change: transform;
  background: linear-gradient(180deg, transparent, var(--ta-glow-c) 50%, transparent);
  animation: ta-scan 16s linear infinite;
}
@keyframes ta-scan { to { transform: translate3d(0, calc(100vh + 160px), 0); } }
[data-skin="titanium"] .app-ambient__spot { display: none; }
```

金色只在三处：

```css
[data-skin="titanium"] .n-menu-item-content--selected::before { /* 侧栏当前项左侧竖条 */
  content: ""; position: absolute; left: 0; top: 8px; bottom: 8px; width: 2px; background: var(--ta-accent-2);
}
[data-skin="titanium"] .n-tabs .n-tabs-tab--active + .n-tabs-bar, [data-skin="titanium"] .n-tabs-bar { background: var(--ta-accent-2); }
[data-skin="titanium"] .app-header::after { /* 顶栏底边居中 64px 短线，静态 */
  content: ""; position: absolute; left: 50%; bottom: 0; width: 64px; height: 1px; margin-left: -32px; background: var(--ta-accent-2);
}
```

主按钮：`background-image: linear-gradient(180deg, lift(primary) 0%, primary 100%)`（用 `color-mix(in srgb, var(--primary-color) 88%, #fff)` 实现）、`box-shadow: var(--ta-shadow-1)`；悬停描边：`box-shadow: 0 0 0 1px color-mix(in srgb, var(--primary-color) 40%, transparent), var(--ta-shadow-2)`。圆角取令牌（控件 6、卡片 8），边线一律 1px 实线，**不使用任何渐变边框或发光**。

### 4.5 指针光晕（深空、极光使用）

`pointerGlow.ts`：

```ts
// 只写一个元素的 transform，不写任何 CSS 变量，不触发整页样式重算。
export function startPointerGlow(spot: HTMLElement): () => void {
  let x = innerWidth * 0.7, y = innerHeight * 0.3, pending = false
  const half = spot.offsetWidth / 2
  const paint = () => { pending = false; spot.style.transform = `translate3d(${x - half}px, ${y - half}px, 0)` }
  const onMove = (e: PointerEvent) => { x = e.clientX; y = e.clientY; if (!pending) { pending = true; requestAnimationFrame(paint) } }
  addEventListener('pointermove', onMove, { passive: true })
  paint()
  return () => removeEventListener('pointermove', onMove)
}
```

`App.vue` 在 `skinId` 为 `deep-space` 或 `aurora` 时启动，切走时停止并清空 `spot.style.transform`。`pointermove` 全局只有这一个监听。光晕跟随通过 `transition: transform 240ms ease-out` 做出"拖尾"感。

### 4.6 性能预算（硬规则，测试做静态检查）

1. 持续动画只允许 `transform`、`opacity`；`filter: blur` 只用在会 `transform` 的大色团上且半径固定；不得对 `filter`、`background-position`、`box-shadow`、`width/height/top/left` 做持续动画（悬停过渡与顶栏 1px 流光除外，后者用 `transform`）。
2. 氛围层 `will-change: transform` 的元素 ≤ 4 个。
3. 同屏 `backdrop-filter` 元素 ≤ 6；表格、日志、代码区域不得使用。
4. 氛围层和装饰伪元素全部 `pointer-events: none`；不得使用 `mix-blend-mode`。
5. 装饰不得依赖业务数据；任何"进度、脉冲、扫描"都不得绑定运行状态——真实状态只来自现有节点/状态组件。
6. 页面不可见时浏览器自动降频，不额外处理；用户决定不联动 `prefers-reduced-motion`，`skins.css` 中**不出现**该媒体查询。
7. 人工核对项：保持页面空闲 1 分钟，任务管理器中浏览器标签页 CPU 应持续低于约 5%（办公机基线）；若超标，先去掉 `filter: blur` 的色团再考虑别的。

## 5. 外观入口 `AppearanceMenu.vue`

- 位置：顶栏 `header-toolbar` 内、现有"浅色/深色"快捷按钮左侧；快捷按钮**保留**（不删除现有入口）。
- 触发：`NButton quaternary size="small"`，图标 `ColorPaletteOutline`（`@vicons/ionicons5`，已安装），文字"外观"。
- 弹层：`NPopover trigger="click" placement="bottom-end" :show-arrow="false" :width="392"`，内边距 16。点选皮肤**不关闭**弹层；点外部或再次点触发按钮关闭。
- 内容自上而下：
  1. 小标题"皮肤"（12px、`text3`）。
  2. 2×2 网格，格间距 12，每格 `168 × 124`：预览区 `168 × 76`、下方皮肤名（14px/600）与一句说明（12px、`text3`）。
  3. 小标题"明暗"。
  4. `NRadioGroup size="small"`，按钮样式，两项"浅色""深色"，与顶栏快捷按钮共用同一个 store 字段。
- 皮肤卡片是 `<button role="radio" :aria-checked>`，外层容器 `role="radiogroup" aria-label="皮肤"`；选中态 2px 主色外环 + 右上角 16px 圆形对勾；键盘左右方向键切换。
- 预览区（纯 CSS 微缩版，用**当前明暗模式**下该皮肤的令牌，让用户看到点下去的真实样子）：底色 `bg` + 两团 `glowA/B` 径向渐变；顶部 10px 高 `chrome` 条；左侧 28px 宽 `chrome` 条；右侧两块 `surface` 小卡（圆角 = `radiusCard ÷ 3` 向上取整、1px `border`）；一枚 8px 圆点 `primary`。
- 文案：

| id | 名称 | 说明 |
| --- | --- | --- |
| `default` | 默认 | 沿用现有配色 |
| `deep-space` | 深空指挥舱 | 冰蓝 · 网格 · 能量流 |
| `aurora` | 极光玻璃 | 青绿 · 玻璃 · 流光 |
| `titanium` | 钛金精密 | 银蓝 · 拉丝 · 掠光 |

- 状态保护：组件不接收也不触发任何业务 emit；切换只调用 `store.setSkinId` / `store.setThemeMode`。

## 6. 弹窗统一（11 处）

### 6.1 统一类（`layout.css`，对所有皮肤生效）

```css
.app-dialog { width: min(var(--dialog-w, 760px), calc(100vw - 48px)); max-height: calc(100dvh - 96px); display: flex; flex-direction: column; }
.app-dialog--sm { --dialog-w: 480px; }
.app-dialog--md { --dialog-w: 760px; }
.app-dialog--lg { --dialog-w: 960px; }
/* preset="card" 与裸 n-card */
.app-dialog > .n-card-header { flex: none; padding: 20px 24px 16px; border-bottom: 1px solid var(--divider-color); }
.app-dialog > .n-card__content { flex: 1 1 auto; min-height: 0; overflow: auto; padding: 20px 24px; }
.app-dialog > .n-card__footer { flex: none; padding: 16px 24px; border-top: 1px solid var(--divider-color); }
/* preset="dialog" */
.app-dialog.n-dialog { padding: 0; }
.app-dialog .n-dialog__title { flex: none; padding: 20px 24px 16px; border-bottom: 1px solid var(--divider-color); }
.app-dialog .n-dialog__content { flex: 1 1 auto; min-height: 0; overflow: auto; padding: 20px 24px; margin: 0; }
.app-dialog .n-dialog__action { flex: none; padding: 16px 24px; border-top: 1px solid var(--divider-color); }
```

弹窗内的 `pre`、原始请求/响应、长错误统一：`font-family: var(--font-family-mono); font-size: 12px; line-height: 18px; white-space: pre-wrap; overflow-wrap: anywhere; background: var(--ta-reading); border: 1px solid var(--divider-color); border-radius: var(--ta-radius-control); padding: 12px; user-select: text;`，**不加 `text-overflow`、不加行数截断**。

新皮肤叠加（`skins.css`）：遮罩 `.n-modal-mask` 深色皮肤 `rgba(2,6,16,.62)`、浅色皮肤 `rgba(18,28,48,.32)`（遮罩色是装饰，允许在 `skins.css` 内出现）；弹窗圆角取 `--ta-radius-card`；阴影 `--ta-shadow-3`；极光额外加玻璃（`blur(18px)`）。

### 6.2 逐个落地

每处：`class` 加 `app-dialog app-dialog--*`；**删除**行内 `width/maxWidth/maxHeight/overflowY` 与行内 `--xxx-color` 变量注入（颜色改用全局变量）；`dialogStyle`/`pickerStyle` 中只剩无法类化的内容才保留。保留 `mask-closable`、`closable`、`loading`、`positive-click` 等行为属性不动。

| # | 文件 | 弹窗 | 尺寸 | 落点 |
| --- | --- | --- | --- | --- |
| 1 | `views/PlanPathsView.vue` | 放弃本次修改 | sm | `n-modal preset="dialog"` 加 class |
| 2 | `views/PlanPathsView.vue` | 删除路径 | sm | 同上；保留 `:loading="deleting"` |
| 3 | `views/PlanPathConfigurationView.vue` | 确认换路并覆盖数据 | md | 内层 `n-card` 加 class，删 `style="width: min(620px, 94vw)"`；保留 `:mask-closable="false" :closable="false"` |
| 4 | `features/history-replay/BaseFormDataPicker.vue` | 选择基础表单数据 | lg | 内层 `n-card.base-form-data-picker` 加 class，替换 `pickerStyle` 的宽度 |
| 5 | `features/path-configuration/ActionOrchestrationEditor.vue` | 动作编排编辑器 | md | 内层 `n-card` 加 class，删 `width: min(720px, 94vw)`；拖拽排序行为不动 |
| 6 | `features/path-configuration/ActionOrchestrationEditor.vue` | 全部人员 | sm | 内层 `n-card` 加 class，删 `width: min(420px, 90vw)` |
| 7 | `features/path-configuration/ActionFlowDialog.vue` | 动作执行流程 | md | `n-modal preset="card"` 加 class，`dialogStyle` 去掉宽度与颜色变量 |
| 8 | `features/run-readiness/RunPreflightDialog.vue` | 开始运行 | md | 同上；保留 `:mask-closable="!loading"` |
| 9 | `features/runs/RunNodePanel.vue` | 计划动作详情 | md | 去掉 `dialogStyle` 中尺寸与颜色注入 |
| 10 | `features/runs/RunNodePanel.vue` | 当前步详情 | lg | 同上 |
| 11 | `features/runs/RunNodePanel.vue` | 第 N 步 · 动作 · 接口详情 | lg | 同上；保留现有 class `run-panel__dialog` 并追加 `app-dialog app-dialog--lg` |

删除颜色注入后，这些弹窗内部原先引用的局部变量（`--run-border-color`、`--flow-border-color`、`--preflight-border-color` 等）必须同步换成全局变量（第 7.1 节映射表），否则会落到回退色。

## 7. 全站改色清单

### 7.1 局部变量命名空间收口

源码里并存 `--run-*`、`--plan-*`、`--path-config-*`、`--picker-*`、`--preflight-*`、`--paths-*`、`--flow-*` 七套局部变量，各自由 `useThemeVars()` 注入。统一为全局变量，映射：

| 原变量（按其定义处取的 Naive 字段） | 替换为 |
| --- | --- |
| `*-text-color`、`*-text-color-1` 或字段为 `textColor1/2` | `--text-color-1`（标题/主文）或 `--text-color-2`（正文） |
| `*-secondary-text-color`、`*-text-secondary-color`、`--flow-secondary-label-color`、字段为 `textColor3` | `--text-color-3` |
| `*-border-color`、`--flow-edge-color`（字段 `borderColor`） | `--border-color` |
| `--run-border-color`、`--divider-color`（字段 `dividerColor`） | `--divider-color` |
| `*-surface-color`、`--flow-surface-color`、`*-card-color`、`*-page-color` | 阅读面用 `--ta-reading`，页面底用 `--body-color` |
| `*-primary-color`、`--flow-direction-color`、`--flow-current-color`、`--path-config-primary-color`、`--picker-active-color` | `--primary-color` |
| `--path-config-warning-color`（原回退 `#d97706`，与主题脱节） | `--warning-color` |
| `--flow-current-background` | `--ta-surface-2` |
| `--flow-user-color` | `--success-color` |

`FlowGraphCanvas.vue` 的 `canvasStyle` 现有写法（从 `useThemeVars()` 取值）可保持，因为 `useThemeVars()` 会跟随 `themeOverrides`；但其中 `--flow-surface-color` 必须改取 `--ta-reading`（画布底即阅读面）。若某处局部变量的定义字段与上表对不上，按字段名就近映射并在提交说明里记一句。

### 7.2 硬编码颜色清除

| 文件:行 | 现状 | 改为 |
| --- | --- | --- |
| `features/flow-graph/FlowGraphNode.vue:261,391,514` | `color: #fff` 压在状态色底上 | `color: var(--ta-on-status)` |
| 同文件 `:293` | `var(--flow-secondary-label-color, #777)` | `var(--text-color-3)` |
| 同文件 `:303` | `var(--n-text-color-3, rgba(120,120,120,.75))` | `var(--text-color-3)` |
| 同文件 `:327` | `box-shadow: 0 1px 3px rgb(0 0 0 / 6%)` | `box-shadow: var(--ta-shadow-1)` |
| 同文件 `:163,326,457` | 圆角 4/8/6 | 节点主体 `var(--ta-radius-node)`；其余内部小元素保持 |
| 同文件各状态规则中的 `var(--success-color, #18a058)` 等 | 带十六进制回退 | 去掉回退值（全局变量已有定义） |
| `features/flow-graph/FlowTreeEdge.vue:125,135` | `stroke: #d03050` | `stroke: var(--error-color)` |
| `features/path-configuration/NodeConfigurationPanel.vue:137-143` | `#e5e7eb` `#64748b` | `var(--divider-color)` `var(--text-color-3)` |
| `features/path-configuration/FormDataHintsPanel.vue:163` | `var(--n-border-color, #eee)` | `var(--divider-color)` |
| `features/path-configuration/ActionOrchestrationEditor.vue:420-430` | 变量回退 `#606266 #d97706 #2080f0` | `var(--text-color-2)` `var(--warning-color)` `var(--primary-color)` |
| `features/run-readiness/RunPreflightDialog.vue:387-404` | `rgba(24,160,88,.08)`、`#18a058`、`rgba(240,160,32,.1)`、`#f0a020`、`#fff` | `color-mix(in srgb, var(--success-color) 10%, transparent)`、`var(--success-color)`、warning 同理、`var(--ta-on-status)` |
| `features/runs/RunNodePanel.vue` 内联 `style="color: var(--run-secondary-text-color, #909090)"` 两处 | 回退 `#909090` | `class="run-panel__muted"`（新增，`color: var(--text-color-3)`） |
| `features/runs/RunNodePanel.vue` 各 `var(--x, #hex)` 回退 | 同上 | 去掉十六进制回退 |
| `features/path-configuration/FormRuntimeFrame.vue:325` | `background: #fff` | **保持不变**（原生表单白底，白名单） |

改完后全库 `rg "#[0-9a-fA-F]{3,8}\b" web/src --glob '!**/appearance/skins.ts'` 只允许剩下：`FormRuntimeFrame.vue` 的 `#fff`、`layout.css` 的首屏底色、第 7.3 节无命中（图表色来自令牌）。测试按此白名单断言。

### 7.3 报告图表

新增 `chartTheme.ts`，**不使用** `echarts.registerTheme`（会触发重新初始化，丢图例选择），改为每个 option 展开合并：

```ts
export function chartBaseOption(t: SkinTokens) {
  return {
    color: [t.primary, t.info, t.accent2, t.success, t.warning],
    textStyle: { color: t.text2, fontFamily: sansStack },
    legend: { textStyle: { color: t.text2 } },   // 不得写 legend.selected，保持图例状态
    tooltip: { backgroundColor: t.raised, borderColor: t.border, textStyle: { color: t.text1 }, extraCssText: `box-shadow:${t.shadow2};border-radius:${t.radiusControl}px;` },
    xAxis: { axisLine: { lineStyle: { color: t.border } }, axisLabel: { color: t.text3 }, splitLine: { lineStyle: { color: t.divider } } },
    yAxis: { axisLine: { lineStyle: { color: t.border } }, axisLabel: { color: t.text3 }, splitLine: { lineStyle: { color: t.divider } } },
  }
}
export function statusPalette(t: SkinTokens) {
  return { 成功: t.success, 失败: t.error, 已停止: t.warning, 已取消: t.text3, 结果待确认: t.info }
}
```

`ReportMetricsPanel.vue`：`resultColors` 改用 `statusPalette`；`failurePalette = [error, warning, accent2, info, text3]`；`buildLatencyOption(…, t.info)` 与 `(…, t.success)`；每个 option 为 `{ ...chartBaseOption(tokens), ...原有内容 }`，其中 tokens 来自 `skinTokens[appStore.skinId][appStore.themeMode]`（`computed`）。`ReportChart.vue` 的 `<VChart :option>` 保持无 `key`，`autoresize` 保持。`accent2` 在图表里**只**作为第三序列的区分色，不用来表达任何状态。

## 8. 组件与页面造型

### 8.1 所有皮肤共用（`layout.css`，包含默认皮肤）

| 区域 | 规格 |
| --- | --- |
| 页面骨架 | `.app-main > .n-layout-scroll-container { padding: 32px var(--app-content-pad-x) 48px }`（原为 40px 上边距，统一到 4 的倍数栈） |
| 页面标题 | 见 2.5；标题与内容块间距 24 |
| 表格 | 行高 44；表头 12px/600/`text2`/`surface2` 底；数字列 `tabular-nums`；hover 行底 `--table-color-hover`；**不用斑马纹** |
| 分页 | 右对齐，距表格 16 |
| 空态 | 保持现有"只文字、不要图标"规则；文字 `text3`，居中，上下 48 |
| 标签 `n-tag` | 12px，高 22，圆角 `--ta-radius-control`；状态标签用 `color-mix(in srgb, var(--status) 12%, transparent)` 底 + 状态色字 |
| 滚动条 | 全站 `scrollbar-width: thin; scrollbar-color: color-mix(in srgb, var(--text-color-3) 45%, transparent) transparent`；WebKit 同步 8px 宽、圆角 4px |
| 聚焦环 | 所有新皮肤：`:focus-visible { outline: 2px solid var(--primary-color); outline-offset: 2px }`；默认皮肤保持 Naive 现有焦点样式 |
| 过渡 | 统一 `160ms var(--ta-ease)`，只过渡 `color, background-color, border-color, box-shadow, opacity`；**不得过渡 `width/height/transform` 以免挪动命中区** |

### 8.2 新皮肤共用造型（`skins.css`）

- 侧栏菜单项高 40，圆角 `radiusControl`，当前项 `primary` 14% 底 + 主色字；钛金额外加第 4.4 节金色竖条。
- 页签：下划线 2px，当前项主色（钛金用 `accent2`）。
- 主按钮：深空＝纯色 + 悬停高光描边；极光＝纯色 + 折光扫过；钛金＝竖向轻渐变 + 内顶高光。
- 次按钮/幽灵按钮：1px `--border-color` 描边，悬停 `--ta-border-strong`。
- 输入框：底取派生输入色，边 `--border-color`，聚焦边 `--primary-color` + `0 0 0 3px color-mix(in srgb, var(--primary-color) 22%, transparent)`。
- 下拉菜单/气泡：`raised` 底、`radiusControl`、`shadow2`，不使用玻璃（含选项列表，便于阅读）。
- 状态点、状态角标、节点状态边框：只用 `--success/warning/error/info-color`，禁止出现 `accent2`。

### 8.3 流程画布（`FlowGraphCanvas.vue` / `FlowGraphNode.vue`）

- 几何不动：节点 `180 × 106`、端口位置、边路径、`layout.ts` 全部不改。圆角只改 `border-radius`，不改 `border-width`、`padding`、`box-sizing`。
- 节点圆角 `--ta-radius-node`；内部状态角标（9px、50%）保持原值。
- 节点底 `--ta-reading`；边 `--border-color`；方向色 `--primary-color`；选中态外环 `0 0 0 4px color-mix(in srgb, var(--info-color) 18%, transparent)`（现状保持）。
- 运行中的"其余节点压暗 `opacity: .56`"保持，不被皮肤改写。
- 画布背景点阵/网格的颜色改取 `--divider-color`；新皮肤不在画布内叠氛围层（画布是阅读面）。
- 控件（缩放、折叠）：`--ta-surface` 底、`--border-color` 边、`--ta-radius-control`。

### 8.4 表单外框（`FormRuntimeFrame.vue` 周边）

仅调整外框容器：`--ta-reading` 底、`--divider-color` 边、`--ta-radius-card` 圆角、`overflow: hidden`、工具栏高度不变（`--path-config-form-toolbar-height` 保持）。iframe 自身保持 `background:#fff; border:0`，不注入任何变量，不改 `sandbox`、`src`、消息协议。

## 9. 实施顺序与每步验收（对应 F-049 三个提交）

### 第一步：接入四套外观与独立深浅切换

1. 写 `color.ts`、`skins.ts`（令牌）、`preferences.ts`、`naiveTheme.ts`、`applyAppearance.ts`。
2. 改 `stores/app.ts`、`App.vue`、`index.html`、`main.ts`；新增 `AppearanceMenu.vue`、`AmbientLayer.vue`（此步 `skins.css` 只放首屏底色与空壳）。
3. `layout.css` 先放首屏底色与全局变量用到的最小规则。
4. 自动验证：`appearance_test.mjs` 的主题与偏好、令牌与对比度、Naive 默认值对照、结构（无 skin 为 key）；`pnpm --dir web typecheck`。
5. 提交："接入四套外观与独立深浅切换"。

### 第二步：统一页面排版与弹窗层次

1. `layout.css` 完整化（第 8.1 节）；`styles.css` 去重。
2. 第 7.1/7.2 节逐文件改色；第 6.2 节 11 个弹窗；第 7.3 节图表；第 8.3、8.4 节画布与表单外框。
3. 自动验证：硬编码色白名单测试、11 个弹窗类断言、`flow_graph_test.mjs`、运行详情生命周期与 `action_detail_structure_test.mjs`、报告相关用例、`typecheck + build`。
4. 提交："统一页面排版与弹窗层次"。

### 第三步：加上三套动效，再做整体验证

1. `skins.css` 写入第 4 节全部氛围、招牌与交互；`pointerGlow.ts` 接入。
2. 自动验证：性能预算静态检查（第 4.6 节 1–4 条）、`pointer-events:none` 断言、无 `prefers-reduced-motion`、`run-f049.ps1` 全量、`git diff --check`。
3. 状态改为 `ready_for_manual`，写入仍需人工检查的视觉项（空闲 CPU、八种组合、长列表滚动）。
4. 提交："完善未来感动效与外观回归"。

## 10. 自动测试（`test/unit/frontend/appearance/appearance_test.mjs`）

用 `node --experimental-strip-types --test` 运行，沿用 F-048 的 Windows 调用方式；`naive-ui` 只在对照用例中 `require`。

1. **目录**：`SkinId` 恰有 4 个；`skinTokens` 含 3 个新皮肤 × 2 模式，且每个都有第 2.1 节全部字段；`skinCatalog` 文案与第 5 节一致。
2. **默认皮肤即现状**：`defaultTokens` 的 light/dark 逐字段等于 `lightTheme.common` / `darkTheme.common`；`buildThemeOverrides('default', *) === undefined`。
3. **对比度**（WCAG 相对亮度，对不透明色算；半透明令牌不参与此项）：`text1/bg ≥ 10`、`text2/bg ≥ 6.5`、`text3/bg ≥ 4.5`、`text3/reading ≥ 4.5`、`primary/reading ≥ 4.5`、`onPrimary/primary ≥ 4.5`、四个状态色与 `reading` ≥ 4.5、`onStatus` 与四个状态色 ≥ 4.5。覆盖 6 套新皮肤。
4. **独立性**：任选皮肤 × 任选明暗共 8 种组合都能解析出令牌；`readSkinId` 对 `null`/`''`/`'x'`/大小写错误值返回 `default`，`readThemeMode` 同理返回 `light`；两个存储键互不影响。
5. **覆盖完整**：`buildThemeOverrides` 对每个新皮肤给出 `primary/info/success/warning/error` 的 `Hover/Pressed/Suppl`，且 `Hover ≠ 基色`；`Layout.color` 为全透明；`common.cardColor` 与 `common.bodyColor` 为不透明色。
6. **状态保护（结构断言，读源码文本）**：`App.vue`、`router`、`FormRuntimeFrame.vue`、`ReportChart.vue`、`FlowGraphCanvas.vue` 中没有任何以 `skinId`/`themeMode` 为值的 `:key`；`FormRuntimeFrame.vue` 不 import `appearance/*`；`form-runtime/` 无改动。
7. **氛围层规则（读 `skins.css`）**：每个 `.app-ambient` 相关选择器与所有装饰伪元素规则都含 `pointer-events: none`（或继承自已含该声明的父规则）；不出现 `prefers-reduced-motion`；`will-change` 出现次数 ≤ 4×（每皮肤）；持续动画的 `@keyframes` 内只出现 `transform`/`opacity`。
8. **弹窗**：`rg "<n-modal"` 命中 11 处；每一处所在元素或其内层 `n-card` 带 `app-dialog` 与一个尺寸类；`dialogStyle`/`pickerStyle` 中不再含 `width`、`maxWidth`。
9. **硬编码颜色白名单**：按第 7.2 节末尾的规则扫描 `web/src`，结果必须等于白名单。
10. **图表**：`chartBaseOption`/`statusPalette` 的颜色全部来自令牌；输出不含 `legend.selected`。
11. **首屏兜底色**：`layout.css` 的 8 条首屏底色与令牌表 `bg` 一致。

`test/run-f049.ps1`：依次运行上述 Node 用例、F-049 涉及的既有回归（`flow_graph_test.mjs`、`action_detail_structure_test.mjs`、运行详情生命周期、报告相关）、`pnpm --dir web typecheck`、`pnpm --dir web build`、`git diff --check`。不启动浏览器，不连接真实目标。

## 11. 常见坑（写给实施者）

1. Naive 的 `themeOverrides` 里 `rgba()` 需写成带空格的标准写法最稳；半透明色只能放 `Layout.headerColor/siderColor` 与 `Card.color`，其余一律不透明。
2. 全局变量写在 `<html>`，所以任何弹窗内**不要再**用 `useThemeVars()` 往 `style` 里塞颜色；那是旧的绕路写法，删掉，不保留。
3. `.n-card` 的 `background-image` 招牌只写 `background-image/size/position/repeat`，不要写 `background` 简写，否则会冲掉 Naive 的 `background-color`。
4. 新皮肤给 `.n-card` 加边框时用 `border`，不要改 `box-sizing` 或内边距，避免表格卡片内部出现 1px 抖动。
5. 极光的玻璃与 `will-change` 色团同屏时，先保证 `backdrop-filter` 元素数量，再考虑色团半径。
6. 不要在 `AmbientLayer` 或氛围 CSS 里读业务 store；氛围层与业务状态零耦合。
7. 浅色版的"光"一律用低饱和柔光团和投影表达；看到发光描边就是做错了。
8. 默认皮肤也会走 `applyAppearance`，所以补全局变量后它的"次要文字"会真正变灰，这是预期的修缺陷，提交说明里写一句。
9. 字体不得引入网络字体或新增依赖；若某机没有 `JetBrains Mono`，字体栈会自然落到 `Cascadia/Consolas`。
