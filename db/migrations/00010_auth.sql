-- +goose Up
CREATE TABLE auth_users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username TEXT NOT NULL,
    normalized_username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE RESTRICT,
    totp_secret_ciphertext BYTEA,
    totp_last_counter BIGINT,
    totp_enrolled_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(normalized_username) > 0),
    CHECK (length(password_hash) > 0)
);

CREATE TABLE auth_challenges (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash BYTEA NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('login', 'totp_enrollment')),
    pending_totp_secret_ciphertext BYTEA,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    CHECK (octet_length(token_hash) = 32),
    CHECK (expires_at > created_at)
);

CREATE TABLE auth_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash BYTEA NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    csrf_token_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (octet_length(token_hash) = 32),
    CHECK (octet_length(csrf_token_hash) = 32),
    CHECK (last_seen_at >= created_at),
    CHECK (expires_at > created_at)
);

CREATE TABLE auth_recovery_codes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    consumed_at TIMESTAMPTZ,
    CHECK (octet_length(code_hash) = 32),
    UNIQUE (user_id, code_hash)
);

CREATE INDEX auth_sessions_user_active_idx
    ON auth_sessions(user_id, expires_at)
    WHERE revoked_at IS NULL;

CREATE INDEX auth_challenges_user_active_idx
    ON auth_challenges(user_id, expires_at)
    WHERE consumed_at IS NULL;

-- +goose Down
DROP TABLE auth_recovery_codes;
DROP TABLE auth_sessions;
DROP TABLE auth_challenges;
DROP TABLE auth_users;
