package appapi

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/store"
)

func TestPostgresPlannerRoundTripIntegration(t *testing.T) {
	pool := openPlannerIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var householdID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO households (name, base_currency, timezone)
		VALUES ('Planner Test', 'CNY', 'Asia/Shanghai')
		RETURNING id
	`).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO household_policies (household_id, liquidity_floor_minor, currency)
		VALUES ($1, 50000, 'CNY')
	`, householdID); err != nil {
		t.Fatalf("insert policy: %v", err)
	}

	var planID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO budget_plans (household_id, period, currency)
		VALUES ($1, '2026-08', 'CNY')
		RETURNING id
	`, householdID).Scan(&planID); err != nil {
		t.Fatalf("insert budget plan: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO budget_lines (budget_plan_id, external_category_ref, planned_minor, kind)
		VALUES ($1, 'food', 100000, 'essential')
	`, planID); err != nil {
		t.Fatalf("insert budget line: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO debts (
			household_id, name, debt_type, original_principal_minor, balance_minor, currency,
			apr, rate_type, term_remaining_months, due_day, repayment_type,
			minimum_payment_minor, scheduled_payment_minor, prepayment_fee_rate,
			prepayment_restricted_months, revolving, active
		) VALUES
			($1, 'active card', 'credit_card', 300000, 200000, 'CNY', 0.18, 'fixed', 0, 20, 'revolving', 20000, 25000, 0.015, 2, TRUE, TRUE),
			($1, 'closed card', 'credit_card', 100000, 0, 'CNY', 0.20, 'fixed', 0, 5, 'revolving', 10000, 0, 0, 0, TRUE, FALSE)
	`, householdID); err != nil {
		t.Fatalf("insert debts: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO financial_goals (
			household_id, name, target_minor, funded_minor, target_date, priority,
			flexibility, monthly_contribution_minor, currency, active
		) VALUES ($1, 'education', 1000000, 200000, DATE '2027-08-01', 1, 'hard', 50000, 'CNY', TRUE)
	`, householdID); err != nil {
		t.Fatalf("insert goal: %v", err)
	}

	planner := NewPostgresPlanner(pool)
	profile, err := planner.Profile(ctx, householdID)
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if profile.Household.ID != householdID || profile.Policy.LiquidityFloor.Minor != 50000 || profile.Policy.LiquidityFloor.Currency != "CNY" {
		t.Fatalf("profile=%#v", profile)
	}

	plan, err := planner.BudgetPlan(ctx, householdID, "2026-08")
	if err != nil {
		t.Fatalf("BudgetPlan: %v", err)
	}
	if plan.ID != planID || len(plan.Lines) != 1 || plan.Lines[0].Planned.Minor != 100000 {
		t.Fatalf("plan=%#v", plan)
	}

	debts, err := planner.Debts(ctx, householdID)
	if err != nil {
		t.Fatalf("Debts: %v", err)
	}
	if len(debts) != 1 || debts[0].Name != "active card" || debts[0].APR != "0.18" || debts[0].ScheduledPayment.Minor != 25000 {
		t.Fatalf("debts=%#v", debts)
	}
	if debts[0].OriginalPrincipal.Minor != 300000 || debts[0].RateType != "fixed" || debts[0].PrepaymentFeeRate != "0.015" || debts[0].PrepaymentRestrictedMonths != 2 || !debts[0].Revolving {
		t.Fatalf("debt simulation fields=%#v", debts[0])
	}

	goalList, err := planner.Goals(ctx, householdID)
	if err != nil {
		t.Fatalf("Goals: %v", err)
	}
	if len(goalList) != 1 || goalList[0].Name != "education" || goalList[0].Target.Minor != 1000000 {
		t.Fatalf("goals=%#v", goalList)
	}
}

func openPlannerIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}
	portRaw := os.Getenv("TEST_POSTGRES_PORT")
	if portRaw == "" {
		portRaw = "5432"
	}
	port, err := strconv.ParseUint(portRaw, 10, 16)
	if err != nil {
		t.Fatalf("parse TEST_POSTGRES_PORT: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := store.OpenPostgres(ctx, config.DatabaseConfig{
		Host: host, Port: uint16(port), Name: os.Getenv("TEST_POSTGRES_DB"),
		User: os.Getenv("TEST_POSTGRES_USER"), Password: os.Getenv("TEST_POSTGRES_PASSWORD"), SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
