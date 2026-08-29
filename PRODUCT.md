# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- **现阶段唯一日常用户：项目作者本人**——自托管运维者兼开发者，理解净资产、Safe-to-Spend、Avalanche/Snowball 等财务概念，主要在手机上使用，也在 PC 上做规划与复盘。
- **未来受众（开源推广后）：其他中国自托管家庭**——具备服务器与一定动手能力的个人/家庭。现阶段 UI 决策先为单一重度用户优化，但不做与"任何中国家庭都能用"相冲突的设计承诺。
- **家庭成员角色（owner/editor/viewer）目前无真实使用者**，是为未来预留的权限模型；设计无需为非技术家庭成员做引导式降级，但不得让未来接入变得困难。

## Product Purpose

Family Finance OS 是面向中国家庭的自托管 AI 智能财务管家，定位是"**家庭财务事实系统 + 确定性决策引擎 + AI 财务顾问**"，不是又一个记账软件。闭环：可靠记录 → 财务画像 → 计算状态 → 发现问题 → 运行方案 → AI 解释 → 用户决策 → 跟踪执行 → 定期复盘。

成功的含义：用户在手机或 PC 上随时能查到可信的净资产、现金流、预算、Safe-to-Spend、债务与目标状态；任何财务数字都可复现、可解释、不依赖 LLM 心算；AI 能回答"为什么 + 怎么办 + 如果这样做会怎样"。

## Positioning

同类产品无法照抄的主张：**记账事实可靠、财务计算确定、AI 负责解释与规划**——在成熟账本（ezBookkeeping）之上补齐家庭财务规划域，而不是重造账本。所有关键金额来自确定性 Finance Engine 的结构化结果，LLM 绝不是数字权威源；AI 只解释、编排、提建议，重大建议必须展示事实、假设、替代方案与不确定性。

## Operating Context

- **部署形态**：单 VPS + Docker Compose 自托管，公网 HTTPS（Caddy 边缘代理），无 HA。
- **配套系统**：日常记账走 ezBookkeeping 的成熟移动 PWA（book.example.com）；本产品 Web UI（finance.example.com）是规划域主界面——首页即财务总览。两个域名、两套独立登录体系并存是真实使用状态。
- **客户端**：手机是一级客户端，不是桌面网页的缩小版；PC 用于规划与复盘。
- **数据节奏**：日常随时记账（ezBookkeeping 侧），月底导入微信/支付宝/京东账单对账；周报/月报/季度与年度复盘由 scheduler 生成。
- **运维者视角**：作者本人就是运维者，`make verify` 质量门禁、备份恢复演练是项目日常的一部分。

## Capabilities and Constraints

- **双权威源模型（最重要约束）**：ezBookkeeping 是交易账本唯一权威源；Finance Core 是规划域（家庭画像、预算、债务合同、目标、资产快照、情景模拟）权威源。UI 中不得出现第二套可编辑交易账本的暗示。
- **Money 全程 int64 最小货币单位**，JSON 中 minor 用 string 序列化，前端按 currency digits 解析——金额显示逻辑不可绕过 `web/src/money.ts`。
- **AI 高风险动作边界**：交易写入只能 proposal → human approval；不自动转账/还贷/交易。UI 必须让"AI 建议"与"已执行事实"视觉可区分。
- **数据质量诚实**：`data_quality=partial` 必须明示；外部文本（商户备注、账单内容）是 untrusted data。
- **家庭 RBAC**：owner/editor/viewer 三角色已实现（当前仅 owner 实际使用）。
- **认证**：Argon2id 密码 + 强制 TOTP + 服务端 Session + CSRF + 登录限流；浏览器不提交 household_id 作为授权依据。
- **复杂度纪律**：V1 明确不引入 Kubernetes、Redis、Kafka、MinIO、向量库、微服务、多 Agent。
- **界面语言**：简体中文（`lang="zh-CN"`），无 i18n 框架。
- **明确非目标**：不替代持牌会计/税务/法律/投资顾问；不自动买卖证券；不做多家庭公开 SaaS。

## Brand Commitments

- 名称 **Family Finance OS**，不缩短为 "FFOS" 或其他别名（截至本记录无此授权）。
- 界面语言为简体中文；产品语气：诚实、精确、专业，不夸大、不贩卖焦虑——财务数字宁可保守呈现，不可显得比实际更确定。
- 无 logo 之外的品牌资产承诺；现有 `web/public/icon.svg` 与 PWA manifest theme-color（#0f172a）是既有事实。

## Evidence on Hand

- 仓库文档体系完整：`PROJECT_PLAN.md`、`docs/00`~`docs/12`、`docs/adr/`、验收证据总账。
- 真实可用代码：Go 后端 + Vue 3 PWA 前端已跑通 V1 + V1.3 + V2 MCP 集成；尚未标记 production-ready（受 `docs/acceptance/v1-production-evidence.md` 约束）。
- **不存在**：用户证言、案例研究、媒体报道、竞品对比素材。任何营销/说服型工作不得编造这些内容。

## Product Principles

1. **可信优先于好看**：财务数字的呈现必须让用户敢于据此决策——来源可解释、不确定性明示、保守而非乐观。
2. **确定性先于生成**：先算清楚，再让 AI 开口；界面永远能区分"算出来的"和"AI 说的"。
3. **手机是一级公民**：核心查询与决策路径为手机 PWA 优化，PC 是增强而非基准。
4. **诚实面对复杂**：多目标冲突时给出取舍，不假装都能实现；数据不足时明说，不补猜。
5. **复杂度有代价**：不为"可能有用"加功能；每个新概念在 UI 上出现前必须值得用户付出理解成本。

## Accessibility & Inclusion

- 手机 + PC 双端公网 HTTPS 访问；视口从 320px 起步。
- 简体中文为主要界面语言；金额按币种精度格式化，不能出现浮点尾差。
- 无既定的 WCAG 等级承诺，但财务决策界面的对比度与可读性按生产软件标准要求。
