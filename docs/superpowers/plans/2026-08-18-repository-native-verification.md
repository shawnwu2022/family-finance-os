# Repository-Native Verification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the Family Finance OS quality gate out of GitHub Actions into a repository-owned, Dockerized `make verify` contract that can run on any Docker-capable Linux host.

**Architecture:** Repository scripts own every verification command. `compose.ci.yaml` supplies pinned PostgreSQL, Go, and Node environments; `scripts/ci/verify.sh` orchestrates the complete sequential gate; GitHub Actions becomes a thin manual wrapper around Make targets.

**Tech Stack:** Bash, GNU Make, Docker Engine, Docker Compose v2, Go 1.26.6, Node 24.19.0, PostgreSQL 18.6, sqlc 1.31.1, goose 3.27.3, govulncheck 1.7.0.

**Spec:** `docs/superpowers/specs/2026-08-18-repository-native-verification-design.md`

## Global Constraints

- Full verification entry point is exactly `make verify`.
- The complete gate must not require GitHub Actions, a GitHub token, Jenkins, Kubernetes, or an always-on CI controller.
- Go is pinned to `1.26.6`; Node to `24.19.0`; PostgreSQL to `18.6`.
- sqlc is pinned to `v1.31.1`; goose to `v3.27.3`; govulncheck to `v1.7.0`.
- Existing CI, MCP Security, and Edge Security checks must not be weakened.
- GitHub workflows become manual `workflow_dispatch` wrappers around repository Make targets.
- Do not modify Finance Core business behavior as part of this plan.

---

### Task 1: Add the structural CI contract first

**Files:**
- Create: `scripts/ci/contract-test.sh`

**Interfaces:**
- Consumes: existing `Makefile` and `.github/workflows/*.yml`.
- Produces: Docker-free command `bash scripts/ci/contract-test.sh`, later exposed as `make verify-contract`.

- [ ] **Step 1: Write the failing structural contract**

Create `scripts/ci/contract-test.sh` with `set -euo pipefail`. It must:

```bash
required=(
  ci/go.Dockerfile
  compose.ci.yaml
  scripts/ci/verify.sh
  scripts/ci/go-verify.sh
  scripts/ci/web-verify.sh
  scripts/ci/mcp-security.sh
  scripts/ci/edge-security.sh
  scripts/ci/restore-verify.sh
)
for path in "${required[@]}"; do
  [[ -f "$path" ]] || { echo "missing repository-native CI file: $path" >&2; exit 1; }
done
```

Then syntax-check every CI shell script:

```bash
while IFS= read -r script; do
  bash -n "$script"
done < <(find scripts/ci -type f -name '*.sh' -print | sort)
```

Require Make targets `verify`, `verify-contract`, `verify-go`, `verify-web`, `verify-mcp-security`, `verify-edge-security`, and `verify-container`.

Require workflow delegation:

```bash
grep -Fq 'make verify' .github/workflows/ci.yml
grep -Fq 'make verify-mcp-security' .github/workflows/mcp-security.yml
grep -Fq 'make verify-edge-security' .github/workflows/edge-security.yml
```

Forbid provider-owned toolchain/test logic in workflow YAML:

```bash
if grep -REq 'actions/setup-(go|node)|go test|npm (ci|test|run)|services:[[:space:]]*$' .github/workflows; then
  echo 'verification logic leaked back into GitHub Actions' >&2
  exit 1
fi
```

Require the top-level verifier to invoke contract, restore, Go, MCP security, web, edge, and container phases.

- [ ] **Step 2: Verify RED against the current layout**

Run the contract against a checkout at `b259ca9192b1070c46cf986e9e74ecbb5331d260`.

Expected: FAIL because `ci/go.Dockerfile`, `compose.ci.yaml`, and the repository-owned CI scripts/Make targets do not exist and workflows still contain setup/test logic.

- [ ] **Step 3: Commit only the failing contract**

```bash
git add scripts/ci/contract-test.sh
git commit -m 'test(ci): define repository-native verification contract'
```

