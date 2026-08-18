# V2.0 Agent Adapter + MCP Channel Design

**Status:** Approved architecture, implementation design baseline  
**Date:** 2026-08-18  
**Target branch:** `feature/v2-agent-adapter`  
**Base:** V1 `main` at the production-acceptance candidate line  

## 1. Purpose

V2.0 adds agent/channel access to Family Finance OS without changing the Finance Core financial truth model.

The first supported channel is MCP over HTTPS, with OpenClaw as the first real interoperability target. MCP is an adapter at the edge. It must not become a second finance engine, a database access layer, or an authorization bypass.

The architectural invariant is:

```text
Agent / OpenClaw / future MCP client
              |
              v
         MCP Adapter
              |
              v
     Agent Tool Boundary
              |
              v
 Existing Finance Core typed APIs
              |
              v
 Deterministic finance/domain engines
```

All material financial numbers remain calculated by existing deterministic Finance Core code. The MCP adapter maps protocol inputs to existing typed operations and maps typed results back to structured protocol results.

## 2. Scope

### 2.1 V2.0 goals

1. Create an explicit, protocol-neutral agent adapter boundary inside Finance Core.
2. Expose the approved Finance Tool Contract over MCP Streamable HTTP.
3. Keep household scope server-side; clients cannot select arbitrary household IDs.
4. Add dedicated MCP authentication, request guardrails, audit metadata, and safe error mapping.
5. Prove real OpenClaw interoperability against the deployed endpoint.
6. Preserve existing REST/PWA behavior and V1 production acceptance independently.
7. Keep deployment as one Finance Core application; do not add a sidecar, Redis, message broker, or second application database.

### 2.2 Non-goals

V2.0 will not:

- create, edit, or delete ledger transactions;
- move money, pay debt, or place securities trades;
- expose SQL, PostgreSQL, ezBookkeeping credentials, or raw internal storage;
- add OAuth/OIDC, full RBAC, or multi-tenant SaaS identity;
- add MCP resources, prompts, sampling, roots, logging, MCP Apps, or Tasks unless a concrete later use case requires them;
- build an OpenClaw-specific finance engine or duplicate Finance Core calculations;
- add a new mobile application;
- depend on a pre-release MCP SDK in a production merge.

## 3. Current protocol baseline

The design is calibrated to the MCP ecosystem as of 2026-08-18.

MCP specification `2026-07-28` is the current GA protocol revision. Its core is stateless: the old `initialize` / `initialized` session handshake is removed for this revision, and `server/discover` is the discovery/negotiation RPC. Streamable HTTP requests carry protocol/routing metadata including `MCP-Protocol-Version`, `Mcp-Method`, and, for named operations such as tool calls, `Mcp-Name`.

The official MCP Go SDK supports protocol negotiation and backward compatibility. At design time, the latest stable Go SDK release is `v1.6.1`, supporting through MCP `2025-11-25`; MCP `2026-07-28` support is present in `v1.7.0-pre.*` releases.

Therefore V2.0 adopts the following dependency policy:

1. Do not hand-roll MCP JSON-RPC, version negotiation, or lifecycle behavior.
2. Keep all business/tool mapping independent of the MCP SDK.
3. At the transport implementation milestone, use the newest **stable** official MCP Go SDK.
4. If stable `v1.7.0+` is available, enable stateless Streamable HTTP and target `2026-07-28`, while retaining SDK-provided backward negotiation.
5. If `v1.7.0+` is still pre-release, ship against stable `v1.6.1` / MCP `2025-11-25` and keep a compatibility test against the `2026-07-28` pre-release line; do not merge a pre-release SDK merely to claim latest-protocol support.
6. Upgrade to `2026-07-28` after the official Go SDK reaches a stable release and the OpenClaw integration suite passes.

OpenClaw currently supports remote MCP definitions with `transport: "streamable-http"`, bearer headers, timeouts, tool filters, and live probing. Its `openclaw mcp doctor --probe` / `openclaw mcp probe` commands will be used as interoperability evidence rather than assuming protocol-version compatibility from documentation alone.

## 4. Architecture

### 4.1 New packages

The preferred package split is:

```text
internal/agentadapter/
    tools.go          protocol-neutral tool names, inputs, outputs
    service.go        server-side household scoping and dispatch
    errors.go         stable adapter error taxonomy
    audit.go          audit interface + hashing metadata

internal/mcpadapter/
    server.go         official MCP SDK wiring
    tools.go          MCP schema/annotation registration
    auth.go           MCP request authentication middleware
    http.go           Streamable HTTP endpoint integration
```

