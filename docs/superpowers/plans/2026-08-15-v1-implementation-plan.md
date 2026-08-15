# Family Finance OS V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现成 ezBookkeeping 账本上完成可测试、可部署的家庭 Finance Core，让用户能在手机/PC 随时查询财务状态、预算、债务、目标、Scenario 并得到基于工具结果的 AI 建议。

**Architecture:** 单 VPS Docker Compose。ezBookkeeping 是交易账本权威源，Finance Core 是模块化单体并通过 HTTP Bearer API 读取账本。所有金额与方案由 deterministic services 计算，LLM 只解释结构化结果。

**Tech Stack:** Caddy 2.11.4, PostgreSQL 18.6, ezBookkeeping 1.6.1, Python 3.13, FastAPI, Pydantic, SQLAlchemy 2.0, psycopg 3, Alembic, pytest, Vue 3/Vite/PWA（前端任务开始时再按当日官方 stable 锁定 Node/Vue/Vite 版本）。

## Global Constraints

- V1 不引入 Kubernetes、Redis、Kafka、MinIO、Vector DB、微服务、HA。
- V1 不依赖 OpenClaw 或 Tesla P40。
- 交易事实只写入 ezBookkeeping；Finance Core 不维护第二套交易账本。
- 权威 Money 类型禁止使用 `float`。
- 默认业务时区 `Asia/Shanghai`；外部交易保留时区语义。
- LLM 不负责关键财务算术；任何数字型财务回答必须源自 Finance Tool。
- AI 不具备自动转账、还贷或证券交易能力。
- 每个功能遵循 RED → GREEN → REFACTOR；提交前运行相关测试。

---

## File Structure

```text
apps/finance-core/src/finance_core/
  main.py
  settings.py
  money.py
  db.py
  household/
  ledger/
  analytics/
  budget/
  debt/
  goals/
  portfolio/
  scenario/
  advisor/
  reports/
  audit/
apps/finance-core/tests/
```

### Task 1: Runtime / Settings / Database foundation

**Files:**
- Create: `apps/finance-core/src/finance_core/settings.py`
- Create: `apps/finance-core/src/finance_core/db.py`
- Create: `apps/finance-core/tests/test_settings.py`
- Modify: `apps/finance-core/src/finance_core/main.py`

**Interfaces:**
- Produces `Settings(db_host: str, db_port: int, db_name: str, db_user: str, db_password: SecretStr, ebk_base_url: str, ebk_api_token: SecretStr, timezone: str)` and builds the SQLAlchemy URL with `sqlalchemy.URL.create()` so passwords never require manual URL-encoding.
- Produces `get_session()` dependency.

- [ ] **Step 1: Write failing settings test**

```python
from finance_core.settings import Settings


def test_default_timezone_is_asia_shanghai() -> None:
    s = Settings(db_host="db", db_port=5432, db_name="finance", db_user="finance_app", db_password="p@ss:/word", ebk_base_url="http://book/api/v1", ebk_api_token="secret")
    assert s.timezone == "Asia/Shanghai"
```

- [ ] **Step 2: Run RED**

```bash
cd apps/finance-core && PYTHONPATH=src pytest -q tests/test_settings.py
```

Expected: import/Settings failure because module is not implemented.

- [ ] **Step 3: Implement Pydantic Settings and SQLAlchemy engine**
- `db_host`, `db_port`, `db_name`, `db_user`, `db_password`, `ebk_base_url`, `ebk_api_token` required；
- 使用 `sqlalchemy.URL.create(drivername="postgresql+psycopg", ...)` 生成连接 URL，禁止人工拼接带密码的 DSN；
- `timezone="Asia/Shanghai"`；
- DB pool defaults kept conservative; no custom tuning without measurement.

- [ ] **Step 4: Run GREEN and full tests**

```bash
PYTHONPATH=src pytest -q
```

- [ ] **Step 5: Commit**

```bash
git add apps/finance-core
git commit -m "feat(core): add runtime settings and database foundation"
```

### Task 2: Money primitives

