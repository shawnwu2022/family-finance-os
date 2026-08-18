# V2 Agent Tool Parity Matrix

This matrix gates the V2 target tool contract against deterministic Finance Core application capabilities.

Status meanings:

- `READY` — a deterministic application-level operation exists and is covered by Agent Adapter parity tests.
- `CORE-PARITY-REQUIRED` — a deterministic primitive may exist, but there is no complete scoped application-level tool operation/signature yet.

MCP `tools/list` must not advertise a target capability until the selected release row is `READY` and has deterministic parity coverage. Adapter code must never fill a `CORE-PARITY-REQUIRED` gap by recalculating financial values itself.

| Approved V2 capability | Boundary status | Reason |
|---|---|---|
| `get_household_overview` | READY | `API.Overview` exists; adapter uses the current Finance Core snapshot, while historical `as_of` remains deferred until the core exposes that contract |
| `get_cashflow` | READY | `API.Cashflow(period)` exists |
| `get_spending_analysis` | READY | `API.SpendingAnalysis(period, compare_periods)` performs deterministic net-spending aggregation for the selected household-local calendar month plus `0..12` prior complete months; only normalized expense/refund events are counted, transfer/credit-card repayment/income/balance-adjustment events are excluded, refunds may make net spending negative, category names come only from ledger metadata, and cross-currency/unknown/missing-category cases are explicitly partial rather than inferred |
| `get_budget_status` | READY | `API.Budget(period)` exists |
| `get_safe_to_spend` | READY | `API.SafeToSpend` exposes the existing deterministic snapshot result and all six components; current snapshot only, because historical `as_of` / `period_end` liquidity cannot be reconstructed truthfully from the current account-balance model |
| `get_debt_status` | READY | `API.Debts` exists; historical `as_of` remains deferred until the core exposes that contract |
| `simulate_extra_debt_payment` | READY | `API.SimulateExtraDebtPayment` loads the full scoped `debt.DebtContract`, compares the baseline with a one-time extra-principal scenario delegated to deterministic debt primitives, applies the proposal at the first contractually eligible month, keeps the existing scheduled-payment rule, preserves prepayment restrictions/fees/caps, and persists no changes |
| `simulate_purchase` | READY | purchase scenario exists for `amount_minor`/`currency`; `category_ref` and `date` remain deferred until Finance Core consumes them with defined deterministic semantics |
| `get_goal_status` | READY | `API.Goals` exists; historical `as_of` remains deferred until the core exposes that contract |
| `simulate_goal` | READY | `API.SimulateGoal` copies the scoped goal, changes only the proposed monthly contribution, and delegates projection to the existing deterministic `goals.ProjectGoal` primitive |
| `get_asset_allocation` | READY | `API.AssetAllocation` derives the current household allocation only from provable account-level asset balances, delegates totals/shares to `portfolio.Summarize`, maps cash/checking/virtual to `cash` and savings/CD to `deposit`, reports coarse investment/receivable/unknown assets as `other` with partial-quality warnings instead of inventing position classes, excludes liabilities/hidden/container accounts, and omits cross-currency balances when Finance Core has no explicit FX valuation |
| `generate_monthly_report` | READY | `API.MonthlyReport(period)` exists; adapter maps `year`/`month` to `YYYY-MM` |

## Agent Adapter allowlist

The verified protocol-neutral boundary contains exactly twelve names:

```text
generate_monthly_report
get_asset_allocation
get_budget_status
get_cashflow
get_debt_status
get_goal_status
get_household_overview
get_safe_to_spend
get_spending_analysis
simulate_extra_debt_payment
simulate_goal
simulate_purchase
```

The original seven are exercised by `TestAgentAdapterDeterministicParity`. Safe-to-spend and goal simulation are additionally exercised by `TestAgentAdapterPhaseOneDeterministicParity`. Extra debt payment is exercised by `TestAgentAdapterExtraDebtPaymentParity`. Spending analysis is exercised by `TestAgentAdapterSpendingAnalysisDeterministicParity`. Asset allocation is exercised by `TestAgentAdapterAssetAllocationDeterministicParity`, while the adapter boundary test additionally proves server-injected household scope, strict empty input, metadata preservation, and exact typed business-payload passthrough. These parity suites compare direct `appapi.API` results with Agent Adapter business payloads for the same scoped household, inputs, clock, and deterministic fixtures rather than duplicating financial calculations in the adapter.

## Release rule

The approved 12-tool list is now fully represented by `READY` rows. MCP transport may claim automated V2.0 tool-contract completion only on an exact candidate commit where all twelve names are registered from this Agent Adapter allowlist and parity, authorization, audit, protocol, CI, Edge Security, and MCP Security gates are green. Real OpenClaw/deployed-endpoint acceptance remains a separate production-release gate.
