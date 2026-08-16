-- +goose Up
CREATE TABLE budget_plans (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    period TEXT NOT NULL CHECK (period ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    currency CHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (household_id, period)
);

CREATE INDEX budget_plans_household_id_idx
    ON budget_plans(household_id, period);

CREATE TABLE budget_lines (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    budget_plan_id BIGINT NOT NULL REFERENCES budget_plans(id) ON DELETE CASCADE,
    external_category_ref TEXT,
    semantic_group TEXT,
    planned_minor BIGINT NOT NULL CHECK (planned_minor >= 0),
    kind TEXT NOT NULL CHECK (kind IN ('essential', 'flexible', 'debt', 'saving', 'investment', 'goal')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (
        (
            external_category_ref IS NOT NULL
            AND length(btrim(external_category_ref)) > 0
            AND semantic_group IS NULL
        )
        OR (
            semantic_group IS NOT NULL
            AND length(btrim(semantic_group)) > 0
            AND external_category_ref IS NULL
        )
    )
);

CREATE INDEX budget_lines_budget_plan_id_idx
    ON budget_lines(budget_plan_id, id);

-- +goose Down
DROP TABLE budget_lines;
DROP TABLE budget_plans;
