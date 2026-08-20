# Repository-Native Verification Design

## Context

Family Finance OS currently stores most verification logic inside GitHub Actions workflows. This made GitHub-hosted runner capacity a de facto prerequisite for progressing development. On 2026-08-18 the account exhausted its 2,000 included Actions minutes and jobs began failing before checkout, so repository code could no longer be verified even though the code itself had not executed.

The project already keeps application/runtime behavior in the repository. Verification must follow the same rule: GitHub Actions may trigger verification, but it must not own the verification logic or be required to run it.

## Goals

1. Make the complete quality gate runnable from any Linux host with Docker Engine and Docker Compose using one command: `make verify`.
2. Pin the Go, Node, PostgreSQL, sqlc, goose, and govulncheck toolchain versions in repository-owned configuration.
3. Preserve the checks already enforced by CI, including database migrations, restore drill, integration tests, race tests, security contracts, frontend checks, and container build.
4. Keep GitHub Actions as an optional manual mirror that delegates to repository targets instead of containing duplicated check logic.
5. Avoid adding Jenkins, Kubernetes, a queue, an always-on CI control plane, or new credentials.

## Non-goals

- Replace GitHub as the source repository.
- Introduce a production deployment orchestrator.
- Add a self-hosted GitHub runner as a mandatory component.
- Change Finance Core, MCP, portfolio, ledger, or HTTP behavior.
- Relax any existing verification or security gate.

## Canonical verification interface

The repository exposes the following stable commands:

- `make verify` — complete verification gate.
- `make verify-contract` — fast structural verification of the repository-native CI contract; does not require Docker.
- `make verify-go` — Go/tooling/database/application checks inside the pinned Go verification image.
- `make verify-web` — frontend checks inside the pinned Node image.
- `make verify-mcp-security` — MCP security contract checks inside the pinned Go verification image.
- `make verify-edge-security` — static and runtime edge/Caddy checks using Docker Compose.
- `make verify-container` — production container build.

`make verify` is the source of truth. CI providers must call these targets rather than reimplementing the commands.

## Runtime architecture

### Host requirements

The full gate requires only:

- Linux-compatible shell environment
- GNU Make
- Docker Engine
- Docker Compose v2 (`docker compose`)
- Git checkout of the repository

Go, Node, PostgreSQL client tools, sqlc, goose, and govulncheck are not host prerequisites.

### Pinned verification images

- Go: `golang:1.26.6-bookworm`
- Node: `node:24.19.0-bookworm`
- PostgreSQL: `postgres:18.6`
- sqlc: `v1.31.1`
- goose: `v3.27.3`
- govulncheck: `v1.7.0`

The Go verifier image is built from `ci/go.Dockerfile` and installs the pinned SQL and vulnerability tools with the same release checksums already used by the existing CI workflow.

`compose.ci.yaml` defines an isolated PostgreSQL service plus Go and web verification services. The repository is bind-mounted read/write because `sqlc generate`, `go mod tidy`, and the frontend build intentionally validate generated/derived source cleanliness through `git diff` checks.

## Verification flow

`scripts/ci/verify.sh` is the top-level orchestrator:

1. Run `scripts/ci/contract-test.sh` before expensive work.
2. Create an isolated Compose project name for the invocation.
3. Build the pinned Go verifier image.
4. Start PostgreSQL and wait for its health check.
5. Run migration up/down/up smoke using goose.
6. Run the backup/restore drill against the isolated PostgreSQL service.
7. Run the full Go verifier, including generated-source checks, module checks, formatting, vet, tests, integration tests, vulnerability scan, race tests, and binary build.
8. Run MCP-specific security checks.
9. Run frontend install/test/PWA/typecheck/build in Node 24.19.0.
10. Run edge security static and runtime Caddy/Compose validation.
11. Build the production container image.
12. Always tear down the isolated Compose project and its volumes.

The flow is deliberately sequential. The project is small, and predictable logs/resource use are more valuable than parallelism at this stage.

## Database verification

The CI Compose PostgreSQL service uses the same test database contract as the current GitHub workflow:

- user: `finance_app`
- password: `test-secret`
- database: `finance`
- PostgreSQL: `18.6`

Go integration tests receive `TEST_POSTGRES_*` environment variables. Migration and restore-drill checks use the same database service, so there is no hidden provider-specific service container behavior.

## Backup/restore drill

The existing `scripts/restore-drill.sh` remains the authoritative restore validator. A new repository script prepares the temporary ezBookkeeping source database, creates finance/ezBookkeeping dumps plus storage archive and checksums, then calls `restore-drill.sh` with the PostgreSQL container ID obtained from Compose.

This preserves the existing recovery gate without putting Docker CLI access inside the Go verification container.

## Security gates

### MCP

MCP verification retains:

- stable MCP SDK version exactly `v1.6.1`
- secure HTTP handler tests
- Agent timeout/audit finalization tests
- MCP race contract

The commands live in `scripts/ci/mcp-security.sh` and are invoked through the Go verifier container.

### Edge/Caddy

`scripts/check-edge-security.sh` remains the static edge policy contract. `scripts/ci/edge-security.sh` adds the runtime checks previously embedded in `.github/workflows/edge-security.yml`:

- Compose configuration with a deterministic test auth hash
- runtime `FINANCE_AUTH_HASH` preservation
- pinned Caddy configuration validation through Compose

## GitHub Actions role

GitHub Actions becomes optional and manual-only while hosted minutes are constrained. Workflows use `workflow_dispatch` and delegate to repository targets:

- CI: `make verify`
- MCP Security: `make verify-mcp-security`
- Edge Security: `make verify-edge-security`

There is no `actions/setup-go`, `actions/setup-node`, PostgreSQL service definition, or duplicated test command list in workflow YAML.

If automatic GitHub checks are desired later, `pull_request` can be re-enabled without changing the quality gate itself.

## Structural anti-drift contract

`scripts/ci/contract-test.sh` provides a fast, Docker-free test that fails when the architecture regresses. It checks that:

- required CI scripts/config files exist
- all CI shell scripts pass `bash -n`
- `Makefile` exposes the canonical verification targets
- GitHub workflows invoke the corresponding `make` target
- workflows do not contain provider-owned Go/Node setup or direct `go test`/`npm test` command lists
- the top-level verifier delegates to all required verification phases

This test is intentionally simple and grep-based because it is testing repository structure, not application semantics.

## Failure behavior

- Any verification command exits non-zero immediately.
- Top-level cleanup runs through `trap` even on failure.
- Compose project names are per-invocation to avoid collisions.
- Temporary files are created with `mktemp -d` and removed on exit.
- No production `.env` or secrets are consumed.
- No verification step writes credentials into git-tracked files.

## Migration strategy

1. Add the structural contract first and demonstrate it fails against the current Actions-owned layout.
2. Add repository-native runtime files and targets.
3. Convert Actions workflows to thin manual wrappers.
4. Run the structural contract locally.
5. Run the full `make verify` on the first available Docker-capable host.
6. Only after the complete gate is green may the blocked V1.3 Task 3 be treated as verified and Task 4 start.

## Acceptance criteria

The design is accepted when:

- `make verify-contract` succeeds without Docker.
- On a Docker-capable Linux host, `make verify` completes successfully from a clean checkout.
- `make verify` requires no GitHub Actions token or runner.
- All checks previously present in CI/MCP Security/Edge Security have an equivalent repository-owned command.
- GitHub workflow YAML contains only provider bootstrap plus calls into the repository-owned verification interface.
