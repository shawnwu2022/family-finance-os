# Family Finance OS V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 ezBookkeeping 交易账本之上完成可测试、可部署的 Go Finance Core，让用户能在手机/PC 随时查询家庭财务状态、预算、债务、目标、Scenario，并获得只解释确定性 Tool Result 的 AI 建议。

**Architecture:** 单 VPS Docker Compose。ezBookkeeping 是 Ledger Source of Truth；Finance Core 是 Go 模块化单体，使用 HTTP Bearer API 读取账本，使用独立 PostgreSQL `finance` 数据库存放家庭规划域。Money/债务/预算/目标/Scenario 全部 deterministic；LLM 只做 tool orchestration 与解释。

**Tech Stack:** Go 1.26.6, `net/http`, `log/slog`, PostgreSQL 18.6, `pgx/v5`, sqlc, goose, `cockroachdb/apd/v3`, ezBookkeeping 1.6.1, Vue 3/TypeScript/Vite/PWA/ECharts, Caddy 2.11.4, Docker Compose, OpenAI-compatible LLM endpoint.

## Global Constraints

- V1 长期运行容器固定为：Caddy、PostgreSQL、ezBookkeeping、Finance Core。
- 不引入 Kubernetes、Redis、Kafka、RabbitMQ、Temporal、MinIO、Vector DB、微服务、HA。
- V1 不依赖 OpenClaw、Tesla P40、Python runtime 或 Rust。
- 交易事实只写入 ezBookkeeping；Finance Core 不维护第二套可编辑 Ledger。
- Finance Core 永远通过 ezBookkeeping HTTP API，而不是直读其内部数据库。
- Money 使用 `int64` 最小货币单位；APR/FX/percentage 用 arbitrary-precision decimal；关键财务计算禁止 `float64`。
- 默认业务时区 `Asia/Shanghai`，外部交易保留发生时区语义。
- LLM 不负责关键财务算术；数字型财务回答必须来自 typed Finance Tool Result。
- AI 不具备自动转账、自动还贷、自动证券交易或未确认交易写入权限。
- 每个行为变更遵循 RED → GREEN → REFACTOR；Finance pure functions 使用 table-driven + fuzz/property tests。

---

## File Structure

```text
cmd/finance-core/
internal/
  config/
  server/
  household/
  ledger/
    ezbookkeeping/
  analytics/
  budget/
  debt/
  goals/
  portfolio/
  scenario/
  advisor/
  llm/
  report/
  scheduler/
  audit/
  store/
    sqlc/          # generated
pkg/
  money/
db/
  migrations/
  queries/
web/
  src/
eval/
tests/fixtures/
```

### Task 1: Runtime settings and PostgreSQL foundation

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/store/postgres.go`
- Create: `internal/store/postgres_test.go`
- Modify: `go.mod`
- Modify: `cmd/finance-core/main.go`

**Interfaces:**
```go
type Config struct {
    ListenAddr string
    Timezone string
    Database DatabaseConfig
    Ledger LedgerConfig
    LLM LLMConfig
}

