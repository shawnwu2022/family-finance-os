package analytics

import (
	"errors"
	"math"
	"testing"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestCalculateCashflowExcludesTransfersAndCreditCardRepayment(t *testing.T) {
	events := []CashflowEvent{
		{Type: CashflowEventIncome, Amount: money.Money{Minor: 300000, Currency: "CNY"}},
		{Type: CashflowEventExpense, Amount: money.Money{Minor: 120000, Currency: "CNY"}},
		{Type: CashflowEventTransfer, Amount: money.Money{Minor: 50000, Currency: "CNY"}},
		{Type: CashflowEventCreditCardRepayment, Amount: money.Money{Minor: 120000, Currency: "CNY"}},
		{Type: CashflowEventBalanceAdjustment, Amount: money.Money{Minor: 9999, Currency: "CNY"}},
	}

	got, err := CalculateCashflow(events, "CNY")
	if err != nil {
		t.Fatalf("CalculateCashflow: %v", err)
	}
	if got.RecognizedIncome.Minor != 300000 {
		t.Fatalf("income = %d, want 300000", got.RecognizedIncome.Minor)
	}
	if got.RecognizedExpense.Minor != 120000 {
		t.Fatalf("expense = %d, want 120000", got.RecognizedExpense.Minor)
	}
	if got.NetCashflow.Minor != 180000 {
		t.Fatalf("net = %d, want 180000", got.NetCashflow.Minor)
	}
}

func TestCalculateCashflowRefundReducesRecognizedExpense(t *testing.T) {
	got, err := CalculateCashflow([]CashflowEvent{
		{Type: CashflowEventExpense, Amount: money.Money{Minor: 10000, Currency: "CNY"}},
		{Type: CashflowEventRefund, Amount: money.Money{Minor: -2500, Currency: "CNY"}},
	}, "CNY")
	if err != nil {
		t.Fatalf("CalculateCashflow: %v", err)
	}
	if got.RecognizedExpense.Minor != 7500 {
		t.Fatalf("expense = %d, want 7500", got.RecognizedExpense.Minor)
	}
	if got.NetCashflow.Minor != -7500 {
		t.Fatalf("net = %d, want -7500", got.NetCashflow.Minor)
	}
}

func TestCalculateCashflowRejectsCurrencyMismatchAndOverflow(t *testing.T) {
	_, err := CalculateCashflow([]CashflowEvent{{Type: CashflowEventIncome, Amount: money.Money{Minor: 1, Currency: "USD"}}}, "CNY")
	if !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("currency error = %v", err)
	}

	_, err = CalculateCashflow([]CashflowEvent{
		{Type: CashflowEventIncome, Amount: money.Money{Minor: math.MaxInt64, Currency: "CNY"}},
		{Type: CashflowEventIncome, Amount: money.Money{Minor: 1, Currency: "CNY"}},
	}, "CNY")
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("overflow error = %v", err)
	}
}

func FuzzCalculateCashflowAccountingIdentity(f *testing.F) {
	f.Add(uint32(300000), uint32(120000), uint32(50000))
	f.Fuzz(func(t *testing.T, incomeMinor, expenseMinor, transferMinor uint32) {
		events := []CashflowEvent{
			{Type: CashflowEventIncome, Amount: money.Money{Minor: int64(incomeMinor), Currency: "CNY"}},
			{Type: CashflowEventExpense, Amount: money.Money{Minor: int64(expenseMinor), Currency: "CNY"}},
			{Type: CashflowEventTransfer, Amount: money.Money{Minor: int64(transferMinor), Currency: "CNY"}},
			{Type: CashflowEventCreditCardRepayment, Amount: money.Money{Minor: int64(expenseMinor), Currency: "CNY"}},
		}
		got, err := CalculateCashflow(events, "CNY")
		if err != nil {
			t.Fatalf("CalculateCashflow: %v", err)
		}
		want := int64(incomeMinor) - int64(expenseMinor)
		if got.NetCashflow.Minor != want {
			t.Fatalf("net = %d, want %d", got.NetCashflow.Minor, want)
		}
	})
}
