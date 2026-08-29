---
name: Family Finance OS
description: 面向中国家庭的自托管 AI 财务管家——精算师书房气质的确定性财务界面
colors:
  paper-mist: "#f5f7fb"
  ledger-ink: "#172033"
  ledger-ink-muted: "#616e85"
  surface: "#ffffff"
  surface-muted: "#f8fafc"
  hairline: "#e5eaf2"
  signal-blue: "#2563eb"
  signal-blue-deep: "#1d4ed8"
  signal-blue-wash: "rgb(37 99 235 / 20%)"
  signal-blue-tint: "#eff6ff"
  audit-green: "#047857"
  audit-green-tint: "#e8f7ef"
  caution-amber: "#a16207"
  caution-amber-tint: "#fff7df"
  risk-red: "#b42318"
  risk-red-tint: "#fff6f4"
typography:
  display:
    fontFamily: "'Archivo Variable', Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif"
    fontSize: "clamp(1.65rem, 5vw, 2.35rem)"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.035em"
  title:
    fontFamily: "'Archivo Variable', Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif"
    fontSize: "1.03rem"
    fontWeight: 700
    lineHeight: 1.4
    letterSpacing: "-0.01em"
  metric:
    fontFamily: "'Archivo Variable', Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif"
    fontSize: "clamp(1.2rem, 4.7vw, 1.75rem)"
    fontWeight: 800
    lineHeight: 1.2
    letterSpacing: "-0.035em"
  body:
    fontFamily: "'Archivo Variable', Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif"
    fontSize: "0.85rem"
    fontWeight: 400
    lineHeight: 1.55
  label:
    fontFamily: "'Archivo Variable', Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif"
    fontSize: "0.7rem"
    fontWeight: 800
    letterSpacing: "0.13em"
    textTransform: "uppercase"
  caption:
    fontFamily: "'Archivo Variable', Inter, ui-sans-serif, -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1.4
rounded:
  control: "10px"
  card: "16px"
  inner: "12px"
  form: "14px"
  pill: "999px"
spacing:
  xs: "0.2rem"
  sm: "0.35rem"
  md: "0.65rem"
  lg: "0.9rem"
  xl: "1rem"
  2xl: "1.25rem"
components:
  button-primary:
    backgroundColor: "{colors.signal-blue}"
    textColor: "#ffffff"
    rounded: "{rounded.control}"
    padding: "0.68rem 1rem"
  button-primary-hover:
    backgroundColor: "{colors.signal-blue-deep}"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ledger-ink}"
    rounded: "{rounded.control}"
    padding: "0.68rem 1rem"
  button-danger:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.risk-red}"
    rounded: "{rounded.control}"
    padding: "0.68rem 1rem"
  input-text:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ledger-ink}"
    rounded: "{rounded.control}"
    padding: "0.68rem 0.75rem"
  card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ledger-ink}"
    rounded: "{rounded.card}"
  status-pill-ok:
    backgroundColor: "{colors.audit-green-tint}"
    textColor: "{colors.audit-green}"
    rounded: "{rounded.pill}"
    padding: "0.2rem 0.6rem"
  status-pill-warn:
    backgroundColor: "{colors.caution-amber-tint}"
    textColor: "{colors.caution-amber}"
    rounded: "{rounded.pill}"
    padding: "0.2rem 0.6rem"
---

# Design System: Family Finance OS

## Overview

**Creative North Star: "精算师的书房"（The Actuary's Study）**

这是一个安静、严谨、可复核的财务工作空间：白纸般的表面、墨水色的文字、单一蓝色印记。它的气质不是消费级 App 的讨好，也不是营销页的热烈，而是一间灯光充足的书房——桌上摊着账本，每个数字都能指回来源，AI 的话与算出来的数字在纸面上泾渭分明。

密度偏舒适偏高：财务总览要求一屏内扫视全局（净资产、结余、Safe-to-Spend、债务风险同屏），但绝不拥挤——信息通过两列卡片网格与清晰的内边距呼吸。表面处理极度克制：白底、1px 发丝边框、一层环境阴影，没有任何装饰性渐变或大面积色块。唯一的色彩叙事是语义性的：蓝是"系统在对你说话"，绿是"账目健康"，琥珀是"需要留意"，红是"数据有问题或超支"——颜色永远携带状态含义，不为美化而出现。