The package boundary is deliberate:

- `agentadapter` may import existing Finance Core domain/application packages.
- `agentadapter` must not import the MCP SDK.
- `mcpadapter` may import `agentadapter` and the MCP SDK.
- Finance/domain packages must not import `agentadapter` or `mcpadapter`.

This keeps MCP replaceable. A future ACP, CLI, messaging, or other agent protocol can reuse `agentadapter` without importing MCP semantics.

### 4.2 No sidecar

MCP runs in the existing Finance Core process and is routed through the existing Caddy edge.

```text
Internet / trusted agent host
          |
        HTTPS
          |
        Caddy
          |
      /mcp route
          |
   finance-core:8000
          |
     MCP Adapter
```

No new host port is opened. PostgreSQL and ezBookkeeping remain internal to the Docker network. Caddy remains the only host-facing service.

## 5. Tool contract

V2.0 exposes only existing read/simulation capabilities:

```text
get_household_overview(as_of)
get_cashflow(period)
get_spending_analysis(period, compare_periods)
get_budget_status(period)
get_safe_to_spend(as_of, period_end)
get_debt_status(as_of)
simulate_extra_debt_payment(debt_id, amount_minor)
simulate_purchase(amount_minor, category_ref, date)
get_goal_status(as_of)
simulate_goal(goal_id, monthly_contribution_minor)
get_asset_allocation(as_of)
generate_monthly_report(year, month)
```

### 5.1 Household scoping

MCP tool schemas must **not** contain `household_id`.

The authenticated MCP principal supplies the household scope server-side:

```text
Bearer credential
      |
      v
MCP principal config
      |
      v
household scope
      |
      v
agentadapter.Service
      |
      v
Finance Core operation
```

This prevents prompt text, model behavior, or a malicious client from selecting another household.

### 5.2 Deterministic parity

For identical business arguments and the same scoped household/data snapshot:

```text
internal typed API result == MCP tool business result
```

The MCP layer may add transport metadata such as request IDs, audit IDs, `as_of`, data-quality flags, and warnings, but it may not recompute money, percentages, debt schedules, budgets, safe-to-spend, goal dates, or asset allocation.

### 5.3 Tool annotations

Where supported by the selected stable MCP SDK/spec version, all V2.0 tools are annotated as non-destructive/read-only from an external side-effect perspective. Simulation tools are computational operations and do not persist changes.

Tool descriptions must state that returned financial values come from Finance Core and that `stale` / `partial` data flags must be preserved by clients and agents.

## 6. Result envelope

The protocol-neutral adapter returns a common metadata envelope around typed business data:

```text
ToolResult[T]
- data: T
- as_of: timestamp/date when meaningful
- quality: complete | partial | stale
- warnings: []stable_warning_code
- audit_id: opaque identifier
```

Rules:

- Money remains in existing exact integer/minor-unit or domain Money representations until final JSON serialization.
- No float-based financial recalculation is introduced in the adapter.
- Raw account numbers, bank card numbers, API tokens, secrets, or full source statements are never added to the result envelope.
- Transaction detail is returned only when an existing approved tool contract explicitly requires it, and existing redaction rules remain in force.

## 7. Authentication and authorization

### 7.1 V2.0 credential model

V2.0 intentionally uses one dedicated MCP bearer credential scoped to one configured household.

Configuration shape:

```env
MCP_ENABLED=false
MCP_TOKEN_FILE=/run/secrets/finance-mcp-token
MCP_HOUSEHOLD_ID=1
MCP_ALLOWED_ORIGINS=https://trusted-agent.example
MCP_REQUEST_TIMEOUT=15s
MCP_MAX_CONCURRENT=4
MCP_MAX_BODY_BYTES=262144
```

Exact environment names may be adjusted to match existing configuration conventions during implementation, but the security properties are mandatory.

The token file must be outside Git and readable only by the service account/container. The application reads the token at startup and compares bearer credentials in constant time.

### 7.2 Why not OAuth yet

The V2.0 deployment is a single-family system with a narrow trusted-agent use case. Full MCP OAuth would introduce authorization-server discovery, client registration/metadata, token persistence, scopes, redirects, and operational burden before there is a multi-principal requirement.

