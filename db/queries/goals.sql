-- name: CreateFinancialGoal :one
INSERT INTO financial_goals (
    household_id,
    name,
    target_minor,
    funded_minor,
    target_date,
    priority,
    flexibility,
    monthly_contribution_minor,
    currency,
    active
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING
    id,
    household_id,
    name,
    target_minor,
    funded_minor,
    target_date,
    priority,
    flexibility,
    monthly_contribution_minor,
    currency,
    active;

-- name: GetFinancialGoal :one
SELECT
    id,
    household_id,
    name,
    target_minor,
    funded_minor,
    target_date,
    priority,
    flexibility,
    monthly_contribution_minor,
    currency,
    active
FROM financial_goals
WHERE id = $1;

-- name: ListFinancialGoalsByHousehold :many
SELECT
    id,
    household_id,
    name,
    target_minor,
    funded_minor,
    target_date,
    priority,
    flexibility,
    monthly_contribution_minor,
    currency,
    active
FROM financial_goals
WHERE household_id = $1
ORDER BY active DESC, priority ASC, target_date ASC, id ASC;
