package analytics

import (
	"reflect"
	"testing"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAggregateSpendingIncludesExpensesAndRefundsOnly(t *testing.T) {
	events := []CashflowEvent{
		{Type: CashflowEventExpense, CategoryID: "food", Amount: money.Money{Minor: 10_000, Currency: "CNY"}},
		{Type: CashflowEventTransfer, CategoryID: "food", Amount: money.Money{Minor: 99_000, Currency: "CNY"}},
		{Type: CashflowEventExpense, CategoryID: "housing", Amount: money.Money{Minor: 20_000, Currency: "CNY"}},
		{Type: CashflowEventRefund, CategoryID: "food", Amount: money.Money{Minor: -3_000, Currency: "CNY"}},
		{Type: CashflowEventIncome, CategoryID: "salary", Amount: money.Money{Minor: 50_000, Currency: "CNY"}},
		{Type: CashflowEventCreditCardRepayment, Amount: money.Money{Minor: 15_000, Currency: "CNY"}},
		{Type: CashflowEventBalanceAdjustment, Amount: money.Money{Minor: 1_000, Currency: "CNY"}},
	}

	got, err := AggregateSpending(events, "CNY")
	if err != nil {
		t.Fatalf("AggregateSpending: %v", err)
	}
	want := SpendingResult{
		Total:            money.Money{Minor: 27_000, Currency: "CNY"},
		TransactionCount: 3,
		Categories: []SpendingCategoryTotal{
			{CategoryID: "food", Amount: money.Money{Minor: 7_000, Currency: "CNY"}, TransactionCount: 2},
			{CategoryID: "housing", Amount: money.Money{Minor: 20_000, Currency: "CNY"}, TransactionCount: 1},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result=%#v want %#v", got, want)
	}
}

func TestAggregateSpendingPreservesNegativeNetRefundAndRejectsCurrencyMismatch(t *testing.T) {
	got, err := AggregateSpending([]CashflowEvent{
		{Type: CashflowEventRefund, CategoryID: "food", Amount: money.Money{Minor: -5_000, Currency: "CNY"}},
	}, "CNY")
	if err != nil {
		t.Fatalf("AggregateSpending refund: %v", err)
	}
	if got.Total.Minor != -5_000 || len(got.Categories) != 1 || got.Categories[0].Amount.Minor != -5_000 {
		t.Fatalf("refund result=%#v", got)
	}

	if _, err := AggregateSpending([]CashflowEvent{
		{Type: CashflowEventExpense, CategoryID: "food", Amount: money.Money{Minor: 1_000, Currency: "USD"}},
	}, "CNY"); err == nil {
		t.Fatal("AggregateSpending accepted currency mismatch")
	}
}
