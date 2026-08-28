# V1.x Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the defined V1.1, V1.2, V1.3, and V1.4 scope without expanding Family Finance OS into a bank-clearing, brokerage, or model-hosting platform.

**Architecture:** Keep Finance Core as the modular Go monolith. V1.1 adds a deterministic read-only data-quality analyzer over the existing ledger port; V1.2 formalizes the already-supported OpenAI-compatible loopback path as an explicit local-AI mode plus a reproducible benchmark gate; V1.3 remains the already-shipped explicit asset snapshot layer; V1.4 extends application-native auth with one-household `owner/editor/viewer` roles enforced server-side. No Redis, extra finance datastore, second authorization engine, or duplicate business logic is introduced.

**Tech Stack:** Go, PostgreSQL migrations, existing ezBookkeeping ledger adapter, existing Vue 3/TypeScript PWA, existing OpenAI-compatible LLM provider, GitHub Actions/repository-native `make verify`.

**Spec:** `docs/09-roadmap.md`

## Global Constraints

- Finance Core remains the single authority for deterministic finance calculations and authorization.
- AI receives only data already filtered by Finance Core.
- V1.1 candidate detection is advisory/read-only; it never mutates ledger transactions automatically.
- V1.2 local AI must not weaken the existing external-LLM HTTPS and redirect-downgrade protections.
- V1.3 completion is the explicit current-asset snapshot layer already in `main`; instrument/position, market-feed, risk, rebalancing and tax-lot work are not V1 release blockers.
- V1.4 is household RBAC, not multi-tenant SaaS: each auth user belongs to exactly one household.
- Existing application-native password + mandatory TOTP + Session + CSRF boundaries remain mandatory.

---

### Task 1: V1.1 deterministic data-quality analyzer

**Files:**
- Create: `internal/dataquality/analyzer.go`
- Create: `internal/dataquality/analyzer_test.go`
- Create: `internal/server/data_quality.go`
- Modify: `internal/appapi/api.go`
- Modify: `internal/server/api.go`
- Test: `internal/dataquality/analyzer_test.go`
- Test: `internal/appapi/api_test.go`

**Interfaces:**
- Consumes: `ledger.Account`, `ledger.Category`, `ledger.Transaction` and the existing `ledger.Ledger` port.
- Produces: deterministic `dataquality.Report` with reference-integrity findings and duplicate candidate groups; `GET /api/v1/data-quality?period=YYYY-MM`.

- [ ] Write analyzer tests covering exact duplicate candidates, time-window exclusion, missing account/category references, transfer destination validation and hidden amounts.
- [ ] Verify the tests fail before the analyzer exists.
- [ ] Implement the analyzer with stable ordering and no automatic ledger mutation.
- [ ] Add the HTTP response DTO and app API method using household timezone period bounds.
- [ ] Add route/server tests and run repository-native verification.
- [ ] Update Roadmap V1.1 status only after exact-head checks pass.

### Task 2: V1.2 local-AI product mode and benchmark gate

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/llm/openai_compatible.go`
- Modify: `cmd/finance-core/main.go`
- Create: `scripts/acceptance/local-ai-benchmark.sh`
- Create: `scripts/acceptance/test-local-ai-contract.sh`
- Modify: `.env.example`
- Modify: `scripts/test-production-ops.sh`
- Test: `internal/llm/openai_compatible_test.go`
- Test: config tests under `internal/config`

**Interfaces:**
- Produces explicit local provider mode using an OpenAI-compatible loopback/private endpoint and exact model IDs; preserves the external-provider HTTPS policy.
- Produces a hardware-independent benchmark runner that records latency/model/schema/tool-grounding metadata without secrets. Actual P40 numbers remain a hardware acceptance item.

- [ ] Add failing configuration/transport tests for explicit local mode and external-mode rejection of plaintext endpoints.
- [ ] Implement the minimal provider-mode configuration without adding a second LLM client stack.
- [ ] Add benchmark contract tests, then the benchmark runner.
- [ ] Run full CI/security/OpenClaw gates and update Roadmap V1.2 engineering status.

### Task 3: V1.3 completion audit

**Files:**
- Review: `internal/portfolio/*`
- Review: `internal/appapi/asset_allocation.go`
- Review: `web/src/components/PortfolioPanel.vue`
- Modify: `docs/09-roadmap.md` only if the audit confirms current scope.

**Interfaces:**
- Existing explicit `(household_id, asset_ref)` snapshots remain the V1.3 contract.

- [ ] Re-run existing portfolio unit/integration/API/PWA tests as part of `make verify` after the surrounding V1.x changes.
- [ ] Confirm no double-count regression and no implicit cross-currency valuation.
- [ ] Keep instrument/position, market-feed, risk, rebalancing and tax-lot work outside the V1 blocker set.

### Task 4: V1.4 household RBAC

**Files:**
- Create: `db/migrations/00011_household_rbac.sql`
- Modify: `internal/auth/postgres_store.go`
- Modify: `internal/auth/service.go`
- Modify: `internal/server/auth.go`
- Modify: `internal/server/api.go`
- Create: `internal/server/rbac_security_test.go`
- Add owner-only member-management service/API files as focused modules instead of enlarging `api.go` further.
- Modify frontend auth/session types and add a small household-member management surface for owners.

**Interfaces:**
- Roles: `owner`, `editor`, `viewer`.
- Owner: read/write finance plus member/role management.
- Editor: read/write finance; no member/role management.
- Viewer: read-only finance; no state-changing finance APIs and no member management.
- Existing bootstrap admin is migrated/defaulted to `owner`.

- [ ] Add RED migration/store/session tests proving role persistence and default owner behavior.
- [ ] Add RED HTTP security tests proving viewers cannot mutate and only owners can manage members.
- [ ] Implement role storage and propagate role into `SessionIdentity`.
- [ ] Enforce authorization centrally before application handlers run; do not duplicate role checks across every domain service.
- [ ] Implement owner-only create/list/update/disable household-member APIs with password hashing and mandatory TOTP enrollment on each member’s first login.
- [ ] Add minimal owner UI for household member administration.
- [ ] Run migration up/down/up, auth security, race, PWA, CI, Edge, MCP and OpenClaw gates.

### Task 5: V1.x closeout

**Files:**
- Modify: `docs/09-roadmap.md`
- Modify: `docs/acceptance/v1-production-evidence.md`
- Modify: GitHub Issue #26 governance checklist.

- [ ] Mark V1.1, V1.2 engineering, V1.3, and V1.4 engineering complete only with exact-head verification evidence.
- [ ] Keep actual P40 benchmark, real-host deployment, real statement/month reconciliation, real external Advisor, real DR and real-phone gates as `NOT RUN` until executed in their real environments.
- [ ] Select the new validated V1 runtime target after all runtime-affecting V1.x PRs merge and refresh the evidence ledger without promoting unexecuted real-world gates.
