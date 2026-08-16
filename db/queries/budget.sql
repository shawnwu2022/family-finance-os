-- name: CreateBudgetPlan :one
INSERT INTO budget_plans (household_id, period, currency)
VALUES ($1, $2, $3)
RETURNING id, household_id, period, currency;

-- name: GetBudgetPlan :one
SELECT id, household_id, period, currency
FROM budget_plans
WHERE id = $1;

-- name: GetBudgetPlanByHouseholdPeriod :one
SELECT id, household_id, period, currency
FROM budget_plans
WHERE household_id = $1 AND period = $2;

-- name: CreateBudgetLine :one
INSERT INTO budget_lines (
    budget_plan_id, external_category_ref, semantic_group, planned_minor, kind
)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, budget_plan_id, external_category_ref, semantic_group, planned_minor, kind;

-- name: ListBudgetLines :many
SELECT id, budget_plan_id, external_category_ref, semantic_group, planned_minor, kind
FROM budget_lines
WHERE budget_plan_id = $1
ORDER BY id;
