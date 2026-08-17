-- +goose Up
CREATE TABLE advice_audits (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    model_role TEXT NOT NULL CHECK (model_role IN ('fast', 'planner', 'reviewer')),
    reviewer_role TEXT CHECK (reviewer_role IS NULL OR reviewer_role = 'reviewer'),
    data_as_of TIMESTAMPTZ NOT NULL,
    prompt_template_version TEXT NOT NULL CHECK (length(btrim(prompt_template_version)) > 0),
    request_sha256 CHAR(64) NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
    advice_sha256 CHAR(64) NOT NULL CHECK (advice_sha256 ~ '^[0-9a-f]{64}$'),
    quality_level TEXT NOT NULL CHECK (quality_level IN ('good', 'partial', 'stale', 'unknown')),
    status TEXT NOT NULL CHECK (status IN ('success', 'blocked', 'error'))
);

CREATE INDEX advice_audits_created_at_idx
    ON advice_audits(created_at DESC, id DESC);

CREATE TABLE advice_audit_tools (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    advice_audit_id BIGINT NOT NULL REFERENCES advice_audits(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK (sequence >= 0),
    tool_name TEXT NOT NULL CHECK (length(btrim(tool_name)) > 0),
    input_sha256 CHAR(64) NOT NULL CHECK (input_sha256 ~ '^[0-9a-f]{64}$'),
    result_sha256 CHAR(64) CHECK (result_sha256 IS NULL OR result_sha256 ~ '^[0-9a-f]{64}$'),
    success BOOLEAN NOT NULL,
    error_code TEXT,
    UNIQUE (advice_audit_id, sequence),
    CHECK (
        (success AND result_sha256 IS NOT NULL AND error_code IS NULL)
        OR
        (NOT success AND result_sha256 IS NULL AND error_code IS NOT NULL AND length(btrim(error_code)) > 0)
    )
);

CREATE INDEX advice_audit_tools_audit_idx
    ON advice_audit_tools(advice_audit_id, sequence);

-- +goose Down
DROP TABLE advice_audit_tools;
DROP TABLE advice_audits;
