# V2.0 Agent Adapter Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a protocol-neutral, server-scoped Agent Adapter over the deterministic Finance Core capabilities that already exist, without adding MCP transport, new financial calculations, or external exposure in this subproject.

**Architecture:** `internal/agentadapter` owns stable external-facing tool names, strict JSON input schemas, a server-selected principal/household scope, dispatch, and a metadata envelope. It delegates to a narrow `FinanceBackend` interface implemented structurally by the existing `*appapi.API`; it never recalculates money or reaches PostgreSQL/ezBookkeeping directly. This plan intentionally exposes only capabilities that already have a deterministic application-level implementation; the remaining V2 tool-contract capabilities are completed in a separate Finance Tool Parity plan before the MCP allowlist is considered complete.

**Tech Stack:** Go 1.26.6, `context`, `encoding/json`, `net/http` DTOs already defined in `internal/server`, existing `internal/appapi` deterministic APIs, existing `internal/report` monthly report types. No new Go module dependency and no MCP SDK in this plan.

**Spec:** `docs/superpowers/specs/2026-08-18-v2-agent-adapter-mcp-design.md`

## Global Constraints

- Base line is V1 `main` commit `05d8afcf0299c3ae0fd9f0422b13e28fe8842a28`; V1 production acceptance remains independent.
- This subproject does **not** register `/mcp`, modify Caddy, change Docker host ports, add OpenClaw code, or add authentication transport code.
- `internal/agentadapter` must not import an MCP SDK, OpenClaw package, PostgreSQL store package, or ezBookkeeping adapter.
- No adapter input contains `household_id`; `Principal.HouseholdID` is the only household selector.
- The adapter never recalculates Money, percentages, safe-to-spend, debt schedules, goal projections, cashflow, net worth, or report metrics.
- Money remains the exact DTO/domain representation produced by Finance Core; no `float64` is introduced.
- Strict JSON decoding rejects unknown fields and multiple JSON values.
- The initial adapter allowlist contains only seven already-implemented capabilities: household overview, cashflow, budget status, debt status, goal status, purchase simulation, and monthly report generation.
- The richer signatures or currently unwired capabilities from the approved 12-tool V2 contract are explicitly deferred to the Finance Tool Parity plan: spending analysis, standalone safe-to-spend, extra-debt-payment simulation, goal simulation, asset allocation, historical/as-of variants, and any purchase fields not yet consumed by the deterministic application API.
- This subproject has no externally reachable agent call, so persistent fail-closed agent audit is not implemented here. The next audit subproject must land before MCP transport is exposed.
- Every behavior change follows RED → GREEN → REFACTOR.

---

## File Structure

Create:

```text
internal/agentadapter/
  tools.go          canonical initial tool names, input types, strict schemas, definitions
  errors.go         protocol-neutral stable adapter error codes/types
  service.go        principal validation, strict decode, server-scoped dispatch, result metadata extraction
  tools_test.go     allowlist/schema/no-household contract tests
  service_test.go   dispatch/scope/error/metadata tests against a fake backend
```

Add integration parity coverage in:

```text
internal/appapi/agent_adapter_test.go
```

No production file outside `internal/agentadapter` is required for this subproject. `*appapi.API` already provides the needed methods, including `MonthlyReport` in `internal/appapi/reports.go`.

## Interfaces Locked By This Plan

### Protocol-neutral names

```go
type ToolName string

const (
    ToolGetHouseholdOverview ToolName = "get_household_overview"
    ToolGetCashflow          ToolName = "get_cashflow"
    ToolGetBudgetStatus      ToolName = "get_budget_status"
    ToolGetDebtStatus        ToolName = "get_debt_status"
    ToolGetGoalStatus        ToolName = "get_goal_status"
    ToolSimulatePurchase     ToolName = "simulate_purchase"
    ToolGenerateMonthlyReport ToolName = "generate_monthly_report"
)
```

### Principal and backend

