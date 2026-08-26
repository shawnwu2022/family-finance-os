# Application-Native Authentication Boundary Design

**Date:** 2026-08-26  
**Status:** Approved design baseline  
**Scope:** Finance Core user authentication and authorization; ezBookkeeping production account hardening; removal of Caddy as an identity/authentication dependency.

## 1. Objective

Family Finance OS contains household net worth, cash flow, debt, goals, portfolio data, AI-advisor outputs, and ledger access credentials. These are high-sensitivity financial data. The public edge proxy must not be the authority that decides whether a human user may access Finance data.

The production security boundary is therefore changed as follows:

- Finance Core performs its own human authentication and household authorization.
- ezBookkeeping performs its own account authentication and 2FA.
- MCP keeps a separate service identity using its existing bearer-token boundary.
- Caddy provides HTTPS termination and reverse proxying only. Caddy authentication is removed from the required security model.
- PostgreSQL, Finance Core, and ezBookkeeping remain unexposed on host/public ports.

A direct request to Finance Core that bypasses Caddy must still be unable to read or mutate household financial data without a valid Finance Core session.

## 2. Security invariants

The implementation MUST preserve all of these invariants:

1. An unauthenticated request can never read household financial data from `/api/v1/*` except explicitly public authentication endpoints.
2. An authenticated browser session can access only the `household_id` bound to that authenticated user. Browser-supplied household IDs are not authorization inputs.
3. MCP authentication remains independent of browser sessions and remains disabled by default.
4. `GET /healthz` may remain unauthenticated but returns only minimal liveness data. Readiness/details must not disclose sensitive configuration.
5. Caddy may be removed or replaced without changing Finance authentication semantics.
6. Finance Core refuses production startup/ready state if no enabled Finance administrator exists after bootstrap.
7. Passwords, TOTP secrets, recovery codes, session tokens, API tokens, and encryption keys must never be committed to Git or logged.
8. Authentication failures must not reveal whether a username exists.
9. Session cookies are `Secure`, `HttpOnly`, `SameSite=Strict`, host-only, and scoped to `/`.
10. All state-changing browser requests require both a valid authenticated session and CSRF protection.
11. Password or 2FA reset invalidates all existing sessions for that user.
12. ezBookkeeping public registration is disabled after initial account creation and production acceptance cannot pass while registration remains enabled.
13. ezBookkeeping two-factor authentication remains enabled and the production owner account must have 2FA enrolled before acceptance.
14. Finance-to-ezBookkeeping API-token use is restricted to the Finance Core internal source address/range and is not accepted from arbitrary public clients.

## 3. Finance Core identity model

### 3.1 Users

Add a Finance authentication user table linked to exactly one household for V1:

- `id BIGSERIAL PRIMARY KEY`
- `username TEXT UNIQUE NOT NULL`
- `password_hash TEXT NOT NULL`
- `household_id BIGINT NOT NULL REFERENCES households(id)`
- `totp_secret_ciphertext BYTEA`
- `totp_last_counter BIGINT`
- `totp_enrolled_at TIMESTAMPTZ`
- `disabled_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ NOT NULL`
- `updated_at TIMESTAMPTZ NOT NULL`

V1 remains a single-household deployment but the schema deliberately binds identity to household so that authorization is not inferred from query parameters and can evolve into RBAC later.

### 3.2 Password hashing

Passwords use Argon2id with explicit parameters encoded in the stored hash string:

- memory: 64 MiB
- iterations: 3
- parallelism: 1
- salt: 16 random bytes
- derived key: 32 bytes
- minimum password length: 14 Unicode characters
- maximum accepted password size: 128 bytes

Use `golang.org/x/crypto/argon2`. Verification must use a constant-time comparison. Unknown usernames run the same Argon2id verification path against a fixed dummy hash to reduce username-enumeration timing differences.

### 3.3 TOTP 2FA

Finance Core requires TOTP for the production administrator account.

