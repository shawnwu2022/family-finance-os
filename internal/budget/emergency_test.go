package budget

import (
	"errors"
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestCalculateEmergencyMonths(t *testing.T) {
	got, err := CalculateEmergencyMonths(
		money.Money{Minor: 600_000, Currency: "CNY"},
		money.Money{Minor: 200_000, Currency: "CNY"},
	)
	if err != nil {
		t.Fatalf("CalculateEmergencyMonths: %v", err)
	}
	if !got.Applicable || got.Value == nil || got.Value.Cmp(apd.New(3, 0)) != 0 {
		t.Fatalf("result=%#v want 3 months", got)
	}
}

func TestCalculateEmergencyMonthsZeroEssentialIsNotApplicable(t *testing.T) {
	got, err := CalculateEmergencyMonths(
		money.Money{Minor: 600_000, Currency: "CNY"},
		money.Money{Currency: "CNY"},
	)
	if err != nil {
		t.Fatalf("CalculateEmergencyMonths: %v", err)
	}
	if got.Applicable || got.Value != nil {
		t.Fatalf("result=%#v want not-applicable", got)
	}
}

func TestCalculateEmergencyMonthsRejectsInvalidInputs(t *testing.T) {
	_, err := CalculateEmergencyMonths(
		money.Money{Minor: 100, Currency: "CNY"},
		money.Money{Minor: 10, Currency: "USD"},
	)
	if !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("currency error=%v", err)
	}

	_, err = CalculateEmergencyMonths(
		money.Money{Minor: -1, Currency: "CNY"},
		money.Money{Minor: 10, Currency: "CNY"},
	)
	if !errors.Is(err, ErrNegativeEmergencyInput) {
		t.Fatalf("negative liquid error=%v", err)
	}

	_, err = CalculateEmergencyMonths(
		money.Money{Minor: 100, Currency: "CNY"},
		money.Money{Minor: -1, Currency: "CNY"},
	)
	if !errors.Is(err, ErrNegativeEmergencyInput) {
		t.Fatalf("negative essential error=%v", err)
	}
}
