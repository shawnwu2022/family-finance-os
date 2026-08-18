-- name: CreateAgentToolAuditAttempt :one
INSERT INTO agent_tool_audits (
    created_at,
    principal_kind,
    household_id,
    protocol,
    protocol_version,
    client_name,
    client_version,
    tool_name,
    input_sha256,
    status
) VALUES (
    sqlc.arg(created_at),
    sqlc.arg(principal_kind),
    sqlc.arg(household_id),
    sqlc.arg(protocol),
    sqlc.arg(protocol_version),
    sqlc.narg(client_name),
    sqlc.narg(client_version),
    sqlc.arg(tool_name),
    sqlc.arg(input_sha256),
    'running'
)
RETURNING id;

-- name: CompleteAgentToolAuditSuccess :one
UPDATE agent_tool_audits
SET output_sha256 = sqlc.arg(output_sha256),
    data_as_of = sqlc.narg(data_as_of),
    status = 'success',
    duration_ms = sqlc.arg(duration_ms)
WHERE id = sqlc.arg(id)
  AND status = 'running'
RETURNING id;

-- name: CompleteAgentToolAuditFailure :one
UPDATE agent_tool_audits
SET status = 'error',
    error_code = sqlc.arg(error_code),
    duration_ms = sqlc.arg(duration_ms)
WHERE id = sqlc.arg(id)
  AND status = 'running'
RETURNING id;

-- name: GetAgentToolAudit :one
SELECT
    id,
    created_at,
    principal_kind,
    household_id,
    protocol,
    protocol_version,
    client_name,
    client_version,
    tool_name,
    input_sha256,
    output_sha256,
    data_as_of,
    status,
    error_code,
    duration_ms
FROM agent_tool_audits
WHERE id = sqlc.arg(id);