```go
type Principal struct {
    Kind        string
    HouseholdID int64
}

type FinanceBackend interface {
    Overview(context.Context, int64) (server.OverviewResponse, error)
    Cashflow(context.Context, int64, string) (server.CashflowResponse, error)
    Budget(context.Context, int64, string) (server.BudgetResponse, error)
    Debts(context.Context, int64) (server.DebtsResponse, error)
    Goals(context.Context, int64) (server.GoalsResponse, error)
    Scenario(context.Context, server.ScenarioRequest) (server.ScenarioResponse, error)
    MonthlyReport(context.Context, int64, string) (report.MonthlyReport, error)
}
```

`*appapi.API` satisfies this interface without wrappers.

### Inputs

```go
type EmptyInput struct{}

type PeriodInput struct {
    Period string `json:"period"`
}

type PurchaseInput struct {
    AmountMinor string `json:"amount_minor"`
    Currency    string `json:"currency"`
}

type MonthlyReportInput struct {
    Year  int `json:"year"`
    Month int `json:"month"`
}
```

`PurchaseInput` deliberately matches the currently implemented deterministic purchase scenario. `category_ref` and `date` are not advertised until Finance Core consumes them with a defined deterministic meaning.

### Definitions and results

```go
type ToolDefinition struct {
    Name        ToolName
    Description string
    InputSchema json.RawMessage
    ReadOnly    bool
}

type Result struct {
    Data     json.RawMessage `json:"data"`
    AsOf     *time.Time      `json:"as_of,omitempty"`
    Quality  string          `json:"quality,omitempty"`
    Warnings []string        `json:"warnings,omitempty"`
    AuditID  string          `json:"audit_id,omitempty"`
}
```

`AuditID` is reserved now so later MCP/audit work does not need to reshape the envelope; it remains empty until the persistent audit subproject is wired.

### Service API

```go
func New(backend FinanceBackend) (*Service, error)
func (s *Service) Definitions() []ToolDefinition
func (s *Service) Call(ctx context.Context, principal Principal, name ToolName, arguments json.RawMessage) (Result, error)
```

---

### Task 1: Lock the Initial Agent Tool Contract

**Files:**
- Create: `internal/agentadapter/tools.go`
- Create: `internal/agentadapter/tools_test.go`

**Interfaces:**
- Consumes: `encoding/json` only.
- Produces: `ToolName`, seven constants, `EmptyInput`, `PeriodInput`, `PurchaseInput`, `MonthlyReportInput`, `ToolDefinition`, `Definitions()` helper or equivalent immutable definition list used by `Service.Definitions()`.

- [ ] **Step 1: Write the failing contract tests**

Create `internal/agentadapter/tools_test.go` with tests that require exactly the seven names, deterministic sort order, strict schemas, and no `household_id` field:

```go
package agentadapter

import (
    "bytes"
    "encoding/json"
    "reflect"
    "testing"
)

func TestDefinitionsExposeOnlyImplementedReadAndSimulationCapabilities(t *testing.T) {
    got := definitions()
    names := make([]ToolName, 0, len(got))
    for _, definition := range got {
        names = append(names, definition.Name)
        if !definition.ReadOnly {
            t.Fatalf("tool %q is not marked read-only/non-destructive", definition.Name)
        }
        if bytes.Contains(definition.InputSchema, []byte("household_id")) {
            t.Fatalf("tool %q exposes household_id: %s", definition.Name, definition.InputSchema)
        }
        if !json.Valid(definition.InputSchema) {
            t.Fatalf("tool %q has invalid schema: %s", definition.Name, definition.InputSchema)
        }
    }
    want := []ToolName{
        ToolGenerateMonthlyReport,
        ToolGetBudgetStatus,
        ToolGetCashflow,
        ToolGetDebtStatus,
        ToolGetGoalStatus,
        ToolGetHouseholdOverview,
        ToolSimulatePurchase,
    }
    if !reflect.DeepEqual(gotNames(got), want) {
        t.Fatalf("tool names = %#v, want %#v", names, want)
    }
}

func TestSchemasRejectAdditionalPropertiesByContract(t *testing.T) {
    for _, definition := range definitions() {
        var schema map[string]any
        if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
            t.Fatalf("decode %s schema: %v", definition.Name, err)
        }
        if value, ok := schema["additionalProperties"].(bool); !ok || value {
            t.Fatalf("tool %q must set additionalProperties=false", definition.Name)
        }
    }
}
```

