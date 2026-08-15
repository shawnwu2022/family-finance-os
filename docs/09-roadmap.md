# 全阶段 Roadmap

> 阶段按“能力成熟度/进入条件”推进，不按固定日历日期承诺。这样避免为了计划表引入不必要复杂度。

| 阶段 | 核心目标 | 新增复杂度 | 是否当前 |
|---|---|---|---|
| V0 | 账本、移动记账、备份上线 | 最小 | 是，先部署 |
| V1 | Finance Core 全闭环 | 模块化单体 | **当前主开发** |
| V1.1 | 自动对账/数据质量 | matching | 后续按需 |
| V1.2 | P40/本地隐私 AI | local worker | 后续按需 |
| V1.3 | Portfolio/Market | 市场数据 | 后续按需 |
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
1. 资产类别汇总；
2. Instrument/Position；
3. 市场价格；
4. 风险敞口；
5. 再平衡建议；
6. 税务成本/lot 只有真实需要再做。

## V1.4 RBAC

先服务家庭成员，不做多租户 SaaS。权限在 Finance Core server-side，AI 只接过滤后的数据。

## V2 Channel

MCP/OpenClaw 作为边缘接入；Finance Core 的 tool contract 不变。这样更换 Agent Framework 不影响核心。

## V3 Advanced Planning

所有随机模型/收益假设必须可配置、版本化、展示区间。禁止把 7% 等长期收益率作为隐藏常量。

## V4 Reliability

先收集真实故障和恢复数据。若单 VPS + 备份已经满足家庭使用，就可以长期停在这里，不以“上 HA”为项目成熟标志。
