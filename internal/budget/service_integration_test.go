package budget

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	householdpkg "github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/store"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestServicePlanRoundTripIntegration(t *testing.T) {
	pool := openBudgetIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	homeService := householdpkg.NewService(pool)
	home, err := homeService.CreateHousehold(ctx, householdpkg.NewHousehold{Name: "预算测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}

	service := NewService(pool)
	plan, err := service.CreatePlan(ctx, NewBudgetPlan{HouseholdID: home.ID, Period: "2026-08", Currency: "CNY"})
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	food, err := service.AddLine(ctx, plan.ID, NewBudgetLine{
		ExternalCategoryRef: "ez-category-food",
		Planned:             money.Money{Minor: 180_000, Currency: "CNY"},
		Kind:                BudgetKindEssential,
	})
	if err != nil {
		t.Fatalf("AddLine food: %v", err)
	}
	debt, err := service.AddLine(ctx, plan.ID, NewBudgetLine{
		SemanticGroup: "consumer-debt",
		Planned:       money.Money{Minor: 90_000, Currency: "CNY"},
		Kind:          BudgetKindDebt,
	})
	if err != nil {
		t.Fatalf("AddLine debt: %v", err)
	}

	got, err := service.GetPlan(ctx, home.ID, "2026-08")
	if err != nil {
		t.Fatalf("GetPlan: %v", err)
	}
	if got.ID != plan.ID || got.HouseholdID != home.ID || got.Period != "2026-08" || got.Currency != "CNY" {
		t.Fatalf("plan = %#v", got)
	}
	if len(got.Lines) != 2 || got.Lines[0] != food || got.Lines[1] != debt {
		t.Fatalf("lines = %#v", got.Lines)
	}
}

func TestServiceRejectsInvalidPlanAndLineIntegration(t *testing.T) {
	pool := openBudgetIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	homeService := householdpkg.NewService(pool)
	home, err := homeService.CreateHousehold(ctx, householdpkg.NewHousehold{Name: "预算约束家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	service := NewService(pool)

	if _, err := service.CreatePlan(ctx, NewBudgetPlan{HouseholdID: home.ID, Period: "2026-08", Currency: "USD"}); err == nil {
		t.Fatal("CreatePlan accepted non-base currency")
	}
	if _, err := service.CreatePlan(ctx, NewBudgetPlan{HouseholdID: home.ID, Period: "2026-13", Currency: "CNY"}); err == nil {
		t.Fatal("CreatePlan accepted invalid period")
	}
	plan, err := service.CreatePlan(ctx, NewBudgetPlan{HouseholdID: home.ID, Period: "2026-09", Currency: "CNY"})
	if err != nil {
		t.Fatalf("CreatePlan valid: %v", err)
	}

	cases := []NewBudgetLine{
		{ExternalCategoryRef: "cat", SemanticGroup: "group", Planned: money.Money{Minor: 100, Currency: "CNY"}, Kind: BudgetKindEssential},
		{ExternalCategoryRef: "cat", Planned: money.Money{Minor: 100, Currency: "USD"}, Kind: BudgetKindEssential},
		{ExternalCategoryRef: "cat", Planned: money.Money{Minor: 100, Currency: "CNY"}, Kind: BudgetKind("mystery")},
	}
	for i, input := range cases {
		if _, err := service.AddLine(ctx, plan.ID, input); err == nil {
			t.Fatalf("AddLine invalid case %d was accepted", i)
		}
	}
}

func openBudgetIntegrationPool(t *testing.T) *pgxpool.Pool {
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
