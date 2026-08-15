package analytics

import (
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestNormalizeTransaction(t *testing.T) {
	t.Parallel()

	accounts := map[string]ledger.Account{
		"bank": {
			ID:       "bank",
			Category: ledger.AccountCategoryChecking,
			Balance:  money.Money{Currency: "CNY"},
			IsAsset:  true,
		},
		"savings": {
			ID:       "savings",
			Category: ledger.AccountCategorySavings,
			Balance:  money.Money{Currency: "CNY"},
			IsAsset:  true,
		},
		"card": {
			ID:          "card",
			Category:    ledger.AccountCategoryCreditCard,
			Balance:     money.Money{Currency: "CNY"},
			IsLiability: true,
		},
	}
	when := time.Date(2026, 8, 16, 1, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		tx   ledger.Transaction
		want CashflowEventType
	}{
		{
			name: "expense",
			tx: ledger.Transaction{
				ID: "expense", Type: ledger.TransactionTypeExpense,
				SourceAccountID: "card", SourceAmount: money.Money{Minor: 12_800, Currency: "CNY"}, OccurredAt: when,
			},
			want: CashflowEventExpense,
		},
		{
			name: "income",
			tx: ledger.Transaction{
				ID: "income", Type: ledger.TransactionTypeIncome,
				SourceAccountID: "bank", SourceAmount: money.Money{Minor: 3_000_000, Currency: "CNY"}, OccurredAt: when,
			},
			want: CashflowEventIncome,
		},
		{
			name: "ordinary transfer",
			tx: ledger.Transaction{
				ID: "transfer", Type: ledger.TransactionTypeTransfer,
				SourceAccountID: "bank", DestinationAccountID: "savings",
				SourceAmount: money.Money{Minor: 50_000, Currency: "CNY"}, OccurredAt: when,
			},
			want: CashflowEventTransfer,
		},
		{
			name: "balance adjustment",
			tx: ledger.Transaction{
				ID: "adjust", Type: ledger.TransactionTypeBalanceModification,
				SourceAccountID: "bank", SourceAmount: money.Money{Minor: 20_000, Currency: "CNY"}, OccurredAt: when,
			},
			want: CashflowEventBalanceAdjustment,
		},
		{
			name: "credit card repayment",
			tx: ledger.Transaction{
				ID: "repayment", Type: ledger.TransactionTypeTransfer,
				SourceAccountID: "bank", DestinationAccountID: "card",
				SourceAmount: money.Money{Minor: 50_000, Currency: "CNY"}, OccurredAt: when,
			},
			want: CashflowEventCreditCardRepayment,
		},
		{
			name: "refund is negative expense",
			tx: ledger.Transaction{
				ID: "refund", Type: ledger.TransactionTypeExpense,
				SourceAccountID: "card", SourceAmount: money.Money{Minor: -8_800, Currency: "CNY"}, OccurredAt: when,
			},
			want: CashflowEventRefund,
		},
		{
			name: "unknown remains unknown",
			tx: ledger.Transaction{
				ID: "unknown", Type: ledger.TransactionTypeUnknown,
				SourceAccountID: "bank", SourceAmount: money.Money{Minor: 700, Currency: "CNY"}, OccurredAt: when,
			},
			want: CashflowEventUnknown,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeTransaction(tt.tx, accounts)
			if got.Type != tt.want {
				t.Fatalf("Type = %v, want %v", got.Type, tt.want)
			}
			if got.TransactionID != tt.tx.ID || got.Amount != tt.tx.SourceAmount || !got.OccurredAt.Equal(tt.tx.OccurredAt) {
				t.Fatalf("event does not preserve source semantics: %#v", got)
			}
		})
	}
}

func TestCreditCardRepaymentRequiresLiabilityCreditCardDestination(t *testing.T) {
	t.Parallel()

	tx := ledger.Transaction{
		ID: "transfer", Type: ledger.TransactionTypeTransfer,
		SourceAccountID: "bank", DestinationAccountID: "not-liability-card",
		SourceAmount: money.Money{Minor: 10_000, Currency: "CNY"},
	}
	accounts := map[string]ledger.Account{
		"not-liability-card": {
			ID: "not-liability-card", Category: ledger.AccountCategoryCreditCard, IsLiability: false,
		},
	}

	if got := NormalizeTransaction(tx, accounts); got.Type != CashflowEventTransfer {
		t.Fatalf("Type = %v, want transfer", got.Type)
	}
}