**Files:**
- Create: `apps/finance-core/src/finance_core/money.py`
- Create: `apps/finance-core/tests/test_money.py`

**Interfaces:**
- `Money(amount_minor: int, currency: str)`
- `Money.add(other) -> Money`; mismatched currency raises `CurrencyMismatchError`.

- [ ] **Step 1: Write failing tests**

```python
import pytest
from finance_core.money import Money, CurrencyMismatchError


def test_money_adds_same_currency() -> None:
    assert Money(1000, "CNY").add(Money(250, "CNY")) == Money(1250, "CNY")


def test_money_rejects_cross_currency_addition() -> None:
    with pytest.raises(CurrencyMismatchError):
        Money(1000, "CNY").add(Money(100, "USD"))
```

- [ ] **Step 2:** Run RED.
- [ ] **Step 3:** Implement immutable Money; never add float constructors.
- [ ] **Step 4:** Run GREEN/full suite.
- [ ] **Step 5:** Commit `feat(core): add exact money primitive`.

### Task 3: ezBookkeeping Ledger Adapter contract

**Files:**
- Create: `apps/finance-core/src/finance_core/ledger/models.py`
- Create: `apps/finance-core/src/finance_core/ledger/port.py`
- Create: `apps/finance-core/src/finance_core/ledger/ezbookkeeping.py`
- Create: `apps/finance-core/tests/fixtures/ebk/*.json`
- Create: `apps/finance-core/tests/test_ebk_adapter.py`

**Interfaces:**
- `LedgerPort.list_accounts() -> list[LedgerAccount]`
- `LedgerPort.list_transactions(start, end) -> list[LedgerTransaction]`
- `EzBookkeepingLedger(base_url, token, timezone)`
- HTTP header: `Authorization: Bearer ...`, `X-Timezone-Name: Asia/Shanghai`.

- [ ] **Step 1:** Capture sanitized official-compatible JSON fixtures from staging or construct them strictly from current official API schema; keep raw fixture and expected normalized object in test.
- [ ] **Step 2:** Write failing HTTPX MockTransport test that asserts Bearer/timezone headers and amount/type normalization.
- [ ] **Step 3:** Run RED.
- [ ] **Step 4:** Implement only read endpoints required by V1; handle pagination/time range explicitly.
- [ ] **Step 5:** Run GREEN/full suite.
- [ ] **Step 6:** Against staging ezBookkeeping 1.6.1 run a manual contract smoke test; do not use production token in tests.
- [ ] **Step 7:** Commit `feat(ledger): add ezbookkeeping read adapter`.

### Task 4: Household persistence

**Files:**
- Create: `apps/finance-core/src/finance_core/household/models.py`
- Create: `apps/finance-core/src/finance_core/household/repository.py`
- Create: `apps/finance-core/tests/test_household.py`
- Create: Alembic initial migration.

**Interfaces:**
- `Household`, `Member`, `IncomeSource`, `HouseholdPolicy`.
- `HouseholdPolicy.liquidity_floor_minor` is explicit, not inferred by LLM.

- [ ] Write failing repository round-trip test using test PostgreSQL.
- [ ] Run RED.
- [ ] Implement SQLAlchemy models/repository and migration.
- [ ] Run migration up/down in disposable DB.
- [ ] Run GREEN/full suite.
- [ ] Commit `feat(household): persist household financial profile`.

### Task 5: Transaction normalization and data quality

**Files:**
- Create: `apps/finance-core/src/finance_core/analytics/normalization.py`
- Create: `apps/finance-core/src/finance_core/analytics/quality.py`
- Create: `apps/finance-core/tests/test_transaction_normalization.py`

**Interfaces:**
- `classify_cashflow_event(tx) -> CashflowEventType`
- `DataQuality(as_of, ledger_synced_at, unknown_amount_minor, level)`

- [ ] Write failing table tests for expense, income, transfer, balance adjustment, credit-card repayment, refund fixture.
- [ ] Run RED.
- [ ] Implement explicit mapping; unknown type returns `UNKNOWN`, never guesses.
- [ ] Run GREEN/full suite.
- [ ] Commit `feat(analytics): normalize ledger events and quality state`.

