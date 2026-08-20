# V2 Asset Allocation Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Promote the final approved V2 capability, `get_asset_allocation`, into the application and Agent/MCP boundaries without inventing security-level classifications or adding a market-data/persistence subsystem.

**Architecture:** `appapi.API` reads the household's current ezBookkeeping account balances, classifies only account categories with deterministic semantics, maps ambiguous asset categories to the explicit `portfolio.AssetClassOther` bucket with a partial-data warning, excludes liabilities, hidden/container accounts, and unsupported cross-currency balances, then delegates totals/shares to the existing `portfolio.Summarize` engine. `agentadapter` exposes an empty-input current-snapshot tool and only passes through the typed application result. Historical `as_of`, position-level equity/fund/gold decomposition, FX conversion, and market-price fetching remain deferred until Finance Core has explicit source data for them.

**Tech Stack:** Go 1.26.6, existing `ledger`, `portfolio`, `appapi`, `agentadapter`, official MCP Go SDK v1.6.1.

**Spec:** `docs/superpowers/specs/2026-08-18-v2-agent-adapter-mcp-design.md`

## Global Constraints

- Financial totals and allocation shares must be calculated by existing deterministic Finance Core code, never by MCP/Agent code.
- MCP/client inputs must not contain `household_id`.
- `get_asset_allocation` is current-snapshot only in this phase; no fake historical reconstruction.
- No FX conversion is introduced. Foreign-currency account balances are omitted with explicit partial-data warnings.
- No investment account is guessed to be equity, fund, gold, fixed income, or property. Ambiguous asset categories map to `other` with an explicit warning.
- No new database table, broker integration, market-price fetcher, sidecar, or background job is introduced.

---

### Task 1: Add typed application-level asset allocation

**Files:**
- Create: `internal/appapi/asset_allocation_test.go`
- Modify: `internal/server/finance_tools.go`
- Create: `internal/appapi/asset_allocation.go`

**Interfaces:**
- Produces:

```go
type AssetAllocationItemResponse struct {
    Class string   `json:"class"`
    Value MoneyDTO `json:"value"`
    Share string   `json:"share,omitempty"`
}

type AssetAllocationResponse struct {
    DataAsOf time.Time                     `json:"data_as_of"`
    Quality  string                        `json:"quality"`
    Currency string                        `json:"currency"`
    Total    MoneyDTO                      `json:"total"`
    Items    []AssetAllocationItemResponse `json:"items"`
    Warnings []string                      `json:"warnings,omitempty"`
}

func (a *API) AssetAllocation(ctx context.Context, householdID int64) (server.AssetAllocationResponse, error)
```

- [ ] **Step 1: Write RED tests**

Cover: cash/checking -> `cash`; savings/CD -> `deposit`; investment/receivables/other supported asset categories -> `other` plus partial warning; liabilities/hidden/container accounts excluded; foreign-currency assets skipped with partial warning; output classes sorted; shares/totals equal `portfolio.Summarize` results; no mutation or inferred equity/fund/gold class.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/appapi -run TestAssetAllocation -v`

Expected: compile failure because `API.AssetAllocation` and the response DTO do not exist.

- [ ] **Step 3: Implement minimal application operation**

Classification is deliberately conservative:

```text
cash, checking, virtual -> cash
savings, certificate_of_deposit -> deposit
investment, receivables, any remaining explicit asset category -> other + partial warning
```

For each visible leaf asset in household base currency, convert account balance to non-negative magnitude and pass it to `portfolio.Summarize`. Do not classify liabilities. Sort response items by class string and copy the `apd.Decimal` share using `String()`.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
go test ./internal/appapi -run TestAssetAllocation -v
go test ./internal/portfolio ./internal/appapi
```

- [ ] **Step 5: Commit**

Commit message: `feat(v2): expose deterministic asset allocation`

---

### Task 2: Add the twelfth Agent Adapter tool

**Files:**
- Modify: `internal/agentadapter/tools.go`
- Modify: `internal/agentadapter/tools_test.go`
- Modify: `internal/agentadapter/service.go`
- Modify fake backend implementations in affected Agent Adapter/MCP tests.
- Create or modify app/adapter parity coverage under `internal/appapi`.

**Interfaces:**
- Add `ToolGetAssetAllocation ToolName = "get_asset_allocation"`.
- Input schema: `{}` only.
- `FinanceBackend` adds:

```go
AssetAllocation(context.Context, int64) (server.AssetAllocationResponse, error)
```

- [ ] **Step 1: Write RED tool-list and dispatch/parity tests**

Expect exactly twelve tool names, no `household_id`, server-side principal scope, strict empty input, and direct `appapi.API.AssetAllocation` business payload equality with Agent Adapter output.

- [ ] **Step 2: Verify RED**

Run targeted `internal/agentadapter` and `internal/appapi` tests. Expected failure: missing tool constant/backend method/dispatch.

- [ ] **Step 3: Implement minimal tool definition and dispatch**

Decode `EmptyInput`, inject principal household ID, call `AssetAllocation`, and use `encodeBackendResult` with response `DataAsOf`, `Quality`, and warnings. Do not recalculate allocations.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
go test ./internal/agentadapter ./internal/appapi ./internal/mcpadapter
go test -race ./internal/agentadapter ./internal/appapi ./internal/mcpadapter
```

- [ ] **Step 5: Commit**

Commit message: `feat(v2): add asset allocation agent tool`

---

### Task 3: Align V2 release gates to the complete twelve-tool contract

**Files:**
- Modify: `docs/v2-agent-tool-parity.md`
- Modify: `docs/v2-mcp-security-acceptance.md`
- Modify exact-count tests/integration expectations that currently assert eleven tools.

- [ ] **Step 1: Write/adjust RED exact-count tests to require twelve tools**

MCP protocol/integration tests must discover exactly the twelve approved READY names and no resources/prompts.

- [ ] **Step 2: Verify RED**

Run the targeted MCP/tool-list tests and require failure while only eleven tools are exposed.

- [ ] **Step 3: Mark `get_asset_allocation` READY and document semantics**

State that the current release uses household base-currency ezBookkeeping asset-account balances; only unambiguous cash/deposit classes are named; ambiguous investment/receivable assets are returned as `other` with partial-data warnings; foreign-currency assets are omitted without implicit FX; historical/position-level decomposition remains deferred.

- [ ] **Step 4: Run final verification**

Require the exact branch head to pass:

- full CI;
- MCP Security;
- Edge Security;
- all twelve tools listed by MCP integration;
- no `household_id` in schemas;
- no new host port, database, queue, market-data dependency, or pre-release MCP SDK.

- [ ] **Step 5: Commit**

Commit message: `docs(v2): complete twelve-tool parity`
