# V2 MCP Transport Implementation Plan

**Goal:** Expose the nine `READY` Agent Adapter tools over the official stable MCP Go SDK using the MCP `2025-11-25` Streamable HTTP lifecycle, without adding authentication/Origin/rate-limit/Caddy/config wiring yet.

**Dependency gate:** As verified on 2026-08-18, use `github.com/modelcontextprotocol/go-sdk` stable `v1.6.1`. Do not use `v1.7.0-pre.*`. The stable line supports MCP through `2025-11-25`; the future `2026-07-28` upgrade remains a separate compatibility milestone after a stable v1.7+ release.

**Architecture:** `internal/mcpadapter` depends on `internal/agentadapter` and the official `mcp` package only. External protocol code receives an `*agentadapter.AuditedService`, never the unaudited `Service`. The exact input schemas from `agentadapter.Definitions()` are registered via raw `Server.AddTool`; the SDK does not regenerate business schemas. The fixed server-side `Principal` is captured when the MCP server is built and never comes from tool arguments.

**Protocol mode:** For stable `2025-11-25`, use the SDK's normal initialized/stateful lifecycle. `StreamableHTTPOptions.JSONResponse=true`; leave `Stateless=false`. The SDK documentation explicitly describes stateless mode as outside/directly undefined by this spec generation, so V2 does not depend on it before the 2026 protocol upgrade.

**Non-goals:** bearer auth, Origin allowlist, body limits, timeout, rate/concurrency guardrails, Caddy route, `.env` configuration, OpenClaw live production configuration, and host exposure changes are all deferred to the MCP HTTP Security/Application plan.

## Task 1 — Add stable SDK dependency while preserving a behavioral RED

Files:
- Modify `go.mod`, `go.sum` through CI-generated `go mod tidy` output.
- Create `internal/mcpadapter/server_test.go` first.

Test imports the official `mcp` package and requires a not-yet-existing `NewServer`/`NewHTTPHandler` API. Because `go mod tidy` runs before `go vet`, first accept the expected intermediate module-file diff, commit the exact CI-generated dependency graph, then rerun to obtain the real behavioral RED (`undefined: NewServer`) before writing production MCP code.

No blank-import production shim is allowed merely to retain the module.

## Task 2 — Make `AuditedService` expose immutable definitions

Files:
- Modify `internal/agentadapter/audit.go`
- Modify `internal/agentadapter/audit_test.go`

Add:

```go
func (s *AuditedService) Definitions() []ToolDefinition
```

It forwards to the base service's defensive-copy definitions. Test mutates the returned schema and proves a second call is unchanged. External protocols must not need access to the unaudited service just to list tools.

## Task 3 — Register exactly the READY tools using raw MCP schemas

Create:
- `internal/mcpadapter/server.go`
- `internal/mcpadapter/server_test.go`

Locked constructor:

```go
type ServerOptions struct {
    Name      string
    Version   string
    Principal agentadapter.Principal
}

func NewServer(service *agentadapter.AuditedService, opts ServerOptions) (*mcp.Server, error)
```

Validation:
- audited service required;
- non-empty implementation name/version;
- principal kind non-empty and household ID > 0;
- duplicate tool names rejected before registration.

For every `ToolDefinition`, register:

```go
&mcp.Tool{
    Name:        string(def.Name),
    Description: def.Description,
    InputSchema: clone(def.InputSchema),
    Annotations: &mcp.ToolAnnotations{
        ReadOnlyHint:    true,
        DestructiveHint: boolPtr(false),
        IdempotentHint:  true,
        OpenWorldHint:   boolPtr(false),
    },
}
```

Use `server.AddTool`, not generic `mcp.AddTool`, so the Agent Adapter remains the schema authority.

Tests via official in-memory transports:
1. connect `mcp.Server` and official `mcp.Client` using `mcp.NewInMemoryTransports()`;
2. `ListTools` returns exactly the nine READY names;
3. every MCP tool's `InputSchema` is JSON-equivalent to the Agent Adapter schema and contains no `household_id`;
4. annotations are read-only/non-destructive/idempotent/closed-world;
5. no resources/prompts are added.

