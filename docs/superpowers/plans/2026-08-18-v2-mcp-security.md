# V2 MCP Security Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Protect the V2 MCP Streamable HTTP endpoint with fail-closed static bearer authentication, Origin validation, bounded request resources, server-side household scope, startup validation, and edge routing that preserves the existing Finance UI Basic Auth.

**Architecture:** `internal/mcpadapter` owns protocol-edge security middleware and remains independent of deployment/config file I/O. `internal/config` parses the MCP feature gate and numeric policy. `cmd/finance-core` loads the token secret, verifies the configured household, builds the audited MCP stack, and registers `/mcp` only when enabled. Caddy routes exact `/mcp` around Finance Basic Auth because both Basic and Bearer use the `Authorization` header; all other Finance routes keep Basic Auth.

**Tech Stack:** Go 1.26.6 `net/http`, `crypto/sha256`, `crypto/subtle`, official MCP Go SDK `v1.6.1`, PostgreSQL 18/sqlc, Caddy 2.11.4, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-08-18-v2-agent-adapter-mcp-design.md`

## Global Constraints

- MCP SDK production dependency remains exactly `github.com/modelcontextprotocol/go-sdk v1.6.1`; pre-release SDK versions are forbidden.
- `MCP_ENABLED=false` is the default and must not require any MCP secret file.
- The MCP client cannot provide or override `household_id`; the authenticated endpoint is scoped to one configured household.
- External MCP calls remain fail-closed on audit persistence.
- The static bearer token is never logged, persisted, returned in errors, stored in Git, or accepted via query string.
- When `Origin` is present it must be valid same-origin or an exact configured trusted origin; invalid origins return 403 before MCP dispatch.
- Non-browser clients may omit `Origin`.
- Default request timeout is `15s`, maximum concurrency is `4`, request rate is `60/minute`, and maximum body size is `262144` bytes.
- No Redis, sidecar, distributed rate limiter, OAuth server, or new host port is introduced.
- `/mcp` is not mounted into the production router until every middleware/config task below is GREEN.
- Finance UI/API Basic Auth remains mandatory for every Finance route except exact `/mcp`.

---

### Task 1: Preserve timeout semantics at the Agent Adapter boundary

**Files:**
- Create: `internal/agentadapter/timeout_test.go`
- Modify: `internal/agentadapter/service.go`

**Interfaces:**
- Consumes: existing `encodeBackendResult` and stable error taxonomy.
- Produces: backend `context.DeadlineExceeded` or `context.Canceled` maps to `CodeTimeout` with a safe message; other backend failures remain `CodeDataUnavailable`.

- [ ] **Step 1: Write the failing tests**

```go
func TestEncodeBackendResultMapsDeadlineToTimeout(t *testing.T) {
    _, err := encodeBackendResult(struct{}{}, context.DeadlineExceeded, nil, "", nil)
    var adapterErr *Error
    if !errors.As(err, &adapterErr) || adapterErr.Code != CodeTimeout {
        t.Fatalf("error=%v want timeout", err)
    }
}

func TestEncodeBackendResultMapsCancellationToTimeout(t *testing.T) {
    _, err := encodeBackendResult(struct{}{}, context.Canceled, nil, "", nil)
    var adapterErr *Error
    if !errors.As(err, &adapterErr) || adapterErr.Code != CodeTimeout {
        t.Fatalf("error=%v want timeout", err)
    }
}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/agentadapter -run TestEncodeBackendResultMaps -v`

Expected: FAIL because both errors currently map to `data_unavailable`.

- [ ] **Step 3: Implement the minimal mapping**

Before the generic backend error branch in `encodeBackendResult`:

```go
if errors.Is(backendErr, context.DeadlineExceeded) || errors.Is(backendErr, context.Canceled) {
    return Result{}, adapterError(CodeTimeout, "tool execution timed out", backendErr)
}
```

Add `errors` to `service.go` imports.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
go test ./internal/agentadapter -run TestEncodeBackendResultMaps -v
go test ./internal/agentadapter ./internal/mcpadapter
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agentadapter/service.go internal/agentadapter/timeout_test.go
git commit -m "fix(v2): preserve MCP timeout semantics"
```

---

### Task 2: Build the fail-closed MCP security middleware

**Files:**
- Create: `internal/mcpadapter/security.go`
- Create: `internal/mcpadapter/security_test.go`
- Keep unchanged: `internal/mcpadapter/http.go` as the protocol-only Streamable HTTP constructor.

**Interfaces:**
- Consumes: a protocol-only `http.Handler` returned by `NewHTTPHandler`.
- Produces:

```go
type SecurityOptions struct {
    Token             []byte
    AllowedOrigins    []string
    RequestTimeout    time.Duration
    MaxConcurrent     int
    RequestsPerMinute int
    MaxBodyBytes      int64
}

func NewSecureHTTPHandler(next http.Handler, opts SecurityOptions) (http.Handler, error)
```

Internal deterministic test hook:

```go
func newSecureHTTPHandlerWithClock(next http.Handler, opts SecurityOptions, now func() time.Time) (http.Handler, error)
```

- [ ] **Step 1: Write constructor/authentication RED tests**

Cover exactly:

```go
func TestNewSecureHTTPHandlerRejectsInvalidOptions(t *testing.T)
func TestSecureHTTPHandlerRequiresBearerOnEveryMethod(t *testing.T)
func TestSecureHTTPHandlerUsesConstantLengthTokenDigest(t *testing.T)
```

Invalid options: nil handler, empty token, timeout `<=0`, concurrency `<=0`, rate `<=0`, body bytes `<=0`, malformed trusted origin. Exercise GET, POST, and DELETE without/malformed/wrong bearer and require 401 without invoking `next`.

Authentication implementation hashes the configured and presented token to fixed-size SHA-256 values and compares them with `subtle.ConstantTimeCompare`:

```go
expected := sha256.Sum256(opts.Token)
presented := sha256.Sum256([]byte(token))
if subtle.ConstantTimeCompare(expected[:], presented[:]) != 1 {
    writeSecurityError(w, http.StatusUnauthorized, "unauthorized", "MCP bearer token is invalid")
    return
}
```

Parse exactly one `Authorization: Bearer <token>` value. Do not inspect query parameters.

- [ ] **Step 2: Verify authentication RED**

Run: `go test ./internal/mcpadapter -run 'Test(NewSecure|SecureHTTPHandlerRequires|SecureHTTPHandlerUses)' -v`

Expected: FAIL because `SecurityOptions`/`NewSecureHTTPHandler` do not exist.

- [ ] **Step 3: Implement authentication and safe security error envelopes**

Use a minimal HTTP-level envelope:

```go
type securityError struct {
    ErrorCode string `json:"error_code"`
    Message   string `json:"message"`
}
```

For 401 set `WWW-Authenticate: Bearer`; never echo the presented credential.

- [ ] **Step 4: Add Origin validation RED tests**

Cover POST and GET, because the MCP transport specification requires Origin validation for all incoming connections:

```go
func TestSecureHTTPHandlerOriginPolicy(t *testing.T)
```

Cases:
- no `Origin`: allowed;
- same origin (`Origin: https://finance.example`, `Host: finance.example`, `X-Forwarded-Proto: https`): allowed;
- exact `AllowedOrigins` entry: allowed;
- malformed origin, `null`, wrong scheme/host/port, path/query/fragment, or untrusted origin: 403 and `next` not called.

Origin canonical validation uses `url.Parse`, requires `http` or `https`, non-empty host, and empty path/query/fragment. Same-origin comparison uses `X-Forwarded-Proto` when present, otherwise `https` for `r.TLS != nil`, otherwise `http`.

- [ ] **Step 5: Implement Origin validation and verify GREEN**

Run: `go test ./internal/mcpadapter -run TestSecureHTTPHandlerOriginPolicy -v`

Expected: PASS.

- [ ] **Step 6: Add body-size RED tests**

```go
func TestSecureHTTPHandlerRejectsOversizedBodyBeforeDispatch(t *testing.T)
```

Use both a known `Content-Length` and a reader with unknown length. `MaxBodyBytes+1` must return 413, must not invoke `next`, and response must not contain request body bytes.

Implementation reads at most `MaxBodyBytes+1` bytes after auth/origin/resource admission and replaces `r.Body` with an `io.NopCloser(bytes.NewReader(body))` only when size is accepted.

- [ ] **Step 7: Add timeout RED tests**

```go
func TestSecureHTTPHandlerAppliesRequestTimeout(t *testing.T)
```

`next` waits on `r.Context().Done()` and records `context.DeadlineExceeded`. Middleware uses `context.WithTimeout` around body-limit + MCP handler dispatch.

- [ ] **Step 8: Add concurrency RED tests**

```go
func TestSecureHTTPHandlerRejectsConcurrencyOverflow(t *testing.T)
```

With `MaxConcurrent=1`, block the first authenticated request in `next`; the second authenticated request must fail immediately with 503 and `error_code=busy`, without entering `next`.

Use a buffered channel semaphore:

```go
select {
case semaphore <- struct{}{}:
    defer func() { <-semaphore }()
default:
    writeSecurityError(w, http.StatusServiceUnavailable, "busy", "MCP endpoint is busy")
    return
}
```

