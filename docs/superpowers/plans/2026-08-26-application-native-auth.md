# Application-Native Authentication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move human authentication and household authorization into Finance Core, add mandatory TOTP-backed server-side sessions, remove Caddy from the identity trust boundary, and harden ezBookkeeping production authentication/network settings.

**Architecture:** Finance Core gains a focused `internal/auth` subsystem backed by PostgreSQL and wired into `internal/server` as explicit public-auth, protected-browser, health, and MCP routes. Browser household scope comes only from the authenticated session context. Caddy becomes TLS/reverse-proxy only; ezBookkeeping keeps its own login/2FA and receives explicit production settings plus a narrow API-token source-IP allowlist.

**Tech Stack:** Go 1.26.6, PostgreSQL 18.6, pgx/v5, goose, `golang.org/x/crypto/argon2` v0.55.0, AES-256-GCM, RFC 6238 TOTP, Vue 3 + TypeScript + Vite PWA, Docker Compose, Caddy 2.11.4, ezBookkeeping 1.6.1 pinned at `6ccd0c462100828c78e203792a5b2feb8d569039`.

**Spec:** `docs/superpowers/specs/2026-08-26-application-native-auth-boundary-design.md`

## Global Constraints

- Finance Core, not Caddy, is authoritative for browser authentication, authorization, CSRF, and application security headers.
- Browser-supplied `household_id` is never an authorization input after cutover.
- MCP remains independently authenticated by its bearer-token boundary and is disabled by default.
- Browser session cookie is `__Host-finance_session; Secure; HttpOnly; SameSite=Strict; Path=/` with no Domain attribute.
- Session idle timeout is 30 minutes; absolute timeout is 12 hours; login/enrollment challenges expire after 5 minutes.
- Passwords use Argon2id: 64 MiB memory, 3 iterations, parallelism 1, 16-byte salt, 32-byte derived key; minimum 14 Unicode characters; maximum accepted password payload 128 bytes.
- TOTP is RFC 6238 SHA-1, 6 digits, 30-second step, accepting current step ±1 and rejecting replay via `totp_last_counter`.
- TOTP secrets are encrypted with AES-256-GCM using a 32-byte key read from `/run/secrets/finance-auth-key`.
- Recovery codes are generated with cryptographically secure randomness, returned once, stored only as hashes, and consumed atomically.
- Finance login throttling is application-native: 5 failures per source IP per 5 minutes and 5 failures per normalized username per 5 minutes.
- ezBookkeeping production registration is disabled, two-factor support is enabled, login failure limits are non-zero, web token lifetime is 12 hours, and API-token source IPs are restricted to Finance Core.
- Reference Docker network uses deterministic private addresses so trusted-proxy and API-token allowlists are testable.
- No plaintext password, TOTP secret, recovery code, browser session token, CSRF token hash material, API token, or encryption key is committed to Git or emitted in logs.
- Existing `make verify*` gates remain the source of truth; new auth/edge checks are added to those gates rather than existing only in GitHub Actions.

---

### Task 1: Authentication persistence schema and query contract

**Files:**
- Create: `db/migrations/00010_auth.sql`
- Create: `db/queries/auth.sql`
- Modify: `internal/store/sqlc/models.go` (generated result)
- Modify: `internal/store/sqlc/querier.go` (generated result)
- Create/Modify generated auth query file: `internal/store/sqlc/auth.sql.go`
- Test: `internal/auth/postgres_store_integration_test.go`

**Interfaces:**
- Produces `auth_users`, `auth_sessions`, `auth_challenges`, and `auth_recovery_codes` tables.
- Produces store methods used by later tasks: `GetUserByNormalizedUsername`, `GetUserByID`, `CreateOrGetAdminUser`, `UpdateTOTPEnrollment`, `UseTOTPCounter`, `CreateChallenge`, `ConsumeChallenge`, `CreateSession`, `GetSessionByTokenHash`, `TouchSession`, `RevokeSession`, `RevokeUserSessions`, `InsertRecoveryCodes`, and `ConsumeRecoveryCode`.

- [ ] **Step 1: Write migration and failing integration expectations**

Create a migration with `-- +goose Up/Down` that defines:

