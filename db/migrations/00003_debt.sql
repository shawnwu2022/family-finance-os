-- +goose Up
CREATE TABLE debts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    debt_type TEXT NOT NULL CHECK (debt_type IN ('mortgage', 'credit_card', 'consumer_loan', 'installment', 'other')),
    original_principal_minor BIGINT NOT NULL CHECK (original_principal_minor >= 0),
    balance_minor BIGINT NOT NULL CHECK (balance_minor >= 0),
    currency CHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    apr NUMERIC CHECK (apr IS NULL OR apr >= 0),
    rate_type TEXT NOT NULL CHECK (rate_type IN ('fixed', 'lpr_spread', 'other_variable')),
    lpr_spread NUMERIC,
    term_remaining_months INTEGER NOT NULL DEFAULT 0 CHECK (term_remaining_months >= 0),
    due_day INTEGER NOT NULL CHECK (due_day BETWEEN 1 AND 31),
    repayment_type TEXT NOT NULL CHECK (repayment_type IN ('annuity', 'equal_principal', 'revolving', 'custom')),
    minimum_payment_minor BIGINT NOT NULL DEFAULT 0 CHECK (minimum_payment_minor >= 0),
    scheduled_payment_minor BIGINT NOT NULL DEFAULT 0 CHECK (scheduled_payment_minor >= 0),
    prepayment_fee_rate NUMERIC NOT NULL DEFAULT 0 CHECK (prepayment_fee_rate >= 0),
    prepayment_restricted_months INTEGER NOT NULL DEFAULT 0 CHECK (prepayment_restricted_months >= 0),
    revolving BOOLEAN NOT NULL DEFAULT FALSE,
    source_account_ref TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (rate_type <> 'lpr_spread' OR lpr_spread IS NOT NULL),
    CHECK (repayment_type NOT IN ('annuity', 'equal_principal') OR term_remaining_months > 0),
    CHECK (repayment_type <> 'revolving' OR minimum_payment_minor > 0),
    CHECK (repayment_type <> 'custom' OR scheduled_payment_minor > 0),
    CHECK (source_account_ref IS NULL OR length(btrim(source_account_ref)) > 0)
);

CREATE INDEX debts_household_active_idx
    ON debts(household_id, active, id);

-- +goose Down
DROP TABLE debts;
