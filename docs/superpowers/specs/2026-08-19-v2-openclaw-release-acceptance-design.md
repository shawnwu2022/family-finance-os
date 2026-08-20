# V2 OpenClaw Release Acceptance Design

## Goal

Turn the remaining V2 production gate into a reproducible **real OpenClaw** acceptance run without introducing a permanent staging server, external model API key, new production service, or a fake Finance backend.

## Decision

Use a controlled ephemeral Linux runner as the release-acceptance host. The runner starts the existing production Docker Compose stack, serves Finance MCP through real Caddy HTTPS using a temporary locally trusted CA, initializes real PostgreSQL and ezBookkeeping data, installs real OpenClaw, runs a local Ollama tool-capable model, executes the existing OpenClaw live smoke, and verifies completed `agent_tool_audits` rows.

This is a release-acceptance environment, not a unit-test substitute. The actual Finance Core binary/container, PostgreSQL, ezBookkeeping, Caddy edge, OpenClaw MCP registry/client, and OpenClaw agent runtime are all exercised.

## Alternatives considered

### Keep operator-only manual acceptance

Lowest implementation cost but leaves the release blocked on an external machine, manually configured OpenClaw, model credentials, TLS endpoint and database access. Rejected because the project is otherwise reproducible and this would remain the only non-repeatable gate.

### Permanent VPS staging

Provides a long-lived real endpoint, but introduces a host, patching, DNS, credentials, backups and recurring cost solely to satisfy one acceptance path. Rejected as inconsistent with the project's low-ops principle.

### Ephemeral real OpenClaw acceptance — selected

No long-lived infrastructure. The run is reproducible from the repository, creates its own non-production secrets and data, and deletes the environment afterwards. It still uses real OpenClaw and real HTTPS rather than an SDK/httptest substitute.

## Environment

- Ubuntu GitHub-hosted Linux runner or any equivalent Docker-capable Linux host.
- Existing `compose.yaml` remains the production topology source.
- A small acceptance-only Compose override may change only local test wiring such as Caddy config/hostnames; it must not add a production service or host port.
- `finance.localhost` and `ebk.localhost` resolve to loopback on the acceptance host.
- Acceptance Caddy configuration preserves the production routing split: `/mcp` reaches Finance Core without Basic Auth; other Finance routes remain Basic Auth protected.
- Caddy uses `tls internal`; the generated local CA root is installed into the runner trust store before OpenClaw/curl probes.

## Data/bootstrap

1. Generate random acceptance-only PostgreSQL/Finance/ezBookkeeping/MCP credentials in a mode-0700 temporary directory. Nothing is committed or uploaded as an artifact.
2. Start PostgreSQL.
3. Install pinned goose `v3.27.3`, migrate the Finance database, and create one CNY household plus a minimal policy required by deterministic Finance operations.
4. Start ezBookkeeping with API-token generation enabled.
5. Use the ezBookkeeping CLI inside its existing container to create an acceptance user and an API session token. Store token output only in the temporary directory.
6. Seed the minimal ledger facts needed for `get_household_overview` and `simulate_purchase`; prefer documented ezBookkeeping HTTP APIs/CLI and never bypass Finance Core with a fake ledger.
7. Start Finance Core with `MCP_ENABLED=true`, the created household id, and the generated MCP bearer file; then start Caddy.

## OpenClaw runtime

- Install OpenClaw through its documented npm/headless path and record the version in sanitized evidence.
- Install/start Ollama locally and pull a pinned tool-capable model. Initial candidate: `qwen3.5:4b`, chosen as a bounded CPU-capable model for two deterministic tool calls, not for general production advice.
- Create an ephemeral OpenClaw config under the temporary directory. It contains the saved Finance MCP server definition and points at the locally trusted HTTPS endpoint. It is deleted on exit.
- The model/provider is explicitly selected for the two acceptance turns. No cloud API key is required.

## Acceptance checks

The run must prove all of the following on one candidate commit:

1. OpenClaw `mcp doctor --probe` succeeds against the HTTPS Finance endpoint.
2. OpenClaw `mcp probe` discovers exactly the twelve release tools and no MCP resources/prompts.
3. Direct missing/wrong bearer requests return 401.
4. A valid bearer with untrusted Origin returns 403.
5. A real `openclaw agent exec` turn invokes `finance__get_household_overview` and finishes with the required marker.
6. A second real agent turn invokes `finance__simulate_purchase` and finishes with the required marker.
7. PostgreSQL contains completed successful `agent_tool_audits` rows for the successful OpenClaw calls.
8. Only sanitized evidence is printed or uploaded: versions, status codes, tool count, audit counts and SHA-256 digests. No bearer, API token, raw agent result, statement data, password, or full OpenClaw config may leave the temporary directory.

## Trigger and release semantics

- Keep normal CI/MCP/Edge workflows unchanged.
- Add a dedicated release-acceptance workflow. It is not a substitute for normal CI and should be intentionally triggered for a release candidate, not on every PR edit.
- Because branch-local `workflow_dispatch` cannot be relied on before the workflow reaches the default branch, the implementation branch may use a narrowly scoped branch push trigger for initial validation. The merged form retains `workflow_dispatch` for future release candidates.
- A green run can satisfy the prior “Real OpenClaw acceptance” requirement only if it used real OpenClaw, actual HTTPS/Caddy routing, real Finance Core and real PostgreSQL audit verification. SDK-only or `httptest` runs remain insufficient.

## Failure policy

Fail closed on any missing dependency, TLS trust issue, OpenClaw diagnostic, tool-count mismatch, model/tool-call failure, HTTP status mismatch, or missing audit row. Always dump only sanitized service status/log tails; secret-bearing raw outputs remain private and are deleted.

## Scope boundaries

No production API behavior, Agent tool semantics, MCP allowlist, Finance database schema, production Compose service, host port, credential, market-data dependency, queue, cache or background worker is added by this work.