Define the small `gotNames` test helper in the same test file; it must return the names in the order supplied so sorting is tested rather than hidden.

- [ ] **Step 2: Run the tests and verify RED**

Run:

```bash
go test ./internal/agentadapter -run 'TestDefinitions|TestSchemas' -v
```

Expected: FAIL to compile because `ToolName`, tool constants, and `definitions` do not exist.

- [ ] **Step 3: Implement the minimal tool contract**

Create `internal/agentadapter/tools.go` with the locked types above. Use these schemas verbatim:

```go
var (
    emptyInputSchema = json.RawMessage(`{"type":"object","additionalProperties":false}`)
    periodInputSchema = json.RawMessage(`{"type":"object","properties":{"period":{"type":"string","pattern":"^[0-9]{4}-(0[1-9]|1[0-2])$"}},"required":["period"],"additionalProperties":false}`)
    purchaseInputSchema = json.RawMessage(`{"type":"object","properties":{"amount_minor":{"type":"string","pattern":"^[0-9]+$"},"currency":{"type":"string","minLength":3,"maxLength":3}},"required":["amount_minor","currency"],"additionalProperties":false}`)
    monthlyReportInputSchema = json.RawMessage(`{"type":"object","properties":{"year":{"type":"integer","minimum":1970,"maximum":9999},"month":{"type":"integer","minimum":1,"maximum":12}},"required":["year","month"],"additionalProperties":false}`)
)
```

Return definitions sorted lexicographically by `Name`. Descriptions must state that results are deterministic Finance Core values and that callers must preserve quality/warning metadata.

- [ ] **Step 4: Run contract tests and verify GREEN**

Run:

```bash
go test ./internal/agentadapter -run 'TestDefinitions|TestSchemas' -v
```

Expected: PASS.

- [ ] **Step 5: Run formatting and commit**

```bash
gofmt -w internal/agentadapter/tools.go internal/agentadapter/tools_test.go
go test ./internal/agentadapter -v
git add internal/agentadapter/tools.go internal/agentadapter/tools_test.go
git commit -m "feat(v2): define agent tool contract"
```

---

### Task 2: Add Stable Errors, Principal Validation, and Strict Dispatch

**Files:**
- Create: `internal/agentadapter/errors.go`
- Create: `internal/agentadapter/service.go`
- Create: `internal/agentadapter/service_test.go`

**Interfaces:**
- Consumes: Task 1 `ToolName` and input types; existing `server` response/request DTOs; `report.MonthlyReport`.
- Produces: `Principal`, `FinanceBackend`, `Result`, `Service`, `New`, `Definitions`, `Call`, stable `ErrorCode` values.

- [ ] **Step 1: Write failing scope and dispatch tests**

Create `internal/agentadapter/service_test.go`. Define a `fakeBackend` implementing the exact `FinanceBackend` signatures and recording every household ID/argument it receives. Add these tests:

