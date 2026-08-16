package debt

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func FuzzExtraPaymentNeverDelaysPayoff(f *testing.F) {
	f.Add(uint32(300_000), uint8(12), uint32(10_000))
	f.Fuzz(func(t *testing.T, principalRaw uint32, termRaw uint8, extraRaw uint32) {
		principal := int64(principalRaw%1_000_000) + 1
		term := int(termRaw%60) + 1
		extra := int64(extraRaw % 100_000)
		debt := DebtContract{
			ID:                         1,
			Name:                       "fuzz",
			OriginalPrincipal:          money.Money{Minor: principal, Currency: "CNY"},
			Balance:                    money.Money{Minor: principal, Currency: "CNY"},
			APR:                        apd.New(0, 0),
			RateType:                   DebtRateFixed,
			TermRemainingMonths:        term,
			DueDay:                     1,
			RepaymentType:              DebtRepaymentAnnuity,
			MinimumPayment:             money.Money{Currency: "CNY"},
			ScheduledPayment:           money.Money{Currency: "CNY"},
			PrepaymentFeeRate:          apd.New(0, 0),
			PrepaymentRestrictedMonths: 0,
			Active:                     true,
		}
		baseline, err := SimulateDebt(debt, money.Money{Currency: "CNY"})
		if err != nil {
			t.Fatalf("baseline: %v", err)
		}
		withExtra, err := SimulateDebt(debt, money.Money{Minor: extra, Currency: "CNY"})
		if err != nil {
			t.Fatalf("with extra: %v", err)
		}
		if withExtra.PayoffMonths > baseline.PayoffMonths {
			t.Fatalf("extra payment delayed payoff: baseline=%d withExtra=%d", baseline.PayoffMonths, withExtra.PayoffMonths)
		}
		if len(withExtra.Payments) > 0 && withExtra.Payments[len(withExtra.Payments)-1].ClosingBalance.Minor != 0 {
			t.Fatalf("final balance=%d", withExtra.Payments[len(withExtra.Payments)-1].ClosingBalance.Minor)
		}
	})
}
