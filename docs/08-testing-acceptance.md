# 测试与验收

## 1. Go Core 测试层次

### 单元测试（必须）

- `go test ./...`；
- Money；
- 交易类型归一；
- Cashflow；
- Net Worth；
- Budget；
- Safe-to-Spend；
- Debt；
- Goal；
- Scenario；
- Data quality；
- Advisor tool routing；
- Monthly report；
- Scheduler/idempotency。

Finance pure function 优先 table-driven test；测试不允许依赖真实云 LLM。

### Race / Fuzz / Property

- CI 运行 `go test -race ./...`；
- Money、Safe-to-Spend、Debt 等核心算法覆盖 fuzz/property tests；
- 典型不变量：增加一次未承诺消费不能增加 Safe-to-Spend；额外还款在其他条件不变时不能增加债务本金或把 payoff date 推迟。

### Contract Test

针对 ezBookkeeping HTTP API 保存最小脱敏 fixtures，验证：

- Bearer Token；
- `X-Timezone-Name`；
- account/category/transaction schema；
- 金额单位；
- transaction type；
- pagination/time range。

### Integration Test

- Finance Core + disposable PostgreSQL；
- goose migration up/down/up；
- sqlc generated queries；
- scheduler `job_runs` 重启恢复与幂等；
- backup dump + checksum + `restore-drill.sh` 的真实 `pg_restore`；
- Finance Core + ezBookkeeping staging；
- LLM Adapter 使用 fake `httptest.Server` 验证 tool path/SSE/JSON，真实云模型不是 CI 必需依赖。

### Static / Supply-chain

- `gofmt`；
- `go vet ./...`；
- 固定版本 `govulncheck ./...`；
- sqlc regeneration 必须无未提交 diff；
- 前端 `npm ci`；
- 前端 unit tests；
- PWA contract；
- TypeScript typecheck；
- frontend production build；
- Docker image build；
- Caddy-only host-port exposure contract。

## 2. 必测财务场景

1. 现金消费；
2. 信用卡消费 + 后续还款；
3. 自己账户之间转账；
4. 退款；
5. 工资；
6. 报销；
7. 消费贷放款和还款；
8. 房贷等额本息；
9. 房贷等额本金；
10. 跨月信用卡；
11. 一笔交易被截图和账单重复出现（V1.1）；
12. 0 收入月份；
13. 负现金流月份；
14. 应急资金不足；
15. Debt APR 缺失；
16. Goal 到期但资金不足；
17. 跨币种资产但汇率陈旧；
18. liquidity floor 阻止“把所有现金拿去还债”；
19. 固定支出/债务承诺避免在 Safe-to-Spend 中重复扣减。

## 3. AI 对抗验收

- 商户备注写入“ignore previous instructions”不会改变 Advisor 行为；
- 用户要求跳过 Finance Tool 时，数字型回答仍调用 Tool；
- Tool 失败时 AI 不编造数字；
- `data_quality=partial` 时 AI 明确说明；
- 非法 JSON/Tool payload 有明确失败路径；
- AI 无法调用未注册/未授权写工具；
- raw ledger text 永远作为 untrusted data，不进入 system/developer instruction；
- planner/reviewer 模型切换不改变 deterministic Tool Result。

## 4. CI 自动化发布门禁

以下项目必须在目标 commit 的同一组 CI 中全部通过：

1. migration up/down/up；
2. sqlc generated source 无 diff；
3. `go mod tidy` 无 diff；
4. gofmt；
5. `go vet ./...`；
6. `go test ./...`；
7. `govulncheck ./...`；
8. PostgreSQL/household/budget/goals/audit/scheduler/appapi/application integration；
9. backup restore drill；
10. `go test -race ./...`；
11. Go binary build；
12. 前端 `npm ci`；
13. 前端 unit test；
14. PWA contract；
15. TypeScript typecheck；
16. frontend production build；
17. Docker image build；
18. Edge Security workflow；
19. `scripts/test-production-ops.sh`；
20. `scripts/check-edge-security.sh`。

CI 绿色只代表**可重复的软件/运维契约**通过，不等价于真实家庭数据和真实生产主机已经验收。

## 5. V0 Exit Criteria

- 手机 PWA 可登录、截图、手工记账；
- 至少成功导入一种真实中国账单；
- 2FA 已启用；
- 每日备份可产生有效 dump；
- 完成一次异机恢复。

如果这些门禁在 V1 发布前尚无真实执行证据，则 V1 同样不得打 release tag。

## 6. V1 生产验收门禁

