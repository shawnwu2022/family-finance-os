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
| `get_spending_analysis` | CORE-PARITY-REQUIRED | no app-level spending-analysis operation exists; comparison/merchant/category semantics must be designed before implementation |
| `get_budget_status` | READY | `API.Budget(period)` exists |
| `get_safe_to_spend` | READY | `API.SafeToSpend` exposes the existing deterministic snapshot result and all six components; current snapshot only, because historical `as_of` / `period_end` liquidity cannot be reconstructed truthfully from the current account-balance model |
| `get_debt_status` | READY | `API.Debts` exists; historical `as_of` remains deferred until the core exposes that contract |
| `simulate_extra_debt_payment` | CORE-PARITY-REQUIRED | `debt.SimulateDebt` uses an explicit extra-monthly amount, while the target tool signature does not yet specify one-time versus recurring payment semantics; the adapter must not guess |
| `simulate_purchase` | READY | purchase scenario exists for `amount_minor`/`currency`; `category_ref` and `date` remain deferred until Finance Core consumes them with defined deterministic semantics |
| `get_goal_status` | READY | `API.Goals` exists; historical `as_of` remains deferred until the core exposes that contract |
| `simulate_goal` | READY | `API.SimulateGoal` copies the scoped goal, changes only the proposed monthly contribution, and delegates projection to the existing deterministic `goals.ProjectGoal` primitive |
| `get_asset_allocation` | CORE-PARITY-REQUIRED | portfolio summarization primitives exist but position/valuation data is not wired into the app-level Finance Core contract; asset classes must not be inferred from insufficient account metadata |
| `generate_monthly_report` | READY | `API.MonthlyReport(period)` exists; adapter maps `year`/`month` to `YYYY-MM` |

## Agent Adapter allowlist

The verified protocol-neutral boundary currently contains exactly nine names:

```text
generate_monthly_report
get_budget_status
get_cashflow
get_debt_status
get_goal_status
get_household_overview
get_safe_to_spend
simulate_goal
simulate_purchase
```

The original seven are exercised by `TestAgentAdapterDeterministicParity`. Safe-to-spend and goal simulation are additionally exercised by `TestAgentAdapterPhaseOneDeterministicParity`. Both suites compare Adapter business payloads with direct `appapi.API` results for the same household, input, clock, and deterministic fixture.

## Release rule

The approved 12-tool list remains the V2 target contract; this matrix controls implementation order rather than reducing scope. Before MCP transport can claim V2.0 tool-contract completion, every capability selected for that release must be `READY`, registered from the Agent Adapter allowlist, and covered by parity, authorization, audit, and protocol tests.
