-- name: CreateHousehold :one
INSERT INTO households (name, base_currency, timezone)
VALUES ($1, $2, $3)
RETURNING id, name, base_currency, timezone;

-- name: GetHousehold :one
SELECT id, name, base_currency, timezone
FROM households
WHERE id = $1;

-- name: CreateHouseholdMember :one
INSERT INTO household_members (household_id, name, kind, active)
VALUES ($1, $2, $3, $4)
RETURNING id, household_id, name, kind, active;

-- name: ListHouseholdMembers :many
SELECT id, household_id, name, kind, active
FROM household_members
WHERE household_id = $1
ORDER BY id;

-- name: CreateIncomeSource :one
INSERT INTO income_sources (
    household_id, member_id, name, amount_minor, currency, cadence, stability, active
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, household_id, member_id, name, amount_minor, currency, cadence, stability, active;

-- name: ListIncomeSources :many
SELECT id, household_id, member_id, name, amount_minor, currency, cadence, stability, active
FROM income_sources
WHERE household_id = $1
ORDER BY id;

-- name: CreateExpenseBaseline :one
INSERT INTO expense_baselines (
    household_id, name, amount_minor, currency, cadence, essential, active
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, household_id, name, amount_minor, currency, cadence, essential, active;

-- name: ListExpenseBaselines :many
SELECT id, household_id, name, amount_minor, currency, cadence, essential, active
FROM expense_baselines
WHERE household_id = $1
ORDER BY id;

-- name: UpsertHouseholdPolicy :one
INSERT INTO household_policies (household_id, liquidity_floor_minor, currency)
VALUES ($1, $2, $3)
ON CONFLICT (household_id) DO UPDATE SET
    liquidity_floor_minor = EXCLUDED.liquidity_floor_minor,
    currency = EXCLUDED.currency,
    updated_at = CURRENT_TIMESTAMP
RETURNING household_id, liquidity_floor_minor, currency;

-- name: GetHouseholdPolicy :one
SELECT household_id, liquidity_floor_minor, currency
FROM household_policies
WHERE household_id = $1;