OAuth/RBAC becomes appropriate when Family RBAC (roadmap V1.4) or multiple external principals are actually required. The `agentadapter` principal/scope abstraction must be designed so the credential backend can later change without changing tool implementations.

### 7.3 Authorization rules

- An authenticated principal maps to exactly one household in V2.0.
- No tool parameter can override that household.
- Only an explicit allowlist of tool names is registered.
- Unknown tools fail closed.
- No database or ledger write path is reachable from the V2.0 adapter.

## 8. HTTP and protocol security

The MCP endpoint is `/mcp` under the Finance Core HTTPS origin.

Mandatory controls:

1. TLS terminates at Caddy; plain external HTTP is redirected/rejected according to existing edge policy.
2. Validate `Origin` when present according to the selected MCP transport behavior; reject unapproved origins with 403.
3. Validate bearer authentication before tool dispatch.
4. Enforce `Content-Type` and MCP protocol headers using the official SDK where applicable.
5. Enforce a request body size limit before JSON decoding.
6. Enforce per-request context timeout.
7. Enforce a small in-process concurrency limit; no Redis/distributed limiter is introduced.
8. Reject malformed/mismatched MCP routing headers and body fields through SDK/spec validation.
9. Do not enable legacy HTTP+SSE solely for compatibility unless a real required client cannot use Streamable HTTP and an explicit follow-up decision approves it.
10. Do not expose MCP on a separate host port.

A simple in-process rate/concurrency guard is sufficient for household scale. It must fail closed under overload and return a controlled, non-secret-bearing error.

## 9. Error model

`agentadapter` defines stable application-facing error codes independent of MCP wire errors. At minimum:

```text
unauthorized
forbidden
invalid_argument
tool_not_found
data_unavailable
data_partial
timeout
busy
internal
```

Mapping rules:

- Validation errors identify the invalid field without dumping raw request bodies.
- Provider/database/internal stack errors are never returned verbatim to the agent.
- Tool failures preserve stable error codes and safe messages.
- `partial`/`stale` financial data is represented explicitly rather than silently converted to complete data.
- Logs may contain request ID, tool name, duration, status, and stable error code; they must not contain bearer tokens or raw sensitive payloads.

## 10. Audit

Every `tools/call` is auditable independently of AI Advisor conversations.

Preferred persisted audit record:

```text
agent_tool_audit
- id
- created_at
- principal_kind
- household_id
- protocol
- protocol_version
- client_name/version when available
- tool_name
- input_hash
- output_hash
- data_as_of
- status
- error_code
- duration_ms
```

Do not persist bearer credentials or full raw tool payloads in the audit table.

Hashing must use a canonicalized representation so the same logical input/output can be compared reproducibly. Audit storage failure should not silently convert a failed financial operation into success; exact fail-open/fail-closed behavior will be defined per audit stage in implementation tests, with security/audit integrity preferred over availability for external agent calls.

## 11. OpenClaw integration

OpenClaw is the first acceptance client, not a framework dependency.

Expected external configuration shape:

```text
mcp.servers.finance
- url: https://<finance-domain>/mcp
- transport: streamable-http
- Authorization: Bearer <secret from external secret store>
- request timeout
- optional include-only tool filter
```

Acceptance evidence must use real OpenClaw commands available at test time, including live probe/doctor capability discovery, followed by at least one real read tool and one simulation tool through an OpenClaw-managed agent/runtime.

No Finance Core package imports OpenClaw code, config formats, or SDKs.

## 12. Configuration and startup behavior

`MCP_ENABLED=false` is the default.

When disabled:

- `/mcp` is not registered or returns a deliberate not-found response according to existing router conventions;
- MCP token configuration is not required;
- no MCP background process exists.

When enabled, startup fails fast if:

- token file is missing/unreadable/empty;
- household scope is invalid or cannot be resolved;
- allowed-origin syntax is invalid;
- timeout/concurrency/body-size values are invalid;
- MCP adapter registration contains duplicate/unknown tool definitions.

The application must never start an externally reachable, unauthenticated MCP endpoint because configuration was incomplete.

## 13. Testing strategy

### 13.1 Agent adapter unit tests

Each tool mapping has tests proving:

- household is injected server-side;
- client input cannot override household scope;
- arguments map to the existing typed API exactly;
- business results pass through without financial recomputation;
- `stale`/`partial`/warning metadata is preserved;
- adapter error codes are stable.

