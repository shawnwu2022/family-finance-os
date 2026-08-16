-- name: CreateDebt :one
INSERT INTO debts (
    household_id,
    name,
    debt_type,
    original_principal_minor,
    balance_minor,
    currency,
    apr,
    rate_type,
    lpr_spread,
    term_remaining_months,
    due_day,
    repayment_type,
    minimum_payment_minor,
    scheduled_payment_minor,
    prepayment_fee_rate,
    prepayment_restricted_months,
    revolving,
    source_account_ref,
    active
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19
)
RETURNING *;

-- name: GetDebt :one
SELECT *
FROM debts
WHERE id = $1;

-- name: ListDebtsByHousehold :many
SELECT *
FROM debts
WHERE household_id = $1
ORDER BY id;

-- name: UpdateDebtBalance :one
UPDATE debts
SET balance_minor = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;