```sql
CREATE TABLE auth_users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL,
    normalized_username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE RESTRICT,
    totp_secret_ciphertext BYTEA,
    totp_last_counter BIGINT,
    totp_enrolled_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auth_challenges (
    id BIGSERIAL PRIMARY KEY,
    token_hash BYTEA NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('login','totp_enrollment')),
    pending_totp_secret_ciphertext BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);

CREATE TABLE auth_sessions (
    id BIGSERIAL PRIMARY KEY,
    token_hash BYTEA NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    csrf_token_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE TABLE auth_recovery_codes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    consumed_at TIMESTAMPTZ,
    UNIQUE(user_id, code_hash)
);

CREATE INDEX auth_sessions_user_active_idx ON auth_sessions(user_id, expires_at)
WHERE revoked_at IS NULL;
```

The integration test must prove uniqueness of normalized username, challenge single-use behavior, recovery-code atomic consumption, and revoked/expired session filtering.

- [ ] **Step 2: Run migration/query generation checks and confirm failure before generated code exists**

Run:

```bash
make verify-go
```

Expected before generation: compile/query-generation failure because auth generated methods/types do not yet exist.

- [ ] **Step 3: Add `db/queries/auth.sql` and regenerate sqlc output**

Use explicit SQL names such as:

```sql
-- name: GetAuthUserByNormalizedUsername :one
SELECT * FROM auth_users WHERE normalized_username = $1;

-- name: GetActiveAuthSessionByTokenHash :one
SELECT s.*, u.username, u.normalized_username, u.household_id, u.disabled_at
FROM auth_sessions s
JOIN auth_users u ON u.id = s.user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > $2;

-- name: ConsumeAuthChallenge :one
UPDATE auth_challenges
SET consumed_at = $2
WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > $2
RETURNING *;

-- name: ConsumeRecoveryCode :one
UPDATE auth_recovery_codes
SET consumed_at = $3
WHERE user_id = $1 AND code_hash = $2 AND consumed_at IS NULL
RETURNING id;
```

Generate using the repository-pinned sqlc flow already used by `make verify-go`; do not hand-diverge generated signatures from the query file.

- [ ] **Step 4: Run auth store integration tests**

Run:

```bash
go test ./internal/auth -run 'TestPostgresAuthStore' -count=1
```

Expected: PASS with a test proving challenge/session/recovery state transitions are atomic.

- [ ] **Step 5: Commit**

```bash
git add db/migrations/00010_auth.sql db/queries/auth.sql internal/store/sqlc internal/auth/postgres_store_integration_test.go
git commit -m "feat: add finance authentication persistence"
```

---

### Task 2: Authentication cryptographic primitives and domain service

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/password_test.go`
- Create: `internal/auth/secretbox.go`
- Create: `internal/auth/secretbox_test.go`
- Create: `internal/auth/totp.go`
- Create: `internal/auth/totp_test.go`
- Create: `internal/auth/tokens.go`
- Create: `internal/auth/service.go`
- Create: `internal/auth/service_test.go`
- Create: `internal/auth/postgres_store.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Produces `auth.PasswordHasher`, `auth.SecretBox`, `auth.Service`, `auth.SessionIdentity`, and `auth.LoginResult` for server/bootstrap tasks.
- Consumes the auth store query contract from Task 1.

- [ ] **Step 1: Write failing primitive tests**

Tests must cover:

```go
func TestPasswordHashRoundTrip(t *testing.T) { /* valid password succeeds; wrong password fails */ }
func TestPasswordPolicy(t *testing.T) { /* <14 runes and >128 bytes are rejected */ }
func TestSecretBoxRoundTripAndTamper(t *testing.T) { /* AES-GCM tamper fails */ }
func TestTOTPWindowAndReplayCounter(t *testing.T) { /* ±1 accepted, older/equal counter rejected */ }
func TestOpaqueTokenUses32RandomBytes(t *testing.T) { /* decoded token has 32 bytes entropy */ }
```

- [ ] **Step 2: Add Argon2id dependency and verify tests fail on missing implementation**

Pin:

```text
golang.org/x/crypto v0.55.0
```

Run:

```bash
go test ./internal/auth -run 'TestPassword|TestSecretBox|TestTOTP|TestOpaque' -count=1
```

Expected: FAIL until the new files are implemented.

- [ ] **Step 3: Implement password hashing**

Store hashes in a self-describing format:

```text
$argon2id$v=19$m=65536,t=3,p=1$<base64-salt>$<base64-key>
```

Use `argon2.IDKey(password, salt, 3, 64*1024, 1, 32)` and `subtle.ConstantTimeCompare` during verification. `Service.VerifyPassword` must perform the same Argon2id work against a fixed valid dummy hash when the username lookup returns no row.