```go
func TestCallInjectsPrincipalHouseholdAndNeverAcceptsHouseholdArgument(t *testing.T) {
    backend := &fakeBackend{overview: server.OverviewResponse{Quality: "good"}}
    service, err := New(backend)
    if err != nil { t.Fatalf("New: %v", err) }

    _, err = service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolGetHouseholdOverview, json.RawMessage(`{"household_id":99}`))
    if !IsCode(err, CodeInvalidArgument) {
        t.Fatalf("error=%v, want %s", err, CodeInvalidArgument)
    }
    if backend.overviewCalls != 0 {
        t.Fatalf("backend called %d times", backend.overviewCalls)
    }

    _, err = service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolGetHouseholdOverview, json.RawMessage(`{}`))
    if err != nil { t.Fatalf("Call: %v", err) }
    if backend.overviewHouseholdID != 42 {
        t.Fatalf("household=%d want 42", backend.overviewHouseholdID)
    }
}

func TestCallRejectsInvalidPrincipalBeforeBackend(t *testing.T) {
    backend := &fakeBackend{}
    service, _ := New(backend)
    _, err := service.Call(context.Background(), Principal{}, ToolGetHouseholdOverview, json.RawMessage(`{}`))
    if !IsCode(err, CodeForbidden) {
        t.Fatalf("error=%v, want %s", err, CodeForbidden)
    }
    if backend.totalCalls() != 0 {
        t.Fatalf("backend was called")
    }
}

func TestCallRejectsUnknownToolAndTrailingJSON(t *testing.T) {
    backend := &fakeBackend{}
    service, _ := New(backend)
    principal := Principal{Kind: "test", HouseholdID: 42}
    if _, err := service.Call(context.Background(), principal, ToolName("execute_sql"), json.RawMessage(`{}`)); !IsCode(err, CodeToolNotFound) {
        t.Fatalf("unknown tool error=%v", err)
    }
    if _, err := service.Call(context.Background(), principal, ToolGetCashflow, json.RawMessage(`{"period":"2026-08"}{}`)); !IsCode(err, CodeInvalidArgument) {
        t.Fatalf("trailing JSON error=%v", err)
    }
}
```

Add table-driven cases for the seven tools that verify exact argument mapping:
- overview → `Overview(ctx, 42)`
- cashflow `2026-08` → `Cashflow(ctx, 42, "2026-08")`
- budget `2026-08` → `Budget(ctx, 42, "2026-08")`
- debt → `Debts(ctx, 42)`
- goals → `Goals(ctx, 42)`
- purchase → `Scenario(ctx, server.ScenarioRequest{HouseholdID:42, Kind:"purchase", Input:<canonical purchase JSON>})`
- report `{year:2026,month:7}` → `MonthlyReport(ctx, 42, "2026-07")`.

- [ ] **Step 2: Run service tests and verify RED**

```bash
go test ./internal/agentadapter -run 'TestCall' -v
```

Expected: FAIL because `Service`, `Principal`, error codes, and `Call` do not exist.

- [ ] **Step 3: Implement stable error taxonomy**

Create `internal/agentadapter/errors.go`:

```go
package agentadapter

import (
    "errors"
    "fmt"
)

type ErrorCode string

const (
    CodeForbidden       ErrorCode = "forbidden"
    CodeInvalidArgument ErrorCode = "invalid_argument"
    CodeToolNotFound    ErrorCode = "tool_not_found"
    CodeDataUnavailable ErrorCode = "data_unavailable"
    CodeDataPartial     ErrorCode = "data_partial"
    CodeAuditUnavailable ErrorCode = "audit_unavailable"
    CodeTimeout         ErrorCode = "timeout"
    CodeBusy            ErrorCode = "busy"
    CodeInternal        ErrorCode = "internal"
)

type Error struct {
    Code ErrorCode
    Message string
    Err error
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }
func (e *Error) Unwrap() error { return e.Err }

func IsCode(err error, code ErrorCode) bool {
    var target *Error
    return errors.As(err, &target) && target.Code == code
}
```

Do not put raw backend error text into `Message`; retain the underlying error only for server-side `errors.Is/As` and logging later.

- [ ] **Step 4: Implement strict decoding and dispatch**

Create `internal/agentadapter/service.go` with the locked interfaces. Implement one generic private helper:

```go
func decodeStrict[T any](raw json.RawMessage) (T, error)
```

It must:
- reject empty JSON;
- use `json.Decoder.DisallowUnknownFields()`;
- require EOF after exactly one JSON value;
- return `CodeInvalidArgument` with a safe message.

`New(nil)` returns an error. `Call` validates non-empty `Principal.Kind` and `HouseholdID > 0` before dispatch. Unknown names return `CodeToolNotFound` before backend invocation.

For purchase dispatch, decode `PurchaseInput`, marshal only that typed value, then set `HouseholdID` internally on `server.ScenarioRequest`. Do not copy arbitrary raw JSON into the backend request.