- [ ] **Step 9: Add deterministic fixed-window rate RED tests**

```go
func TestSecureHTTPHandlerEnforcesPerProcessRateLimit(t *testing.T)
```

With `RequestsPerMinute=2`, allow two authenticated requests, reject the third with 429 and `error_code=busy`; advance the injected clock by one minute and require the next request to succeed. The limiter is process-local and mutex-protected.

- [ ] **Step 10: Implement timeout/concurrency/rate/body middleware in this order**

Outer to inner:

```text
Origin validation
Bearer authentication
Rate limit
Concurrency admission
Request timeout
Body-size pre-read
MCP Streamable HTTP handler
```

This rejects untrusted/unauthenticated requests before they consume execution capacity.

- [ ] **Step 11: Verify complete security middleware**

Run:

```bash
go test ./internal/mcpadapter -v
go test -race ./internal/mcpadapter
```

Expected: PASS.

- [ ] **Step 12: Commit**

```bash
git add internal/mcpadapter/security.go internal/mcpadapter/security_test.go
git commit -m "feat(v2): secure MCP HTTP boundary"
```

---

### Task 3: Add fail-fast MCP runtime configuration

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `.env.example`

**Interfaces:**
- Produces:

```go
type MCPConfig struct {
    Enabled           bool
    TokenFile         string
    HouseholdID       int64
    AllowedOrigins    []string
    RequestTimeout    time.Duration
    MaxConcurrent     int
    RequestsPerMinute int
    MaxBodyBytes      int64
}
```

and `Config` gains `MCP MCPConfig`.

- [ ] **Step 1: Write disabled-default RED test**

No MCP environment variables beyond the existing mandatory database config:

```go
if cfg.MCP.Enabled {
    t.Fatal("MCP must be disabled by default")
}
```

Disabled mode must not require `MCP_TOKEN_FILE` or `MCP_HOUSEHOLD_ID`.

- [ ] **Step 2: Write enabled-valid RED test**

Use:

```text
MCP_ENABLED=true
MCP_TOKEN_FILE=/run/secrets/finance-mcp-token
MCP_HOUSEHOLD_ID=42
MCP_ALLOWED_ORIGINS=https://trusted.example,https://ops.example:8443
MCP_REQUEST_TIMEOUT=15s
MCP_MAX_CONCURRENT=4
MCP_REQUESTS_PER_MINUTE=60
MCP_MAX_BODY_BYTES=262144
```

Require exact parsed values.

- [ ] **Step 3: Write fail-fast table tests**

When enabled reject:
- missing/blank token file;
- household ID `<=0` or non-integer;
- malformed origin list entry;
- timeout `<=0` or invalid duration;
- concurrency `<=0`/invalid;
- rate `<=0`/invalid;
- body bytes `<=0`/invalid;
- invalid `MCP_ENABLED` boolean.

- [ ] **Step 4: Implement parser with defaults**

Defaults when enabled and values omitted:

```go
TokenFile:         "/run/secrets/finance-mcp-token"
RequestTimeout:    15 * time.Second
MaxConcurrent:     4
RequestsPerMinute: 60
MaxBodyBytes:      262144
```

`MCP_HOUSEHOLD_ID` remains mandatory when enabled. `MCP_ALLOWED_ORIGINS` may be empty; same-origin requests and non-browser clients are still supported safely.

- [ ] **Step 5: Document `.env.example` without any secret value**

Add:

```dotenv
MCP_ENABLED=false
MCP_TOKEN_FILE=/run/secrets/finance-mcp-token
MCP_HOUSEHOLD_ID=
MCP_ALLOWED_ORIGINS=
MCP_REQUEST_TIMEOUT=15s
MCP_MAX_CONCURRENT=4
MCP_REQUESTS_PER_MINUTE=60
MCP_MAX_BODY_BYTES=262144
```

- [ ] **Step 6: Verify GREEN**