- RFC 6238 compatible TOTP
- SHA-1, 6 digits, 30-second period for broad authenticator compatibility
- accept only the current counter and ±1 adjacent counter
- remember `totp_last_counter` and reject replay of an already accepted or older counter
- TOTP secret is encrypted at rest using AES-256-GCM
- encryption key is read from `FINANCE_AUTH_KEY_FILE`, a `0600` file outside the repository

The first successful password verification for a user without enrolled TOTP enters a restricted enrollment flow rather than creating a normal Finance session. The server generates the TOTP secret and returns an enrollment payload sufficient for an authenticator app. A valid first TOTP code must be confirmed before a normal session is created.

### 3.4 Recovery codes

On TOTP enrollment, generate 10 one-time recovery codes using cryptographically secure randomness. Only hashes are stored. Plaintext codes are returned exactly once to the browser and are never logged. A consumed code is deleted/marked consumed atomically.

## 4. Session model

Finance browser authentication uses opaque server-side sessions; it does not use JWTs.

A successful password + TOTP or recovery-code authentication generates a random 256-bit session token. The browser receives the raw token only in a cookie named `__Host-finance_session`. PostgreSQL stores only `SHA-256(session_token)`.

Session fields:

- `id BIGSERIAL PRIMARY KEY`
- `token_hash BYTEA UNIQUE NOT NULL`
- `user_id BIGINT NOT NULL`
- `csrf_token_hash BYTEA NOT NULL`
- `created_at TIMESTAMPTZ NOT NULL`
- `last_seen_at TIMESTAMPTZ NOT NULL`
- `expires_at TIMESTAMPTZ NOT NULL`
- `revoked_at TIMESTAMPTZ`

V1 timeouts:

- idle timeout: 30 minutes
- absolute timeout: 12 hours

The server refreshes `last_seen_at` at a bounded cadence rather than on every request. Expired and revoked sessions are rejected before API dispatch.

Logout revokes the current session server-side and expires the cookie. Password/TOTP changes revoke all sessions for the user.

## 5. CSRF and browser request security

SameSite cookies are not the only CSRF control.

Each session receives an independent random CSRF token. The plaintext token is returned by the authenticated session endpoint and kept only in browser memory. State-changing requests (`POST`, `PUT`, `PATCH`, `DELETE`) under `/api/v1/` must include `X-CSRF-Token`; the server hashes and constant-time compares it against the session record.

Existing same-origin `Origin` validation and JSON `Content-Type` enforcement remain defense in depth.

Finance Core itself sets security headers for browser responses so these controls do not depend on Caddy:

- `Content-Security-Policy: default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; connect-src 'self'; img-src 'self' data:; worker-src 'self'`
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: no-referrer`
- `Permissions-Policy` disabling unneeded sensors/camera/microphone/geolocation unless a future feature explicitly requires them
- `Cache-Control: no-store` for authentication and Finance API responses

## 6. Authentication HTTP surface

Public authentication endpoints:

- `GET /api/v1/auth/session` — returns `authenticated:false` when no valid session; when authenticated returns the current user/household display context and CSRF token.
- `POST /api/v1/auth/login` — verifies username/password and either requests TOTP/recovery verification or begins first-login TOTP enrollment. It does not create a full session before second factor succeeds.
- `POST /api/v1/auth/totp/enroll/confirm` — confirms first TOTP code for a short-lived enrollment challenge and creates the first full session.
- `POST /api/v1/auth/verify` — completes TOTP or one-time recovery-code verification for an already enrolled account and creates a session.
- `POST /api/v1/auth/logout` — requires a valid session + CSRF token and revokes it.

Short-lived login/enrollment challenges are random opaque values; PostgreSQL stores only their hashes. Challenges expire after 5 minutes and are single-use.

Authentication responses use generic invalid-credential errors and do not identify whether the password, TOTP, recovery code, or username was wrong.

## 7. Login abuse protection

Finance Core applies application-level login throttling independently of the reverse proxy.

V1 uses bounded in-memory failure windows because Finance Core is a single instance:

- maximum 5 failed authentication attempts per source IP per 5 minutes
- maximum 5 failed authentication attempts per normalized username per 5 minutes
- return `429` during the active throttle window
- successful full authentication clears the username failure bucket
- never permanently lock an account based only on remote failures
- cap stored buckets and periodically evict expired entries to prevent unbounded memory use

This is independent of Caddy and complements host/network rate limiting.

## 8. Household authorization

The current browser API accepts `household_id` from query strings and request bodies. That becomes invalid as an authorization model.

After this change:

- auth middleware resolves the session to `user_id` and `household_id` and injects them into request context;
- browser endpoints obtain household scope only from authenticated context;
- frontend requests no longer send `household_id`;
- if a transitional endpoint receives a household ID, it must be ignored or rejected rather than trusted;
- MCP remains separately scoped by `MCP_HOUSEHOLD_ID` inside the authenticated MCP service-identity path.

The household selector/`localStorage` household ID control is removed from the V1 Finance UI.

## 9. Bootstrap and secret handling

`finance-bootstrap` creates or idempotently reconciles the initial Finance administrator.

Non-secret configuration:

- `FINANCE_ADMIN_USERNAME`
- `FINANCE_ADMIN_PASSWORD_FILE=/run/secrets/finance-admin-password`
- `FINANCE_AUTH_KEY_FILE=/run/secrets/finance-auth-key`

The plaintext password and encryption key are never stored in `.env`. Both files live outside Git and must be regular files readable by the service account with group/other permissions disabled (`0600` recommended). Preflight validates file type, readability, and mode.

The bootstrap job hashes the password before storage. A later bootstrap with an existing user does not silently reset password or 2FA; password reset requires an explicit maintenance operation.

## 10. Caddy boundary

Remove Finance `basic_auth` and `FINANCE_AUTH_USER` / `FINANCE_AUTH_HASH` from the required deployment contract.

Finance Caddy route becomes a pure HTTPS reverse proxy to `finance-core:8000`. Existing edge header hardening may remain duplicated, but Finance Core is authoritative for authentication, authorization, CSRF, and application security headers.

No application container receives a public host port. Public exposure remains only Caddy `80/443` in the reference deployment.

## 11. ezBookkeeping production hardening

The project pins ezBookkeeping v1.6.1. Its upstream configuration supports two-factor authentication, login rate limits, session/token lifetime, trusted-proxy ranges, API-token generation, and API-token source-IP restrictions. Production configuration must explicitly set the security-sensitive values rather than rely on broad defaults.

Required production settings:

- `EBK_AUTH_ENABLE_TWO_FACTOR=true`
- `EBK_USER_ENABLE_REGISTER=false` after initial owner account creation
- `EBK_SECURITY_MAX_FAILURES_PER_IP_PER_MINUTE=5` (must never be `0`)
- `EBK_SECURITY_MAX_FAILURES_PER_USER_PER_MINUTE=5` (must never be `0`)
- `EBK_SECURITY_TOKEN_EXPIRED_TIME=43200` (12 hours)
- `EBK_SECURITY_TOKEN_MIN_REFRESH_INTERVAL=1800` (30 minutes)
- `EBK_SECURITY_ENABLE_API_TOKEN=true` only because Finance Core requires its dedicated ledger token
- `EBK_SECURITY_API_TOKEN_ALLOWED_REMOTE_IPS` restricted to the Finance Core internal source address/range
- `EBK_SECURITY_TRUSTED_PROXY_IPS` restricted to the actual Caddy internal source address/range rather than the upstream broad RFC1918 defaults

The production owner account must have 2FA enrolled. Because upstream `enable_two_factor` enables the feature but does not prove a particular account enrolled it, production acceptance must include an explicit manual or automated evidence step showing the owner account is enrolled before release is marked ready.

`EBK_SECURITY_SECRET_KEY` and `EBK_API_TOKEN` are treated as secrets. The target implementation should migrate these from ordinary `.env` values to protected files/secret injection where supported by the wrapper/runtime; they must never be printed in acceptance evidence.

## 12. API-token network isolation

Finance Core is the only consumer of the ezBookkeeping API token.

The reference Compose network must use a deterministic private subnet and deterministic service addresses (or an equivalently narrow stable source range) so `EBK_SECURITY_API_TOKEN_ALLOWED_REMOTE_IPS` can allow Finance Core but reject arbitrary clients. Caddy must not share the Finance Core identity or token.

The project security check must verify:

- no direct host ports for Finance Core, ezBookkeeping, or PostgreSQL;
- API-token allowed IPs are non-empty and do not contain wildcards or broad public ranges;
- trusted-proxy IPs are non-empty and correspond only to the proxy path used by the deployment;
- registration is disabled in production configuration;
- two-factor support and login-failure limits are enabled.

## 13. Frontend behavior

On load, the Finance PWA first calls `/api/v1/auth/session`.

- unauthenticated: render only the Finance login experience;
- password verified but TOTP not enrolled: render TOTP enrollment/confirmation;
- second factor required: render TOTP/recovery verification;
- authenticated: render Dashboard and Finance functions;
- `401` from any Finance API clears local authenticated state and returns to login;
- logout clears in-memory CSRF state and returns to login.

The UI no longer exposes `household_id` as a user-editable field.

## 14. MCP separation

`/mcp` is not authenticated by browser sessions. It remains protected by the existing MCP bearer-token, origin, rate, concurrency, timeout, request-size, and audit boundary.

Browser-auth middleware must explicitly route `/mcp` to the MCP handler without treating an MCP bearer token as a Finance user session. Likewise, a valid browser cookie grants no MCP access.

## 15. Migration compatibility

This change is intentionally fail-closed.

Existing deployments using Caddy Basic Auth must create the Finance admin password file and auth encryption-key file, run migration/bootstrap, enroll TOTP, and only then remove Caddy Basic Auth. Deployment documentation must provide this ordered migration so there is no window in which Finance data becomes unauthenticated.

Recommended cutover sequence:

1. deploy DB migration and Finance application-native auth while Caddy Basic Auth still exists;
2. create/reconcile Finance administrator through bootstrap;
3. verify direct Finance Core requests without a session return `401` for protected APIs;
4. enroll Finance TOTP and verify login/logout/session expiration;
5. verify ezBookkeeping production account 2FA and hardened settings;
6. remove Caddy Basic Auth and its secrets;
7. rerun edge/auth security acceptance.

## 16. Testing and acceptance

Implementation is not complete until automated tests prove at least:

- protected Finance APIs return `401` without a session even when called directly against Finance Core;
- password-only authentication cannot access Finance data;
- invalid TOTP and replayed TOTP counters are rejected;
- recovery codes are one-time;
- session token is not stored plaintext in PostgreSQL;
- revoked, idle-expired, and absolute-expired sessions fail;
- CSRF is required for authenticated unsafe API methods;
- client-supplied household IDs cannot cross the authenticated household boundary;
- browser sessions cannot authenticate `/mcp` and MCP bearer tokens cannot authenticate browser APIs;
- Caddyfile contains no Finance `basic_auth` dependency;
- Compose exposes no app/database host ports;
- ezBookkeeping production security variables meet the required values;
- preflight rejects missing, permissive, or repository-local Finance auth secret files;
- Go unit/integration/race/security tests, Web tests/build, MCP security, edge security, runtime image checks, and production contract checks remain green.

## 17. Non-goals for this change

To keep V1 operationally simple, this change does not introduce:

- Authentik, Keycloak, Pocket ID, or another identity-provider service;
- social login/OIDC for Finance;
- passkeys/WebAuthn for Finance;
- multi-household membership/RBAC UI;
- email password reset;
- SMS authentication;
- JWT browser sessions;
- Redis-backed sessions.

Those may be added later if multi-member requirements justify the extra operational dependency.

## 18. Decision

V1 uses Finance Core application-native password + mandatory TOTP authentication with server-side opaque sessions and household authorization. ezBookkeeping retains its own account system with 2FA and tightened production settings. Caddy is no longer part of the identity trust boundary and can be replaced without weakening authorization.