For monthly report dispatch, validate `1970 <= Year <= 9999` and `1 <= Month <= 12`, then format exactly with:

```go
period := fmt.Sprintf("%04d-%02d", input.Year, input.Month)
```

Map backend failures to `CodeDataUnavailable` with a safe message such as `"finance data is unavailable"`; do not expose `err.Error()` as the external message.

- [ ] **Step 5: Run service tests and verify GREEN**

```bash
go test ./internal/agentadapter -run 'TestCall' -v
```

Expected: PASS.

- [ ] **Step 6: Run the whole package and commit**

```bash
gofmt -w internal/agentadapter/errors.go internal/agentadapter/service.go internal/agentadapter/service_test.go
go test ./internal/agentadapter -v
git add internal/agentadapter/errors.go internal/agentadapter/service.go internal/agentadapter/service_test.go
git commit -m "feat(v2): add scoped agent dispatch"
```

---

### Task 3: Preserve Finance Core Metadata Without Recalculation

**Files:**
- Modify: `internal/agentadapter/service.go`
- Modify: `internal/agentadapter/service_test.go`

**Interfaces:**
- Consumes: each existing backend response's `DataAsOf`, `Quality`, and `Warnings` where available.
- Produces: the locked `Result` envelope with copied `Data`, `AsOf`, `Quality`, `Warnings`, and currently empty `AuditID`.

- [ ] **Step 1: Write failing metadata tests**

Add cases such as:

```go
func TestCallPreservesOverviewMetadataAndBusinessPayload(t *testing.T) {
    asOf := time.Date(2026, 8, 18, 1, 2, 3, 0, time.UTC)
    source := server.OverviewResponse{
        DataAsOf: asOf,
        Quality: "partial",
        NetWorth: server.MoneyDTO{Minor: 12345, Currency: "CNY"},
        Warnings: []string{"source_partial"},
    }
    backend := &fakeBackend{overview: source}
    service, _ := New(backend)
    result, err := service.Call(context.Background(), Principal{Kind:"test", HouseholdID:42}, ToolGetHouseholdOverview, json.RawMessage(`{}`))
    if err != nil { t.Fatalf("Call: %v", err) }
    if result.AsOf == nil || !result.AsOf.Equal(asOf) || result.Quality != "partial" {
        t.Fatalf("metadata=%#v", result)
    }
    if !reflect.DeepEqual(result.Warnings, []string{"source_partial"}) {
        t.Fatalf("warnings=%#v", result.Warnings)
    }
    var got server.OverviewResponse
    if err := json.Unmarshal(result.Data, &got); err != nil { t.Fatalf("decode: %v", err) }
    if !reflect.DeepEqual(got, source) { t.Fatalf("data=%#v want %#v", got, source) }
    if result.AuditID != "" { t.Fatalf("audit id must be empty before audit wiring") }
}
```

Add equivalent representative assertions for:
- cashflow (`DataAsOf`, `Quality`, `Warnings`);
- purchase scenario (`Warnings`, no fabricated `AsOf`/`Quality`);
- monthly report (`DataAsOf`, `Quality`, `Warnings`).

Also mutate `result.Warnings` after return and assert the fake backend's source slice did not change, proving defensive copying.

- [ ] **Step 2: Run metadata tests and verify RED**

```bash
go test ./internal/agentadapter -run 'TestCallPreserves' -v
```

Expected: FAIL until metadata extraction and defensive copies are implemented.

- [ ] **Step 3: Implement metadata extraction**

In each dispatch branch:
1. call the backend once;
2. JSON-marshal the exact returned business response into `Result.Data`;
3. copy metadata from the response, without deriving new financial values.

Use a helper:

```go
func cloneWarnings(values []string) []string {
    return append([]string(nil), values...)
}
```

For `server.OverviewResponse`, `CashflowResponse`, `BudgetResponse`, `DebtsResponse`, `GoalsResponse`, set `AsOf`, `Quality`, and `Warnings` directly. For `server.ScenarioResponse`, set only `Warnings`. For `report.MonthlyReport`, set `AsOf`, `Quality`, `Warnings` directly.

