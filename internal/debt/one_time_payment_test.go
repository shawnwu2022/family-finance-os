package debt

import (
	"testing"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestSimulateOneTimeExtraPaymentAppliesOnlyOnce(t *testing.T) {
	debt := baseDebtForTest()
	extra := money.Money{Minor: 50_000, Currency: "CNY"}

	got, err := SimulateOneTimeExtraPayment(debt, extra)
	if err != nil {
		t.Fatalf("SimulateOneTimeExtraPayment: %v", err)
	}
	if len(got.Payments) == 0 {
		t.Fatal("payments are empty")
	}
	if got.Payments[0].ExtraPrincipal != extra {
		t.Fatalf("month 1 extra=%#v want %#v", got.Payments[0].ExtraPrincipal, extra)
	}
	for i := 1; i < len(got.Payments); i++ {
		if got.Payments[i].ExtraPrincipal.Minor != 0 {
			t.Fatalf("month %d extra=%#v want zero", i+1, got.Payments[i].ExtraPrincipal)
		}
	}

	baseline, err := SimulateDebt(debt, money.Money{Currency: "CNY"})
	if err != nil {
		t.Fatalf("baseline SimulateDebt: %v", err)
	}
	if got.TotalInterest.Minor >= baseline.TotalInterest.Minor {
		t.Fatalf("one-time interest=%d baseline=%d", got.TotalInterest.Minor, baseline.TotalInterest.Minor)
	}
}

func TestSimulateOneTimeExtraPaymentWaitsUntilRestrictionExpires(t *testing.T) {
	debt := baseDebtForTest()
	debt.PrepaymentRestrictedMonths = 1
	extra := money.Money{Minor: 50_000, Currency: "CNY"}

	got, err := SimulateOneTimeExtraPayment(debt, extra)
	if err != nil {
		t.Fatalf("SimulateOneTimeExtraPayment: %v", err)
	}
	if len(got.Payments) < 2 {
		t.Fatalf("payments=%d want at least 2", len(got.Payments))
	}
	if got.Payments[0].ExtraPrincipal.Minor != 0 {
		t.Fatalf("restricted month extra=%#v want zero", got.Payments[0].ExtraPrincipal)
	}
	if got.Payments[1].ExtraPrincipal != extra {
		t.Fatalf("first eligible month extra=%#v want %#v", got.Payments[1].ExtraPrincipal, extra)
	}
	for i := 2; i < len(got.Payments); i++ {
		if got.Payments[i].ExtraPrincipal.Minor != 0 {
			t.Fatalf("month %d extra=%#v want zero", i+1, got.Payments[i].ExtraPrincipal)
		}
	}
}
