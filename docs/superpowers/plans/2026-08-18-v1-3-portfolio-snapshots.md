# V1.3 Portfolio Current Asset Snapshots Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Use superpowers:test-driven-development for every behavior change.

**Goal:** Add household-scoped explicit current asset snapshots that improve deterministic asset allocation without introducing broker, market-price, FX-conversion, background-worker, or write-capable MCP complexity.

**Architecture:** PostgreSQL stores one current explicit snapshot per `(household_id, asset_ref)`. A narrow snapshot store is injected into `appapi.API`. `API.AssetAllocation` merges explicit snapshot valuations with existing conservative ezBookkeeping account fallback, excluding covered linked accounts to avoid double counting, then delegates totals/shares to `portfolio.Summarize`. Finance HTTP exposes authenticated CRUD using the existing `household_id` query convention. Agent/MCP remains read/simulation-only and automatically benefits through the existing typed allocation operation.

**Tech Stack:** Go 1.26.6, PostgreSQL, goose, sqlc, pgx/v5, existing Finance Core packages, official MCP Go SDK v1.6.1, existing `net/http` server.

**Spec:** `docs/superpowers/specs/2026-08-18-v1-3-portfolio-snapshots-design.md`

## Global Constraints

- Never use float money arithmetic.
- Never infer equity/fund/gold/fixed-income/property from a generic account.
- Never perform network market-price or FX requests in this slice.
- Every persistence query is household-scoped.
- Explicit linked snapshots replace the corresponding coarse ledger account for allocation only; they are never added on top of that account balance.
- Agent/MCP adds no write tool and remains exactly twelve read/simulation tools.
- No new process, sidecar, queue, host port, cache, credential, or third-party dependency.
- Preserve V2 exact-head security behavior and default `MCP_ENABLED=false`.

---

## Task 1: Define and validate the current snapshot domain contract

**Files:**
- Modify: `internal/portfolio/models.go`
- Create: `internal/portfolio/snapshot_test.go`
- Create: `internal/portfolio/snapshot.go`

**Interfaces:**

```go
type SnapshotSourceKind string

const (
    SnapshotSourceManual SnapshotSourceKind = "manual"
    SnapshotSourceImport SnapshotSourceKind = "import"
)

type AssetSnapshot struct {
    AssetRef         string
    Name             string
    Class            AssetClass
    Value            money.Money
    SourceCurrency   string
    ValuationAsOf    time.Time
    FXAsOf           *time.Time
    SourceAccountRef string
    SourceKind       SnapshotSourceKind
}

func ValidateAssetSnapshot(snapshot AssetSnapshot) error
```

- [ ] **Step 1: RED — write table-driven validation tests**

Cover valid manual/import snapshots and rejection of: blank ref/name, unsupported class, negative value, malformed/blank currency, malformed source currency, zero valuation timestamp, unsupported source kind, and foreign source currency without `FXAsOf`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/portfolio -run TestValidateAssetSnapshot -v`

Expected: compile failure because the snapshot types/validator do not exist.

- [ ] **Step 3: GREEN — implement only the domain types and validator**

Trim textual identity fields before validation at mutation boundaries; the pure validator itself must reject malformed facts and perform no conversion/inference.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/portfolio -v`

- [ ] **Step 5: Commit**

Commit: `feat(portfolio): define explicit asset snapshots`

---

## Task 2: Add PostgreSQL persistence with household isolation

**Files:**
- Create: `db/migrations/00008_portfolio_asset_snapshots.sql`
- Create: `db/queries/portfolio.sql`
- Modify generated sqlc files under: `internal/store/sqlc/`
- Create: `internal/portfolio/postgres_store.go`
- Create: `internal/portfolio/postgres_store_integration_test.go`

**Schema:**

```sql
CREATE TABLE portfolio_asset_snapshots (
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    asset_ref TEXT NOT NULL CHECK (length(btrim(asset_ref)) > 0),
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    asset_class TEXT NOT NULL CHECK (asset_class IN ('cash','deposit','fixed_income','equity','fund','gold','property','other')),
    value_minor BIGINT NOT NULL CHECK (value_minor >= 0),
    currency CHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    source_currency CHAR(3) NOT NULL CHECK (source_currency ~ '^[A-Z]{3}$'),
    valuation_as_of TIMESTAMPTZ NOT NULL,
    fx_as_of TIMESTAMPTZ,
    source_account_ref TEXT,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('manual','import')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (household_id, asset_ref),
    CHECK (source_currency = currency OR fx_as_of IS NOT NULL)
);
```