- [ ] **Step 4: Run metadata and all package tests**

```bash
go test ./internal/agentadapter -run 'TestCallPreserves' -v
go test ./internal/agentadapter -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/agentadapter/service.go internal/agentadapter/service_test.go
git add internal/agentadapter/service.go internal/agentadapter/service_test.go
git commit -m "feat(v2): preserve agent result metadata"
```

---

### Task 4: Prove Deterministic Parity Against the Real Application API

**Files:**
- Create: `internal/appapi/agent_adapter_test.go`

**Interfaces:**
- Consumes: `agentadapter.New`, existing `appapi.API`, existing fake ledger/planner test fixtures in package `appapi`, existing `API.MonthlyReport`.
- Produces: integration evidence that the adapter returns byte-decodable business payloads identical to direct Finance Core calls for the same household and inputs.

- [ ] **Step 1: Write the parity integration test**

Create `internal/appapi/agent_adapter_test.go` in package `appapi` so it can reuse `fakeLedger` and `fakePlanner`. Build the same deterministic CNY fixture style already used by `api_test.go` and `reports_test.go`. Add one table test for each initial tool.

For each case:
1. call the direct `API` method with household `42`;
2. call `agentadapter.Service.Call` with `Principal{Kind:"test", HouseholdID:42}`;
3. decode `Result.Data` into the same direct-response Go type;
4. compare with `reflect.DeepEqual`.

For purchase parity, direct call must be:

```go
raw := json.RawMessage(`{"amount_minor":"10000","currency":"CNY"}`)
direct, err := api.Scenario(ctx, server.ScenarioRequest{
    HouseholdID: 42,
    Kind: "purchase",
    Input: raw,
})
```

The adapter receives the same business arguments without `household_id`.

For report parity:

```go
direct, err := api.MonthlyReport(ctx, 42, "2026-07")
result, err := service.Call(ctx, principal, agentadapter.ToolGenerateMonthlyReport, json.RawMessage(`{"year":2026,"month":7}`))
```

Because `GeneratedAt` comes from the injected `Now`, both direct and adapter calls must share a fixed `Now` function so equality is deterministic.

- [ ] **Step 2: Run the parity test and verify RED**

```bash
go test ./internal/appapi -run TestAgentAdapterDeterministicParity -v
```

Expected: the newly written test must fail before any parity defects are corrected. If it passes immediately, inspect the test to ensure all seven tools are actually exercised and direct results are compared, not just successful status.

- [ ] **Step 3: Correct adapter-only parity defects**

Fix only mapping/serialization defects in `internal/agentadapter`. Do not modify finance calculations to make parity pass. If a test exposes a genuine Finance Core calculation defect, stop this task and handle it as a separate core bugfix with its own TDD cycle.

- [ ] **Step 4: Run parity plus existing application tests**

```bash
go test ./internal/appapi -run 'TestAgentAdapterDeterministicParity|TestAPIComposesDeterministicHouseholdSnapshot|TestMonthlyReportUsesExplicitPeriodDeterministicSnapshot' -v
go test ./internal/agentadapter ./internal/appapi -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/appapi/agent_adapter_test.go internal/agentadapter/*.go
git add internal/appapi/agent_adapter_test.go internal/agentadapter
git commit -m "test(v2): prove agent adapter parity"
```

---

### Task 5: Harden the Boundary Against Scope and Concurrency Regressions

**Files:**
- Modify: `internal/agentadapter/service_test.go`

**Interfaces:**
- Consumes: completed `Service`.
- Produces: regression tests for cross-household isolation, input immutability, cancellation propagation, and concurrent read calls.

- [ ] **Step 1: Add failing boundary-hardening tests**

Add:

```go
func TestConcurrentCallsKeepPrincipalScopesIsolated(t *testing.T) {
    backend := newConcurrentFakeBackend()
    service, _ := New(backend)
    const calls = 32
    var wg sync.WaitGroup
    for i := 0; i < calls; i++ {
        i := i
        wg.Add(1)
        go func() {
            defer wg.Done()
            householdID := int64(100 + i)
            _, err := service.Call(context.Background(), Principal{Kind:"test", HouseholdID:householdID}, ToolGetHouseholdOverview, json.RawMessage(`{}`))
            if err != nil { t.Errorf("Call(%d): %v", householdID, err) }
        }()
    }
    wg.Wait()
    if !backend.sawExactlyHouseholds(100, 131) {
        t.Fatalf("scopes=%v", backend.scopes())
    }
}
```

Also add tests that:
- a cancelled context reaches the backend as cancelled;
- mutating the caller's `arguments` byte slice after `Call` returns does not mutate captured typed backend input;
- a JSON `household_id` attack is rejected for every one of the seven tool names, not only overview;
- backend raw errors never appear in `(*Error).Message`.

Use a mutex in the concurrent fake; the test itself must be race-safe.

- [ ] **Step 2: Run under race detector and verify RED if any guard is missing**

```bash
go test -race ./internal/agentadapter -run 'TestConcurrent|TestCallRejectsHousehold|TestCallDoesNotExposeBackendError|TestCallPropagatesCancellation' -v
```

Expected: at least one test fails if the implementation has a scope/cancellation/error-message defect. If all pass immediately, keep them as regression coverage and proceed; they validate existing behavior rather than requiring artificial production changes.

- [ ] **Step 3: Implement only required hardening fixes**

Any fix must remain inside `internal/agentadapter`. Do not add shared mutable principal state to `Service`; household scope must stay a per-call value.

- [ ] **Step 4: Run package race tests and full Go regression suite**

```bash
go test -race ./internal/agentadapter ./internal/appapi
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/agentadapter/service_test.go internal/agentadapter/*.go
git add internal/agentadapter
git commit -m "test(v2): harden agent adapter boundary"
```

---

### Task 6: Record the Deliberate V2 Tool-Parity Decomposition

**Files:**
- Create: `docs/v2-agent-tool-parity.md`
- Modify: `docs/superpowers/specs/2026-08-18-v2-agent-adapter-mcp-design.md`

**Interfaces:**
- Consumes: code inventory and the approved V2 design.
- Produces: an explicit compatibility matrix that prevents later MCP work from advertising tools whose application-level contract does not yet exist.

- [ ] **Step 1: Write the tool-parity matrix**

Create `docs/v2-agent-tool-parity.md` with exactly these status rules:

```text
READY = deterministic app-level operation exists and is covered by Agent Adapter parity tests.
CORE-PARITY-REQUIRED = deterministic primitive may exist, but there is no complete app-level tool operation/signature yet.
```

Initial matrix:

| Approved V2 capability | Boundary status | Reason |
|---|---|---|
| `get_household_overview` | READY | `API.Overview` exists; initial adapter uses current-time snapshot, historical `as_of` is deferred |
| `get_cashflow` | READY | `API.Cashflow(period)` exists |
| `get_spending_analysis` | CORE-PARITY-REQUIRED | no app-level spending-analysis operation exists |
| `get_budget_status` | READY | `API.Budget(period)` exists |
| `get_safe_to_spend` | CORE-PARITY-REQUIRED | safe-to-spend exists inside snapshot/overview but no standalone app-level operation with approved signature |
| `get_debt_status` | READY | `API.Debts` exists; historical `as_of` is deferred |
| `simulate_extra_debt_payment` | CORE-PARITY-REQUIRED | debt simulator primitive exists but no app-level scoped operation/signature |
| `simulate_purchase` | READY | purchase scenario exists for amount/currency; `category_ref` and `date` semantics are deferred until core consumes them |
| `get_goal_status` | READY | `API.Goals` exists; historical `as_of` is deferred |
| `simulate_goal` | CORE-PARITY-REQUIRED | goal projection primitive exists but no app-level scoped operation/signature |
| `get_asset_allocation` | CORE-PARITY-REQUIRED | portfolio summarizer exists but valuations are not wired into app-level Finance Core |
| `generate_monthly_report` | READY | `API.MonthlyReport(period)` exists; adapter maps year/month to period |

