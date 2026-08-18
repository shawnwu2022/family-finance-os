# V2 MCP Security Acceptance

**Scope:** V2 Finance Core-native MCP adapter and HTTPS security boundary  
**Branch:** `feature/v2-agent-adapter`  
**V1 base:** `05d8afcf0299c3ae0fd9f0422b13e28fe8842a28`  
**Automated acceptance:** required on the exact candidate commit  
**Real OpenClaw acceptance:** required separately before V2.0 production release

## Decision rule

V2 MCP security is automated-ready only when the exact candidate commit has all repository CI/security workflows green. V2.0 production acceptance additionally requires the real OpenClaw live checks below. Automated SDK clients and `httptest` servers are not substitutes for the real OpenClaw/deployed-endpoint gate.

## Automated security gates

| Gate | Required evidence |
|---|---|
| Stable MCP dependency | Official `github.com/modelcontextprotocol/go-sdk` exactly `v1.6.1`; pre-release versions rejected by CI |
| Tool allowlist | MCP advertises exactly the nine `READY` tools from `docs/v2-agent-tool-parity.md`; no unverified target tool is exposed |
| Schema/scope isolation | Tool schemas do not expose `household_id`; server-side principal supplies one positive configured household |
| Deterministic parity | Adapter business payloads match direct Finance Core typed API results for parity fixtures |
| Audit fail-closed | Audit attempt must persist before tool execution; successful completion must persist before result disclosure |
| Audit finalization after timeout | Request cancellation does not prevent bounded audit completion; a backend deadline remains the stable `timeout` result rather than being overwritten by `audit_unavailable` |
| Audit storage hygiene | `agent_tool_audits` stores hashes/metadata only; no raw bearer, request body, result body, or backend error text columns |
| Bearer authentication | Missing, malformed, duplicate, Basic, query-string, and wrong bearer credentials are rejected; credential comparison uses fixed-size SHA-256 plus constant-time compare |
| Origin validation | Missing Origin is allowed for non-browser clients; exact same-origin/trusted origins pass; malformed, `null`, wrong-scheme, and untrusted origins return 403 before dispatch |
| Body limit | Known and unknown-length bodies above `MCP_MAX_BODY_BYTES` return 413 before MCP dispatch |
| Timeout | Request context receives configured deadline; Finance backend cancellation/deadline maps to stable `timeout` rather than `data_unavailable` |
| Concurrency | Requests above `MCP_MAX_CONCURRENT` are rejected immediately with controlled `503 busy` and do not enter the downstream MCP handler |
| Rate limit | Process-local fixed-minute limit enforces `MCP_REQUESTS_PER_MINUTE`; overflow returns controlled `429 busy`; limiter is race-tested |
| Safe error mapping | Backend/database secret-bearing error strings do not reach MCP clients |
| Configuration | `MCP_ENABLED=false` by default; disabled mode adds no token/scope prerequisite; enabled mode fails fast for invalid scope/origin/timeout/concurrency/rate/body settings |
| Secret loading | Bearer comes only from `MCP_TOKEN_FILE`; missing, directory, empty, whitespace-bearing, or oversized files fail startup without echoing secret contents |
| Household startup validation | Configured household must exist in PostgreSQL before a handler is returned |
| Real PostgreSQL MCP integration | CI creates a household, builds the secure handler, connects with the official Streamable HTTP client, discovers nine tools, calls a scoped tool, and verifies a successful audit row |
| Application feature gate | `/mcp` is mounted only when `MCP_ENABLED=true`; otherwise existing V1 router behavior remains unchanged |
| Caddy Authorization separation | Exact `/mcp` bypasses Caddy Basic Auth so the application receives MCP Bearer; all other Finance UI/API routes remain under Caddy Basic Auth |
| Edge exposure | Caddy remains the only host-port publisher; only 80/tcp, 443/tcp, and 443/udp are exposed; no MCP sidecar or extra host port |
| Git/Compose secret hygiene | No literal `MCP_TOKEN` environment variable; `./secrets` is ignored except `.gitkeep` and mounted read-only at `/run/secrets` |
| Live acceptance helper contract | `scripts/acceptance/openclaw-mcp-live-smoke.sh` is shell-validated by production-ops CI and must contain real OpenClaw probe/agent calls, read+simulation checks, 401/403 probes, and hashed evidence output |
| V1 regression | Existing V1 production-ops, restore, frontend/PWA, Go integration, race, container build, and edge checks remain green |

## Exact CI gates

The candidate commit must have successful results for:

- `CI`
  - stable MCP SDK gate
  - full `go test ./...`
  - MCP adapter contract
  - `govulncheck ./...`
  - PostgreSQL/household/budget/goals/advice-audit/agent-audit/scheduler/app API integrations
  - application builder integration
  - MCP startup/application integration
  - full Go race test
  - Finance Core binary build
  - frontend unit/PWA/typecheck/build
  - production-ops and Finance edge contract
  - Finance Core container build
