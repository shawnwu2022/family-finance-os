-- +goose Up
CREATE TABLE monthly_reports (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    period TEXT NOT NULL CHECK (period ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    kind TEXT NOT NULL CHECK (kind = 'monthly'),
    data_as_of TIMESTAMPTZ NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL,
    quality TEXT NOT NULL CHECK (quality IN ('good', 'partial', 'stale', 'unknown')),
    content_hash CHAR(64) NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    payload JSONB NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (household_id, period, kind)
);

CREATE INDEX monthly_reports_household_period_idx
    ON monthly_reports(household_id, period DESC);

-- +goose Down
DROP TABLE monthly_reports;
