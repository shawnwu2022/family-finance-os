# V2 Agent Tool Parity Matrix

This matrix gates the V2 target tool contract against deterministic Finance Core application capabilities.

Status meanings:

- `READY` — a deterministic application-level operation exists and is covered by Agent Adapter parity tests.
- `CORE-PARITY-REQUIRED` — a deterministic primitive may exist, but there is no complete scoped application-level tool operation/signature yet.

MCP `tools/list` must not advertise a target capability until the selected release row is `READY` and has deterministic parity coverage. Adapter code must never fill a `CORE-PARITY-REQUIRED` gap by recalculating financial values itself.

| Approved V2 capability | Boundary status | Reason |
|---|---|---|
| `get_household_overview` | READY | `API.Overview` exists; initial adapter uses the current Finance Core snapshot, while historical `as_of` is deferred until the core exposes that contract |
| `get_cashflow` | READY | `API.Cashflow(period)` exists |
| `get_spending_analysis` | CORE-PARITY-REQUIRED | no app-level spending-analysis operation exists |
| `get_budget_status` | READY | `API.Budget(period)` exists |
| `get_safe_to_spend` | CORE-PARITY-REQUIRED | safe-to-spend exists inside the deterministic snapshot/overview but no standalone app-level operation with the approved signature exists |
| `get_debt_status` | READY | `API.Debts` exists; historical `as_of` is deferred until the core exposes that contract |
| `simulate_extra_debt_payment` | CORE-PARITY-REQUIRED | debt simulation primitives exist but no scoped app-level operation/signature exists |
| `simulate_purchase` | READY | purchase scenario exists for `amount_minor`/`currency`; `category_ref` and `date` are deferred until Finance Core consumes them with defined deterministic semantics |
| `get_goal_status` | READY | `API.Goals` exists; historical `as_of` is deferred until the core exposes that contract |
| `simulate_goal` | CORE-PARITY-REQUIRED | goal projection primitives exist but no scoped app-level operation/signature exists |
| `get_asset_allocation` | CORE-PARITY-REQUIRED | portfolio summarization primitives exist but valuations are not wired into the app-level Finance Core contract |
| `generate_monthly_report` | READY | `API.MonthlyReport(period)` exists; adapter maps `year`/`month` to `YYYY-MM` |

## Initial Agent Adapter allowlist

The first protocol-neutral boundary therefore contains exactly seven names:

```text
generate_monthly_report
get_budget_status
get_cashflow
get_debt_status
get_goal_status
get_household_overview
simulate_purchase
```

All seven are exercised by `TestAgentAdapterDeterministicParity`, which compares the Adapter business payload with the direct `appapi.API` result for the same household, input, clock, and deterministic fixture.

## Release rule

The approved 12-tool list remains the V2 target contract; this matrix controls implementation order rather than reducing scope. Before MCP transport can claim V2.0 tool-contract completion, every capability selected for that release must be `READY`, registered from the Agent Adapter allowlist, and covered by parity, authorization, audit, and protocol tests.