- `Edge Security`
- `MCP Security`
  - stable SDK check
  - MCP HTTP security contract
  - Agent timeout and audit finalization
  - MCP security race contract

A previous green commit is not sufficient evidence for a later candidate commit.

## Deployment contract

MCP remains disabled by default. To enable it in a real deployment:

1. Create `secrets/finance-mcp-token` outside Git with a high-entropy token and restrictive host permissions.
2. Set `MCP_ENABLED=true`.
3. Set `MCP_HOUSEHOLD_ID` to the intended existing household.
4. Optionally set exact `MCP_ALLOWED_ORIGINS` for browser-capable trusted clients.
5. Keep the documented defaults unless a measured need justifies changes:
   - `MCP_REQUEST_TIMEOUT=15s`
   - `MCP_MAX_CONCURRENT=4`
   - `MCP_REQUESTS_PER_MINUTE=60`
   - `MCP_MAX_BODY_BYTES=262144`
6. Deploy through the existing Caddy HTTPS edge. Do not publish Finance Core or MCP directly on a host port.

## Live OpenClaw acceptance helper

The repository includes `scripts/acceptance/openclaw-mcp-live-smoke.sh`. It is an operator-run real-environment check; CI validates its contract but does not execute the external acceptance itself.

Prerequisites:

1. The Finance MCP endpoint is deployed through HTTPS with `MCP_ENABLED=true`.
2. OpenClaw is installed and already has an enabled saved MCP server for Finance. The script does not add, edit, or persist OpenClaw configuration.
3. The saved server points at the deployed Finance `/mcp` endpoint and supplies its bearer credential through the operator's OpenClaw secret/config mechanism.
4. A Finance MCP bearer file is present on the acceptance host. By default the script reads `secrets/finance-mcp-token`; override with `FINANCE_MCP_SMOKE_TOKEN_FILE` when the deployment stores the acceptance copy elsewhere.
5. OpenClaw has a usable model/provider for the two agent turns.

Optional acceptance variables:

```text
OPENCLAW_FINANCE_MCP_SERVER=finance
FINANCE_MCP_SMOKE_URL=https://finance.example.com/mcp
FINANCE_MCP_SMOKE_TOKEN_FILE=/secure/path/finance-mcp-token
OPENCLAW_FINANCE_SMOKE_AGENT_TIMEOUT=120
OPENCLAW_FINANCE_SMOKE_MODEL=<provider/model>
OPENCLAW_FINANCE_SMOKE_CONFIG=/path/to/pinned-openclaw-config.json5
```

Run from the repository root:

```bash
FINANCE_ENV_FILE=/path/to/production.env \
  bash scripts/acceptance/openclaw-mcp-live-smoke.sh
```

The script performs all of the following in one run:

- `openclaw mcp doctor <server> --probe --json`;
- `openclaw mcp probe <server> --json` and verifies the selected server points at the expected HTTPS endpoint, exposes exactly the nine release tools, and exposes no MCP resources/prompts;
- direct HTTP checks for missing/invalid bearer = 401 and untrusted Origin with a valid bearer = 403;
- one `openclaw agent exec --code-mode direct --json` turn that must call `get_household_overview`;
- one equivalent agent turn that must call `simulate_purchase`;
- only sanitized evidence on stdout: status codes, tool count, and SHA-256 digests of OpenClaw JSON results. Raw probe/agent output remains in a mode-0700 temporary directory and is deleted on exit.

A PASS from this helper is necessary but not by itself sufficient for production release: the corresponding successful calls still need completed `agent_tool_audits` rows verified against the deployment database.

## Real OpenClaw acceptance — NOT RUN

These are real-environment gates and remain **NOT RUN** until a deployed HTTPS endpoint and OpenClaw runtime are available:

1. Configure the deployed Finance MCP endpoint in OpenClaw using Streamable HTTP and an external secret reference for the bearer credential.
2. Run `scripts/acceptance/openclaw-mcp-live-smoke.sh` and retain its sanitized hash/status output as acceptance evidence.
3. Confirm exactly the intended V2 release tool allowlist is discoverable.
4. Confirm the helper's real OpenClaw read turn invoked the Finance household-overview tool successfully.
5. Confirm the helper's deterministic purchase simulation turn succeeded.
6. Confirm the helper recorded 401 for invalid bearer and 403 for untrusted Origin.
7. Verify the corresponding successful calls have completed `agent_tool_audits` rows.
8. Record only sanitized evidence: no bearer token, raw statement, credential, full account number, raw agent result, or secret-bearing logs.

## Production release status

**BLOCKED until the Real OpenClaw acceptance section is completed with sanitized evidence.**

This block is independent of V1 production acceptance. V1 release evidence must continue to use the V1 release-candidate commit and its own production acceptance document.
