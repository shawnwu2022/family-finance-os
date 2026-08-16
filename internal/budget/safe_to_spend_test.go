package budget

import (
	"errors"
	"testing"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestCalculateSafeToSpendReturnsComponents(t *testing.T) {
	got, err := CalculateSafeToSpend(SafeToSpendInput{
		LiquidDiscretionaryPool:        money.Money{Minor: 1_000_000, Currency: "CNY"},
		UpcomingMandatoryExpenses:      money.Money{Minor: 200_000, Currency: "CNY"},
		DebtCommitments:                money.Money{Minor: 100_000, Currency: "CNY"},
		EssentialReserveUntilPeriodEnd: money.Money{Minor: 300_000, Currency: "CNY"},
		EmergencyFundGapReserved:       money.Money{Minor: 150_000, Currency: "CNY"},
		HardGoalContributions:          money.Money{Minor: 50_000, Currency: "CNY"},
	})
	if err != nil {
		t.Fatalf("CalculateSafeToSpend: %v", err)
	}
	if got.Amount.Minor != 200_000 {
		t.Fatalf("amount=%d want 200000", got.Amount.Minor)
	}
	if got.IsDeficit {
		t.Fatal("positive safe-to-spend marked as deficit")
	}
	if got.Components.DebtCommitments.Minor != 100_000 || got.Components.EmergencyFundGapReserved.Minor != 150_000 {
		t.Fatalf("components=%#v", got.Components)
	}
}

func TestCalculateSafeToSpendPreservesNegativeDeficit(t *testing.T) {
	got, err := CalculateSafeToSpend(SafeToSpendInput{
		LiquidDiscretionaryPool:        money.Money{Minor: 500_000, Currency: "CNY"},
		UpcomingMandatoryExpenses:      money.Money{Minor: 200_000, Currency: "CNY"},
		DebtCommitments:                money.Money{Minor: 100_000, Currency: "CNY"},
		EssentialReserveUntilPeriodEnd: money.Money{Minor: 300_000, Currency: "CNY"},
		EmergencyFundGapReserved:       money.Money{Minor: 100_000, Currency: "CNY"},
		HardGoalContributions:          money.Money{Minor: 50_000, Currency: "CNY"},
	})
	if err != nil {
		t.Fatalf("CalculateSafeToSpend: %v", err)
	}
	if got.Amount.Minor != -250_000 || !got.IsDeficit {
		t.Fatalf("result=%#v want -250000 deficit", got)
	}
}

func TestCalculateSafeToSpendAvoidsDoubleCountingReservedDebt(t *testing.T) {
	got, err := CalculateSafeToSpend(SafeToSpendInput{
		LiquidDiscretionaryPool:        money.Money{Minor: 1_000_000, Currency: "CNY"},
		UpcomingMandatoryExpenses:      money.Money{Minor: 200_000, Currency: "CNY"},
		DebtCommitments:                money.Money{Minor: 100_000, Currency: "CNY"},
		DebtCommitmentsAlreadyReserved: true,
		EssentialReserveUntilPeriodEnd: money.Money{Minor: 300_000, Currency: "CNY"},
		EmergencyFundGapReserved:       money.Money{Minor: 100_000, Currency: "CNY"},
		HardGoalContributions:          money.Money{Minor: 50_000, Currency: "CNY"},
	})
	if err != nil {
		t.Fatalf("CalculateSafeToSpend: %v", err)
	}
	if got.Amount.Minor != 350_000 {
		t.Fatalf("amount=%d want 350000", got.Amount.Minor)
	}
	if got.Components.DebtCommitments.Minor != 0 {
		t.Fatalf("applied debt=%d want 0", got.Components.DebtCommitments.Minor)
	}
}

func TestCalculateSafeToSpendRejectsCurrencyMismatch(t *testing.T) {
	_, err := CalculateSafeToSpend(SafeToSpendInput{
		LiquidDiscretionaryPool:        money.Money{Minor: 100, Currency: "CNY"},
		UpcomingMandatoryExpenses:      money.Money{Minor: 1, Currency: "USD"},
		DebtCommitments:                money.Money{Currency: "CNY"},
		EssentialReserveUntilPeriodEnd: money.Money{Currency: "CNY"},
		EmergencyFundGapReserved:       money.Money{Currency: "CNY"},
		HardGoalContributions:          money.Money{Currency: "CNY"},
	})
	if !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("error=%v want currency mismatch", err)
	}
}

func FuzzCalculateSafeToSpendPurchaseMonotonicity(f *testing.F) {
	f.Add(uint32(1_000_000), uint32(100_000), uint32(50_000))
	f.Fuzz(func(t *testing.T, liquid, purchaseA, purchaseDelta uint32) {
		purchaseB := uint64(purchaseA) + uint64(purchaseDelta)
		if purchaseB > uint64(liquid) || uint64(purchaseA) > uint64(liquid) {
			t.Skip()
		}
		base := SafeToSpendInput{
			LiquidDiscretionaryPool:        money.Money{Minor: int64(uint64(liquid) - uint64(purchaseA)), Currency: "CNY"},
			UpcomingMandatoryExpenses:      money.Money{Minor: 10_000, Currency: "CNY"},
			DebtCommitments:                money.Money{Minor: 5_000, Currency: "CNY"},
			EssentialReserveUntilPeriodEnd: money.Money{Minor: 20_000, Currency: "CNY"},
			EmergencyFundGapReserved:       money.Money{Currency: "CNY"},
			HardGoalContributions:          money.Money{Currency: "CNY"},
		}
		before, err := CalculateSafeToSpend(base)
		if err != nil {
			t.Fatalf("before: %v", err)
		}
		base.LiquidDiscretionaryPool.Minor = int64(uint64(liquid) - purchaseB)
		after, err := CalculateSafeToSpend(base)
		if err != nil {
			t.Fatalf("after: %v", err)
		}
		if after.Amount.Minor > before.Amount.Minor {
			t.Fatalf("larger purchase increased safe-to-spend: before=%d after=%d", before.Amount.Minor, after.Amount.Minor)
		}
	})
}
