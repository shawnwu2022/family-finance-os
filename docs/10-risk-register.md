# 风险登记表

| ID | 风险 | 概率 | 影响 | V1 缓解 |
|---|---|---|---|---|
| R01 | 截图 OCR 金额错误 | 中 | 高 | 用户确认；月底账单校验 |
| R02 | 截图与正式账单重复 | 高 | 中/高 | V1 人工对账；V1.1 自动 reconciliation |
| R03 | 信用卡还款重复计支出 | 中 | 高 | transaction-type normalization tests |
| R04 | LLM 自行算错 | 中 | 高 | deterministic tools；禁止心算关键数值 |
| R05 | AI 建议过度确定 | 中 | 高 | assumptions/data-quality/alternatives 输出规范 |
| R06 | VPS 磁盘损坏 | 低/中 | 极高 | 每日异地备份 + restore drill |
| R07 | 第三方 VLM 隐私 | 中 | 高 | 最小披露；V1.2 本地 OCR/脱敏 |
| R08 | API Token 泄漏 | 低/中 | 高 | secret 管理；内网；轮换；allowlist 可选 |
| R09 | ezBookkeeping API 变化 | 中 | 中 | 固定版本 + contract tests + release notes |
| R10 | 过度工程化 | 高 | 中 | Complexity Gate；模块化单体 |
| R11 | P40 驱动/兼容问题 | 高 | 低（V1） | 不进入关键路径 |
| R12 | 家庭隐私越权 | 低（单用户）→中 | 高 | V1.4 server-side RBAC；OpenClaw 非边界 |
| R13 | 市场价格陈旧 | 后期中 | 中/高 | valuation timestamp + stale checks |
| R14 | Prompt Injection | 中 | 高 | 外部文本当 data；tool allowlist；无直接写权限 |
| R15 | Debt 参数不完整 | 中 | 高 | 缺 APR/费用时阻止“最优”结论 |
| R16 | 计划目标互相冲突 | 高 | 中 | Scenario/constraint 结果显示 unmet goals |
