package goals

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

func TestServiceGoalRoundTripIntegration(t *testing.T) {
	pool := openGoalsIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	homeService := householdpkg.NewService(pool)
	home, err := homeService.CreateHousehold(ctx, householdpkg.NewHousehold{
		Name: "目标测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}

	service := NewService(pool)
	targetDate := time.Date(2030, time.June, 30, 15, 45, 0, 0, time.FixedZone("CST", 8*60*60))
	created, err := service.CreateGoal(ctx, NewFinancialGoal{
		HouseholdID: home.ID,
		Name:        "孩子教育金",
		Target:      money.Money{Minor: 5_000_000, Currency: "CNY"},
		Funded:      money.Money{Minor: 1_200_000, Currency: "CNY"},
		TargetDate:  targetDate,
		Priority:    1,
		Flexibility: GoalFlexibilityHard,
		MonthlyContribution: money.Money{
			Minor: 80_000, Currency: "CNY",
		},
		Active: true,
	})
	if err != nil {
		t.Fatalf("CreateGoal: %v", err)
	}
	if created.ID <= 0 || created.HouseholdID != home.ID {
		t.Fatalf("created = %#v", created)
	}

	got, err := service.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetGoal: %v", err)
	}
	assertGoalRoundTrip(t, got, created)

	goals, err := service.ListGoals(ctx, home.ID)
	if err != nil {
		t.Fatalf("ListGoals: %v", err)
	}
	if len(goals) != 1 {
		t.Fatalf("goals len = %d, want 1", len(goals))
	}
	assertGoalRoundTrip(t, goals[0], created)
}

func TestServiceRejectsNonBaseCurrencyIntegration(t *testing.T) {
	pool := openGoalsIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	homeService := householdpkg.NewService(pool)
	home, err := homeService.CreateHousehold(ctx, householdpkg.NewHousehold{
		Name: "目标币种约束家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}

	service := NewService(pool)
	_, err = service.CreateGoal(ctx, NewFinancialGoal{
		HouseholdID: home.ID,
		Name:        "美元目标",
		Target:      money.Money{Minor: 100_000, Currency: "USD"},
		Funded:      money.Money{Minor: 10_000, Currency: "USD"},
		TargetDate:  time.Date(2028, time.December, 31, 0, 0, 0, 0, time.UTC),
		Priority:    1,
		Flexibility: GoalFlexibilityFlexible,
		MonthlyContribution: money.Money{
			Minor: 5_000, Currency: "USD",
		},
		Active: true,
	})
	if err == nil {
		t.Fatal("CreateGoal accepted a currency different from household base currency")
	}
}

func assertGoalRoundTrip(t *testing.T, got, want FinancialGoal) {
	t.Helper()
	if got.ID != want.ID || got.HouseholdID != want.HouseholdID || got.Name != want.Name ||
		got.Target != want.Target || got.Funded != want.Funded || got.Priority != want.Priority ||
		got.Flexibility != want.Flexibility || got.MonthlyContribution != want.MonthlyContribution ||
		got.Active != want.Active {
		t.Fatalf("goal = %#v, want %#v", got, want)
	}
	if got.TargetDate.Year() != want.TargetDate.Year() || got.TargetDate.Month() != want.TargetDate.Month() || got.TargetDate.Day() != want.TargetDate.Day() {
		t.Fatalf("target date = %v, want calendar date %04d-%02d-%02d", got.TargetDate, want.TargetDate.Year(), want.TargetDate.Month(), want.TargetDate.Day())
	}
	if got.TargetDate.Hour() != 0 || got.TargetDate.Minute() != 0 || got.TargetDate.Second() != 0 {
		t.Fatalf("persisted target date carries time-of-day: %v", got.TargetDate)
	}
}

func openGoalsIntegrationPool(t *testing.T) *pgxpool.Pool {
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