### Task 6: Cashflow and Net Worth

**Files:**
- Create: `apps/finance-core/src/finance_core/analytics/cashflow.py`
- Create: `apps/finance-core/src/finance_core/analytics/networth.py`
- Create: `apps/finance-core/tests/test_cashflow.py`
- Create: `apps/finance-core/tests/test_networth.py`

**Interfaces:**
- `calculate_cashflow(events, period) -> CashflowSummary`
- `calculate_net_worth(accounts, valuations) -> NetWorthSummary`

- [ ] Write failing test proving internal transfer does not alter income/expense.
- [ ] Write failing test proving credit-card payment does not double count expense.
- [ ] Write failing net-worth asset-minus-liability test.
- [ ] Run RED.
- [ ] Implement minimal pure functions.
- [ ] Run GREEN/full suite.
- [ ] Commit `feat(analytics): add cashflow and net worth engine`.

### Task 7: Budget and Safe-to-Spend

**Files:**
- Create: `apps/finance-core/src/finance_core/budget/models.py`
- Create: `apps/finance-core/src/finance_core/budget/service.py`
- Create: `apps/finance-core/src/finance_core/budget/safe_to_spend.py`
- Create: `apps/finance-core/tests/test_budget.py`
- Create: `apps/finance-core/tests/test_safe_to_spend.py`

**Interfaces:**
- `BudgetPlan`, `BudgetLine(kind=essential|discretionary|financial_goal)`
- `calculate_safe_to_spend(input) -> SafeToSpendResult` including components and deficit.

- [ ] Write failing test with explicit components and expected remaining amount.
- [ ] Write failing negative-result test; result must preserve deficit rather than clamp to zero.
- [ ] Run RED.
- [ ] Implement calculation and persistence.
- [ ] Run GREEN/full suite.
- [ ] Commit `feat(budget): add budget and safe-to-spend`.

### Task 8: Emergency Fund

**Files:**
- Create: `apps/finance-core/src/finance_core/budget/emergency.py`
- Create: `apps/finance-core/tests/test_emergency_fund.py`

**Interfaces:**
- `calculate_emergency_months(eligible_liquid_minor, essential_monthly_minor)`.

- [ ] Test zero essential burn and normal case.
- [ ] Run RED.
- [ ] Implement explicit `None/not_applicable` for zero denominator rather than divide-by-zero/guess.
- [ ] Run GREEN.
- [ ] Commit `feat(budget): calculate emergency fund coverage`.

### Task 9: Debt Engine

**Files:**
- Create: `apps/finance-core/src/finance_core/debt/models.py`
- Create: `apps/finance-core/src/finance_core/debt/simulator.py`
- Create: `apps/finance-core/tests/test_debt_simulator.py`

**Interfaces:**
- `DebtContract`
- `simulate_debt_plan(contracts, extra_monthly_minor, liquidity_available_minor, liquidity_floor_minor, strategy) -> DebtPlanResult`

- [ ] Write failing Avalanche ordering test.
- [ ] Write failing Snowball ordering test.
- [ ] Write failing liquidity-floor test proving extra payment is capped.
- [ ] Write failing missing-APR test that marks optimal-interest comparison unavailable.
- [ ] Run RED.
- [ ] Implement monthly amortization with `Decimal` rates and integer money outputs; document rounding policy.
- [ ] Run GREEN/full suite.
- [ ] Cross-check one fixture with an independent spreadsheet/calculator and store expected fixture.
- [ ] Commit `feat(debt): add constrained debt payoff simulator`.

### Task 10: Goal Engine

**Files:**
- Create: `apps/finance-core/src/finance_core/goals/models.py`
- Create: `apps/finance-core/src/finance_core/goals/service.py`
- Create: `apps/finance-core/tests/test_goals.py`

**Interfaces:**
- `FinancialGoal`
- `project_goal(goal, as_of) -> GoalProjection`