func Load(getenv func(string) string) (Config, error)
func (c DatabaseConfig) URL() *url.URL
func OpenPostgres(ctx context.Context, cfg DatabaseConfig) (*pgxpool.Pool, error)
```

- [ ] RED: `Config` defaults timezone to `Asia/Shanghai`, requires DB credentials, preserves passwords containing `@:/` through `url.UserPassword` rather than manual DSN concatenation.
- [ ] Run `go test ./internal/config -run Test -v` and verify expected undefined-type/function failures.
- [ ] Add `pgx/v5` dependency and minimal config/store implementation.
- [ ] GREEN: run `go test ./internal/config ./internal/store`.
- [ ] Add `/readyz` later only after PostgreSQL connection exists; `/healthz` must remain process-liveness only.
- [ ] Commit `feat(core): add runtime settings and postgres foundation`.

### Task 2: Exact Money and Decimal primitives

**Files:**
- Create: `pkg/money/money.go`
- Create: `pkg/money/money_test.go`
- Create: `pkg/money/money_fuzz_test.go`

**Interfaces:**
```go
type Money struct {
    Minor int64
    Currency string
}
func (m Money) Add(other Money) (Money, error)
func (m Money) Sub(other Money) (Money, error)
```

- [ ] RED table tests: same-currency add/subtract, overflow boundary, cross-currency rejection.
- [ ] RED fuzz property: `m.Add(zero) == m` for valid Money without overflow.
- [ ] Add `apd/v3` only for rate/ratio types; do not wrap ordinary Money in decimal.
- [ ] GREEN: `go test ./pkg/money -run Test -v && go test ./pkg/money -fuzz=Fuzz -fuzztime=3s`.
- [ ] Commit `feat(core): add exact money primitives`.

### Task 3: ezBookkeeping Ledger Adapter contract

**Files:**
- Create: `internal/ledger/port.go`
- Create: `internal/ledger/models.go`
- Create: `internal/ledger/ezbookkeeping/client.go`
- Create: `internal/ledger/ezbookkeeping/client_test.go`
- Create: `tests/fixtures/ezbookkeeping/*.json`

**Interfaces:**
```go
type Ledger interface {
    ListAccounts(ctx context.Context) ([]Account, error)
    ListCategories(ctx context.Context) ([]Category, error)
    ListTransactions(ctx context.Context, q TransactionQuery) ([]Transaction, error)
}
```

- [ ] Capture sanitized fixtures strictly from the current ezBookkeeping 1.6.1 HTTP API schema.
- [ ] RED using `httptest.Server`: assert Bearer auth, `X-Timezone-Name`, pagination, amount/type normalization.
- [ ] Implement only read endpoints needed by V1.
- [ ] Unknown API enum/type must fail closed or map to explicit `Unknown`; never infer silently.
- [ ] GREEN + manual staging smoke test with non-production API token.
- [ ] Commit `feat(ledger): add ezbookkeeping read adapter`.

### Task 4: Household persistence and migrations

**Files:**
- Create: `internal/household/models.go`
- Create: `internal/household/service.go`
- Create: `db/migrations/00001_household.sql`
- Create: `db/queries/household.sql`
- Generate: `internal/store/sqlc/*`

**Interfaces:** Household, Member, IncomeSource, ExpenseBaseline, HouseholdPolicy.

- [ ] RED repository round-trip integration test against disposable PostgreSQL.
- [ ] Implement goose SQL up/down migration.
- [ ] Generate queries with sqlc; generated files are committed and CI verifies regeneration produces no diff once SQL tooling is enabled.
- [ ] `HouseholdPolicy.liquidity_floor_minor` must be explicit structured data, never inferred by LLM.
- [ ] Migration up/down/up smoke test.
- [ ] Commit `feat(household): persist household financial profile`.

### Task 5: Transaction normalization and Data Quality

**Files:**
- Create: `internal/analytics/normalization.go`
- Create: `internal/analytics/normalization_test.go`
- Create: `internal/analytics/quality.go`

**Interfaces:** `NormalizeTransaction`, `CashflowEventType`, `DataQuality`.

- [ ] RED table tests: expense, income, transfer, balance adjustment, credit-card repayment, refund, unknown.
- [ ] Ensure credit-card repayment is not treated as new consumption.
- [ ] Unknown type remains `Unknown` and contributes to data-quality warning.
- [ ] GREEN and commit `feat(analytics): normalize ledger events and quality state`.

### Task 6: Cashflow and Net Worth

**Files:**
- Create: `internal/analytics/cashflow.go`
- Create: `internal/analytics/cashflow_test.go`
- Create: `internal/analytics/networth.go`
- Create: `internal/analytics/networth_test.go`

**Interfaces:** `CalculateCashflow`, `CalculateNetWorth` as pure functions.

- [ ] RED: internal transfer does not alter income/expense.
- [ ] RED: credit-card purchase counts once; repayment does not double count.
- [ ] RED: asset minus liability net worth with explicit currency valuation inputs.
- [ ] GREEN; add property tests for accounting invariants.
- [ ] Commit `feat(analytics): add cashflow and net worth engine`.

### Task 7: Budget Engine

**Files:**
- Create: `internal/budget/models.go`
- Create: `internal/budget/service.go`
- Create: `internal/budget/service_test.go`
- Create: `db/migrations/00002_budget.sql`
- Create: `db/queries/budget.sql`

**Interfaces:** `BudgetPlan`, `BudgetLine`, kinds `Essential|Flexible|Debt|Saving|Investment|Goal`.

- [ ] RED: planned/actual/remaining/utilization calculations.
- [ ] Persist plan only; actual spending remains derived from Ledger.
- [ ] V1 excludes envelope/rollover unless explicitly added by a later accepted requirement.
- [ ] GREEN + migration tests.
- [ ] Commit `feat(budget): add household budget engine`.

### Task 8: Safe-to-Spend and Emergency Fund

**Files:**
- Create: `internal/budget/safe_to_spend.go`
- Create: `internal/budget/safe_to_spend_test.go`
- Create: `internal/budget/emergency.go`
- Create: `internal/budget/emergency_test.go`

**Interfaces:**
```go
func CalculateSafeToSpend(in SafeToSpendInput) SafeToSpendResult
func CalculateEmergencyMonths(liquid, essentialMonthly money.Money) (DecimalResult, error)
```

- [ ] RED: explicit component subtraction and negative deficit preservation; never clamp silently.
- [ ] RED: prevent double counting a payment already included in essential/fixed commitments.
- [ ] RED: zero monthly essential expense returns not-applicable rather than divide-by-zero.
- [ ] Add fuzz/property tests: increasing an uncommitted purchase cannot increase Safe-to-Spend.
- [ ] Commit `feat(budget): add safe-to-spend and emergency coverage`.

### Task 9: Debt Engine

**Files:**
- Create: `internal/debt/models.go`
- Create: `internal/debt/simulator.go`
- Create: `internal/debt/simulator_test.go`
- Create: `internal/debt/simulator_fuzz_test.go`
- Create: `db/migrations/00003_debt.sql`
- Create: `db/queries/debt.sql`

**Interfaces:** Chinese-relevant debt contract fields: principal/balance, APR, fixed/LPR-spread/other variable rate, term, due day, annuity/equal-principal/revolving/custom payment, minimum payment, prepayment fee/restriction.

- [ ] RED golden tests for equal-payment mortgage, equal-principal mortgage, revolving credit.
- [ ] RED Avalanche and Snowball plans with liquidity-floor constraint.
- [ ] RED: extra payment must never increase principal or push payoff date later with all else equal.
- [ ] Implement exact decimal rounding policy and document it.
- [ ] Commit `feat(debt): add debt payoff simulator`.

### Task 10: Goal Engine

**Files:**
- Create: `internal/goals/models.go`
- Create: `internal/goals/service.go`
- Create: `internal/goals/service_test.go`
- Create: `db/migrations/00004_goals.sql`
- Create: `db/queries/goals.sql`

- [ ] RED: zero-return deterministic contribution forecast and target-date gap.
- [ ] Separate user-supplied expected return/inflation assumptions from facts.
- [ ] Return infeasible/conflicting goal state explicitly.
- [ ] Commit `feat(goals): add goal planning engine`.

### Task 11: Basic Portfolio Summary

**Files:**
- Create: `internal/portfolio/models.go`
- Create: `internal/portfolio/service.go`
- Create: `internal/portfolio/service_test.go`

- [ ] V1 stores summary valuations/allocation only; no tax lots, brokerage execution, stock-picking or automatic trading.
- [ ] RED allocation totals and stale-FX warning.
- [ ] Asset classes: cash/deposit/fixed-income/equity/fund/gold/property/other.
- [ ] Commit `feat(portfolio): add household allocation summary`.

### Task 12: Scenario Engine

**Files:**
- Create: `internal/scenario/service.go`
- Create: `internal/scenario/service_test.go`

**Interfaces:** `SimulatePurchase`, `SimulateExtraDebtPayment`, `SimulateBudgetChange`, `SimulateSavingsChange`, `SimulateIncomeDrop`, `SimulateGoal`.

- [ ] RED purchase scenario compares Safe-to-Spend, saving rate, liquidity floor, debt/goal delay.
- [ ] Scenario is pure/non-mutating: simulation cannot persist budget/debt/goal changes.
- [ ] GREEN + invariant tests.
- [ ] Commit `feat(scenario): add non-mutating what-if engine`.

### Task 13: AI Advisor Tool Boundary

**Files:**
- Create: `internal/advisor/tools.go`
- Create: `internal/advisor/tools_test.go`
- Create: `internal/llm/provider.go`
- Create: `internal/llm/openai_compatible.go`
- Create: `internal/llm/openai_compatible_test.go`

- [ ] RED: all numeric advice routes through typed tools, never raw SQL or arbitrary code.
- [ ] Implement thin OpenAI-compatible HTTP/SSE adapter; no LangChain-style framework.
- [ ] Provider roles remain config values: fast/planner/reviewer; model IDs never appear in domain branches.
- [ ] No destructive/write tools in V1 Advisor registry.
- [ ] Commit `feat(advisor): establish typed finance tool boundary`.

### Task 14: AI Advisor Policy and Audit

**Files:**
- Create: `internal/advisor/service.go`
- Create: `internal/advisor/policy.go`
- Create: `internal/advisor/policy_test.go`
- Create: `internal/audit/advice.go`
- Create: `db/migrations/00005_advice_audit.sql`

- [ ] RED prompt-injection fixture where merchant text says “ignore previous instructions”; it remains untrusted data.
- [ ] RED tool failure/data quality partial paths: assistant must not invent numbers.
- [ ] Store tool calls/result hashes/model role, not raw secrets.
- [ ] Major recommendation may request reviewer role but reviewer is a second pass, not a multi-Agent system.
- [ ] Commit `feat(advisor): enforce advice policy and audit`.

### Task 15: HTTP API and Finance Dashboard

**Files:**
- Create: `internal/server/api.go`
- Create: `internal/server/api_test.go`
- Create: `web/package.json`
- Create: `web/src/*`
- Create: `web/vite.config.ts`
- Create: `internal/webassets/embed.go`

- [ ] Pin current stable Vue 3/TypeScript/Vite versions at implementation time after fresh official verification.
- [ ] API: overview/cashflow/budget/debts/goals/scenarios/advisor/reports.
- [ ] Dashboard mobile-first: Net Worth, income/expense, savings rate, Safe-to-Spend, emergency months, debt, goals, warnings, AI entry.
- [ ] Build frontend to `web/dist` and embed into Finance Core binary with `go:embed`.
- [ ] PWA installability and responsive mobile smoke tests.
- [ ] Commit `feat(web): add mobile-first finance dashboard`.

### Task 16: Reports and In-process Scheduler

**Files:**
- Create: `internal/report/monthly.go`
- Create: `internal/report/monthly_test.go`
- Create: `internal/scheduler/scheduler.go`
- Create: `internal/scheduler/scheduler_test.go`
- Create: `db/migrations/00006_job_runs.sql`

- [ ] Monthly report is generated from deterministic metrics first; LLM narrative is optional enhancement.
- [ ] Scheduler uses Go timers + `job_runs` idempotency; no Redis/Celery/Temporal.
- [ ] Startup catches up missed scheduled jobs according to explicit policy.
- [ ] Commit `feat(report): add reports and lightweight scheduler`.

### Task 17: Backup, Security, CI and Production Acceptance

**Files:**
- Modify: `scripts/backup.sh`
- Modify: `docs/07-operations.md`
- Modify: `.github/workflows/ci.yml`
- Create: finance/e2e acceptance fixtures as needed.

- [ ] Upgrade backup path to `pg_dump -Fc` + ezBookkeeping storage + encrypted restic repository over SFTP to the local server when target credentials are available.
- [ ] CI: gofmt, vet, tests, race tests, govulncheck, sqlc generate/verify, frontend type/unit/build, Docker build.
- [ ] Confirm only Caddy exposes host 80/443; PostgreSQL/ezBookkeeping/Finance Core remain internal Docker network.
- [ ] Complete real ezBookkeeping 1.6.1 contract smoke test and at least one real Chinese bill import.
- [ ] Complete backup restore drill.
- [ ] V1 exit criteria from `docs/08-testing-acceptance.md` all pass before tagging V1.
- [ ] Commit `chore(release): satisfy V1 production acceptance`.
