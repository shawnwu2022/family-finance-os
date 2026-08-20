# V1.3 Portfolio Snapshot Acceptance

## Scope

This acceptance note covers the V1.3 **current asset snapshot** slice only. It does not claim completion of a broker integration, market-data service, instrument/position model, risk engine, rebalancing engine, or tax-lot subsystem.

The implemented contract is:

- one explicit current snapshot per `(household_id, asset_ref)`;
- deterministic asset classes: `cash`, `deposit`, `fixed_income`, `equity`, `fund`, `gold`, `property`, `other`;
- non-negative integer-minor-unit value in an explicit reporting currency;
- source currency and valuation timestamp, with explicit FX timestamp required when source currency differs from stored value currency;
- optional linked ledger account reference;
- manual/import source kind;
- household-scoped PostgreSQL persistence and Finance HTTP CRUD;
- deterministic merge into `get_asset_allocation` without adapter-side recalculation.

## Deterministic allocation rule

`API.AssetAllocation` treats an explicit snapshot as an already-valued reporting-currency fact. An included snapshot with `SourceAccountRef` covers the corresponding coarse ledger account, so the account balance is not added a second time.

If a snapshot value is not in the household base/reporting currency, it is omitted and the result becomes `partial`; that omitted snapshot does **not** cover its linked ledger account. No implicit market-price lookup or FX conversion occurs.

Ledger accounts not covered by an included snapshot retain the conservative fallback:

- cash/checking/virtual -> `cash`;
- savings/certificate-of-deposit -> `deposit`;
- coarse investment/receivable/unknown categories -> `other` with partial-quality warnings;
- liabilities, hidden accounts, and aggregate/container accounts are excluded;
- unsupported cross-currency ledger balances are omitted rather than guessed.

Totals and shares are delegated once to `portfolio.Summarize`.

## HTTP contract

Finance Core exposes:

```text
GET    /api/v1/portfolio/assets?household_id=<id>
PUT    /api/v1/portfolio/assets/{asset_ref}?household_id=<id>
DELETE /api/v1/portfolio/assets/{asset_ref}?household_id=<id>
```

The routes reuse the existing household query convention and HTTP hardening: strict JSON, bounded request bodies, `no-store`, sanitized backend errors, path-controlled `asset_ref`, and idempotent `204 No Content` delete semantics.

## MCP / Agent boundary

V1.3 adds no MCP mutation tool. The V2 allowlist remains exactly twelve read/simulation tools. `get_asset_allocation` automatically benefits from the stronger Finance Core semantics through typed API parity; the Agent Adapter performs no portfolio calculation.

The real OpenClaw/deployed-endpoint gate in `docs/v2-mcp-security-acceptance.md` is unchanged and remains required separately before V2.0 production release.

## Verified implementation checkpoints

| Task | Exact head | Evidence |
|---|---|---|
| 1. Domain snapshot contract | `45260223b92611e3db6f874837aade1d699c8eeb` | CI, MCP Security, Edge Security all successful |
| 2. PostgreSQL persistence | `23c3c8b0f816d700236868d132b6a2960801f058` | CI, MCP Security, Edge Security successful; migration up/down/up and real PostgreSQL portfolio integration included |
| 3. Typed App API CRUD + repository-native verification baseline | `c22c26b1e5467860c864fe7877cc2c02346ca3d5` | CI `32217377805`, MCP Security `32217377913`, Edge Security `32217377801` successful |
| 4. Snapshot allocation merge + Agent parity | `92e557a1b47a3c68173248dadf6cb1dc60741cb8` | CI `32220034271`, MCP Security `32220034239`, Edge Security `32220034241` successful |
| 5. Finance HTTP CRUD | `28c036b6ee75e5c94b218d1fec53667597c500f1` | CI `32220871307`, MCP Security `32220871304`, Edge Security `32220871478` successful |
| 6. Production PostgreSQL/application/MCP wiring | `c6e6df090b90c66877c3951786d992b4ce808daf` | CI `32222330298`, MCP Security `32222330281`, Edge Security `32222330293` successful |

Task 6's application integration creates a real PostgreSQL household/profile, writes and reads an explicit snapshot through the production HTTP handler, connects with the official MCP Streamable HTTP client, confirms the allowlist remains exactly twelve tools, and verifies `get_asset_allocation` sees the persisted explicit property allocation.

## Final candidate rule

The final PR head, including release-documentation changes, must again have successful `CI`, `MCP Security`, and `Edge Security` runs before this slice is considered exact-head verified. PR metadata should record those final run IDs so the evidence does not require a self-referential documentation commit.

## Explicitly deferred

The following remain outside this slice and require separate entry criteria:

- Instrument/Position quantities and identifiers;
- broker/exchange integrations;
- market-price polling or streaming;
- FX feed or implicit currency conversion;
- risk/exposure analytics that require position metadata;
- rebalancing recommendations;
- cost-basis/tax-lot accounting;
- background workers, queues, caches, additional services, or new host ports.
