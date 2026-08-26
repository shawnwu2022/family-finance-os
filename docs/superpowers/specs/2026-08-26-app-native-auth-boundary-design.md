# Application-Native Authentication Boundary Design

**Date:** 2026-08-26  
**Status:** Approved design baseline, self-reviewed  
**Scope:** Finance Core human authentication and household authorization; ezBookkeeping production account hardening; removal of Caddy as an identity/authentication dependency.

## 1. Objective

Family Finance OS contains household net worth, cash flow, debt, goals, portfolio data, AI-advisor outputs, and ledger access credentials. These are high-sensitivity financial data. The public edge proxy must not be the authority that decides whether a human user may access Finance data.

The production security boundary is changed as follows:

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
4. `GET /healthz` may remain unauthenticated but returns only minimal liveness data. `/readyz` is not a public data endpoint and must not disclose dependency/configuration details to unauthenticated callers.
5. Caddy may be removed or replaced without changing Finance authentication semantics.
6. Finance Core refuses ready state if no enabled Finance administrator exists after bootstrap.
7. Passwords, TOTP secrets, recovery codes, session tokens, API tokens, and encryption keys must never be committed to Git or logged.
8. Authentication failures must not reveal whether a username exists.
9. Session cookies are `Secure`, `HttpOnly`, `SameSite=Strict`, host-only, and scoped to `/`.
10. All state-changing browser requests require both a valid authenticated session and CSRF protection.
11. Password or 2FA reset invalidates all existing sessions for that user.
12. ezBookkeeping public registration is disabled after initial account creation and production acceptance cannot pass while registration remains enabled.
13. ezBookkeeping two-factor support is enabled and the production owner account must have 2FA enrolled before acceptance.
14. Finance-to-ezBookkeeping API-token use is restricted to the Finance Core internal address and is not accepted from arbitrary clients.
15. Browser authentication cannot be satisfied by MCP credentials, proxy headers, or a client-provided household identifier.

## 3. Finance Core identity model

### 3.1 Users

Add `finance_users` linked to exactly one household for V1:

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

Finance usernames are 3-64 ASCII characters from `[a-z0-9._-]`. Input is trimmed and lower-cased before lookup/storage. This avoids Unicode/case normalization ambiguity in an authentication identifier.

V1 remains a single-household deployment, but identity is explicitly bound to household so authorization is never inferred from query parameters and can evolve into RBAC later.

### 3.2 Password hashing

Passwords use Argon2id with parameters encoded in the stored PHC-style hash string:

- memory: 64 MiB
- iterations: 3
- parallelism: 1
- salt: 16 random bytes
- derived key: 32 bytes
- minimum password length: 14 Unicode characters
- maximum accepted password size: 128 bytes

Use `golang.org/x/crypto/argon2`. Verification uses constant-time comparison. Unknown usernames execute the same Argon2id verification path against a fixed valid dummy hash to reduce timing-based username enumeration.

At most two Argon2 password verifications may run concurrently in one Finance Core process. Excess verification requests fail closed with `429` so a remote attacker cannot intentionally exhaust memory using concurrent login attempts.

### 3.3 TOTP 2FA

Finance Core requires TOTP for the production administrator account.

- RFC 6238 compatible TOTP
- SHA-1, 6 digits, 30-second period for broad authenticator compatibility
- accept only current counter and ±1 adjacent counter
- persist `totp_last_counter` and reject replay of an already accepted or older counter
- encrypt TOTP secret at rest with AES-256-GCM
- encryption key is read from `FINANCE_AUTH_KEY_FILE`
- key file contains base64 for exactly 32 random bytes and is generated with `openssl rand -base64 32`

The first successful password verification for a user without enrolled TOTP enters a restricted enrollment flow rather than creating a normal Finance session. The server generates the secret, stores it encrypted only inside a short-lived enrollment challenge, and returns `secret` plus `otpauth_uri`. The V1 UI may display the secret for manual authenticator entry; QR rendering is optional and is not a security dependency. A valid first TOTP code must be confirmed before a normal session is created.

### 3.4 Recovery codes

On TOTP enrollment, generate 10 one-time recovery codes using cryptographically secure randomness. Only SHA-256 hashes are stored in `finance_recovery_codes`; plaintext codes are returned exactly once and never logged. Consumption is atomic and one-time.

## 4. Login challenges

Add `finance_auth_challenges`:

- `id BIGSERIAL PRIMARY KEY`
- `token_hash BYTEA UNIQUE NOT NULL`
- `user_id BIGINT NOT NULL`
- `kind TEXT NOT NULL` (`verify` or `totp_enroll`)
- `payload_ciphertext BYTEA` (encrypted pending TOTP secret for enrollment only)
- `created_at TIMESTAMPTZ NOT NULL`
- `expires_at TIMESTAMPTZ NOT NULL`
- `used_at TIMESTAMPTZ`

