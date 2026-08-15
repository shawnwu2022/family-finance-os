package household

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/store"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestServiceProfileRoundTripIntegration(t *testing.T) {
	pool := openIntegrationPool(t)
	service := NewService(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	home, err := service.CreateHousehold(ctx, NewHousehold{
		Name:         "测试家庭",
		BaseCurrency: "CNY",
		Timezone:     "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}

	member, err := service.AddMember(ctx, home.ID, NewMember{
		Name:   "主要成员",
		Kind:   MemberKindAdult,
		Active: true,
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	memberID := member.ID
	if _, err := service.AddIncomeSource(ctx, home.ID, NewIncomeSource{
		MemberID:  &memberID,
		Name:      "工资",
		Amount:    money.Money{Minor: 3_800_000, Currency: "CNY"},
		Cadence:   CadenceMonthly,
		Stability: IncomeStabilityStable,
		Active:    true,
	}); err != nil {
		t.Fatalf("AddIncomeSource: %v", err)
	}

	if _, err := service.AddExpenseBaseline(ctx, home.ID, NewExpenseBaseline{
		Name:      "家庭必要支出",
		Amount:    money.Money{Minor: 1_650_000, Currency: "CNY"},
		Cadence:   CadenceMonthly,
		Essential: true,
		Active:    true,
	}); err != nil {
		t.Fatalf("AddExpenseBaseline: %v", err)
	}

	if _, err := service.SetPolicy(ctx, home.ID, NewHouseholdPolicy{
		LiquidityFloor: money.Money{
			Minor:    12_000_000,
			Currency: "CNY",
		},
	}); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	profile, err := service.GetProfile(ctx, home.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile.Household != home {
		t.Fatalf("Household = %#v, want %#v", profile.Household, home)
	}
	if len(profile.Members) != 1 || profile.Members[0] != member {
		t.Fatalf("Members = %#v", profile.Members)
	}
	if len(profile.IncomeSources) != 1 || profile.IncomeSources[0].Amount.Minor != 3_800_000 {
		t.Fatalf("IncomeSources = %#v", profile.IncomeSources)
	}
	if len(profile.ExpenseBaselines) != 1 || !profile.ExpenseBaselines[0].Essential {
		t.Fatalf("ExpenseBaselines = %#v", profile.ExpenseBaselines)
	}
	if profile.Policy.LiquidityFloor != (money.Money{Minor: 12_000_000, Currency: "CNY"}) {
		t.Fatalf("LiquidityFloor = %#v", profile.Policy.LiquidityFloor)
	}
}

func TestServiceRejectsPolicyCurrencyDifferentFromHouseholdBase(t *testing.T) {
	pool := openIntegrationPool(t)
	service := NewService(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	home, err := service.CreateHousehold(ctx, NewHousehold{
		Name:         "币种约束家庭",
		BaseCurrency: "CNY",
		Timezone:     "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}

	_, err = service.SetPolicy(ctx, home.ID, NewHouseholdPolicy{
		LiquidityFloor: money.Money{
			Minor:    100_000,
			Currency: "USD",
		},
	})
	if err == nil {
		t.Fatal("SetPolicy accepted a liquidity floor in a non-base currency")
	}
}

func openIntegrationPool(t *testing.T) *pgxpool.Pool {
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
		Host:     host,
		Port:     uint16(port),
		Name:     os.Getenv("TEST_POSTGRES_DB"),
		User:     os.Getenv("TEST_POSTGRES_USER"),
		Password: os.Getenv("TEST_POSTGRES_PASSWORD"),
		SSLMode:  "disable",
	})
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
