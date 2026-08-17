-- +goose Up
CREATE TABLE job_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    job_name TEXT NOT NULL CHECK (length(btrim(job_name)) > 0),
    scheduled_for TIMESTAMPTZ NOT NULL,
    period TEXT NOT NULL DEFAULT '' CHECK (period = '' OR period ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed')),
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    error_code TEXT,
    UNIQUE (household_id, job_name, scheduled_for),
    CHECK (
        (status = 'running' AND finished_at IS NULL AND error_code IS NULL)
        OR
        (status = 'succeeded' AND finished_at IS NOT NULL AND error_code IS NULL)
        OR
        (status = 'failed' AND finished_at IS NOT NULL AND error_code IS NOT NULL AND length(btrim(error_code)) > 0)
    )
);

CREATE INDEX job_runs_household_job_scheduled_idx
    ON job_runs(household_id, job_name, scheduled_for DESC, id DESC);

-- +goose Down
DROP TABLE job_runs;
