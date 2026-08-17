-- name: CreateAdviceAudit :one
INSERT INTO advice_audits (
    created_at,
    model_role,
    reviewer_role,
    data_as_of,
    prompt_template_version,
    request_sha256,
    advice_sha256,
    quality_level,
    status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: CreateAdviceAuditTool :one
INSERT INTO advice_audit_tools (
    advice_audit_id,
    sequence,
    tool_name,
    input_sha256,
    result_sha256,
    success,
    error_code
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAdviceAudit :one
SELECT *
FROM advice_audits
WHERE id = $1;

-- name: ListAdviceAuditTools :many
SELECT *
FROM advice_audit_tools
WHERE advice_audit_id = $1
ORDER BY sequence ASC;