### 13.2 MCP protocol tests

Using the official Go SDK test client/transport where possible:

- discovery/version negotiation for supported protocol versions;
- tools list contains only the approved allowlist;
- JSON schemas match typed inputs;
- valid `tools/call` works;
- invalid arguments fail safely;
- unknown tool fails closed;
- invalid/missing bearer fails;
- invalid Origin fails;
- mismatched protocol routing metadata fails on versions where required;
- body-size limit, timeout, cancellation, and concurrency behavior;
- no secret-bearing fields in responses/log fixtures.

Tests must be version-aware: legacy lifecycle behavior through `2025-11-25` must not be incorrectly asserted against the stateless `2026-07-28` protocol.

### 13.3 Deterministic parity integration tests

For representative tools, invoke the existing Finance Core API and the MCP adapter against the same database snapshot and compare the normalized business payload. Differences in transport metadata are ignored; financial/domain values must match exactly.

### 13.4 OpenClaw live smoke

Production/staging acceptance includes:

1. configure remote Streamable HTTP MCP server using a secret reference/environment variable;
2. `openclaw mcp doctor --probe` or the current equivalent succeeds;
3. expected Finance tools are discoverable;
4. one overview/cashflow call returns controlled real data;
5. one purchase/debt/goal simulation returns a deterministic result;
6. an invalid credential is rejected;
7. evidence is sanitized and contains no bearer token or raw sensitive financial statement.

## 14. CI and dependency gates

V2 work must preserve all existing V1 CI gates.

New gates:

- `go test ./internal/agentadapter/...`
- MCP adapter tests
- race coverage for MCP concurrent calls
- `govulncheck ./...` including the MCP SDK dependency tree
- exact `go mod tidy` cleanliness
- MCP dependency must be an official `modelcontextprotocol` Go SDK release
- production merge rejects a pre-release MCP SDK version
- container build and edge-exposure checks remain green

The repository must not add Node/TypeScript solely to implement MCP because Finance Core is already Go and the official Go SDK is the natural integration boundary.

## 15. Rollout

V2 development is isolated from V1 production acceptance.

Recommended sequence:

1. Agent adapter boundary and parity tests.
2. Audit persistence.
3. MCP SDK dependency gate/spike.
4. MCP tool registration and protocol tests.
5. Auth/origin/timeout/concurrency guardrails.
6. Application/Caddy wiring without new exposed ports.
7. CI gates.
8. Staging OpenClaw live probe + calls.
9. Security review.
10. Merge V2.0 only after V1 behavior remains unchanged and all V2 gates pass.

V1 may be tagged from its validated release candidate independently. V2 branch commits are never substituted as V1 acceptance evidence.

## 16. Deferred extensions

The following are explicit later decisions, not hidden V2.0 scope:

- multiple MCP principals and Family RBAC integration;
- MCP OAuth/OIDC;
- tool write capabilities with human confirmation;
- MCP Apps/UI resources;
- asynchronous Tasks;
- local P40/vision workers;
- market-data tools;
- Capacitor/native-mobile channel;
- additional agent protocols.

Each requires a separate design/acceptance gate.

## 17. Exit criteria

V2.0 is complete only when all are true:

1. Existing Finance Core typed calculations remain the only source of financial truth.
2. MCP is an edge adapter inside the existing application, with no extra service/database.
3. Approved read/simulation tools are exposed and no write tool is reachable.
4. Household scope is server-side and cross-household selection is impossible by tool arguments.
5. Bearer authentication, Origin validation, body limits, timeouts, and concurrency limits are tested.
6. MCP tool results match existing Finance Core business results exactly for parity fixtures.
7. Tool calls produce sanitized audit records.
8. Existing V1 CI/security/backup tests remain green.
9. The production merge uses a stable official MCP Go SDK; no pre-release SDK is shipped.
10. A real OpenClaw Streamable HTTP probe and representative tool calls pass with sanitized evidence.
11. No new host ports, secrets in Git, or raw sensitive data in logs are introduced.

## 18. Architecture decision summary

**Decision:** implement Finance Core-native MCP as an adapter over the existing typed Finance Tool Contract.

**Rejected:** independent MCP sidecar, OpenClaw-specific finance plugin, direct database access, pre-release SDK in production, and early OAuth/RBAC complexity.

This preserves the project rule: keep the deterministic core small and stable; add replaceable integration complexity only at the edge.
