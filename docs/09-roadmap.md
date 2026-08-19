# 全阶段 Roadmap

> 阶段按“能力成熟度/进入条件”推进，不按固定日历日期承诺。这样避免为了计划表引入不必要复杂度。

| 阶段 | 核心目标 | 新增复杂度 | 是否当前 |
|---|---|---|---|
| V0 | 账本、移动记账、备份上线 | 最小 | 是，先部署 |
| V1 | Finance Core 全闭环 | 模块化单体 | **当前主开发** |
| V1.1 | 自动对账/数据质量 | matching | 后续按需 |
| V1.2 | P40/本地隐私 AI | local worker | 后续按需 |
| V1.3 | Portfolio/Market | 先显式资产快照，后按需市场数据 | **当前增量：Snapshot 层完成** |
| V1.4 | 家庭 RBAC | 权限模型 | 后续按需 |
| V2 | MCP/OpenClaw/渠道 | Agent Adapter | 后续 |
| V2.1 | 统一移动端 | PWA/Capacitor | 后续 |
| V3 | 高级规划 | Forecast/Reviewer | 后续 |
| V4 | HA/规模化 | 复制/多节点 | **最后按需** |

## V0 Backlog

- Compose 生产基线；
- 域名/TLS；
- ezBookkeeping 账户体系；
- 分类体系；
- 2FA；
- AI screenshot provider；
- 支付宝/微信真实导入验证；
- API Token；
- backup + restore drill。

## V1 Epic

1. Ledger Adapter；
2. Household；
3. Metrics；
4. Budget/Safe-to-Spend；
5. Debt；
6. Goals；
7. Portfolio summary；
8. Scenario；
9. Advisor；
10. Dashboard；
11. Reports；
12. Audit/Data Quality；
13. Security/Operations；
14. Real-month acceptance。

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

1. **当前资产快照 + 资产类别汇总 — 已完成。** Finance Core 按 `(household_id, asset_ref)` 保存显式 current snapshot，支持 `cash/deposit/fixed_income/equity/fund/gold/property/other`；显式 snapshot 已经是报告币种的确定性估值事实。linked snapshot 在 allocation 中替换对应粗粒度 ledger account，避免双重计算；uncovered account 继续使用保守 fallback。跨币种 snapshot 若其 `Value.Currency` 不是家庭基准币则跳过并标记 partial；本阶段不做隐式 FX 转换或价格抓取。
2. Instrument/Position — **延期**，只在需要持仓数量、成本基础、证券标识和多账户聚合时进入。
3. 市场价格/FX feed — **延期**，只有明确的数据源、刷新频率、故障语义和运维收益后再引入。
4. 风险敞口 — **延期**，依赖可验证的 position/instrument 数据。
5. 再平衡建议 — **延期**，依赖风险模型和用户投资政策，不提前生成。
6. 税务成本/lot — 只有真实需要再做。

当前 V1.3 snapshot 层提供 Finance HTTP CRUD，并被现有 deterministic `get_asset_allocation` 读取；Agent/MCP 不增加写工具，仍保持十二个 read/simulation tools。

## V1.4 RBAC

先服务家庭成员，不做多租户 SaaS。权限在 Finance Core server-side，AI 只接过滤后的数据。

## V2 Channel

MCP/OpenClaw 作为边缘接入；Finance Core 的 tool contract 不变。这样更换 Agent Framework 不影响核心。

## V3 Advanced Planning

所有随机模型/收益假设必须可配置、版本化、展示区间。禁止把 7% 等长期收益率作为隐藏常量。

## V4 Reliability

先收集真实故障和恢复数据。若单 VPS + 备份已经满足家庭使用，就可以长期停在这里，不以“上 HA”为项目成熟标志。
