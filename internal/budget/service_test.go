package budget

import (
	"errors"
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestCalculateBudgetLineMetrics(t *testing.T) {
	line := BudgetLine{Planned: money.Money{Minor: 10000, Currency: "CNY"}}
	got, err := CalculateBudgetLine(line, money.Money{Minor: 7500, Currency: "CNY"})
	if err != nil {
		t.Fatalf("CalculateBudgetLine: %v", err)
	}
	if got.Remaining.Minor != 2500 {
		t.Fatalf("remaining=%d want 2500", got.Remaining.Minor)
	}
	want := apd.New(75, -2)
	if got.Utilization == nil || got.Utilization.Cmp(want) != 0 {
		t.Fatalf("utilization=%v want 0.75", got.Utilization)
	}
}

func TestCalculateBudgetLineOverBudgetPreservesDeficit(t *testing.T) {
	line := BudgetLine{Planned: money.Money{Minor: 10000, Currency: "CNY"}}
	got, err := CalculateBudgetLine(line, money.Money{Minor: 12500, Currency: "CNY"})
	if err != nil {
		t.Fatalf("CalculateBudgetLine: %v", err)
	}
	if got.Remaining.Minor != -2500 {
		t.Fatalf("remaining=%d want -2500", got.Remaining.Minor)
	}
	want := apd.New(125, -2)
	if got.Utilization == nil || got.Utilization.Cmp(want) != 0 {
		t.Fatalf("utilization=%v want 1.25", got.Utilization)
	}
}

func TestCalculateBudgetLineRefundPreservesSignedActual(t *testing.T) {
	line := BudgetLine{Planned: money.Money{Minor: 10000, Currency: "CNY"}}
	got, err := CalculateBudgetLine(line, money.Money{Minor: -1000, Currency: "CNY"})
	if err != nil {
		t.Fatalf("CalculateBudgetLine: %v", err)
	}
	if got.Remaining.Minor != 11000 {
		t.Fatalf("remaining=%d want 11000", got.Remaining.Minor)
	}
	want := apd.New(-1, -1)
	if got.Utilization == nil || got.Utilization.Cmp(want) != 0 {
		t.Fatalf("utilization=%v want -0.1", got.Utilization)
	}
}

func TestCalculateBudgetLineZeroPlannedIsNotApplicable(t *testing.T) {
	line := BudgetLine{Planned: money.Money{Currency: "CNY"}}
	got, err := CalculateBudgetLine(line, money.Money{Currency: "CNY"})
	if err != nil {
		t.Fatalf("CalculateBudgetLine: %v", err)
	}
	if got.Utilization != nil {
		t.Fatalf("utilization=%v want nil", got.Utilization)
	}
}

func TestCalculateBudgetLineRejectsInvalidPlanAndCurrencyMismatch(t *testing.T) {
	_, err := CalculateBudgetLine(BudgetLine{Planned: money.Money{Minor: -1, Currency: "CNY"}}, money.Money{Currency: "CNY"})
	if !errors.Is(err, ErrNegativePlanned) {
		t.Fatalf("negative planned error=%v", err)
	}
	_, err = CalculateBudgetLine(BudgetLine{Planned: money.Money{Minor: 100, Currency: "CNY"}}, money.Money{Minor: 1, Currency: "USD"})
	if !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("currency error=%v", err)
	}
}

func FuzzCalculateBudgetLineRemainingInvariant(f *testing.F) {
	f.Add(uint32(10000), uint32(7500), uint32(1000))
	f.Fuzz(func(t *testing.T, planned, actual, delta uint32) {
		if uint64(actual)+uint64(delta) > uint64(^uint32(0)) {
			t.Skip()
		}
		line := BudgetLine{Planned: money.Money{Minor: int64(planned), Currency: "CNY"}}
		before, err := CalculateBudgetLine(line, money.Money{Minor: int64(actual), Currency: "CNY"})
		if err != nil {
			t.Fatalf("before: %v", err)
		}
		after, err := CalculateBudgetLine(line, money.Money{Minor: int64(actual) + int64(delta), Currency: "CNY"})
		if err != nil {
			t.Fatalf("after: %v", err)
		}
		if after.Remaining.Minor > before.Remaining.Minor {
			t.Fatalf("remaining increased: before=%d after=%d", before.Remaining.Minor, after.Remaining.Minor)
		}
	})
}