- [ ] Write failing on-track/off-track/no-months-left tests.
- [ ] Run RED.
- [ ] Implement deterministic no-hidden-return V1 projection.
- [ ] Run GREEN.
- [ ] Commit `feat(goals): add deterministic goal projections`.

### Task 11: Portfolio summary V1

**Files:**
- Create: `apps/finance-core/src/finance_core/portfolio/models.py`
- Create: `apps/finance-core/src/finance_core/portfolio/service.py`
- Create: `apps/finance-core/tests/test_portfolio_summary.py`

**Interfaces:**
- Asset classes: CASH, DEPOSIT, MONEY_MARKET, FIXED_INCOME, EQUITY, GOLD, REAL_ESTATE, OTHER.
- `summarize_allocation(snapshots) -> AllocationSummary`.

- [ ] Write failing allocation percentage test and stale valuation test.
- [ ] Run RED.
- [ ] Implement summary only; no market API or security recommendations.
- [ ] Run GREEN.
- [ ] Commit `feat(portfolio): add household allocation summary`.

### Task 12: Scenario Engine

**Files:**
- Create: `apps/finance-core/src/finance_core/scenario/models.py`
- Create: `apps/finance-core/src/finance_core/scenario/service.py`
- Create: `apps/finance-core/tests/test_purchase_scenario.py`

**Interfaces:**
- `ScenarioResult(assumptions, before, after, deltas, violations, data_quality)`.
- `simulate_purchase(...)`, `simulate_extra_debt_payment(...)`.

- [ ] Write failing purchase test showing Safe-to-Spend and debt-plan impact.
- [ ] Run RED.
- [ ] Compose existing budget/debt services; do not duplicate formulas.
- [ ] Run GREEN/full suite.
- [ ] Commit `feat(scenario): add composable what-if simulations`.

### Task 13: Finance Tool layer

**Files:**
- Create: `apps/finance-core/src/finance_core/advisor/tools.py`
- Create: `apps/finance-core/tests/test_advisor_tools.py`

**Interfaces:** exact tool list from `docs/05-ai-advisor.md`.

- [ ] Write failing tests proving `get_safe_to_spend` returns calculation + data-quality metadata.
- [ ] Run RED.
- [ ] Implement thin orchestration over services.
- [ ] Run GREEN.
- [ ] Commit `feat(advisor): expose deterministic finance tools`.

### Task 14: LLM Adapter and Advisor policy

**Files:**
- Create: `apps/finance-core/src/finance_core/advisor/llm.py`
- Create: `apps/finance-core/src/finance_core/advisor/service.py`
- Create: `apps/finance-core/src/finance_core/advisor/prompts.py`
- Create: `apps/finance-core/tests/test_advisor_policy.py`

**Interfaces:**
- `LLMPort.complete(messages, tools) -> LLMResponse`
- `Advisor.ask(user_id, question) -> AdviceResponse`

- [ ] Create fake LLM that asks for a tool, then returns explanation.
- [ ] Write failing test: numeric spending question must invoke a finance tool before final response.
- [ ] Write failing test: tool failure returns unavailable state, not fabricated value.
- [ ] Write failing prompt-injection fixture inside merchant description.
- [ ] Run RED.
- [ ] Implement OpenAI-compatible Adapter behind `LLMPort`; model id comes only from settings.
- [ ] Implement system policy and tool allowlist; no transaction-write tool in V1.
- [ ] Run GREEN/full suite.
- [ ] Commit `feat(advisor): add tool-grounded AI advisor`.

### Task 15: REST API

**Files:**
- Create focused routers under each module.
- Modify: `main.py`.
- Create: `tests/test_api_*.py`.

**Endpoints minimum:**
- `/api/v1/overview`
- `/api/v1/cashflow`
- `/api/v1/budget`
- `/api/v1/safe-to-spend`
- `/api/v1/debts`
- `/api/v1/goals`
- `/api/v1/scenarios/purchase`
- `/api/v1/advisor/chat`

