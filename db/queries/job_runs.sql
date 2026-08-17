-- name: ClaimJobRun :one
INSERT INTO job_runs (
    household_id,
    job_name,
    scheduled_for,
    period,
    status,
    started_at
) VALUES (
    sqlc.arg(household_id),
    sqlc.arg(job_name),
    sqlc.arg(scheduled_for),
    sqlc.arg(period),
    'running',
    sqlc.arg(started_at)
)
ON CONFLICT (household_id, job_name, scheduled_for) DO UPDATE SET
    period = EXCLUDED.period,
    status = 'running',
    started_at = EXCLUDED.started_at,
    finished_at = NULL,
    error_code = NULL
WHERE job_runs.status = 'failed'
RETURNING id;

-- name: FinishJobRun :execrows
UPDATE job_runs
SET
    status = sqlc.arg(status),
    finished_at = sqlc.arg(finished_at),
    error_code = sqlc.narg(error_code)
WHERE household_id = sqlc.arg(household_id)
  AND job_name = sqlc.arg(job_name)
  AND scheduled_for = sqlc.arg(scheduled_for)
  AND status = 'running';

-- name: RecoverInterruptedJobRuns :execrows
UPDATE job_runs
SET
    status = 'failed',
    finished_at = sqlc.arg(recovered_at),
    error_code = 'process_restarted'
WHERE status = 'running';

-- name: ListJobRuns :many
SELECT id, household_id, job_name, scheduled_for, period, status, started_at, finished_at, error_code
FROM job_runs
WHERE household_id = sqlc.arg(household_id)
  AND job_name = sqlc.arg(job_name)
ORDER BY scheduled_for DESC, id DESC;

-- name: ListSchedulerHouseholds :many
SELECT id, timezone
FROM households
ORDER BY id;
