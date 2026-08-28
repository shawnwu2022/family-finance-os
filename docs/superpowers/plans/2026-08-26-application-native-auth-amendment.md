# Application-Native Auth Plan Amendment

**Date:** 2026-08-26

## 1. Focused pgx authentication store

Task 1 uses a focused pgx-backed `internal/auth/PostgresStore` with explicit parameterized SQL instead of adding auth queries to sqlc.

Reason: the repository already uses direct pgx SQL for focused persistence/bootstrap components, while auth state transitions require explicit transactional semantics and are security-sensitive. Keeping those statements beside the auth store reduces generated-code surface and avoids coupling the authentication boundary to a new sqlc query set. The database schema remains a goose migration and all SQL remains parameterized.

The Task 1 RED/GREEN contract is unchanged: challenge consumption, session revocation/expiry filtering, normalized-username uniqueness, and recovery-code single-use behavior must be proven by PostgreSQL integration tests. Existing `sqlc generate` drift checks remain unchanged because no auth sqlc query files are introduced; schema-derived model changes are still regenerated and committed.

## 2. Recoverable CSRF token without plaintext-at-rest

The approved design requires `/api/v1/auth/session` to return the current CSRF token after a page reload while also requiring that sensitive session material not be stored plaintext in PostgreSQL. Storing only `csrf_token_hash` cannot satisfy both requirements: a one-way hash cannot reconstruct the browser token after reload.

Task 2 therefore strengthens `auth_sessions` with:

```sql
csrf_token_hash BYTEA NOT NULL,
csrf_token_ciphertext BYTEA NOT NULL
```

The raw CSRF token is generated randomly. Finance Core stores:

- `SHA-256(raw_csrf_token)` for constant-time request verification; and
- AES-256-GCM ciphertext of the raw token using the same protected `FINANCE_AUTH_KEY_FILE` key used for TOTP secret encryption.

`AuthenticateSession` decrypts the ciphertext only for an otherwise valid, non-expired, non-revoked session and returns the raw token to the browser session bootstrap endpoint. The database never stores the plaintext token. This avoids rotating CSRF state on every reload and therefore avoids invalidating another tab's legitimate requests.

This amendment strengthens storage semantics only; it does not change the approved trust boundary or cookie/session model.

## 3. Atomic second-factor completion

Second-factor completion must not be split into independently committed database updates. The pgx store will expose transactional operations so the following state changes commit or roll back together:

- enrollment challenge consumption + TOTP enrollment/counter update + recovery-code insertion + session creation;
- login challenge consumption + monotonic TOTP counter update + session creation; or
- login challenge consumption + one-time recovery-code consumption + session creation.

This prevents concurrent requests from opening a replay window between validating a factor and recording its consumption.

## 4. Reverse-proxy source address is not trusted by default

Application-native login throttling needs a client source key, but Finance Core must not accept a forged `X-Forwarded-For` from an arbitrary direct caller. Task 4 will therefore resolve source IP as follows:

1. use the TCP peer address by default;
2. honor forwarded client IP headers only when the immediate peer belongs to the explicitly configured Finance trusted-proxy CIDR; and
3. in the reference deployment, restrict that CIDR to the deterministic Caddy address `172.30.0.10/32`.

A direct request to Finance Core cannot spoof Caddy merely by adding forwarded headers. Caddy remains outside the identity trust boundary; this proxy trust is used only to obtain the source address for abuse controls and request metadata, never to authenticate a user.
