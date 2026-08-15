# Family Finance OS Design Specification

## Scope

面向中国家庭的单家庭、自托管 AI 财务管家。V1 以低运维复杂度为首要工程约束，在云 VPS 上运行 Caddy、PostgreSQL、ezBookkeeping 与 Finance Core。

## Requirements

- 随时随地手机/PC 记账与查询；
- ezBookkeeping 提供交易录入、截图识别、账单导入；
- Finance Core 提供 Household、Cashflow、NetWorth、Budget、Safe-to-Spend、Debt、Goals、Portfolio Summary、Scenario、Advisor；
- 关键财务数值 deterministic；
- AI provider 可替换；
- 本地服务器仅为 V1 备份；
- P40/OpenClaw 后续按需；
- V1 不做 HA/微服务/消息队列/向量数据库。

## System Boundaries

- `LedgerPort`: 从 ezBookkeeping 读取规范化账户/交易/分类；
- `PlanningRepository`: PostgreSQL 中的 Finance Core 规划域；
- `FinanceServices`: 纯计算与业务规则；
- `Advisor`: 调用 FinanceServices 暴露的 tool contracts，再调用 LLM；
- `Web/API`: 用户入口；
- `Scheduler`: 进程内或 cron 触发周/月报告。

## Error Handling

- 外部账本失败：返回 typed dependency error，并标记数据不可用；
- 数据过期：返回 stale，而不是使用未标记旧值；
- LLM 失败：Finance metric endpoints 不受影响；
- LLM 非法结构：有限重试后返回解释服务不可用；
- 缺失关键债务参数：阻止最优策略结论；
- 计算不得 silently clamp 不合理结果。

## Testing

- TDD；
- Finance pure functions 使用 table-driven tests；
- ezBookkeeping 使用 contract fixtures；
- Advisor 使用 fake LLM；
- prompt injection fixture；
- staging 使用真实 ezBookkeeping；
- 月度真实账本人工抽查。

## Deployment

Docker Compose，Caddy 仅公网入口，PostgreSQL 18.6 volume 按官方 18+ 规则挂载 `/var/lib/postgresql`。ezBookkeeping 固定 1.6.1。Finance Core 使用 Python 3.13 stable line。

## Future Extension

Reconciliation → Local AI/P40 → Portfolio → Household RBAC → MCP/OpenClaw → Unified Mobile → Advanced Planning → HA。每项有 roadmap gate；不得提前引入。