- [ ] Write failing API test for each endpoint status/schema before implementation.
- [ ] Implement endpoints only as adapters to services.
- [ ] Verify OpenAPI does not expose internal DB/SQL/tool internals.
- [ ] Run full test suite.
- [ ] Commit `feat(api): expose finance core v1 endpoints`.

### Task 16: Finance PWA Dashboard

**Files:**
- Create: `apps/finance-web/` only when this task begins.

**Interfaces:** consumes `/api/v1/*` only.

- [ ] Before scaffolding, re-check current Node LTS/Vue/Vite/PWA plugin official releases; record in a new dated research baseline.
- [ ] Create component tests first for Overview cards using fixed API fixture.
- [ ] Implement mobile-first home: Net Worth, Income/Expense/Surplus, Savings Rate, Safe-to-Spend, Emergency Months, Debt warning, Budget warning, AI input.
- [ ] Add Budget/Debt/Goals pages only to V1 scope; avoid duplicating ezBookkeeping transaction UI.
- [ ] Add installable PWA manifest and responsive smoke test.
- [ ] Commit `feat(web): add mobile-first finance dashboard`.

### Task 17: Monthly report and in-process scheduler

**Files:**
- Create: `reports/service.py`, `reports/scheduler.py`, tests.

- [ ] Write failing report fixture test from deterministic monthly summary.
- [ ] Run RED.
- [ ] Generate structured report before AI prose; report stores data `as_of`.
- [ ] Use simple in-process scheduler or host cron; do not add queue.
- [ ] Run GREEN.
- [ ] Commit `feat(reports): add monthly financial review`.

### Task 18: Audit / Advice traceability

**Files:**
- Create `audit/models.py`, `audit/repository.py`, migration, tests.

- [ ] Test that AdviceRecord stores tool names, data_as_of, prompt_version, model role/provider metadata without storing secrets.
- [ ] Run RED.
- [ ] Implement persistence.
- [ ] Run GREEN.
- [ ] Commit `feat(audit): record advice provenance`.

### Task 19: Production deployment and backup verification

**Files:**
- Modify `compose.yaml`, `Caddyfile`, `.env.example`, `scripts/backup.sh`.
- Create deployment smoke scripts if needed.

- [ ] Run `docker compose config` and ensure only Caddy publishes host ports.
- [ ] Start staging stack with fresh data directory.
- [ ] Verify Postgres 18 volume is mounted at `/var/lib/postgresql`.
- [ ] Verify ezBookkeeping storage persists after container recreate.
- [ ] Verify API Token works only as intended.
- [ ] Run backup and inspect dumps with `pg_restore --list`.
- [ ] Restore on local/disposable host and run smoke checks.
- [ ] Commit `ops: verify production compose and recovery`.

### Task 20: Real household acceptance month

**Files:**
- Add dated acceptance report under `docs/acceptance/`.

- [ ] Use system for one complete calendar month.
- [ ] Import at least Alipay/WeChat statement source actually used by household.
- [ ] Reconcile sampled transactions.
- [ ] Compare Dashboard totals to manual source totals.
- [ ] Verify debt plan with independent calculation sample.
- [ ] Review AI advice for tool-grounding and data-quality messaging.
- [ ] Record actual maintenance time, duplicate rate, missing transaction rate, cloud AI cost, VPS resource usage.
- [ ] Use measured results to decide V1.1/V1.2 priorities.
- [ ] Commit `docs: record v1 household acceptance results`.

---

## Plan Self-Review Mapping

| Spec requirement | Tasks |
|---|---|
| Accurate ledger integration | 3,5,6 |
| Household profile | 4 |
| Budget/Safe-to-Spend | 7,8 |
| Debt | 9 |
| Goals | 10 |
| Asset allocation | 11 |
| What-if | 12 |
| AI grounded in tools | 13,14 |
| Mobile access | 15,16 |
| Reports | 17 |
| Audit | 18 |
| Deploy/backup | 19 |
| Real-world validation | 20 |

No V1 task introduces Redis, Kafka, MinIO, P40, OpenClaw, MCP server, HA, or microservices.