State explicitly: the MCP `tools/list` allowlist cannot claim V2.0 complete until every required V2.0 row selected for release is `READY` and tested.

- [ ] **Step 2: Update the design document status and decomposition note**

Change the design header to:

```text
**Status:** Approved 2026-08-18; implementation decomposed by verified Finance Core parity
```

In section 5, immediately before the 12-tool list, add one paragraph: the list is the target V2 tool contract; implementation is gated by `docs/v2-agent-tool-parity.md`, and MCP must not advertise a target tool until its row is `READY`.

This is not a scope reduction: it makes the approved contract implementation-order explicit.

- [ ] **Step 3: Verify documentation has no placeholders or contradictory completion claims**

Run:

```bash
grep -RniE '\b(TBD|TODO|implement later|fill in details)\b' docs/v2-agent-tool-parity.md docs/superpowers/specs/2026-08-18-v2-agent-adapter-mcp-design.md && exit 1 || true
grep -n 'CORE-PARITY-REQUIRED' docs/v2-agent-tool-parity.md
grep -n 'implementation decomposed by verified Finance Core parity' docs/superpowers/specs/2026-08-18-v2-agent-adapter-mcp-design.md
```

Expected: first command exits 0 through `|| true` with no matching placeholder output; the other commands find the expected lines.

- [ ] **Step 4: Final subproject verification**

Run:

```bash
go test ./internal/agentadapter ./internal/appapi -v
go test -race ./internal/agentadapter ./internal/appapi
go test ./...
gofmt -l internal/agentadapter internal/appapi/agent_adapter_test.go
```

Expected:
- all tests PASS;
- race tests PASS;
- `gofmt -l` prints nothing.

- [ ] **Step 5: Commit documentation**

```bash
git add docs/v2-agent-tool-parity.md docs/superpowers/specs/2026-08-18-v2-agent-adapter-mcp-design.md
git commit -m "docs(v2): gate MCP tools on core parity"
```

---

## Completion Criteria For This Subproject

This plan is complete only when:

1. `internal/agentadapter` exists with no MCP/OpenClaw/DB/ledger dependency.
2. Exactly seven verified current capabilities are defined; no `household_id` appears in any input schema.
3. `Principal.HouseholdID` is injected server-side and cannot be overridden by arguments.
4. Strict JSON rejects unknown fields, malformed/trailing input, and scope-injection attempts before backend execution.
5. Result business payloads are direct serialization of Finance Core application results; metadata is copied without recalculation.
6. Deterministic parity tests compare all seven adapter paths to direct `appapi.API` results.
7. Race tests prove call-local household scope.
8. The full Go test suite remains green.
9. The target 12-tool contract has an explicit parity matrix; MCP work cannot silently advertise unwired capabilities.
10. No external endpoint is added yet, so V1 deployment/network/security behavior is unchanged.

## Subsequent V2 Subplans

After this boundary is green, continue in this order:

1. **Finance Tool Parity:** create deterministic app-level operations and signatures for the target rows still marked `CORE-PARITY-REQUIRED`; update the matrix only through parity tests.
2. **Agent Tool Audit:** migration `00007_agent_tool_audit.sql`, sqlc queries/store, canonical input/output hashing, fail-closed attempt/completion state machine, and `AuditID` population.
3. **MCP SDK/Transport:** select the newest stable official MCP Go SDK available at implementation time, register only `READY` tools, and test protocol/version behavior without hand-rolled MCP.
4. **MCP HTTP Security:** bearer credential file, fixed household principal, Origin allowlist, 262144-byte body limit, 15s timeout, concurrency 4, request rate 60/minute, controlled error mapping.
5. **Application/Edge/CI:** `/mcp` opt-in wiring, Caddy route without new ports, config/preflight/CI/govulncheck/race gates.
6. **OpenClaw Acceptance:** real Streamable HTTP probe and representative read/simulation calls with sanitized evidence.

Each subplan must remain independently reviewable and must not claim V2.0 release completion until the design exit criteria are all satisfied.
