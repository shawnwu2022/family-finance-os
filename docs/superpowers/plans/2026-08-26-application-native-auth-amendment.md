# Application-Native Auth Plan Amendment

**Date:** 2026-08-26

Task 1 uses a focused pgx-backed `internal/auth/PostgresStore` with explicit parameterized SQL instead of adding auth queries to sqlc.

Reason: the repository already uses direct pgx SQL for focused persistence/bootstrap components, while auth state transitions require explicit transactional semantics and are security-sensitive. Keeping those statements beside the auth store reduces generated-code surface and avoids coupling the authentication boundary to a new sqlc query set. The database schema remains a goose migration and all SQL remains parameterized.

The Task 1 RED/GREEN contract is unchanged: challenge consumption, session revocation/expiry filtering, normalized-username uniqueness, and recovery-code single-use behavior must be proven by PostgreSQL integration tests. Existing `sqlc generate` drift checks remain unchanged because no auth sqlc files are introduced.
