# 全阶段 Roadmap

> 阶段按“能力成熟度/进入条件”推进，不按固定日历日期承诺。这样避免为了计划表引入不必要复杂度。

## 当前主线状态

- `main` 已完成 V1 Finance Core 工程闭环、V1.3 显式 Portfolio snapshot 层和 V2 MCP/OpenClaw Agent Channel 的工程集成；
- repository-native `make verify`、MCP Security、Edge Security、Real OpenClaw acceptance 已在最终集成候选上通过；
- 当前首要工作不是继续堆功能，而是完成真实生产部署、完整自然月 reconciliation、真实灾备恢复、移动端与 secret hygiene 等 production acceptance；
- 在 `docs/acceptance/v1-production-evidence.md` 的所有 required gates 全部 PASS 前，不标记 production-ready、不创建最终 release tag。

| 阶段 | 核心目标 | 新增复杂度 | 当前状态 |
|---|---|---|---|
| V0 | 账本、移动记账、备份上线 | 最小 | **工程能力完成；真实部署/验收进行中** |
| V1 | Finance Core 全闭环 | 模块化单体 | **工程闭环完成；Real-month/production acceptance 当前** |
| V1.1 | 自动对账/数据质量 | matching | 后续按痛点进入 |
| V1.2 | P40/本地隐私 AI | local worker | 后续按 benchmark 进入 |
| V1.3 | Portfolio/Market | 先显式资产快照，后按需市场数据 | **Snapshot 层完成并已进入 main** |
| V1.4 | 家庭 RBAC | 权限模型 | 后续按需 |
| V2 | MCP/OpenClaw/渠道 | Agent Adapter | **完成并已进入 main** |
| V2.1 | 统一移动端 | PWA/Capacitor | 后续 |
| V3 | 高级规划 | Forecast/Reviewer | 后续 |
| V4 | HA/规模化 | 复制/多节点 | **最后按需** |

## V0 Backlog

工程实现已经覆盖：

- Compose 生产基线；
- 域名/TLS 入口设计；
- ezBookkeeping 账户/分类体系能力；
- 2FA 运维要求；
- AI screenshot provider 扩展边界；
- API Token bootstrap；
- backup + restore drill。

仍需在真实环境完成：

- 生产域名/DNS/TLS 实际上线；
- ezBookkeeping 管理员与 2FA 实际启用；
- 支付宝/微信/银行等真实账单导入验证；
- 真实生产 backup + off-host restore；
- evidence ledger 记录。

## V1 Epic

1. Ledger Adapter — **完成**；
2. Household — **完成**；
3. Metrics — **完成**；
4. Budget/Safe-to-Spend — **完成**；
5. Debt — **完成**；
6. Goals — **完成**；
7. Portfolio summary — **完成**；
8. Scenario — **完成**；
9. Advisor — **完成**；
10. Dashboard — **完成**；
11. Reports — **完成**；
12. Audit/Data Quality — **完成基础层**；
13. Security/Operations — **完成工程门禁**；
14. Real-month acceptance — **当前 P0**。

## V1.1 Data Quality

引入前先统计：每月人工纠错分钟数、重复候选数、漏账数。只有形成稳定痛点才开发自动匹配。

## V1.2 Local AI

Benchmark gate：
- CPU OCR latency/accuracy；
- P40 OCR/VLM compatibility；
- 每月调用成本；
- 隐私收益；
- 运维复杂度。

结论可能是“继续云端”，也可能是“P40 值得加入”；不预设答案。

## V1.3 Portfolio

逐级增加，不一次造券商系统：

1. **当前资产快照 + 资产类别汇总 — 已完成并进入 main。** Finance Core 按 `(household_id, asset_ref)` 保存显式 current snapshot，支持 `cash/deposit/fixed_income/equity/fund/gold/property/other`；显式 snapshot 已经是报告币种的确定性估值事实。linked snapshot 在 allocation 中替换对应粗粒度 ledger account，避免双重计算；uncovered account 继续使用保守 fallback。跨币种 snapshot 若其 `Value.Currency` 不是家庭基准币则跳过并标记 partial；本阶段不做隐式 FX 转换或价格抓取。
2. Instrument/Position — **延期**，只在需要持仓数量、成本基础、证券标识和多账户聚合时进入。
3. 市场价格/FX feed — **延期**，只有明确的数据源、刷新频率、故障语义和运维收益后再引入。
4. 风险敞口 — **延期**，依赖可验证的 position/instrument 数据。
5. 再平衡建议 — **延期**，依赖风险模型和用户投资政策，不提前生成。
6. 税务成本/lot — 只有真实需要再做。

当前 V1.3 snapshot 层提供 Finance HTTP CRUD，并被现有 deterministic `get_asset_allocation` 读取；Agent/MCP 不增加写工具，仍保持十二个 read/simulation tools。

## V1.4 RBAC

先服务家庭成员，不做多租户 SaaS。权限在 Finance Core server-side，AI 只接过滤后的数据。

## V2 Channel

**已完成并进入 main。**

当前实现：

- protocol-neutral Agent Adapter；
- 12 个受控 read/simulation Finance tools；
- Streamable HTTP `/mcp`；
- 独立 Bearer、Origin/body/timeout/concurrency/rate-limit 边界；
- PostgreSQL fail-closed agent tool audit；
- Real OpenClaw + local Ollama release acceptance；
- 不增加 MCP sidecar、Redis、第二套财务逻辑或模型侧数值权威。

后续若增加新 Agent Framework/消息渠道，优先复用既有 Finance tool contract，不复制核心业务逻辑。

## V2.1 Unified Mobile

现有 PWA 先完成真实手机 production acceptance。只有出现明确的系统级能力需求（例如更稳定的后台任务、原生分享入口、文件/相机深度集成）时，再评估 Capacitor；不为了“有原生壳”提前增加双端维护成本。

## V3 Advanced Planning

所有随机模型/收益假设必须可配置、版本化、展示区间。禁止把 7% 等长期收益率作为隐藏常量。

## V4 Reliability

先收集真实故障和恢复数据。若单 VPS + 备份已经满足家庭使用，就可以长期停在这里，不以“上 HA”为项目成熟标志。