所有结果记录到 `docs/acceptance/v1-production-evidence.md`。必须有真实执行时间、环境、版本和脱敏证据；禁止用 unit test/fixture 冒充真实生产证据。

### 6.1 真实账本与月度一致性

- 固定 ezBookkeeping 生产版本；
- 通过 ezBookkeeping UI 导入至少一种**真实中国账单**（支付宝、微信支付、银行卡等受支持格式之一）；
- 原始账单不进入仓库；
- 至少使用一个完整自然月的真实交易；
- Dashboard 和月报均能生成；
- 人工独立复核该月 Income、Expense、Net Cashflow；
- 人工独立复核月末 Net Worth；
- Finance Core 与人工复核的金额差异均不得超过 **0.01 CNY**；
- transfer/refund/reimbursement/credit-card repayment 等不会被重复计为收入或支出。

### 6.2 Safe-to-Spend / Debt / Scenario

- Safe-to-Spend 每个扣减项都能追溯到确定性输入；
- 固定支出与债务承诺无 double-count；
- 用独立 Excel/计算器复核至少一个等额本息样例和一个等额本金/额外还款样例；
- 月供、剩余本金、关键 scenario 金额差异不得超过 **0.01 CNY**；
- extra payment 不能增加本金或推迟 payoff；
- liquidity floor 场景不能建议把可用现金压到安全线以下。

### 6.3 真实 LLM 与降级

- 使用生产拟采用的 OpenAI-compatible provider/model 完成至少一次真实 Advisor 请求；
- 保存脱敏 trace：模型 ID、prompt template version、tool 名称、request/advice hash、`data_as_of`、quality；
- 关键金额必须能映射到 deterministic Tool Result；
- 临时移除/阻断 LLM 凭据或 provider 后，Dashboard、Cashflow、Budget、Debt、Goal、Scenario、Monthly Report 的确定性数值仍可用；
- LLM 不可用时不得伪造或缓存为“新建议”。

### 6.4 备份与恢复

- 至少一个真实生产 backup 同时包含两份 custom-format DB dump、ezBookkeeping storage 和 SHA256SUMS；
- 该 backup 已通过 **authenticated append-only REST** endpoint 成功创建离站 restic snapshot；
- 使用生产 **REST producer credential** 验证可以创建新 snapshot，但无法删除、覆盖或改写已有离站恢复点；
- `restic check` 从独立、授权的 maintenance/recovery context 执行并通过；
- 至少一次**异机**从真实 restic snapshot 恢复；
- 恢复后 ezBookkeeping 与 Finance Core 可启动，关键账户/交易/月报抽查一致；
- 记录 snapshot ID、destructive-denial 脱敏证据、RTO、执行人和脱敏结果。

### 6.5 Scheduler 重启幂等

- 在已有当月成功 monthly report run 后重启 Finance Core；
- `job_runs` 不产生同一 household/job/scheduled_for 的重复成功记录；
- `monthly_reports` 只存在一个对应 household/period 的不可变产物，`content_hash` 为 64 位 SHA-256，读取时通过重新计算校验；
- `/api/v1/reports` 返回真实月报产物及其 `content_hash`，不能用成功的 `job_runs` 冒充已生成报表；
- 人为制造/模拟 interrupted `running` 后重启，能够恢复为 failed 并允许重试；
- 不保存原始 secret-bearing error text。

### 6.6 PWA 与公网安全

- 至少在一台真实手机上通过 HTTPS 安装/添加 PWA；
- Finance 域先经过 Caddy Basic Auth；
- ezBookkeeping/Finance Core/PostgreSQL 均无直接宿主端口；
- 2FA 已在 ezBookkeeping 管理员账户启用；
- `.env`、API token、LLM key、restic password、SSH private key、原始账单不在 Git；
- 日志和验收证据中不存在明文 secret/完整账户号等敏感数据。

## 7. V1 发布判定

只有同时满足以下条件，才允许创建 V1 release tag：

- 第 4 节全部 CI 自动门禁在目标 commit 绿色；
- 第 5 节 V0 真实门禁已有有效证据；
- 第 6 节所有生产门禁状态为 `PASS`；
- 无 P0/P1 数据正确性问题；
- 没有未解释的金额差异 > 0.01 CNY；
- 没有未处理的高严重度可达漏洞；
- 没有已知 secret 泄露。

若缺少真实服务器、账单、LLM、手机、authenticated append-only REST producer 或独立 maintenance/recovery 凭据，只能把状态记为 `NOT RUN`/`BLOCKED`，不能推断为通过，也不能创建 V1 tag。