- [ ] **Step 4: Implement AES-256-GCM, TOTP, tokens, recovery codes, and service state transitions**

`SecretBox` accepts exactly 32 key bytes. TOTP uses HMAC-SHA1 with dynamic truncation and `counter = unixTime/30`. `Service` must expose:

```go
type SessionIdentity struct {
    UserID int64
    Username string
    HouseholdID int64
    CSRFToken string
}

type LoginResult struct {
    ChallengeToken string
    Step string // "verify_totp" or "enroll_totp"
    TOTPSecret string
    OTPAuthURI string
}

func (s *Service) BeginLogin(ctx context.Context, username, password string, now time.Time) (LoginResult, error)
func (s *Service) ConfirmEnrollment(ctx context.Context, challenge, code string, now time.Time) (SessionIssue, error)
func (s *Service) VerifySecondFactor(ctx context.Context, challenge, code string, recovery bool, now time.Time) (SessionIssue, error)
func (s *Service) AuthenticateSession(ctx context.Context, rawSessionToken string, now time.Time) (SessionIdentity, error)
func (s *Service) Logout(ctx context.Context, rawSessionToken string, now time.Time) error
```

`SessionIssue` contains raw session token + raw CSRF token for the HTTP layer only; the store sees hashes only.

- [ ] **Step 5: Run auth package tests including race-sensitive service tests**

Run:

```bash
go test -race ./internal/auth -count=1
```

