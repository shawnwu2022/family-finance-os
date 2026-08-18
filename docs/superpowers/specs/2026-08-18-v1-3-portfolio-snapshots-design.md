# V1.3 Portfolio Current Asset Snapshots — Design

**Status:** approved under the project's standing auto-decision instruction  
**Base:** V2 automated candidate `64d3d814f319ecaa9f9e6dcfa0b8b97e32d615fb`  
**Branch:** `feature/v1-3-portfolio-snapshots`

## 1. Problem

The deterministic `portfolio.Summarize` primitive can already aggregate explicit valuations by asset class, but the live Finance Core application currently has only coarse ezBookkeeping account metadata. That is enough to prove cash/deposit classes, but a generic investment account cannot truthfully be decomposed into equity, fund, gold, fixed income, property, or other positions.

The current V2 `get_asset_allocation` therefore behaves conservatively: unambiguous cash/deposit accounts are classified directly, coarse investment/receivable/unknown accounts are returned as `other`, and unsupported cross-currency balances are omitted with partial-quality warnings. This is correct but intentionally incomplete.

V1.3 needs a small authoritative input surface that lets the household explicitly provide current asset facts without introducing a broker integration or market-data subsystem.

## 2. Decision

Implement **current explicit asset snapshots** as the first V1.3 Portfolio persistence layer.

Each snapshot states a stable asset reference, explicit asset class, current value in the household reporting currency, source currency, valuation timestamp, optional FX timestamp, and optional ezBookkeeping account reference. Finance Core treats these fields as user/importer-supplied facts; it does not infer a more specific class.

Snapshots are current-state records, not a price-history database. Instrument/Position/Market services may later become upstream producers of the same snapshot contract.

## 3. Alternatives Considered

### A. Account-level class overrides

Store a mapping from one ezBookkeeping investment account to one asset class.

**Rejected as the primary design:** cheap, but a mixed brokerage account would still be misclassified. It encodes an assumption rather than a position/asset fact.

### B. Current explicit asset snapshots — selected

Store one current authoritative valuation per stable asset reference, optionally linked to an ezBookkeeping account.

**Selected because:** it solves the current truthfulness gap, stays deterministic, supports property/gold/manual assets, requires no market feed, and creates a stable downstream contract for future Instrument/Position/Market producers.

### C. Full Instrument/Position/Price/FX stack now

Create instruments, fractional positions, price history, FX history, scheduled refresh, provider adapters, stale-price rules, and reconciliation in one phase.

**Deferred:** highest fidelity but unnecessarily large operational and precision surface for the current requirement. It would violate the project's preference for simple, reliable increments.

## 4. Data Model

Add one Finance Core table:

```text
portfolio_asset_snapshots
  household_id        BIGINT FK households(id)
  asset_ref           TEXT
  name                TEXT
  asset_class         TEXT
  value_minor         BIGINT >= 0
  currency            CHAR(3)
  source_currency     CHAR(3)
  valuation_as_of     TIMESTAMPTZ
  fx_as_of            TIMESTAMPTZ NULL
  source_account_ref  TEXT NULL
  source_kind         TEXT  -- manual | import
  created_at          TIMESTAMPTZ
  updated_at          TIMESTAMPTZ
  PRIMARY KEY (household_id, asset_ref)
```

Allowed `asset_class` values are exactly the existing Finance Core classes:

```text
cash
deposit
fixed_income
equity
fund
gold
property
other
```

Rules:

- `asset_ref` is household-scoped, stable, trimmed, and non-empty.
- `name` is trimmed and non-empty.
- `currency` and `source_currency` are uppercase ISO-style three-character codes.
- `value_minor` is never negative.
- `valuation_as_of` is required.
- if `source_currency != currency`, `fx_as_of` is required; Finance Core does not manufacture an FX timestamp.
- `source_account_ref` is optional and identifies the ezBookkeeping account whose coarse balance this snapshot replaces for allocation purposes.
- `source_kind` is deliberately limited to `manual` and `import` in this phase. Future market/broker producers can extend the enum in a migration when they exist.
- the table stores current state only: `PUT` replaces the current snapshot for `(household_id, asset_ref)` and updates `updated_at`.

No JSON metadata column is added. Unknown provider-specific state does not belong in the core contract.

## 5. Domain Contract

Add `portfolio.AssetSnapshot`:

```go
type AssetSnapshot struct {
    AssetRef          string
    Name              string
    Class             AssetClass
    Value             money.Money
    SourceCurrency    string
    ValuationAsOf     time.Time
    FXAsOf            *time.Time
    SourceAccountRef  string
    SourceKind        SnapshotSourceKind
}
```

Add validation that mirrors the database constraints and preserves the existing no-float money model.

Add a narrow repository interface consumed by `appapi`:

```go
type AssetSnapshotStore interface {
    ListAssetSnapshots(context.Context, int64) ([]portfolio.AssetSnapshot, error)
    UpsertAssetSnapshot(context.Context, int64, portfolio.AssetSnapshot) (portfolio.AssetSnapshot, error)
    DeleteAssetSnapshot(context.Context, int64, string) error
}
```

The PostgreSQL implementation lives with other Finance Core persistence adapters and is generated from sqlc queries.

## 6. Allocation Merge Semantics

`appapi.API.AssetAllocation` remains the single typed application operation used by Agent/MCP.

For each call:

1. Load the household profile and base/reporting currency.
2. Load explicit snapshots from `AssetSnapshotStore` when configured.
3. Validate each snapshot; invalid persisted data fails closed as data unavailable rather than being guessed around.
4. Include only snapshots whose `Value.Currency` equals the household base currency.
   - A non-base snapshot is omitted and quality becomes `partial`.
   - No FX conversion is introduced here.
5. Record every non-empty `SourceAccountRef` from an included snapshot as **covered**.
6. Load ezBookkeeping accounts.
7. Skip hidden accounts, multi-subaccount containers, non-assets, and liabilities as today.
8. Skip a ledger account entirely when its account ID is covered by one or more included explicit snapshots. This prevents double counting.
9. For uncovered ledger accounts, retain the existing conservative fallback:
   - cash/checking/virtual -> `cash`
   - savings/CD -> `deposit`
   - investment/receivables/unknown -> `other` + partial warning
   - foreign-currency account -> omit + partial warning
10. Feed the merged valuation set into the existing deterministic `portfolio.Summarize` function.
11. Sort class output deterministically and preserve existing quality/warning behavior.

### Why an explicit linked snapshot replaces the whole ledger account

A source account can contain several snapshots; all of them are summed by `portfolio.Summarize`. Once the household/importer claims explicit assets for that account, combining those position-level values with the coarse account balance would double count the same economic assets.

This phase intentionally does **not** require the sum of linked snapshots to equal the ledger account balance. Market movement, brokerage cash, stale valuation times, and provider semantics can make equality inappropriate. Reconciliation belongs in the later Data Quality/Portfolio integration phase.

## 7. HTTP/Application Management Surface

Follow the existing Finance HTTP convention where household scope is supplied by the `household_id` query parameter and resource identity is in the path:

```text
GET    /api/v1/portfolio/assets?household_id=42
PUT    /api/v1/portfolio/assets/{asset_ref}?household_id=42
DELETE /api/v1/portfolio/assets/{asset_ref}?household_id=42
```

`PUT` is idempotent and replaces the current snapshot for the path `asset_ref` within the scoped household.

Request body:

```json
{
  "name": "沪深300 ETF",
  "asset_class": "fund",
  "value_minor": "1250000",
  "currency": "CNY",
  "source_currency": "CNY",
  "valuation_as_of": "2026-08-18T12:00:00Z",
  "source_account_ref": "brokerage-1",
  "source_kind": "manual"
}
```

`value_minor` is JSON-string encoded, matching existing `MoneyDTO` precision rules.

The body never contains `household_id`; handlers reuse the existing `parseHouseholdID` query-scope helper. The server passes that validated scope to the typed Finance API.

`DELETE` is idempotent and returns `204 No Content` whether or not the scoped asset existed. This avoids leaking cross-household existence and keeps retry behavior simple.

## 8. Agent/MCP Contract

No new Agent/MCP tool is added in this phase.

`get_asset_allocation` automatically becomes more precise because it calls the same `appapi.API.AssetAllocation` operation. Agent/MCP still cannot mutate portfolio facts.

This preserves the read/simulation-only V2 MCP security boundary and avoids expanding remote mutation capability before a dedicated authorization model exists.

## 9. UI Scope

Backend CRUD is required in this phase because a snapshot store without an input surface is not usable.

A dashboard editor is a separate follow-on slice. The existing web/PWA can continue displaying allocation through the current Finance Core reporting surfaces; a richer portfolio-management screen should be added only after the backend contract is stable and tested.

## 10. Quality and Security

- no asset class inference beyond explicit snapshot facts and the existing deterministic ledger fallback;
- no market-price or FX network call;
- no float monetary arithmetic;
- no raw brokerage credential or token persistence;
- household ID always participates in the primary key/query predicate;
- API validation rejects unsupported classes, negative values, invalid currencies, zero valuation timestamp, invalid source kind, or foreign source currency without `fx_as_of`;
- Agent/MCP remains read-only and receives only typed allocation results;
- existing audit/security gates remain unchanged.

## 11. Operations

One additive PostgreSQL migration and sqlc query set only.

No new process, sidecar, host port, queue, cache, scheduled job, credential, third-party provider, or backup target is introduced. Existing database backup/restore automatically covers the table.

## 12. Acceptance Criteria

The slice is complete when all of the following are proven on one exact branch head:

1. migration up/down/up passes;
2. sqlc generated sources are committed and reproducible;
3. domain validation rejects invalid snapshots;
4. PostgreSQL store is household-isolated and CRUD-tested;
5. HTTP CRUD validates scope and body contract;
6. linked explicit snapshots replace their coarse ledger account without double counting;
7. unlinked explicit assets such as property are included;
8. uncovered coarse investment accounts remain `other` with partial warnings;
9. cross-currency snapshots/accounts are not silently converted;
10. direct `appapi.API.AssetAllocation` and Agent Adapter output remain byte-equivalent at the typed business-payload level;
11. all existing 12 MCP tools and security contracts remain unchanged;
12. full `CI`, `Edge Security`, and `MCP Security` pass on the exact candidate commit.

## 13. Deferred Work

- instrument master and identifiers;
- fractional quantity/cost basis;
- market-price history;
- FX history/conversion engine;
- automatic broker/provider ingestion;
- valuation refresh scheduler;
- account-vs-position reconciliation;
- drift/rebalancing advice;
- tax lots;
- historical portfolio `as_of` queries;
- write-capable Agent/MCP portfolio tools.
