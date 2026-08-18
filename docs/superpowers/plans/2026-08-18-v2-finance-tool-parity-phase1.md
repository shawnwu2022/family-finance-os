# V2 Finance Tool Parity Phase 1 Implementation Plan

**Goal:** Promote two unambiguous existing deterministic capabilities—safe-to-spend and goal contribution simulation—into typed application operations and the protocol-neutral Agent Adapter, without MCP/network changes or new financial algorithms.

**Architecture:** `appapi.API` remains the application boundary. It reuses the existing `snapshot.safeToSpend` result and `goals.ProjectGoal` primitive. `server` supplies stable transport-neutral DTOs only; no HTTP routes are added. `agentadapter` adds two tool definitions and maps scoped inputs to the new app-level methods. All financial values are copied from existing deterministic results.

**Non-goals:** spending-analysis semantics, asset-class inference, historical safe-to-spend, and extra-debt-payment semantics remain `CORE-PARITY-REQUIRED` after this phase.

## Task 1 — Typed current Safe-to-Spend operation

Files:
- Modify `internal/server/api.go` — add DTOs only.
- Modify `internal/appapi/api.go` — add `SafeToSpend(ctx, householdID)`.
- Modify `internal/appapi/api_test.go` — TDD contract.

Locked DTO:

```go
type SafeToSpendComponentsResponse struct {
    LiquidDiscretionaryPool        MoneyDTO `json:"liquid_discretionary_pool"`
    UpcomingMandatoryExpenses      MoneyDTO `json:"upcoming_mandatory_expenses"`
    DebtCommitments                MoneyDTO `json:"debt_commitments"`
    EssentialReserveUntilPeriodEnd MoneyDTO `json:"essential_reserve_until_period_end"`
    EmergencyFundGapReserved       MoneyDTO `json:"emergency_fund_gap_reserved"`
    HardGoalContributions          MoneyDTO `json:"hard_goal_contributions"`
}

type SafeToSpendResponse struct {
    DataAsOf   time.Time                     `json:"data_as_of"`
    Quality    string                        `json:"quality"`
    Period     string                        `json:"period"`
    Amount     MoneyDTO                      `json:"amount"`
    IsDeficit  bool                          `json:"is_deficit"`
    Components SafeToSpendComponentsResponse `json:"components"`
    Warnings   []string                      `json:"warnings,omitempty"`
}
```

Behavior:
1. Load household profile.
2. Resolve current household-local `YYYY-MM` using existing `periodAt(a.now(), timezone)`.
3. Build the existing snapshot exactly once.
4. Return `snapshot.safeToSpend.Amount`, `IsDeficit`, and **`snapshot.safeToSpend.Components`**—never recalculate from DTO fields.
5. Copy `snapshot.asOf`, quality, period, warnings.
6. No `as_of`/historical argument yet; current-account balances cannot truthfully reconstruct historical liquidity.

TDD:
- RED: test amount, deficit flag, all six component amounts, period, quality, as-of, warnings from existing fixture.
- GREEN: minimal method above.
- Verify `go test ./internal/appapi -run TestSafeToSpend -v` then `go test ./...`.

## Task 2 — Typed deterministic Goal Simulation

Files:
- Modify `internal/server/api.go` — add response DTO.
- Modify `internal/appapi/api.go` — add `SimulateGoal`.
- Modify `internal/appapi/api_test.go` — TDD.

Locked method:

```go
func (a *API) SimulateGoal(ctx context.Context, householdID, goalID, monthlyContributionMinor int64) (server.GoalSimulationResponse, error)
```

Locked response:

```go
type GoalSimulationResponse struct {
    DataAsOf            time.Time `json:"data_as_of"`
    Quality             string    `json:"quality"`
    GoalID              int64     `json:"goal_id"`
    MonthlyContribution MoneyDTO  `json:"monthly_contribution"`
    MonthsRemaining     int       `json:"months_remaining"`
    RequiredMonthly     MoneyDTO  `json:"required_monthly"`
    ProjectedFunded     MoneyDTO  `json:"projected_funded"`
    GapAtTarget         MoneyDTO  `json:"gap_at_target"`
    CapacityShortfall   MoneyDTO  `json:"capacity_shortfall"`
    Status              string    `json:"status"`
    Warnings            []string  `json:"warnings,omitempty"`
}
```

Behavior:
1. Reject non-positive `goalID` and negative contribution.
2. Load profile/current-period snapshot once.
3. Find the active goal in `snapshot.goals`; if absent return stable `ErrGoalNotFound` from `appapi`.
4. Copy the goal value and replace only `MonthlyContribution` with `{Minor: monthlyContributionMinor, Currency: goal.Target.Currency}`.
5. Use the same available-monthly rule as `projectGoalDTOs`: `max(snapshot.cashflow.NetCashflow, 0)`.
6. Call existing `goals.ProjectGoal`; do not reproduce its formula.
7. Return exact projection values plus snapshot metadata/warnings.

TDD:
- RED: contribution changes projected funded/status deterministically; original planner goal remains unchanged; missing goal and negative amount reject.
- GREEN: minimal method and small shared helper for available monthly if needed.
- Verify targeted + full Go suite.

## Task 3 — Add two READY tools to Agent Adapter

Files:
- Modify `internal/agentadapter/tools.go`
- Modify `internal/agentadapter/tools_test.go`
- Modify `internal/agentadapter/service.go`
- Modify `internal/agentadapter/service_test.go`
- Modify `internal/agentadapter/hardening_test.go`

Add names:

```text
get_safe_to_spend
simulate_goal
```

Inputs:
- `get_safe_to_spend`: `{}` only.
- `simulate_goal`: `{ "goal_id": <positive integer>, "monthly_contribution_minor": "<digits>" }`.

`FinanceBackend` adds:

```go
SafeToSpend(context.Context, int64) (server.SafeToSpendResponse, error)
SimulateGoal(context.Context, int64, int64, int64) (server.GoalSimulationResponse, error)
```

Rules:
- no household ID in schemas;
- parse contribution with `strconv.ParseInt`, fail on overflow;
- metadata copied directly from response;
- fake/hardening backends implement the two added interface methods;
- all nine tool schemas reject `household_id`.

TDD:
- RED tool-contract test expects exactly nine names.
- RED dispatch tests prove principal household injection and typed arguments.
- GREEN minimal definitions/dispatch.

## Task 4 — Extend deterministic app/adapter parity to all nine READY tools

File:
- Modify `internal/appapi/agent_adapter_test.go`

Add direct API vs adapter comparisons for:
- `SafeToSpend(ctx, 42)` vs `get_safe_to_spend({})`.
- `SimulateGoal(ctx, 42, goalID, amountMinor)` vs `simulate_goal`.

Use the fixed clock already in the parity fixture. Equality remains `reflect.DeepEqual` after JSON decode.

Verify:

```bash
go test ./internal/appapi -run TestAgentAdapterDeterministicParity -v
go test -race ./internal/agentadapter ./internal/appapi
go test ./...
```

## Task 5 — Update parity matrix and final gate

Files:
- Modify `docs/v2-agent-tool-parity.md`

Changes:
- `get_safe_to_spend` -> `READY`, explicitly current-snapshot only; historical `as_of/period_end` remains deferred until historical liquidity data exists.
- `simulate_goal` -> `READY`.
- Initial Agent Adapter allowlist becomes nine names.
- Keep `get_spending_analysis`, `simulate_extra_debt_payment`, `get_asset_allocation` as `CORE-PARITY-REQUIRED` with explicit reasons.

Final verification for the exact commit:
- full GitHub CI Go/Web/Container success;
- Edge Security success;
- no MCP SDK/module dependency added;
- `main` remains unchanged.
