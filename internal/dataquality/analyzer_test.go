package dataquality

import (
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAnalyzeDetectsDeterministicDuplicateCandidates(t *testing.T) {
	t0 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	accounts := []ledger.Account{{ID: "cash"}}
	categories := []ledger.Category{{ID: "food", Type: ledger.CategoryTypeExpense}}
	txs := []ledger.Transaction{
		{ID: "tx-b", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: t0.Add(90 * time.Second), SourceAccountID: "cash", SourceAmount: money.Money{Minor: 1280, Currency: "CNY"}, Comment: "lunch"},
		{ID: "tx-a", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: t0, SourceAccountID: "cash", SourceAmount: money.Money{Minor: 1280, Currency: "CNY"}, Comment: "lunch"},
		{ID: "tx-c", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: t0.Add(20 * time.Minute), SourceAccountID: "cash", SourceAmount: money.Money{Minor: 1280, Currency: "CNY"}, Comment: "lunch"},
	}

	report := Analyze(accounts, categories, txs, Options{DuplicateWindow: 5 * time.Minute})
	if report.Quality != QualityReview {
		t.Fatalf("quality=%q want %q", report.Quality, QualityReview)
	}
	if report.CheckedTransactions != 3 {
		t.Fatalf("checked=%d want 3", report.CheckedTransactions)
	}
	if len(report.DuplicateGroups) != 1 {
		t.Fatalf("duplicate groups=%d want 1", len(report.DuplicateGroups))
	}
	group := report.DuplicateGroups[0]
	if len(group.TransactionIDs) != 2 || group.TransactionIDs[0] != "tx-a" || group.TransactionIDs[1] != "tx-b" {
		t.Fatalf("unexpected duplicate IDs: %#v", group.TransactionIDs)
	}
	if group.Amount != (money.Money{Minor: 1280, Currency: "CNY"}) {
		t.Fatalf("unexpected duplicate amount: %#v", group.Amount)
	}
}

func TestAnalyzeDuplicateWindowIsMeasuredFromGroupStart(t *testing.T) {
	t0 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	accounts := []ledger.Account{{ID: "cash"}}
	categories := []ledger.Category{{ID: "food", Type: ledger.CategoryTypeExpense}}
	txs := []ledger.Transaction{
		{ID: "tx-a", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: t0, SourceAccountID: "cash", SourceAmount: money.Money{Minor: 1280, Currency: "CNY"}, Comment: "lunch"},
		{ID: "tx-b", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: t0.Add(4 * time.Minute), SourceAccountID: "cash", SourceAmount: money.Money{Minor: 1280, Currency: "CNY"}, Comment: "lunch"},
		{ID: "tx-c", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: t0.Add(8 * time.Minute), SourceAccountID: "cash", SourceAmount: money.Money{Minor: 1280, Currency: "CNY"}, Comment: "lunch"},
	}

	report := Analyze(accounts, categories, txs, Options{DuplicateWindow: 5 * time.Minute})
	if len(report.DuplicateGroups) != 1 {
		t.Fatalf("duplicate groups=%d want 1", len(report.DuplicateGroups))
	}
	group := report.DuplicateGroups[0]
	if len(group.TransactionIDs) != 2 || group.TransactionIDs[0] != "tx-a" || group.TransactionIDs[1] != "tx-b" {
		t.Fatalf("transitive clustering over-grouped transactions: %#v", group.TransactionIDs)
	}
}

func TestAnalyzeFlagsReferenceAndShapeProblems(t *testing.T) {
	t0 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	accounts := []ledger.Account{{ID: "cash"}}
	categories := []ledger.Category{{ID: "salary", Type: ledger.CategoryTypeIncome}}
	txs := []ledger.Transaction{
		{ID: "missing-account", Type: ledger.TransactionTypeExpense, CategoryID: "missing-category", OccurredAt: t0, SourceAccountID: "ghost", SourceAmount: money.Money{Minor: 500, Currency: "CNY"}},
		{ID: "bad-transfer", Type: ledger.TransactionTypeTransfer, OccurredAt: t0.Add(time.Minute), SourceAccountID: "cash", SourceAmount: money.Money{Minor: 100, Currency: "CNY"}},
		{ID: "hidden", Type: ledger.TransactionTypeIncome, CategoryID: "salary", OccurredAt: t0.Add(2 * time.Minute), SourceAccountID: "cash", SourceAmount: money.Money{Minor: 1000, Currency: "CNY"}, AmountHidden: true},
	}

	report := Analyze(accounts, categories, txs, Options{})
	got := map[IssueKind]int{}
	for _, issue := range report.Issues {
		got[issue.Kind]++
	}
	for _, kind := range []IssueKind{IssueMissingSourceAccount, IssueMissingCategory, IssueTransferMissingDestination, IssueHiddenAmount} {
		if got[kind] != 1 {
			t.Fatalf("issue %q count=%d want 1; all=%#v", kind, got[kind], report.Issues)
		}
	}
	if report.Quality != QualityReview {
		t.Fatalf("quality=%q want %q", report.Quality, QualityReview)
	}
}

func TestAnalyzeReturnsCleanForConsistentUniqueTransactions(t *testing.T) {
	t0 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	accounts := []ledger.Account{{ID: "cash"}, {ID: "bank"}}
	categories := []ledger.Category{{ID: "food", Type: ledger.CategoryTypeExpense}}
	txs := []ledger.Transaction{
		{ID: "expense", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: t0, SourceAccountID: "cash", SourceAmount: money.Money{Minor: 1280, Currency: "CNY"}},
		{ID: "transfer", Type: ledger.TransactionTypeTransfer, OccurredAt: t0.Add(time.Hour), SourceAccountID: "cash", DestinationAccountID: "bank", SourceAmount: money.Money{Minor: 5000, Currency: "CNY"}, DestinationAmount: &money.Money{Minor: 5000, Currency: "CNY"}},
	}

	report := Analyze(accounts, categories, txs, Options{})
	if report.Quality != QualityClean {
		t.Fatalf("quality=%q want %q", report.Quality, QualityClean)
	}
	if len(report.Issues) != 0 || len(report.DuplicateGroups) != 0 {
		t.Fatalf("unexpected findings: issues=%#v duplicates=%#v", report.Issues, report.DuplicateGroups)
	}
}
