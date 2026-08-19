# V2 OpenClaw Release Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Execute the remaining V2 production gate with real OpenClaw against an ephemeral real Finance Core/Caddy/PostgreSQL/ezBookkeeping HTTPS deployment using a local Ollama model and sanitized evidence.

**Architecture:** Keep `scripts/acceptance/openclaw-mcp-live-smoke.sh` as the generic real-environment validator. Add an acceptance-only provisioner plus Compose/Caddy override that creates a disposable production-shaped environment, then calls the existing smoke and verifies database audit rows. A dedicated release workflow runs this provisioner intentionally and keeps normal CI separate.

**Tech Stack:** Bash, Docker Compose, Caddy 2.11.4, PostgreSQL 18.6, ezBookkeeping 1.6.1, Finance Core Go 1.26.6 image, goose 3.27.3, OpenClaw CLI, Ollama, `qwen3.5:4b`, GitHub Actions Ubuntu runner.

**Spec:** `docs/superpowers/specs/2026-08-19-v2-openclaw-release-acceptance-design.md`

## Global Constraints

- The acceptance run must use real OpenClaw; MCP SDK/`httptest` clients are not substitutes.
- The Finance endpoint must be reached through real Caddy HTTPS.
- The Finance backend must be the production Finance Core container using real PostgreSQL and real ezBookkeeping; no fake ledger/backend mode.
- No cloud model API key is required; use local Ollama.
- No production service, host port, database schema, Agent/MCP allowlist, or tool semantics change.
- No secret/token/raw agent output is committed, uploaded as an artifact, or printed.
- Evidence is limited to versions, status codes, tool/audit counts and SHA-256 digests.
- All generated credentials/config/state live in a mode-0700 temporary directory and are deleted on exit.

---

## Task 1: Lock the release-acceptance repository contract

**Files:**
- Create: `scripts/acceptance/test-openclaw-release-acceptance.sh`
- Create: `compose.openclaw-acceptance.yaml`
- Create: `Caddyfile.acceptance`
- Create later tasks: `scripts/acceptance/openclaw-ephemeral-release-acceptance.sh`
- Create later tasks: `.github/workflows/openclaw-release-acceptance.yml`

**Interfaces:**
- Produces a structural contract that prevents the release workflow from silently becoming SDK-only or bypassing Caddy/audit verification.

- [ ] **Step 1: RED — add the structural contract test first**

The test must fail until all required acceptance files exist. Once present it must assert:

```bash
required=(
  compose.openclaw-acceptance.yaml
  Caddyfile.acceptance
  scripts/acceptance/openclaw-ephemeral-release-acceptance.sh
  .github/workflows/openclaw-release-acceptance.yml
)
```

It must also require the provisioner to invoke `scripts/acceptance/openclaw-mcp-live-smoke.sh`, require an `agent_tool_audits` query, require `openclaw agent exec`, require the workflow to call only the repository provisioner rather than duplicating acceptance logic, and reject `actions/setup-go`/SDK-only tool calls inside the release workflow.

- [ ] **Step 2: Verify RED**

Run in a normal checkout:

```bash
bash scripts/acceptance/test-openclaw-release-acceptance.sh
```

Expected: failure naming the first absent required file.

- [ ] **Step 3: Add acceptance Caddy/Compose skeletons**

`Caddyfile.acceptance` must preserve production route semantics and use internal TLS:

```caddyfile
{$EBK_DOMAIN} {
    tls internal
    reverse_proxy ezbookkeeping:8080
}

{$FINANCE_DOMAIN} {
    tls internal
    @mcp path /mcp
    handle @mcp {
        reverse_proxy finance-core:8000
    }
    handle {
        basic_auth {
            {$FINANCE_AUTH_USER} {$FINANCE_AUTH_HASH}
        }
        reverse_proxy finance-core:8000
    }
}
```

`compose.openclaw-acceptance.yaml` must override Caddy to run the acceptance Caddyfile and enable ezBookkeeping API token generation, without adding services or ports.

- [ ] **Step 4: Commit RED/skeleton contract**

Commit: `test(v2): define real OpenClaw release acceptance contract`

---

## Task 2: Provision a real disposable Finance/ezBookkeeping HTTPS environment

**Files:**
- Create/modify: `scripts/acceptance/openclaw-ephemeral-release-acceptance.sh`
- Modify: `scripts/acceptance/test-openclaw-release-acceptance.sh`

**Interfaces:**
- Consumes: existing `compose.yaml`, migrations, `openclaw-mcp-live-smoke.sh`.
- Produces: an acceptance environment file, MCP bearer file, ezBookkeeping API token, household id and trusted `https://finance.localhost/mcp` endpoint inside one temp workspace.

- [ ] **Step 1: Extend RED contract for bootstrap guarantees**

Require the provisioner to contain fail-closed checks for Docker Compose, pinned goose 3.27.3 checksum, `finance.localhost` hosts entry, Caddy local CA trust installation, Finance migration, household insert, ezBookkeeping `userdata user-add`, ezBookkeeping `userdata user-session-new --type api`, and service health before any OpenClaw call.

- [ ] **Step 2: Implement deterministic bootstrap**

The provisioner must:

```text
create 0700 temp dir
write 0600 env/token/config files
generate random local-only passwords/tokens
add finance.localhost + ebk.localhost -> 127.0.0.1
start postgres only
install pinned goose and run db/migrations up
insert one CNY household and policy
start ezbookkeeping
create acceptance user via container CLI
generate API token via container CLI
write EBK_API_TOKEN into the temporary env file
start finance-core and caddy
extract Caddy local root CA and update host trust
wait for HTTPS /healthz through Basic Auth
```

Use traps to remove Compose resources, temp files, hosts entries and local CA trust material even on failure.

- [ ] **Step 3: Seed minimal ledger data through real ezBookkeeping interfaces**

Use documented ezBookkeeping CLI/HTTP paths only. Seed the smallest account/transaction set required for `get_household_overview` and `simulate_purchase` to return structured deterministic results. Do not write ezBookkeeping tables directly unless its official CLI/import path cannot represent the required data; if direct DB seed becomes unavoidable, stop and revise the design rather than hiding it.

- [ ] **Step 4: Static verification**

Run:

```bash
bash -n scripts/acceptance/openclaw-ephemeral-release-acceptance.sh
bash scripts/acceptance/test-openclaw-release-acceptance.sh
```

- [ ] **Step 5: Commit**

Commit: `feat(v2): provision ephemeral OpenClaw acceptance stack`

---

## Task 3: Run real OpenClaw with local Ollama and verify audits

**Files:**
- Modify: `scripts/acceptance/openclaw-ephemeral-release-acceptance.sh`
- Create: `.github/workflows/openclaw-release-acceptance.yml`
- Modify: `scripts/acceptance/test-openclaw-release-acceptance.sh`

**Interfaces:**
- Consumes: `https://finance.localhost/mcp`, MCP bearer file, household id.
- Produces: sanitized `openclaw_release_acceptance=PASS` evidence only after two real OpenClaw tool calls and matching audit rows.

- [ ] **Step 1: Extend RED contract for real OpenClaw/model semantics**

Require exact command families:

```text
npm install -g openclaw
ollama pull qwen3.5:4b
openclaw mcp doctor ... --probe --json
openclaw mcp probe ... --json
openclaw agent exec ... --model ollama/qwen3.5:4b --code-mode direct --json
```

Require the provisioner to call the existing `openclaw-mcp-live-smoke.sh` rather than reimplement the tool/status checks.

- [ ] **Step 2: Create ephemeral OpenClaw config**

Write a JSON5 config in the private temp directory with an enabled saved MCP server named `finance`, transport `streamable-http`, URL `https://finance.localhost/mcp`, and Authorization header populated from the local MCP bearer. Configure Ollama as the selected model provider. Never print this file.

- [ ] **Step 3: Install/start Ollama and OpenClaw**

Pin the acceptance model string to `qwen3.5:4b`. Verify Ollama `/api/tags`, run a minimal direct model smoke, then invoke the existing live-smoke helper with the explicit OpenClaw config/model and endpoint variables.

- [ ] **Step 4: Verify completed audit rows**

After live smoke PASS, query PostgreSQL for successful completed rows for at least:

```text
get_household_overview
simulate_purchase
```

scoped to the acceptance household. Require each count >= 1 and print only counts.

- [ ] **Step 5: Add dedicated release workflow**

The workflow must use `permissions: contents: read`, check out the exact candidate, then run only:

```bash
bash scripts/acceptance/openclaw-ephemeral-release-acceptance.sh
```

For branch validation, allow a narrowly scoped push trigger on `feature/v2-openclaw-release-acceptance`; retain `workflow_dispatch` for the merged/default-branch form. Do not upload temp logs/config/token artifacts.

- [ ] **Step 6: Execute the real workflow**

Expected: real OpenClaw probe, two real agent tool calls, 401/403 checks, successful audit counts, and final `openclaw_release_acceptance=PASS`.

- [ ] **Step 7: Commit**

Commit: `ci(v2): run real OpenClaw release acceptance`

---

## Task 4: Record release evidence and integrate the stacked chain safely

**Files:**
- Modify: `docs/v2-mcp-security-acceptance.md`
- Modify: PR metadata for the acceptance branch / PR #20 only after exact-head evidence exists.

**Interfaces:**
- Consumes: successful release-acceptance workflow run and normal CI/MCP/Edge exact-head results.
- Produces: a new V2 candidate eligible for PR #20 production release consideration.

- [ ] **Step 1: Update acceptance documentation from NOT RUN to evidence-backed result**

Record only:

```text
candidate SHA
OpenClaw version
Ollama model id
release workflow run id
12-tool count
401/401/403 statuses
doctor/probe/read/simulation SHA-256 digests
audit counts
```

Do not include bearer/API tokens, full configs or raw agent payloads.

- [ ] **Step 2: Run normal exact-head gates on the final V2 candidate**

Require CI, MCP Security and Edge Security all successful on the same head as the release evidence.

- [ ] **Step 3: Diff audit**

Confirm the acceptance work changes only release tooling/docs and does not change Agent/MCP tool behavior, schema, production Compose topology or host ports.

- [ ] **Step 4: Merge acceptance infrastructure into `feature/v2-agent-adapter` only after exact-green evidence**

Use expected-head protection. Do not merge PR #20 to `main` until its new head has both normal exact-head gates and the real OpenClaw acceptance evidence.

- [ ] **Step 5: Rebase/merge the advanced V2 base into descendant V1.3 branches**

Refresh PR #21 and then PR #23 so their stacked diffs remain clean. Re-run their exact-head gates after the base movement; do not silently rely on previous evidence.