Expected: PASS; recovery code cannot be consumed twice; TOTP replay cannot succeed concurrently.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/auth
git commit -m "feat: add finance authentication service"
```

---

### Task 3: Runtime config, administrator bootstrap, and offline recovery commands

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `internal/bootstrap/bootstrap_test.go`
- Modify: `internal/bootstrap/bootstrap_integration_test.go`
- Modify: `cmd/finance-core/main.go`
- Modify: `cmd/finance-core/main_test.go`
- Modify: `compose.yaml`
- Modify: `.env.example`

**Interfaces:**
- Produces `config.AuthConfig{KeyFile, AdminUsername, AdminPasswordFile}` and bootstrapped enabled admin bound to the bootstrapped household.
- Produces maintenance commands `auth-reset-password` and `auth-reset-totp` that require direct server/DB access and never expose secrets over HTTP.

- [ ] **Step 1: Write failing config/bootstrap/CLI tests**

Required expectations:

```go
func TestLoadRequiresAuthKeyFileForServe(t *testing.T) {}
func TestBootstrapCreatesAdminWithoutResettingExistingCredentials(t *testing.T) {}
func TestAuthResetPasswordRevokesAllSessions(t *testing.T) {}
func TestAuthResetTOTPRevokesSessionsAndRecoveryCodes(t *testing.T) {}
```

- [ ] **Step 2: Implement `AuthConfig` and secret-file readers**

Runtime config accepts paths only:

```text
FINANCE_AUTH_KEY_FILE=/run/secrets/finance-auth-key
FINANCE_ADMIN_USERNAME=finance
FINANCE_ADMIN_PASSWORD_FILE=/run/secrets/finance-admin-password
```

Do not add `FINANCE_ADMIN_PASSWORD` or raw auth-key environment variables.

- [ ] **Step 3: Extend bootstrap atomically**

`finance-bootstrap` must:

1. create/reconcile the household and budget as today;
2. read the admin password file;
3. validate/hash password;
4. create the first admin bound to that exact `household_id` if none exists;
5. leave existing password/TOTP untouched on rerun;
6. print only IDs/non-secret status.

- [ ] **Step 4: Add offline recovery commands**

Commands:

```bash
/finance-core auth-reset-password --username finance --password-file /run/secrets/finance-admin-password
/finance-core auth-reset-totp --username finance
```

Password reset replaces the hash and revokes sessions. TOTP reset clears encrypted TOTP/counter/enrollment state, deletes recovery codes, revokes sessions, and forces enrollment on next login.

- [ ] **Step 5: Run config/bootstrap/CLI tests**

Run:

```bash
go test ./internal/config ./internal/bootstrap ./cmd/finance-core -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config internal/bootstrap cmd/finance-core compose.yaml .env.example
git commit -m "feat: bootstrap finance administrator securely"
```

---

### Task 4: Finance Core auth HTTP surface, session middleware, and household authorization

**Files:**
- Create: `internal/server/auth.go`
- Create: `internal/server/auth_test.go`
- Create: `internal/server/auth_security_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/api.go`
- Modify: `internal/server/portfolio_http.go`
- Modify: `internal/server/request_security.go`
- Modify: `internal/server/scoped_api.go`
- Modify: `cmd/finance-core/application.go`
- Modify: `cmd/finance-core/application_test.go`
- Test: existing `internal/server/*_test.go`

**Interfaces:**
- Consumes `auth.Service` from Task 2.
- Produces public auth endpoints and protected browser middleware.
- Produces request context carrying authenticated `user_id` and `household_id`.

- [ ] **Step 1: Write failing direct-to-Finance security tests**

At minimum:

```go
func TestProtectedAPIRejectsMissingSession(t *testing.T) {}
func TestPasswordOnlyLoginCannotReadDashboard(t *testing.T) {}
func TestSessionCookieAttributes(t *testing.T) {}
func TestUnsafeRequestRequiresCSRF(t *testing.T) {}
func TestClientHouseholdIDCannotOverrideSessionHousehold(t *testing.T) {}
func TestBrowserCookieDoesNotAuthenticateMCP(t *testing.T) {}
func TestMCPBearerDoesNotAuthenticateBrowserAPI(t *testing.T) {}
func TestHealthzRemainsMinimalAndPublic(t *testing.T) {}
```

- [ ] **Step 2: Split route classes explicitly**

`server.NewHandler` must assemble in this order:

```text
/healthz                           public minimal liveness
/api/v1/auth/session               public-but-session-aware
/api/v1/auth/login                 public auth
/api/v1/auth/totp/enroll/confirm   public challenge completion
/api/v1/auth/verify                public challenge completion
/api/v1/auth/logout                protected + CSRF
/mcp                               existing independent MCP handler
/api/v1/*                          protected browser Finance API
/                                  web assets/login shell
```

The web shell may remain publicly downloadable; financial JSON endpoints must not.

- [ ] **Step 3: Implement cookie/session/auth handlers and security headers**

Use only generic invalid-credential messages. Set `Cache-Control: no-store` on auth and Finance API responses. Set Finance Core security headers itself, including CSP, frame denial, `nosniff`, no-referrer, and restrictive Permissions-Policy.

- [ ] **Step 4: Remove browser household IDs from authorization flow**

Browser Finance API handlers resolve household from authenticated context. For transitional query/body `household_id` inputs, reject mismatched/non-zero values with `400 invalid_request` rather than silently trusting them. MCP continues to use `MCP_HOUSEHOLD_ID` in the separate MCP path.

- [ ] **Step 5: Run server/application tests with race detector**

Run:

```bash
go test -race ./internal/server ./cmd/finance-core -count=1
```

Expected: PASS and all direct unauthenticated `/api/v1/*` requests return 401 except public auth endpoints.

- [ ] **Step 6: Commit**

```bash
git add internal/server cmd/finance-core/application.go cmd/finance-core/application_test.go
git commit -m "feat: enforce finance application authentication"
```

---

### Task 5: Finance PWA login, TOTP enrollment/verification, logout, and household-ID removal

**Files:**
- Create: `web/src/auth.ts`
- Create: `web/src/auth-api.test.mjs`
- Create: `web/src/components/LoginPanel.vue`
- Create: `web/src/components/TOTPPanel.vue`
- Modify: `web/src/api.ts`
- Modify: `web/src/types.ts`
- Modify: `web/src/App.vue`
- Modify: `web/src/style.css`
- Modify: `web/public/sw.js`
- Modify: `web/src/sw.test.mjs`

**Interfaces:**
- Consumes `GET /api/v1/auth/session`, `POST /api/v1/auth/login`, `/api/v1/auth/totp/enroll/confirm`, `/api/v1/auth/verify`, and `/api/v1/auth/logout`.
- Produces in-memory CSRF state used by all unsafe API requests.

- [ ] **Step 1: Write failing frontend API/auth state tests**

Tests must prove:

```text
- dashboard requests contain no household_id
- unsafe requests include X-CSRF-Token only after authentication
- a 401 clears auth state
- session bootstrap chooses login/TOTP/dashboard state correctly
- logout clears CSRF state
- household ID is not persisted to localStorage
```

- [ ] **Step 2: Implement auth API client**

`auth.ts` owns auth state and exposes typed operations. `api.ts` gets CSRF from an injected/current auth state and stops accepting household ID parameters for browser operations.

- [ ] **Step 3: Replace editable household ID with authenticated household context**

`App.vue` loads session before any dashboard request. Login screen is the only unauthenticated view. TOTP enrollment shows the secret/otpauth enrollment data and the one-time recovery codes only at successful enrollment, with explicit instruction to save them securely.

- [ ] **Step 4: Prevent service-worker caching of sensitive API/auth responses**

Service worker must never cache `/api/`, auth responses, or HTML containing transient recovery-code state. Existing static-shell caching may remain.

- [ ] **Step 5: Run web tests/build**

Run:

```bash
make verify-web
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web
git commit -m "feat: add finance login and two-factor flow"
```

---

### Task 6: Remove Caddy authentication and harden ezBookkeeping/network/secret boundaries

**Files:**
- Modify: `Caddyfile`
- Modify: `Caddyfile.acceptance`
- Modify: `compose.yaml`
- Modify: `.env.example`
- Modify: `scripts/preflight.sh`
- Modify: `scripts/check-edge-security.sh`
- Modify: `scripts/ci/edge-security.sh`
- Create/Modify: focused scripts under `scripts/ci/` for auth/ezBookkeeping contract assertions
- Modify: `docs/06-security-privacy.md`
- Modify: `docs/07-operations.md`

**Interfaces:**
- Finance Caddy route becomes pure reverse proxy.
- Reference app network becomes deterministic; intended addresses:

```text
subnet          172.30.0.0/24
caddy           172.30.0.10
ezbookkeeping   172.30.0.20
finance-core    172.30.0.30
postgres        172.30.0.40
```

- ezBookkeeping API-token allowlist permits only `172.30.0.30` (or the exact syntax verified against pinned upstream config).
- ezBookkeeping trusted proxy permits only `172.30.0.10/32`.

- [ ] **Step 1: Write failing shell contract tests**

Assertions must fail on the current branch because Caddy still contains `basic_auth` and Compose still passes secrets in ordinary environment variables.

- [ ] **Step 2: Remove Finance Basic Auth from Caddy and Compose**

Delete `FINANCE_AUTH_USER` and `FINANCE_AUTH_HASH` from `.env.example` and Caddy environment. Finance route only sets non-authoritative defense-in-depth headers and proxies to `finance-core:8000`.

- [ ] **Step 3: Pin deterministic network addresses and ezBookkeeping hardening variables**

Set production ezBookkeeping values corresponding to pinned upstream 1.6.1 keys:

```text
EBK_AUTH_ENABLE_TWO_FACTOR=true
EBK_USER_ENABLE_REGISTER=false
EBK_SECURITY_MAX_FAILURES_PER_IP_PER_MINUTE=5
EBK_SECURITY_MAX_FAILURES_PER_USER_PER_MINUTE=5
EBK_SECURITY_TOKEN_EXPIRED_TIME=43200
EBK_SECURITY_TOKEN_MIN_REFRESH_INTERVAL=1800
EBK_SECURITY_ENABLE_API_TOKEN=true
EBK_SECURITY_API_TOKEN_ALLOWED_REMOTE_IPS=172.30.0.30
EBK_SECURITY_TRUSTED_PROXY_IPS=172.30.0.10/32
```

The pinned upstream configuration confirms `enable_two_factor`, `max_failures_per_ip_per_minute`, `max_failures_per_user_per_minute`, `token_expired_time`, `token_min_refresh_interval`, `api_token_allowed_remote_ips`, and `trusted_proxy_ips`; contract tests must catch accidental zero/blank/wildcard relaxation.

- [ ] **Step 4: Move Finance auth secrets and ledger token to protected file injection**

Reference secret paths:

```text
/run/secrets/finance-auth-key
/run/secrets/finance-admin-password
/run/secrets/ezbookkeeping-api-token
/run/secrets/ezbookkeeping-secret-key
```

Where upstream ezBookkeeping does not natively support `*_FILE`, use the container entrypoint/wrapper to read a mounted `0600` file immediately before exec and export it only to the child process; do not duplicate plaintext into `.env` or generated files on disk.

- [ ] **Step 5: Harden preflight**

Preflight rejects missing/non-regular/repository-local/group-or-other-readable secret files, registration enabled, 2FA disabled, zero login limits, blank/wildcard API-token allowlist, broad trusted proxy ranges, and direct host ports on app/database containers.

- [ ] **Step 6: Run edge/production contract tests**

Run:

```bash
make verify-edge-security
./scripts/test-production-ops.sh
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add Caddyfile Caddyfile.acceptance compose.yaml .env.example scripts docs/06-security-privacy.md docs/07-operations.md
git commit -m "security: harden finance and ledger authentication edge"
```

---

### Task 7: End-to-end authentication acceptance and fail-closed release evidence

**Files:**
- Create: `scripts/acceptance/finance-auth-live-smoke.sh`
- Create: `scripts/ci/auth-security.sh`
- Modify: `Makefile`
- Modify: `scripts/ci/verify.sh`
- Modify: `docs/08-testing-acceptance.md`
- Modify: `docs/acceptance/v1-production-evidence.md`
- Modify: `README.md`

**Interfaces:**
- Produces `make verify-auth-security` and includes it in `make verify`.
- Produces live-smoke steps for real deployment without storing credentials in evidence.

- [ ] **Step 1: Add negative security acceptance cases**

Automated acceptance must exercise direct Finance Core HTTP and prove:

```text
401 unauthenticated dashboard
401 password-only / pre-2FA dashboard
403 or 401 invalid CSRF on unsafe operation
401 revoked session
401 idle-expired session
401 absolute-expired session
replayed TOTP rejected
reused recovery code rejected
household query/body override rejected
browser cookie rejected by /mcp
MCP bearer rejected by browser API
```

- [ ] **Step 2: Add positive flow smoke**

Use generated temporary credentials and isolated test DB to prove password -> TOTP enrollment -> authenticated session -> dashboard -> logout. Output only status codes, test IDs, and non-secret hashes; never print raw session/TOTP/recovery material.

- [ ] **Step 3: Add production evidence checklist for human ezBookkeeping 2FA enrollment**

Because the upstream configuration flag only enables the feature and does not prove the owner enrolled it, the evidence ledger must require a dated production check that the owner account shows 2FA enabled. Store only a redacted assertion, timestamp, version/commit, and operator result.

- [ ] **Step 4: Run repository-native verification**

Run:

```bash
make verify-auth-security
make verify
```

Expected: all gates PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/acceptance/finance-auth-live-smoke.sh scripts/ci/auth-security.sh Makefile scripts/ci/verify.sh docs/08-testing-acceptance.md docs/acceptance/v1-production-evidence.md README.md
git commit -m "test: add finance authentication release gate"
```

---

### Task 8: Migration cutover review, final security verification, and PR readiness

**Files:**
- Review all files changed by Tasks 1-7
- Update: `docs/07-operations.md` only if verification reveals a missing cutover step
- Update: `docs/acceptance/v1-production-evidence.md` only with testable status, never invented production evidence

**Interfaces:**
- Consumes every prior task.
- Produces a merge-ready PR whose code is fail-closed before Caddy Basic Auth is removed in a real deployment.

- [ ] **Step 1: Verify ordered upgrade path**

The documented production upgrade order must be:

```text
1. deploy DB migration + Finance app-native auth while old Caddy Basic Auth still exists
2. create/reconcile Finance admin via bootstrap
3. verify direct Finance Core protected APIs reject requests without Finance session
4. enroll Finance TOTP and save recovery codes
5. verify ezBookkeeping owner 2FA + hardened production settings
6. remove Caddy Basic Auth configuration
7. rerun auth, edge, MCP, runtime-image, backup/restore, and full repository gates
```

- [ ] **Step 2: Run focused verification**

```bash
go test -race ./internal/auth ./internal/server ./cmd/finance-core -count=1
make verify-web
make verify-auth-security
make verify-mcp-security
make verify-edge-security
```

Expected: PASS.

- [ ] **Step 3: Run full verification**

```bash
make verify
```

Expected: PASS with no skipped auth security gate.

- [ ] **Step 4: Review diff for secret leakage and unsafe compatibility fallbacks**

Search the diff for plaintext credential examples, raw tokens, permissive wildcard IPs, `basic_auth`, and any code path that falls back to unauthenticated Finance access. The only remaining unauthenticated Finance endpoints are minimal `/healthz` and the explicit auth endpoints.

- [ ] **Step 5: Mark implementation PR ready only after green verification**

The PR summary must list the exact auth guarantees and distinguish automated evidence from production/manual evidence still outstanding.