Challenge tokens are random 256-bit opaque values; only SHA-256 hashes are stored. Challenges expire after 5 minutes and are atomically marked used. They do not authorize Finance data access.

## 5. Session model

Browser authentication uses opaque server-side sessions; it does not use JWTs.

A successful password + TOTP or recovery-code authentication generates a random 256-bit session token. The browser receives the raw token only in a cookie named `__Host-finance_session`. PostgreSQL stores only `SHA-256(session_token)`.

Add `finance_sessions`:

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
- `last_seen_at` update cadence: at most once every 5 minutes per session

Expired/revoked sessions are rejected before API dispatch. Logout revokes the current session and expires the cookie. Password/TOTP reset revokes every session for the affected user.

## 6. CSRF and browser security

SameSite cookies are defense in depth, not the only CSRF control.

Each session receives an independent random 256-bit CSRF token. The plaintext token is returned by the authenticated session endpoint and kept only in browser memory. State-changing requests (`POST`, `PUT`, `PATCH`, `DELETE`) under `/api/v1/` must include `X-CSRF-Token`; the server hashes and constant-time compares it with the stored hash.

Existing same-origin `Origin` validation and JSON `Content-Type` enforcement remain enabled for unsafe API requests, including authentication POSTs.

Finance Core itself sets browser security headers, independent of Caddy:

- `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; connect-src 'self'; img-src 'self' data:; worker-src 'self'`
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: no-referrer`
- `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=()`
- `Cache-Control: no-store` on authentication and Finance API responses

`style-src 'unsafe-inline'` is limited to styles only; scripts remain self-only. This preserves Vue/ECharts runtime styling without allowing inline script execution.

## 7. Authentication HTTP surface

Public authentication endpoints:

- `GET /api/v1/auth/session` — returns `authenticated:false` without a valid session; when authenticated returns username, household display context, and current CSRF token.
- `POST /api/v1/auth/login` — verifies username/password and returns a short-lived challenge. It never creates a full session by password alone.
- `POST /api/v1/auth/totp/enroll/confirm` — confirms first TOTP code for a `totp_enroll` challenge, creates recovery codes, then creates the first full session.
- `POST /api/v1/auth/verify` — completes TOTP or recovery-code verification for a `verify` challenge and creates a session.
- `POST /api/v1/auth/logout` — requires a valid session + CSRF token and revokes it.

Authentication errors use one stable `invalid_credentials` result for invalid username/password/TOTP/recovery-code cases. Throttling uses `429 rate_limited` without stating whether the account exists.

## 8. Login abuse protection and client IP

Finance Core applies application-level throttling independently of proxy authentication.

V1 limits:

- maximum 5 failed authentication attempts per resolved source IP per 5 minutes
- maximum 5 failed authentication attempts per normalized username per 5 minutes
- maximum two concurrent Argon2 verifications
- successful full authentication clears the username failure bucket
- no permanent remote-triggered account lockout
- failure buckets are bounded and expired entries are periodically evicted

Client-IP resolution is also fail-closed:

- use `RemoteAddr` by default;
- accept `X-Forwarded-For` / `X-Real-IP` only when `RemoteAddr` belongs to `FINANCE_TRUSTED_PROXY_CIDRS`;
- reference Compose sets `FINANCE_TRUSTED_PROXY_CIDRS=172.30.0.10/32` (Caddy only);
- a replacement proxy must explicitly update that value; authentication correctness never depends on the forwarded IP.

## 9. Household authorization

The current browser API accepts `household_id` from query strings and request bodies. That ceases to be an authorization input.

After this change:

- auth middleware resolves session → user → `household_id` and injects both into request context;
- browser Finance endpoints obtain household scope only from context;
- frontend requests no longer send `household_id`;
- browser endpoints reject a supplied `household_id` field/query parameter with `400 invalid_request` during migration rather than silently trusting it;
- MCP remains separately scoped by `MCP_HOUSEHOLD_ID` inside the authenticated MCP service-identity path.

The household selector and `localStorage` household ID are removed from the Finance UI.

## 10. Bootstrap, maintenance, and Finance secret files

`finance-bootstrap` creates or idempotently reconciles the initial Finance administrator.

Non-secret configuration:

- `FINANCE_ADMIN_USERNAME=finance`
- `FINANCE_ADMIN_PASSWORD_FILE=/run/secrets/finance-admin-password`
- `FINANCE_AUTH_KEY_FILE=/run/secrets/finance-auth-key`

Both files live outside Git. They must be regular readable files with no group/other permissions (`0600` recommended). Secret readers remove only a single trailing LF or CRLF; they do not trim other whitespace from passwords.

Bootstrap hashes the initial password before storage. Re-running bootstrap for an existing user does not silently reset password or TOTP.

Explicit maintenance commands are required for recovery:

- `finance-core auth set-password --username <name> --password-file <path>` — validates/hash-replaces password and revokes all sessions.
- `finance-core auth reset-totp --username <name>` — clears encrypted TOTP state, last counter, and recovery codes and revokes all sessions; next login must re-enroll TOTP.

These commands require direct administrative access to the deployment host/database environment; they are never exposed as unauthenticated HTTP endpoints.

`FINANCE_AUTH_KEY_FILE` is a disaster-recovery secret and must be escrowed separately from the database backup. Loss of this key makes stored TOTP secrets undecryptable and requires maintenance reset/re-enrollment.

## 11. Caddy boundary

Remove Finance `basic_auth` and `FINANCE_AUTH_USER` / `FINANCE_AUTH_HASH` from the required deployment contract.

The Finance route becomes a pure HTTPS reverse proxy to `finance-core:8000`. Edge headers may remain duplicated, but Finance Core is authoritative for authentication, authorization, CSRF, session validity, and application security headers.

No application container receives a public host port. Public exposure remains only Caddy `80/443` in the reference deployment.

## 12. Deterministic reference network

The reference Compose `app` network uses `172.30.0.0/24` with fixed addresses:

- Caddy: `172.30.0.10`
- Finance Core: `172.30.0.20`
- finance-migrate: `172.30.0.21`
- finance-bootstrap: `172.30.0.22`
- ezBookkeeping: `172.30.0.30`
- PostgreSQL: `172.30.0.40`

This is not an identity mechanism; it exists so trusted-proxy and ezBookkeeping API-token source restrictions can be narrow and testable.

## 13. ezBookkeeping production hardening

The project pins ezBookkeeping v1.6.1. Upstream v1.6.1 configuration supports 2FA, password/token failure limits, session/token lifetime, trusted proxy CIDRs, API-token generation, and API-token source-IP restrictions.

Required production environment values:

- `EBK_AUTH_ENABLE_TWO_FACTOR=true`
- `EBK_USER_ENABLE_REGISTER=false` after the owner is created
- `EBK_SECURITY_MAX_FAILURES_PER_IP_PER_MINUTE=5`
- `EBK_SECURITY_MAX_FAILURES_PER_USER_PER_MINUTE=5`
- `EBK_SECURITY_TOKEN_EXPIRED_TIME=43200`
- `EBK_SECURITY_TOKEN_MIN_REFRESH_INTERVAL=1800`
- `EBK_SECURITY_ENABLE_API_TOKEN=true`
- `EBK_SECURITY_API_TOKEN_ALLOWED_REMOTE_IPS=172.30.0.20`
- `EBK_SECURITY_TRUSTED_PROXY_IPS=172.30.0.10/32`

Neither failure limit may be `0`. API-token allowed IPs may not be empty or wildcarded. Trusted proxy IPs may not use the upstream broad RFC1918 defaults in production.

Upstream `enable_two_factor=true` enables the capability but does not prove the owner enrolled it. Production acceptance therefore requires explicit evidence that the owner account has completed 2FA enrollment.

### 13.1 ezBookkeeping secret key

`EBK_SECURITY_SECRET_KEY` is removed from `.env`/Compose configuration. Add project wrapper variable:

- `EBK_SECURITY_SECRET_KEY_FILE=/run/secrets/ebk-secret-key`

The custom ezBookkeeping container entrypoint reads the file at process start, validates it is non-empty, exports `EBK_SECURITY_SECRET_KEY` only inside the container process environment, and then execs the pinned upstream entrypoint. The secret value is therefore absent from Compose config and `docker inspect ... Config.Env`.

The secret file is generated once with `openssl rand -base64 32`, stored outside Git with `0600`, and separately escrowed for disaster recovery because changing/losing the upstream secret key can invalidate encrypted/signed application data.

### 13.2 Finance ledger API token

`EBK_API_TOKEN` is removed from `.env`. Finance Core reads:

- `EBK_API_TOKEN_FILE=/run/secrets/ebk-api-token`

The token file is outside Git, `0600`, and mounted read-only only into Finance Core. The token value is not passed in Compose environment and is never mounted into Caddy.

## 14. API-token network isolation

Finance Core is the only consumer of the ezBookkeeping API token.

With the fixed reference network, ezBookkeeping accepts API-token requests only from `172.30.0.20`. Requests arriving through Caddy originate from `172.30.0.10` and therefore cannot use the Finance Core token even if the token leaks into a browser or URL. The token is also never intentionally exposed to frontend code.

Security checks must prove:

- no direct host ports for Finance Core, ezBookkeeping, or PostgreSQL;
- fixed reference subnet and addresses match the documented contract;
- API-token allowed IP is exactly the Finance Core address;
- trusted proxy is exactly the Caddy `/32` address;
- registration is disabled for production;
- 2FA support and nonzero login-failure limits are enabled;
- secret values are file-backed and absent from Compose environment.

## 15. Frontend behavior

On load, the Finance PWA first calls `/api/v1/auth/session`.

- unauthenticated: render only the Finance login experience;
- password verified but TOTP not enrolled: render TOTP enrollment/confirmation;
- second factor required: render TOTP/recovery verification;
- authenticated: render Dashboard and Finance functions;
- `401` from a Finance API clears authenticated state and returns to login;
- logout clears in-memory CSRF state and returns to login.

The UI no longer exposes `household_id`.

## 16. MCP separation

`/mcp` is not authenticated by browser sessions. It remains protected by the existing MCP bearer-token, origin, rate, concurrency, timeout, body-size, and audit boundary.

Browser-auth middleware explicitly routes `/mcp` to the MCP handler without treating an MCP bearer token as a Finance user session. A valid Finance browser cookie grants no MCP access. A valid MCP bearer token grants no browser Finance API access.

## 17. Migration compatibility

The cutover is fail-closed and ordered:

1. create/escrow Finance auth key, Finance admin password file, ezBookkeeping secret-key file, and ezBookkeeping API-token file;
2. deploy DB migration and Finance application-native auth while existing Caddy Basic Auth is still present;
3. run bootstrap and verify the enabled Finance administrator exists;
4. verify direct Finance Core protected API requests without a session return `401`;
5. enroll Finance TOTP, save recovery codes, and verify login/logout/expiry;
6. configure and verify ezBookkeeping owner 2FA and hardened settings;
7. enable narrow trusted-proxy/API-token source rules and verify Finance→ezBookkeeping still works;
8. remove Caddy Basic Auth and `FINANCE_AUTH_USER` / `FINANCE_AUTH_HASH`;
9. rerun application-auth, MCP, edge, runtime-image, and production acceptance gates.

At no point is Finance intentionally exposed without either the old Caddy auth or the new Finance Core auth active.

## 18. Testing and acceptance

Implementation is incomplete until automated tests prove at least:

- direct Finance Core protected APIs return `401` without a session;
- password-only authentication cannot access Finance data;
- unknown-user and wrong-password flows do not produce distinct user-visible errors;
- invalid TOTP and replayed TOTP counters are rejected;
- recovery codes are one-time;
- login/enrollment challenges expire and are one-time;
- session token and CSRF token are not stored plaintext in PostgreSQL;
- revoked, idle-expired, and absolute-expired sessions fail;
- unsafe authenticated requests fail without CSRF;
- client-supplied household IDs are rejected and cannot cross scope;
- browser sessions cannot authenticate `/mcp` and MCP bearer tokens cannot authenticate browser APIs;
- app security headers are present when Finance Core is called directly;
- Caddyfile contains no Finance `basic_auth` dependency after cutover;
- Compose exposes no app/database host ports;
- ezBookkeeping production values exactly match the hardened contract;
- Compose/inspect contract contains no Finance password, Finance auth key, ezBookkeeping secret key, or ezBookkeeping API token values;
- preflight rejects missing, repository-local, symlinked/non-regular, unreadable, or group/other-readable auth secret files;
- Go unit/integration/race/security tests, Web tests/build, MCP security, edge security, runtime-image checks, backup/restore checks, and production contracts remain green.

Production evidence additionally records, without secrets:

- Finance TOTP enrollment confirmed;
- Finance recovery codes were presented and stored securely by the operator;
- ezBookkeeping owner 2FA enrollment confirmed;
- registration disabled;
- direct unauthenticated Finance API negative test passed;
- API-token request from non-Finance internal source rejected.

## 19. Non-goals

To keep V1 operationally simple, this change does not introduce:

- Authentik, Keycloak, Pocket ID, or another identity-provider service;
- social login/OIDC for Finance;
- passkeys/WebAuthn for Finance;
- multi-household membership/RBAC UI;
- email/SMS password reset;
- JWT browser sessions;
- Redis-backed sessions.

Those may be introduced later only when multi-member requirements justify the additional operational dependency.

## 20. Decision

V1 uses Finance Core application-native Argon2id password + mandatory TOTP authentication with server-side opaque sessions, explicit CSRF protection, and session-derived household authorization. ezBookkeeping retains its own account system with owner 2FA and explicit production hardening. Caddy is removed from the identity trust boundary and may be replaced without weakening Finance authorization.
