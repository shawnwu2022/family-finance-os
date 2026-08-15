# Go-first Technical Baseline Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the disposable Python `/healthz` skeleton with a minimal Go-first Finance Core baseline and make repository documentation, Docker Compose, CI, and the V1 implementation plan consistent with the final architecture.

**Architecture:** Finance Core becomes a Go modular monolith. This migration deliberately implements only runtime/health scaffolding and repository tooling; PostgreSQL persistence, pgx/sqlc/goose, financial domain logic, Vue UI, and AI tools remain subsequent V1 tasks. Production builds use Go 1.26.6; the runtime image is non-root and contains no Python/Node runtime.

**Tech Stack:** Go 1.26.6, `net/http`, `log/slog`, PostgreSQL 18.6, ezBookkeeping 1.6.1, Caddy 2.11.4, Docker Compose; planned V1 data stack `pgx/v5 + sqlc + goose + apd/v3`; planned frontend Vue 3 + TypeScript + Vite + PWA.

## Global Constraints

- Keep V1 to four long-running containers: Caddy, PostgreSQL, ezBookkeeping, Finance Core.
- No Kubernetes, Redis, Kafka, MinIO, vector DB, microservices, HA, OpenClaw critical path, or P40 critical path.
- No business feature is added in this migration.
- Finance Core must build as one Go binary and use Go standard library HTTP routing at baseline.
- The health endpoint contract remains `GET /healthz -> 200 {"service":"finance-core","status":"ok"}`.
- Production Finance Core runs as non-root.
- All Python Finance Core files are removed after the Go baseline passes tests.
- Documentation must contain a single authoritative V1 stack; no live Python-Core instructions may remain.

---

### Task 1: Record the final architecture baseline

**Files:**
- Create: `docs/adr/0007-go-finance-core.md`
- Modify: `README.md`
- Modify: `PROJECT_PLAN.md`
- Modify: `docs/01-architecture.md`
- Modify: `docs/07-operations.md`
- Modify: `docs/08-testing-acceptance.md`
- Modify: `docs/11-research-baseline-2026-08-15.md`
- Modify: `docs/superpowers/plans/2026-08-15-v1-implementation-plan.md`

**Interfaces:**
- Declares Go as the Finance Core source language.
- Declares Python only as a future optional AI/OCR worker language.
- Declares Rust out of V1.
- Declares `pgx/v5 + sqlc + goose + apd/v3` for later V1 persistence/decimal tasks.

- [x] Update all listed docs to the same architecture and version baseline.
- [x] Search the repository for stale `FastAPI|Pydantic|SQLAlchemy|Alembic|pytest|Python 3.13` references and retain them only where describing historical decisions or future Python AI workers.
- [x] Verify no contradictory live implementation instruction remains.

### Task 2: RED — define the Go HTTP health contract

**Files:**
- Create: `internal/server/server_test.go`

**Interfaces:**
- Future `server.NewHandler() http.Handler`.
- `GET /healthz` returns status 200, JSON content type, and exact service/status fields.

- [x] Write the failing handler test before production Go source exists.
- [x] Run `go test ./...` and confirm failure is due to missing `internal/server` implementation.

### Task 3: GREEN — implement minimal Go Finance Core runtime

**Files:**
- Create: `go.mod`
- Create: `cmd/finance-core/main.go`
- Create: `internal/server/server.go`

**Interfaces:**
- `server.NewHandler() http.Handler`.
- CLI mode `serve` starts HTTP server on `FINANCE_LISTEN_ADDR`, default `:8000`.
- CLI mode `healthcheck` performs a local HTTP GET against `http://127.0.0.1:8000/healthz` and exits non-zero on failure.

- [x] Implement the minimum code needed for the RED test to pass.
- [x] Run `go test ./...` and confirm GREEN.
- [x] Run `gofmt` and re-run the tests.

### Task 4: Replace Python build/runtime scaffolding

**Files:**
- Delete: `apps/finance-core/`
- Create: `Dockerfile`
- Modify: `compose.yaml`
- Modify: `Makefile`
- Modify: `.gitignore`
- Modify: `.env.example`

**Interfaces:**
- Docker builder uses `golang:1.26.6`.
- Final image runs `/finance-core serve` as non-root.
- Compose healthcheck uses `/finance-core healthcheck`; no Python runtime is referenced.
- `make test` runs `go test ./...`.

- [x] Apply build/runtime changes only after Go unit tests are GREEN.
- [x] Run static shell/YAML checks available in the environment.
- [x] Confirm no tracked Python Finance Core file remains.

### Task 5: Establish developer-tool configuration without adding runtime services

**Files:**
- Create: `sqlc.yaml`
- Create: `db/migrations/.gitkeep`
- Create: `db/queries/.gitkeep`
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- sqlc configured for PostgreSQL and future `pgx/v5` generation into `internal/store/sqlc`.
- CI uses Go 1.26.6 and runs `gofmt`, `go vet`, `go test`, `go test -race` and Docker build.
- sqlc/goose are development/build tools; they are not long-running containers.

- [x] Add configuration only; do not invent domain schema in this migration.
- [x] Validate workflow YAML syntax.

### Task 6: Final verification and publication

**Files:** all migration files.

- [x] Run `go test ./...` with the locally available toolchain before the final Go-version directive update if necessary.
- [x] Set the repository production baseline to Go 1.26.6.
- [x] Run all locally available static checks.
- [x] Confirm `git diff --check` is clean.
- [x] Confirm stale Python Core implementation files/references are removed.
- [ ] Commit the migration on `feature/go-baseline`.
- [ ] Publish the branch to GitHub and open one draft PR against `main`.
- [ ] Use GitHub Actions as the authoritative target-toolchain verification for Go 1.26.6 and Docker build; fix any failures before marking the migration ready.