界面语言为简体中文，文案语气诚实、精确、保守——宁可让数字显得比实际更不确定，绝不显得比实际更确定。

**Key Characteristics:**
- 单一蓝色印记（#2563eb）承担全部交互色彩；语义色（绿/琥珀/红）只用于状态
- 1px 发丝边框 + 单一环境阴影定义所有容器，无多层阴影叙事
- 大写 eyebrow 标签（letter-spacing 0.13em）是每个面板的"档案标签"
- 金额数字是视觉主角：metric 级字重 800、负字距、任何地方都清晰可读
- 扎实可靠的手感：700-800 字重的控件文字、按下位移反馈、清晰的 focus 环

## Colors

调色板的性格是"白纸 + 墨水 + 单一印记 + 语义色信号"：中性色承担 95% 的表面，蓝色只在交互与强调处出现，绿/琥珀/红是状态灯而非装饰。

### Primary
- **信号蓝 Signal Blue** (#2563eb)：全系统唯一的交互色——按钮底色、链接、eyebrow 标签、focus 环、图标与图表强调。它的稀有性就是它的价值：在任何一屏上蓝色面积不超过 10%。
- **信号蓝·深 Signal Blue Deep** (#1d4ed8)：主按钮 hover 态、强调卡边框。只作为 #2563eb 的按压深化，不独立出现。

### Secondary
- **审计绿 Audit Green** (#047857)：数据质量健康（`data_quality=full`）、成功状态。配底 #e8f7ef 药丸（`--success-tint`）。
- **谨慎琥珀 Caution Amber** (#a16207)：数据质量 partial/stale/unknown、预算超支风险预警。配底 #fff7df 药丸（`--warning-tint`）。
- **风险红 Risk Red** (#b42318)：错误、超支、不可恢复状态。配底 #fff6f4（`--danger-tint`）。危险按钮描边使用其 45% 透明变体。
- **墨灰基准** (#616e85)：辅助文字在纸雾背景上仍保持 ≥4.5:1 对比（WCAG AA）。

### Neutral
- **纸雾 Paper Mist** (#f5f7fb)：应用背景。左上角有一层 7% 透明的信号蓝径向渐变（34rem 半径），是整个系统唯一的"环境光"。
- **账本墨 Ledger Ink** (#172033)：正文与标题文字。唯一允许在白色表面上的高对比色。
- **墨灰 Ledger Ink Muted** (#616e85)：辅助文字、标签、提示、图表轴。所有次级信息的标准色；在纸雾背景与所有表面上保持 ≥4.5:1（WCAG AA）。
- **瓷白 Surface** (#ffffff)：所有卡片、面板、输入框表面。
- **雾白 Surface Muted** (#f8fafc)：嵌套区域（AI 回答框、表单内嵌区）的背景。
- **发丝 Hairline** (#e5eaf2)：唯一边框色。所有容器、分隔线、输入框描边。

### Named Rules
- **Single Voice Rule（单一印记规则）。** 信号蓝是系统中唯一的声音。装饰性多色、渐变按钮、品牌色块在这个系统里没有位置；蓝色出现即意味着"这里可以交互"或"这里是系统强调"。
- **Semantic Color Rule（语义色纪律）。** 绿、琥珀、红只允许出现在状态上下文（数据质量、预算风险、错误、超支）。永远不为视觉趣味引入它们；状态色不承载品牌表达——静态图表柱色同样不受语义色污染，普通支出不等于错误。
- **Two-Step Danger Rule（危险两步规则）。** 破坏性操作的按钮默认为描边态（白底红字红边），hover/按压才填充为实心风险红。危险必须比主要操作多一道视觉门槛。

## Typography

**Display Font:** Archivo Variable（自托管，`@fontsource-variable/archivo`，拉丁/数字子集 100-900 可变字重；fallback: Inter → 系统字族）
**Body Font:** Archivo Variable（同栈）
**CJK 字族策略:** 中文字符不随 Archivo 加载（unicode-range 隔离），按系统字族回落：PingFang SC / Hiragino Sans GB（iOS/macOS）、Microsoft YaHei（Windows）——完整 CJK 字体 3MB+ 不适合自托管 PWA
**Label/Mono Font:** 无独立 mono 字族；金额启用 `tabular-nums` 等宽数字

**Character:** Archivo 是机构报表风格的 grotesque——为报纸数据栏与年度报告设计，数字紧凑清晰、大写宽正，与"精算师的书房"的书房气完全同源。个性来自字重对比（400 → 800）与负字距的大数字；中文正文走系统苹方/雅黑，与 Archivo 的中性几何感天然协调。`font-display: swap` 保证文字先以回落字体呈现、字体就绪后无阻塞切换。

### Hierarchy
- **Display** (700, clamp(1.65rem, 5vw, 2.35rem), -0.035em)：页面标题（"家庭财务总览"），每屏最多一个。
- **Metric** (800, clamp(1.2rem, 4.7vw, 1.75rem), -0.035em)：指标卡的金额数字。系统真正的视觉主角，任何界面优先保证它的可读性。
- **Title** (700, 1.03rem, -0.01em)：面板标题（"现金流"、"预算"）。
- **Body** (400, 0.85rem, 1.55)：正文、AI 回答、提示。行高 1.55 保证中文长段可读。
- **Label** (800, 0.7rem, 0.13em, 大写)：eyebrow 面板标签——"FAMILY FINANCE OS"、"CASHFLOW"。英文大写是刻意的档案标签语言，与中文标题形成对照。
- **Caption** (400, 0.75-0.82rem)：辅助说明、状态药丸文字（药丸内为 800 字重）。

### Named Rules
**The Number Protagonist Rule（数字主角规则）。** 界面为金额服务：metric 数字永远是卡片中最大、最重的元素，hint 与 label 退居其后。禁止让标题比金额更抢眼。

**The Aligned Digits Rule（对齐数位规则）。** 所有金额与表格数字启用 `font-variant-numeric: tabular-nums`——逐位等宽是账本的基本尊严，右对齐的金额列必须上下对齐可扫描。

## Layout

单列流式布局，`app-shell` 容器 `min(1180px, 100%)` 居中，左右内边距 1rem（移动）/1.5rem（桌面），并尊重 `env(safe-area-inset-*)`（iPhone 刘海与底部安全区是真实使用场景）。

**月份上下文**：`period` select + ‹ › 步进按钮（`period-stepper`，按钮 44px 触控尺寸配负 margin 吸回视觉位置）——切换月份即自动加载；历史月显示中性底"历史月"药丸 + 面板标题如实命名"X 年X月现金流"（不再错标"本月"）；数据加载中在状态行明示，失败后不残留"正在切换"假状态。键盘 `←`/`→` 切月、`/` 聚焦 AI 顾问（顶栏 `keyboard-hint` 明示按键，仅桌面显示——触屏无键盘）。AI 回答在 meta 声明行锚定"提问时查看"的月份，切月后不产生归属歧义。

移动优先（手机是一级客户端）：默认单列，指标为 2 列网格（gap 0.75rem）；≥720px 时指标变 4 列、内容区变 2 列、顶栏变横向。断点只有一个（720px），刻意不引入更多响应式层级。**720px 是唯一断点来源**：桌面侧 `min-width: 720px`（style.css），移动侧 `max-width: 719.98px`（组件 scoped 样式，避开亚像素重叠）——新增响应式行为时不得再引入第三个值。

间距节奏以 rem 离散值构成：卡片内边距 1rem，网格 gap 0.65-0.9rem，面板间距 0.9rem，顶栏下边距 1.25rem。节奏来自小步距的重复，而非大开大合。

密度哲学：一屏总览优先。首页要求净资产、本月结余、储蓄率、Safe-to-Spend、应急资金月数、债务风险同屏可见——这是产品的核心承诺，布局密度为此服务。

## Elevation & Depth

**环境光哲学**：全系统只有一个阴影值——`0 12px 32px rgb(15 23 42 / 7%)`——像一盏从左上方来的柔光灯，让所有卡片从纸雾背景上轻轻浮起同一高度。阴影不表达层级，只表达"这是一个可触碰的表面"。

### Shadow Vocabulary
- **环境阴影**（`0 12px 32px rgb(15 23 42 / 7%)`）：metric-card、panel、loading-state 共用。hover 不加深、弹窗不更重——深度一律恒定。

### Named Rules
**The Constant Light Rule（恒定光源规则）。** 阴影是环境，不是层级。卡片、面板、弹窗浮起的高度相同；需要区分层级时用边框与背景色调（surface vs surface-muted vs tint），不用阴影加码。

## Shapes

圆角语言分三级：控件级 10px（按钮、输入框）、容器级 16px（卡片、面板）、嵌套级 12-14px（AI 回答框、内嵌表单），状态药丸用全圆 999px。所有圆角是连续的柔和曲面，无斜切、无非对称圆角。

边框无处不在但极轻：1px 发丝线（#e5eaf2）是容器的默认轮廓，输入框、分隔线、嵌套区域共享同一条线。强调卡（emphasis）用 25% 透明的信号蓝边框替代普通边框，是该系统唯一的"高亮边框"手法。

图表柱形顶部圆角 8px（`borderRadius: [8, 8, 0, 0]`），与卡片的圆角语言延续。

## Components

### Buttons
- **Shape:** 10px 圆角
- **Primary:** 信号蓝底 (#2563eb) + 白字，字重 700，padding 0.68rem × 1rem
- **Hover / Active:** hover 深化为 #1d4ed8；active 下沉 1px（`translateY(1px)`）——按压有物理反馈
- **Secondary:** 白底 + 墨字（600 字重）+ 发丝边框——用于"退出登录""取消"等非主路径操作；hover 换雾白底、边框加深
- **Danger:** 白底 + 风险红描边（45% 透明）+ 风险红文字（600 字重）——用于"停用成员""删除资产"等破坏性操作；默认描边态，hover 才填充风险红（两步视觉确认）
- **Disabled:** opacity 0.58，`cursor: not-allowed`
- **Focus:** 3px 20% 信号蓝 outline，offset 2px
- **Ghost/Link 变体:** `.button-link`——无边框透明底、信号蓝文字（700 字重），用于行内操作（如"管理"入口）；padding 0.5rem × 0.85rem 配负 margin 保持视觉紧凑，移动端触控高度 ≥44px

### Cards / Containers
- **Corner Style:** 16px 圆角
- **Background:** 瓷白 (#ffffff)；强调卡用 145° 线性渐变 #eff6ff → #ffffff(70%) + 25% 蓝边框
- **Shadow Strategy:** 单一环境阴影（见 Elevation）
- **Border:** 1px 发丝线；仅强调卡换蓝色 25% 边框
- **Internal Padding:** 1rem；metric-card 最小高度 118px
- **面板结构:** 面板标题 = eyebrow 标签 + 标题 + 右侧操作区（space-between）

### Inputs / Fields
- **Style:** 白底、1px 发丝边框、10px 圆角，padding 0.68rem × 0.75rem
- **Focus:** 3px 20% 信号蓝 outline，offset 2px——与按钮一致，全系统统一 focus 语言
- **Textarea:** 全宽、垂直可伸缩、行高 1.55

### Status Pills
- **Style:** 全圆 999px、浅色语义底 + 深色语义字（800 字重、0.75rem）
- **状态映射:** 健康=绿底绿字；partial/stale/unknown=琥珀底琥珀字；错误=红底红字
- **历史月标记:** `.status-pill--period` 用中性底（雾白 + 墨灰）——历史月是事实陈述，不携带数据质量语义，故不能用语义色
- **用途:** 数据质量状态是产品诚实性承诺的视觉锚点，药丸必须显眼且不可省略

### Notices
- **Style:** 无阴影的浅色语义底容器（琥珀 #fffdf4 / 红 #fff6f4），10-16px 圆角，0.85rem 文字
- **用途:** 数据质量警告、错误信息。与面板的最大区别：notice 无边框无阴影，靠底色说话

### Navigation
- **Style:** 无传统导航栏——顶栏 = eyebrow + H1 + 用户标识 + 上下文控件（月份选择、household 切换、刷新）
- **Mobile:** 顶栏纵排；≥720px 变 space-between 横排
- 上下文控件区在移动端为 2 列网格，桌面为 `minmax(200px, auto) auto auto`——月份列必须容纳 44px 触控步进器，与刷新按钮零重叠

### Charts (ECharts)
- **Style:** 柱状图，柱顶圆角 8px，最大柱宽 54px；轴线与刻度线隐藏，分割线用虚线（发丝色）
- **颜色:** 柱色从 CSS token 读取——收入=信号蓝（系统在陈述事实），支出=账本墨；轴标签用墨灰。刻意不用绿/红：普通支出不是"错误"，静态柱色不携带价值判断（语义色纪律的延伸）
- **加载:** 图表组件经 `defineAsyncComponent` 拆为独立 chunk，不进首屏 bundle
- **高度:** 220px，随容器宽度响应

### Advisor Result（AI 回答区）
- **Style:** 雾白底 (#f8fafc)、12px 圆角、无阴影、行高 1.6、`white-space: pre-wrap`
- **Blocked 态:** 橙红底 `--blocked-bg` (#fff7ed) + `--blocked-text` (#9a3412)——AI 拒答或要求确认时必须视觉可区分
- **review-box:** 左侧 3px 信号蓝竖线的引用式复核框——AI 建议附带的"事实/假设/替代方案"结构化复核区

### Signature: Metric Card（指标卡）
系统的标志性组件。label（墨灰 700 字重 0.78rem）+ value（metric 级 800 字重负字距大数字）+ hint（墨灰 0.79rem）三段式，最小高度 118px。emphasis 变体用蓝渐变底 + 蓝边框标记"最重要的那个数字"（如 Safe-to-Spend）。**danger 变体**：当数字为负面语义（负 Safe-to-Spend/负净现金流/负净资产）时，切风险红系渐变 + 红边框 + 红数值——即使同时是 emphasis，风险红也覆盖权威蓝（语义色在"坏数字时刻"优先于品牌强调）。

## Do's and Don'ts

### Do:
- **Do** 让蓝色只出现在交互与系统强调处——任何一屏蓝色面积 ≤10%
- **Do** 用语义药丸明示数据质量（full/partial/stale），这是产品的诚实性承诺
- **Do** 让数值决定语义色：预算使用率 ≥100% 行文字转风险红、80-99% 转琥珀；负的 Safe-to-Spend/净资产/净现金流用 danger 卡（语义色强度先于品牌强调）
- **Do** 给破坏性与不可逆操作加确认门槛：角色变更、覆盖已有 asset_ref 的保存都必须 confirm；覆盖类冲突在输入时即用琥珀提示明示
- **Do** 把失败如实说清：错误码翻译为中文（errorText 映射），失败后不残留加载/切换中的假状态，旧数据标注其归属月份而非宣称"已过期"
- **Do** 保持金额数字为任何卡片中最大最重的元素（字重 800、负字距）
- **Do** 为所有交互控件提供 3px 20% 蓝色 focus 环——键盘可达性是生产标准
- **Do** 在 AI 输出与确定性数字之间保持视觉区隔（雾白底 / review-box 引用线 / `advisor-result__meta` 声明行）
- **Do** 尊重 `prefers-reduced-motion`（全系统已有全局禁动规则）

### Don't:
- **Don't** 引入第二个品牌色或多色渐变按钮——单一印记是系统身份
- **Don't** 用阴影制造层级——层级用边框、底色、嵌套表达，阴影恒定
- **Don't** 在状态语义之外使用绿/琥珀/红做装饰
- **Don't** 让装饰元素挤占一屏总览的空间——密度为扫视服务
- **Don't** 硬编码金额格式——金额必须经 `money.ts` 按币种精度解析（int64 minor 单位以 string 传输）
- **Don't** 让 AI 输出看起来像系统计算结果——两者的视觉可信等级必须可区分