---

### Task 2: Add the pinned verification runtime and repository scripts

**Files:**
- Create: `ci/go.Dockerfile`
- Create: `compose.ci.yaml`
- Create: `scripts/ci/go-verify.sh`
- Create: `scripts/ci/web-verify.sh`
- Create: `scripts/ci/mcp-security.sh`
- Create: `scripts/ci/edge-security.sh`
- Create: `scripts/ci/restore-verify.sh`
- Create: `scripts/ci/verify.sh`

**Interfaces:**
- Consumes: existing Go module, web package, migrations, `scripts/restore-drill.sh`, `scripts/check-edge-security.sh`, `compose.yaml`, and `Dockerfile`.
- Produces: complete provider-neutral gate run by `scripts/ci/verify.sh`.

- [ ] **Step 1: Create the Go verifier image**

`ci/go.Dockerfile` must start from `golang:1.26.6-bookworm`, install `curl`, `ca-certificates`, `git`, `postgresql-client`, and `tar`, then install:

```text
sqlc v1.31.1
sha256 497ae4fcdfa64c5b0c311ffe4c2bd991e43991e82e5367792ed78bc2dca27354

goose v3.27.3
sha256 ca18112e2438b3ad608af9a5938beafd01fa36a4a19a3edbe4f29226ca5c8533

govulncheck v1.7.0
```

Set `WORKDIR /workspace`.

- [ ] **Step 2: Create `compose.ci.yaml`**

Define:

```yaml
services:
  postgres:
    image: postgres:18.6
    environment:
      POSTGRES_USER: finance_app
      POSTGRES_PASSWORD: test-secret
      POSTGRES_DB: finance
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U finance_app -d finance"]
      interval: 5s
      timeout: 5s
      retries: 20

  go:
    build:
      context: .
      dockerfile: ci/go.Dockerfile
    working_dir: /workspace
    volumes:
      - .:/workspace
      - go-mod-cache:/go/pkg/mod
      - go-build-cache:/root/.cache/go-build
    environment:
      TEST_POSTGRES_HOST: postgres
      TEST_POSTGRES_PORT: "5432"
      TEST_POSTGRES_DB: finance
      TEST_POSTGRES_USER: finance_app
      TEST_POSTGRES_PASSWORD: test-secret
    depends_on:
      postgres:
        condition: service_healthy

  web:
    image: node:24.19.0-bookworm
    working_dir: /workspace/web
    volumes:
      - .:/workspace
      - web-node-modules:/workspace/web/node_modules

volumes:
  go-mod-cache:
  go-build-cache:
  web-node-modules:
```

Do not publish the PostgreSQL port to the host.

- [ ] **Step 3: Create `scripts/ci/go-verify.sh`**

Run, in order:

```bash
sqlc generate
git diff --exit-code -- internal/store/sqlc
test -z "$(git ls-files --others --exclude-standard internal/store/sqlc)"
go mod tidy
git diff --exit-code -- go.mod go.sum
```

Verify MCP SDK exactly `v1.6.1` and reject prerelease versions. Then run:

```bash
mapfile -t gofiles < <(find cmd internal -type f -name '*.go' -print)
test -z "$(gofmt -l "${gofiles[@]}")"
go vet ./...
go test ./...
go test ./internal/mcpadapter -v
govulncheck ./...
go test ./internal/store -run TestOpenPostgresIntegration -v
go test ./internal/household -run Integration -v
go test ./internal/budget -run Integration -v
go test ./internal/goals -run Integration -v
go test ./internal/portfolio -run Integration -v
go test ./internal/audit -run Integration -v
go test ./internal/audit -run TestAgentPostgresRecorder -v
go test ./internal/scheduler -run Integration -v
go test ./internal/appapi -run TestPostgresPlannerRoundTripIntegration -v
go test ./cmd/finance-core -run TestBuildApplicationHandlerWithoutLLMIntegration -v
go test ./cmd/finance-core -run MCP -v
go test -race ./...
CGO_ENABLED=0 go build -trimpath -o /tmp/finance-core ./cmd/finance-core
```