Run: `go test ./internal/config -v`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go .env.example
git commit -m "feat(v2): add fail-fast MCP configuration"
```

---

### Task 4: Mount MCP in Finance Core only after startup validation succeeds

**Files:**
- Modify: `internal/server/api.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/finance-core/application.go`
- Modify: `cmd/finance-core/application_test.go`
- Create: `cmd/finance-core/mcp.go`
- Create: `cmd/finance-core/mcp_test.go`

**Interfaces:**
- `server` gains:

```go
func WithMCP(handler http.Handler) HandlerOption
```

- `cmd/finance-core/mcp.go` produces:

```go
func buildMCPHandler(ctx context.Context, cfg config.MCPConfig, pool *pgxpool.Pool, financeAPI *appapi.API) (http.Handler, error)
func loadMCPToken(path string) ([]byte, error)
```

- [ ] **Step 1: Write router RED tests**

When `WithMCP` is absent, `POST /mcp` must reach the existing web/not-found behavior and never an MCP handler. When present, GET/POST/DELETE exact `/mcp` must reach the supplied handler; `/mcp/anything` must not.

- [ ] **Step 2: Implement `WithMCP` and exact route registration**

Extend `handlerConfig` with `mcp http.Handler`; in `NewHandler` register before the web fallback:

```go
if cfg.mcp != nil {
    mux.Handle("/mcp", cfg.mcp)
}
```

- [ ] **Step 3: Write token-file RED tests**

`loadMCPToken` must reject missing file, directory path, empty/whitespace-only file, and token containing ASCII whitespace. It returns a copied, trimmed byte slice and never includes token bytes in error text.

- [ ] **Step 4: Implement token loading**

Use `os.ReadFile`, cap accepted file size at 4096 bytes, `bytes.TrimSpace`, and reject any remaining `unicode.IsSpace` rune/ASCII whitespace. Error messages include only the path/context, never file contents.

- [ ] **Step 5: Write enabled startup RED integration test**

Using the existing PostgreSQL integration fixture:
1. create a household with ID `householdID`;
2. write a temporary secret file;
3. build `MCPConfig{Enabled:true,...}`;
4. call `buildMCPHandler`;
5. connect an official `StreamableClientTransport` with an HTTP client that adds `Authorization: Bearer <test-token>` to every request;
6. require initialize, exactly 9 tools, and one overview call to work.

A configured nonexistent household must make startup fail before a handler is returned.

- [ ] **Step 6: Implement the audited MCP stack**

Build only when enabled:

```go
base, err := agentadapter.New(financeAPI)
audited, err := agentadapter.NewAudited(base, audit.NewAgentPostgresRecorder(pool), time.Now)
mcpServer, err := mcpadapter.NewServer(audited, mcpadapter.ServerOptions{
    Name:      "family-finance-os",
    Version:   "v2",
    Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: cfg.HouseholdID},
})
transport, err := mcpadapter.NewHTTPHandler(mcpServer)
secure, err := mcpadapter.NewSecureHTTPHandler(transport, mcpadapter.SecurityOptions{
    Token:             token,
    AllowedOrigins:    cfg.AllowedOrigins,
    RequestTimeout:    cfg.RequestTimeout,
    MaxConcurrent:     cfg.MaxConcurrent,
    RequestsPerMinute: cfg.RequestsPerMinute,
    MaxBodyBytes:      cfg.MaxBodyBytes,
})
```

Verify household existence first using committed sqlc `GetHousehold`; do not call the ledger merely to validate scope.

- [ ] **Step 7: Wire `buildApplicationHandler`**

If `cfg.MCP.Enabled` is false, behavior and required files remain identical to V1. If true, build the secure handler and add `server.WithMCP(mcpHandler)` to `server.NewHandler`.

- [ ] **Step 8: Verify GREEN**

Run:

```bash
go test ./internal/server ./cmd/finance-core -v
go test ./cmd/finance-core -run MCP -v
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/server cmd/finance-core
git commit -m "feat(v2): mount MCP after secure startup validation"
```

---

### Task 5: Resolve Caddy Basic/Bearer conflict without opening a new port

**Files:**
- Modify: `Caddyfile`
- Modify: `compose.yaml`
- Modify: `.gitignore`
- Create: `secrets/.gitkeep`
- Modify: `scripts/check-edge-security.sh`
- Modify: `scripts/test-production-ops.sh` if its environment contract requires the new optional MCP variables.

**Interfaces:**
- Caddy keeps ports `80/tcp`, `443/tcp`, `443/udp` as the only host-published ports.
- Exact `/mcp` bypasses Finance Basic Auth solely to allow the application-level MCP Bearer header.
- All other Finance paths retain Basic Auth.

- [ ] **Step 1: Write edge-contract RED assertions**

`check-edge-security.sh` must require:
- an exact `@mcp path /mcp` matcher;
- an MCP `handle @mcp` block with `reverse_proxy finance-core:8000` and no `basic_auth`;
- a fallback Finance `handle` block that contains the existing Basic Auth variables;
- no second host port/service for MCP;
- Finance Core receives optional MCP configuration but no literal bearer token environment variable.

- [ ] **Step 2: Change Caddy routing**

Use:

```caddyfile
{$FINANCE_DOMAIN} {
    encode zstd gzip

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

- [ ] **Step 3: Add a non-secret directory mount**

`.gitignore`:

```gitignore
secrets/*
!secrets/.gitkeep
```

Compose Finance Core:

```yaml
volumes:
  - ./secrets:/run/secrets:ro
environment:
  MCP_ENABLED: ${MCP_ENABLED:-false}
  MCP_TOKEN_FILE: ${MCP_TOKEN_FILE:-/run/secrets/finance-mcp-token}
  MCP_HOUSEHOLD_ID: ${MCP_HOUSEHOLD_ID:-}
  MCP_ALLOWED_ORIGINS: ${MCP_ALLOWED_ORIGINS:-}
  MCP_REQUEST_TIMEOUT: ${MCP_REQUEST_TIMEOUT:-15s}
  MCP_MAX_CONCURRENT: ${MCP_MAX_CONCURRENT:-4}
  MCP_REQUESTS_PER_MINUTE: ${MCP_REQUESTS_PER_MINUTE:-60}
  MCP_MAX_BODY_BYTES: ${MCP_MAX_BODY_BYTES:-262144}
```

No `MCP_TOKEN` environment variable exists.

- [ ] **Step 4: Verify edge/ops GREEN**

Run:

```bash
bash scripts/check-edge-security.sh
bash scripts/test-production-ops.sh
docker compose config >/dev/null
```

Expected: PASS with `MCP_ENABLED=false` and no secret file.

- [ ] **Step 5: Commit**

```bash
git add Caddyfile compose.yaml .gitignore secrets/.gitkeep scripts/check-edge-security.sh scripts/test-production-ops.sh
git commit -m "feat(v2): route MCP through protected HTTPS edge"
```

---

### Task 6: Add explicit CI security gates and finalize the subproject

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `docs/superpowers/specs/2026-08-18-v2-agent-adapter-mcp-design.md`
- Create: `docs/v2-mcp-security-acceptance.md`

**Interfaces:**
- CI exposes MCP security as named gates rather than relying only on `go test ./...`.

- [ ] **Step 1: Add explicit Go security test gate**

After `MCP adapter contract`:

```yaml
- name: MCP security contract
  run: go test ./internal/mcpadapter -run 'Security|Origin|Bearer|Body|Timeout|Concurrent|Rate' -v
```

- [ ] **Step 2: Add application MCP integration gate**

With the existing PostgreSQL service variables:

```yaml
- name: MCP application integration
  env:
    TEST_POSTGRES_HOST: 127.0.0.1
    TEST_POSTGRES_PORT: '5432'
    TEST_POSTGRES_DB: finance
    TEST_POSTGRES_USER: finance_app
    TEST_POSTGRES_PASSWORD: test-secret
  run: go test ./cmd/finance-core -run MCP -v
```

- [ ] **Step 3: Document acceptance boundaries**

`docs/v2-mcp-security-acceptance.md` records automated PASS requirements:
- bearer missing/wrong/valid;
- Origin absent/same/trusted/untrusted;
- body 413;
- timeout stable `timeout` code;
- concurrency/rate overload;
- 9-tool allowlist/scope injection rejection;
- audit fail-closed;
- `/mcp` Caddy Basic-Auth bypass plus application Bearer protection;
- no new host port;
- MCP disabled by default.

It must separately label real OpenClaw live probe as a later real-environment acceptance item, not automated-complete evidence.

- [ ] **Step 4: Run final verification**

Required fresh verification on the final commit:

```bash
go test ./...
go test -race ./...
govulncheck ./...
bash scripts/check-edge-security.sh
bash scripts/test-production-ops.sh
docker build -t family-finance-core:v2-security-test .
```

Then require GitHub `CI` and `Edge Security` workflows to complete `success` on the exact final commit SHA.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/ci.yml docs/superpowers/specs/2026-08-18-v2-agent-adapter-mcp-design.md docs/v2-mcp-security-acceptance.md
git commit -m "docs(v2): finalize MCP security gates"
```

## Self-Review

- Spec coverage: bearer, Origin, body, timeout, concurrency, rate, audit, server-side household scope, disabled-by-default startup, Caddy edge, no new host port, secret hygiene, and CI are each assigned to a concrete task.
- Dependency policy: no new library is required; all middleware uses Go standard library plus the already-pinned official MCP SDK.
- Separation: protocol-only `NewHTTPHandler` remains testable; secure production mounting occurs only after security/config validation.
- Caddy authorization conflict is resolved explicitly rather than attempting impossible simultaneous Basic and Bearer values in one `Authorization` header.
- Real OpenClaw interoperability remains a separate real-environment acceptance gate and is not claimed by this plan.