Add a deterministic list index on `(household_id, asset_class, asset_ref)` if query planning benefits; the primary-key prefix already handles household lookups, so do not add redundant indexes without evidence.

**Queries:**
- `ListPortfolioAssetSnapshotsByHousehold`
- `UpsertPortfolioAssetSnapshot`
- `DeletePortfolioAssetSnapshot`

Every mutation predicate includes `household_id`.

- [ ] **Step 1: RED — integration tests first**

Tests must prove: create/upsert round-trip, stable ordering by `asset_ref`, same `asset_ref` can exist in two households, updating household A cannot touch household B, delete is household-scoped, invalid DB facts are rejected by constraints, and timestamps/nullable refs round-trip.

- [ ] **Step 2: Verify RED**

Run the repository's PostgreSQL integration command for the new test. Expected failure: missing migration/query/store.

- [ ] **Step 3: GREEN — migration, sqlc query definitions, generated code, store mapping**

`portfolio.PostgresStore` must call `ValidateAssetSnapshot` on writes and validate decoded rows on reads. Invalid persisted facts fail closed.

- [ ] **Step 4: Verify migration/sqlc/store**

Run:

```bash
go generate ./...
go test ./internal/portfolio -v
```

and the repository PostgreSQL integration target used by CI. Verify migration up/down/up and committed generated sources.

- [ ] **Step 5: Commit**

Commit: `feat(portfolio): persist current asset snapshots`

---

## Task 3: Add typed App API snapshot CRUD

**Files:**
- Modify: `internal/appapi/api.go`
- Create: `internal/appapi/portfolio_snapshots.go`
- Create: `internal/appapi/portfolio_snapshots_test.go`
- Modify: `internal/server/finance_tools.go` or create `internal/server/portfolio_snapshots.go` for DTOs

**Dependencies:**

```go
type AssetSnapshotStore interface {
    ListAssetSnapshots(context.Context, int64) ([]portfolio.AssetSnapshot, error)
    UpsertAssetSnapshot(context.Context, int64, portfolio.AssetSnapshot) (portfolio.AssetSnapshot, error)
    DeleteAssetSnapshot(context.Context, int64, string) error
}
```

Add optional `Portfolio AssetSnapshotStore` to `appapi.Dependencies`. Existing V2 read behavior remains available when the store is nil; production wiring in Task 6 supplies the PostgreSQL store.

**Typed API operations:**

```go
ListPortfolioAssets(ctx context.Context, householdID int64) (server.PortfolioAssetsResponse, error)
UpsertPortfolioAsset(ctx context.Context, householdID int64, assetRef string, request server.PortfolioAssetUpsertRequest) (server.PortfolioAssetResponse, error)
DeletePortfolioAsset(ctx context.Context, householdID int64, assetRef string) error
```

- [ ] **Step 1: RED — typed API tests**

Prove trim/canonicalization, path `asset_ref` authority, string-encoded money mapping, stable list ordering, validation errors before persistence, scoped store calls, and idempotent delete semantics.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/appapi -run 'Test.*PortfolioAsset' -v`

- [ ] **Step 3: GREEN — implement minimal typed API/DTO mapping**

Do not add any financial calculation here.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/appapi ./internal/server`

- [ ] **Step 5: Commit**

Commit: `feat(appapi): manage portfolio asset snapshots`

---

## Task 4: Merge explicit snapshots into deterministic asset allocation

**Files:**
- Modify: `internal/appapi/asset_allocation.go`
- Modify: `internal/appapi/asset_allocation_test.go`
- Modify: `internal/appapi/agent_adapter_asset_allocation_test.go`

- [ ] **Step 1: RED — allocation merge tests**

Add fixtures proving:

1. two explicit snapshots linked to one investment account produce their explicit `equity`/`fund` classes;
2. the linked coarse investment account balance is omitted so there is no double counting;
3. an unlinked property snapshot is included;
4. uncovered investment remains `other` + partial warning;
5. non-base explicit snapshot is omitted + partial warning;
6. a snapshot linked to an account is considered covering only when that snapshot itself is included;
7. hidden/liability/container fallback behavior remains unchanged;
8. sorted output and exact `portfolio.Summarize` shares/totals remain deterministic.