- [ ] **Step 4: Create MCP and web verifier scripts**

`scripts/ci/mcp-security.sh` runs the same four checks as the current MCP Security workflow:

```bash
go list -m -f '{{.Version}}' github.com/modelcontextprotocol/go-sdk
go test ./internal/mcpadapter -run 'NewSecureHTTPHandler|SecureHTTPHandler' -v
go test ./internal/agentadapter -run 'TestEncodeBackendResultMaps|TestAuditedCallPreservesTimeoutWhenRequestCancelsBeforeFailureAuditCompletion' -v
go test -race ./internal/mcpadapter -run 'NewSecureHTTPHandler|SecureHTTPHandler' -v
```

`scripts/ci/web-verify.sh` runs:

```bash
npm ci --ignore-scripts
npm test
npm run check:pwa
npm run typecheck
npm run build
```

- [ ] **Step 5: Create restore verification wrapper**

`scripts/ci/restore-verify.sh` runs on the host. Obtain the CI PostgreSQL container ID with:

```bash
postgres_container="$(${COMPOSE[@]} ps -q postgres)"
```

Create an `ezbookkeeping_ci_${$}` database, insert a `restore_probe` table, dump `finance` and the temporary database with `pg_dump -Fc`, create `ezbookkeeping-storage/probe.txt`, generate `SHA256SUMS`, write a temporary env file, and invoke:

```bash
FINANCE_ENV_FILE="$env_file" POSTGRES_DOCKER_CONTAINER="$postgres_container" \
  bash scripts/restore-drill.sh "$backup_dir"
```

Always drop the temporary source database and remove temporary files.

- [ ] **Step 6: Create edge verification wrapper**

`scripts/ci/edge-security.sh` must first run:

```bash
bash scripts/check-edge-security.sh
```

Create a temporary env file containing the deterministic values from the current Edge Security workflow, including:

```text
EBK_DOMAIN=book.example.test
FINANCE_DOMAIN=finance.example.test
FINANCE_AUTH_USER=finance
FINANCE_AUTH_HASH=$2a$14$Zkx19XLiW6VYouLHR5NmfOFU0z2GTNmpkT/5qqR7hx4IjWJPDhjvG
POSTGRES_USER=postgres
POSTGRES_PASSWORD=test-postgres
FINANCE_DB_PASSWORD=test-finance
EBK_DB_PASSWORD=test-ebk
EBK_SECURITY_SECRET_KEY=test-secret-key
```

Then run production Compose config, verify the runtime hash value in the Caddy container, and validate Caddy configuration exactly as the current workflow does.

- [ ] **Step 7: Create the top-level orchestrator**

`scripts/ci/verify.sh` must:

```bash
bash scripts/ci/contract-test.sh
docker compose version >/dev/null
```

Create a unique `COMPOSE_PROJECT_NAME`, install a cleanup trap that runs `docker compose -f compose.ci.yaml down -v --remove-orphans`, then execute:

```bash
docker compose -f compose.ci.yaml build go
docker compose -f compose.ci.yaml up -d postgres
docker compose -f compose.ci.yaml run --rm go goose -dir db/migrations postgres 'postgres://finance_app:test-secret@postgres:5432/finance?sslmode=disable' up
docker compose -f compose.ci.yaml run --rm go goose -dir db/migrations postgres 'postgres://finance_app:test-secret@postgres:5432/finance?sslmode=disable' down
docker compose -f compose.ci.yaml run --rm go goose -dir db/migrations postgres 'postgres://finance_app:test-secret@postgres:5432/finance?sslmode=disable' up
bash scripts/ci/restore-verify.sh
docker compose -f compose.ci.yaml run --rm go bash scripts/ci/go-verify.sh
docker compose -f compose.ci.yaml run --rm go bash scripts/ci/mcp-security.sh
docker compose -f compose.ci.yaml run --rm web bash /workspace/scripts/ci/web-verify.sh
bash scripts/ci/edge-security.sh
docker build -t "${COMPOSE_PROJECT_NAME}-finance-core:verify" .
```