## Task 4 — Map MCP tool calls to audited deterministic calls

Create/modify:
- `internal/mcpadapter/server.go`
- `internal/mcpadapter/server_test.go`

Handler algorithm:

1. Read `req.Session.InitializeParams()`.
2. Build `agentadapter.CallMetadata`:
   - `Protocol = "mcp"`
   - `ProtocolVersion = init.ProtocolVersion`
   - `ClientName/ClientVersion` from `init.ClientInfo` when non-nil.
3. Call `AuditedService.Call(ctx, fixedPrincipal, metadata, ToolName(req.Params.Name), req.Params.Arguments)`.
4. On success, JSON-encode the full `agentadapter.Result` envelope. Return:
   - `StructuredContent = result`;
   - one `TextContent` containing the same JSON for clients that primarily consume content blocks;
   - `IsError = false`.
5. On a stable adapter error, return a safe object `{error_code, message}` as both structured content and JSON text content with `IsError=true`; return a nil Go error so a business/tool error is not converted into an MCP protocol failure.
6. If an unexpected non-adapter error escapes, return only `internal` + generic `tool call failed`; never expose `err.Error()`.

Tests:
- valid overview call uses the constructor's household and produces a successful completed audit with non-empty `audit_*` ID;
- invalid `household_id` argument is `IsError=true`/`invalid_argument`, base backend not executed, and the failed external attempt is still audited;
- a backend/database error does not appear in MCP content;
- client name/version/protocol version observed by the audit recorder match the actual official MCP client initialization;
- result `data`, `as_of`, `quality`, `warnings`, and `audit_id` survive the wire mapping.

## Task 5 — Add Streamable HTTP handler and HTTP protocol smoke

Create:
- `internal/mcpadapter/http.go`
- `internal/mcpadapter/http_test.go`

Locked API:

```go
func NewHTTPHandler(server *mcp.Server) (http.Handler, error)
```

Implementation uses:

```go
mcp.NewStreamableHTTPHandler(
    func(*http.Request) *mcp.Server { return server },
    &mcp.StreamableHTTPOptions{JSONResponse: true},
)
```

Do not set `Stateless=true` on the stable `2025-11-25` path.

HTTP test:
1. `httptest.NewServer(handler)`;
2. official `mcp.Client` connects with `mcp.StreamableClientTransport{Endpoint: ts.URL}`;
3. initialization succeeds and negotiated protocol is the stable supported revision;
4. `ListTools` returns nine;
5. one overview call returns `IsError=false` with audit ID;
6. close the client session cleanly.

This proves the remote Streamable HTTP path before any application/Caddy/security wiring.

## Task 6 — CI and dependency gates

Modify `.github/workflows/ci.yml` only if needed to add an explicit:

```bash
go test ./internal/mcpadapter -v
```

The existing `govulncheck ./...`, `go mod tidy` cleanliness, race, and final build remain mandatory.

Add a production dependency guard script/test that fails if the selected MCP module version contains a pre-release suffix. Prefer a small shell check in `scripts/test-production-ops.sh` or a Go build-time dependency test, without network access:

```text
require github.com/modelcontextprotocol/go-sdk v1.6.1 (or later stable selected by an approved update)
reject -pre, -rc, -beta, -alpha in the selected version
```

For this implementation commit, exact version is `v1.6.1`.

## Completion gate

For the exact final MCP transport commit require:
- nine MCP tools only;
- exact Agent Adapter input schemas;
- no household ID in schemas;
- all MCP calls pass through `AuditedService`;
- official in-memory protocol tests success;
- official Streamable HTTP `httptest` success;
- safe tool-error mapping with no backend text leak;
- stable `go-sdk v1.6.1`, no pre-release dependency;
- gofmt, vet, full tests, govulncheck, explicit agent audit PostgreSQL integration, race, binary build all green;
- Web/Container/Edge Security remain green;
- no `/mcp` application route or production exposure yet.

Only after this plan is green may the MCP HTTP Security/Application wiring subproject begin.
