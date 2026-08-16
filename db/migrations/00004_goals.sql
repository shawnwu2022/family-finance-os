-- +goose Up
CREATE TABLE financial_goals (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    target_minor BIGINT NOT NULL CHECK (target_minor >= 0),
    funded_minor BIGINT NOT NULL CHECK (funded_minor >= 0),
    target_date DATE NOT NULL,
    priority INTEGER NOT NULL CHECK (priority > 0),
    flexibility TEXT NOT NULL CHECK (flexibility IN ('hard', 'flexible')),
    monthly_contribution_minor BIGINT NOT NULL DEFAULT 0 CHECK (monthly_contribution_minor >= 0),
    currency CHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX financial_goals_household_active_priority_idx
    ON financial_goals(household_id, active, priority, target_date, id);

-- +goose Down
DROP TABLE financial_goals;