Remove the temporary production image during cleanup when present.

- [ ] **Step 8: Syntax-check the scripts**

Run:

```bash
for f in scripts/ci/*.sh; do bash -n "$f"; done
```

Expected: PASS.

- [ ] **Step 9: Commit the runtime**

```bash
git add ci/go.Dockerfile compose.ci.yaml scripts/ci
git commit -m 'build(ci): add provider-neutral verification runtime'
```

---

### Task 3: Make repository targets canonical and reduce Actions to wrappers

**Files:**
- Modify: `Makefile`
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/mcp-security.yml`
- Modify: `.github/workflows/edge-security.yml`
- Modify: `README.md`

**Interfaces:**
- Consumes: Task 2 scripts.
- Produces: stable `make verify*` commands and optional manual GitHub workflow wrappers.

- [ ] **Step 1: Add Make targets**

Add the following phony targets:

```make
verify:
	./scripts/ci/verify.sh

verify-contract:
	./scripts/ci/contract-test.sh

verify-go:
	docker compose -f compose.ci.yaml up -d postgres
	docker compose -f compose.ci.yaml run --rm go bash scripts/ci/go-verify.sh

verify-web:
	docker compose -f compose.ci.yaml run --rm web bash /workspace/scripts/ci/web-verify.sh

verify-mcp-security:
	docker compose -f compose.ci.yaml run --rm go bash scripts/ci/mcp-security.sh

verify-edge-security:
	./scripts/ci/edge-security.sh

verify-container:
	docker build -t family-finance-os:verify .
```

Keep existing developer targets unchanged.

- [ ] **Step 2: Replace GitHub workflow bodies**

Each workflow uses only:

```yaml
on:
  workflow_dispatch:

permissions:
  contents: read
```

CI job:

```yaml
steps:
  - uses: actions/checkout@v4
  - run: make verify
```

MCP Security job calls `make verify-mcp-security`; Edge Security calls `make verify-edge-security`.

- [ ] **Step 3: Document the new canonical command**

Add a README section stating:

```text
Full verification is repository-native. On any Linux host with Docker Engine + Docker Compose v2, run `make verify`.
GitHub Actions is an optional manual mirror and is not required for development or release verification.
```

List `make verify-contract` as the fast no-Docker architecture check.

- [ ] **Step 4: Verify structural GREEN**

Run:

```bash
make verify-contract
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add Makefile .github/workflows README.md
git commit -m 'ci: make repository verification canonical'
```

---

### Task 4: Verify, open stacked PR, and preserve the V1.3 gate

**Files:**
- Modify: PR metadata only; no additional production files required unless verification exposes a defect.

**Interfaces:**
- Consumes: completed Tasks 1-3.
- Produces: independently reviewable infrastructure PR stacked on `feature/v1-3-portfolio-snapshots`.

- [ ] **Step 1: Run local/static verification available in the execution environment**

At minimum:

```bash
make verify-contract
for f in scripts/ci/*.sh; do bash -n "$f"; done
docker compose -f compose.ci.yaml config
```

If Docker is unavailable, record that the first two can be established but full verification remains pending a Docker-capable host.

- [ ] **Step 2: Run the complete gate on a Docker-capable host**

```bash
make verify
```

Expected: PASS from a clean checkout without a GitHub Actions runner or token.

- [ ] **Step 3: Open a Draft stacked PR**

Head: `feature/repository-native-ci`

Base: `feature/v1-3-portfolio-snapshots`

Title:

```text
ci: make verification repository-native
```

The body must state that the PR does not change Finance Core business behavior and that GitHub Actions is now optional/manual.

- [ ] **Step 4: Re-establish the V1.3 Task 3 gate**

Once `make verify` is green on the exact combined head, update V1.3 PR #21 with that evidence. Only then mark Task 3 GREEN and begin V1.3 Task 4 RED tests.