Extend direct Agent Adapter parity to use explicit snapshots and prove no adapter-side recalculation.

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/appapi -run 'TestAssetAllocation|TestAgentAdapterAssetAllocation' -v
```

Expected: explicit snapshots are ignored before implementation.

- [ ] **Step 3: GREEN — merge snapshots before ledger fallback**

Do not change `portfolio.Summarize`. Build one merged `[]portfolio.Valuation`, track covered source account refs, then delegate once.

- [ ] **Step 4: Verify GREEN + race on affected packages**

Run:

```bash
go test ./internal/portfolio ./internal/appapi ./internal/agentadapter ./internal/mcpadapter
go test -race ./internal/portfolio ./internal/appapi ./internal/agentadapter ./internal/mcpadapter
```

- [ ] **Step 5: Commit**

Commit: `feat(portfolio): merge explicit snapshots into allocation`

---

## Task 5: Expose authenticated Finance HTTP CRUD

**Files:**
- Modify: `internal/server/api.go`
- Modify/create: `internal/server/api_test.go` or a focused `internal/server/portfolio_assets_test.go`
- Modify any test fake implementing `server.FinanceAPI`

**Routes:**

```text
GET    /api/v1/portfolio/assets?household_id=42
PUT    /api/v1/portfolio/assets/{asset_ref}?household_id=42
DELETE /api/v1/portfolio/assets/{asset_ref}?household_id=42
```

- [ ] **Step 1: RED — HTTP contract tests**

Prove: missing/invalid household query -> 400; unknown JSON fields -> 400; over-limit body -> 400/413 according to existing helper behavior; invalid/missing asset ref -> 400; valid PUT uses path ref not body-selected identity; GET returns stable typed JSON; DELETE returns 204 and empty body; no cache; backend errors remain sanitized.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/server -run 'Test.*Portfolio' -v`

- [ ] **Step 3: GREEN — extend `FinanceAPI` and route registration**

Reuse `parseHouseholdID`, `decodeStrictJSON`, and existing response hardening. Do not create a parallel HTTP stack.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/server -v`

- [ ] **Step 5: Commit**

Commit: `feat(server): expose portfolio snapshot CRUD`

---

## Task 6: Wire PostgreSQL store into the application

**Files:**
- Modify: `cmd/finance-core/application.go`
- Modify: `cmd/finance-core/application_test.go`
- Modify relevant PostgreSQL/app integration tests
- Modify any `householdScopedAPI`/reporting wrapper compile-time fakes as required by the expanded `FinanceAPI`

- [ ] **Step 1: RED — application integration test**

Build the real handler against PostgreSQL, upsert one linked portfolio snapshot through HTTP, retrieve it, and prove the Finance API allocation/MCP read path sees the explicit class while MCP still exposes exactly twelve tools and no mutation tool.

- [ ] **Step 2: Verify RED**

Run the application builder/integration target. Expected failure before store wiring.

- [ ] **Step 3: GREEN — production wiring**

Construct one `portfolio.NewPostgresStore(pool)` and inject it into `appapi.Dependencies`. Do not add config switches or credentials.

- [ ] **Step 4: Verify GREEN**

Run application builder, app API, MCP startup, and PostgreSQL integration tests.

- [ ] **Step 5: Commit**

Commit: `feat(app): wire portfolio snapshot store`

---

## Task 7: Release contract and exact-head verification

**Files:**
- Modify: `docs/09-roadmap.md`
- Modify: `docs/v2-agent-tool-parity.md` only if semantics text needs to describe explicit snapshot precedence
- Modify: `docs/v2-mcp-security-acceptance.md` only if acceptance wording needs the stronger allocation semantics; do not relax the live OpenClaw gate
- Add a focused V1.3 acceptance note if needed

- [ ] **Step 1: Update documentation**

Mark V1.3 step 1/current explicit snapshot layer implemented while keeping Instrument/Position, market prices, risk exposure, and rebalancing deferred. Document that V2 remains twelve tools and read-only.

- [ ] **Step 2: Run full exact-head verification**

Required on the final candidate SHA:

- `CI` success: migration up/down/up, generated sqlc clean, formatting, vet, full tests, MCP adapter, `govulncheck`, all integrations, race, binary/frontend/container builds;
- `Edge Security` success;
- `MCP Security` success;
- PR remains mergeable;
- V2 PR #20 head remains unchanged and retains its own prior all-green evidence.

- [ ] **Step 3: Review changed files against the design**

Confirm no market/FX network dependency, no extra process/port/queue/cache, no new MCP write tool, no hidden float money arithmetic, and no household-unscoped SQL mutation.

- [ ] **Step 4: Open a stacked Draft PR**

Base it on `feature/v2-agent-adapter` while PR #20 remains unmerged. State explicitly that it must not be merged before its base branch and that it does not change the V2 live OpenClaw production gate.

- [ ] **Step 5: Keep branch/PR for follow-up UI slice**

Do not merge automatically while the upstream V2 production gate is still pending real OpenClaw/deployed-endpoint acceptance.
