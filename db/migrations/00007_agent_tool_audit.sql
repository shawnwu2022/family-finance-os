-- +goose Up
CREATE TABLE agent_tool_audits (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    principal_kind TEXT NOT NULL CHECK (length(btrim(principal_kind)) > 0),
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    protocol TEXT NOT NULL CHECK (length(btrim(protocol)) > 0),
    protocol_version TEXT NOT NULL CHECK (length(btrim(protocol_version)) > 0),
    client_name TEXT,
    client_version TEXT,
    tool_name TEXT NOT NULL CHECK (length(btrim(tool_name)) > 0),
    input_sha256 CHAR(64) NOT NULL CHECK (input_sha256 ~ '^[0-9a-f]{64}$'),
    output_sha256 CHAR(64) CHECK (output_sha256 IS NULL OR output_sha256 ~ '^[0-9a-f]{64}$'),
    data_as_of TIMESTAMPTZ,
    status TEXT NOT NULL CHECK (status IN ('running', 'success', 'error')),
    error_code TEXT,
    duration_ms BIGINT CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CHECK (
        (status = 'running' AND output_sha256 IS NULL AND data_as_of IS NULL AND error_code IS NULL AND duration_ms IS NULL)
        OR
        (status = 'success' AND output_sha256 IS NOT NULL AND error_code IS NULL AND duration_ms IS NOT NULL)
        OR
        (status = 'error' AND output_sha256 IS NULL AND data_as_of IS NULL AND error_code IS NOT NULL AND length(btrim(error_code)) > 0 AND duration_ms IS NOT NULL)
    )
);

CREATE INDEX agent_tool_audits_household_created_idx
    ON agent_tool_audits(household_id, created_at DESC, id DESC);

CREATE INDEX agent_tool_audits_tool_created_idx
    ON agent_tool_audits(tool_name, created_at DESC, id DESC);

-- +goose Down
DROP TABLE agent_tool_audits;
