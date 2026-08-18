package appapi

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestSpendingAnalysisAggregatesCurrentAndPriorMonthsFromOneLedgerRead(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	book := &spendingLedger{
		accounts: []ledger.Account{
			{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, IsAsset: true},
			{ID: "card", Category: ledger.AccountCategoryCreditCard, Structure: ledger.AccountStructureSingle, IsLiability: true},
		},
		categories: []ledger.Category{
			{ID: "food", Name: "餐饮", Type: ledger.CategoryTypeExpense},
			{ID: "housing", Name: "住房", Type: ledger.CategoryTypeExpense},
		},
		transactions: []ledger.Transaction{
			// 2026-08-01 00:30 in Asia/Shanghai: proves household-local month boundaries.
			{ID: "aug-food", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 7, 31, 16, 30, 0, 0, time.UTC), SourceAmount: money.Money{Minor: 10_000, Currency: "CNY"}},
			{ID: "aug-refund", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC), SourceAmount: money.Money{Minor: -2_000, Currency: "CNY"}},
			{ID: "aug-housing", Type: ledger.TransactionTypeExpense, CategoryID: "housing", OccurredAt: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC), SourceAmount: money.Money{Minor: 20_000, Currency: "CNY"}},
			{ID: "aug-transfer", Type: ledger.TransactionTypeTransfer, OccurredAt: time.Date(2026, 8, 11, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", DestinationAccountID: "card", SourceAmount: money.Money{Minor: 15_000, Currency: "CNY"}},
			{ID: "aug-usd", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 8, 12, 1, 0, 0, 0, time.UTC), SourceAmount: money.Money{Minor: 3_000, Currency: "USD"}},
			{ID: "aug-unknown", Type: ledger.TransactionTypeUnknown, CategoryID: "food", OccurredAt: time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC), SourceAmount: money.Money{Minor: 1_000, Currency: "CNY"}},
			{ID: "jul-food", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC), SourceAmount: money.Money{Minor: 5_000, Currency: "CNY"}},
			{ID: "jul-legacy", Type: ledger.TransactionTypeExpense, CategoryID: "legacy", OccurredAt: time.Date(2026, 7, 11, 1, 0, 0, 0, time.UTC), SourceAmount: money.Money{Minor: 2_500, Currency: "CNY"}},
			{ID: "jun-food", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 6, 9, 1, 0, 0, 0, time.UTC), SourceAmount: money.Money{Minor: 4_000, Currency: "CNY"}},
			{ID: "may-ignored", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 5, 9, 1, 0, 0, 0, time.UTC), SourceAmount: money.Money{Minor: 9_999, Currency: "CNY"}},
		},
	}
	planner := &spendingPlanner{profile: household.Profile{
		Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
	}}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := api.SpendingAnalysis(context.Background(), 42, "2026-08", 2)
	if err != nil {
		t.Fatalf("SpendingAnalysis: %v", err)
	}
	if !got.DataAsOf.Equal(now) || got.Quality != "partial" || got.Currency != "CNY" {
		t.Fatalf("metadata=%#v", got)
	}
	if got.Current.Period != "2026-08" || got.Current.Total.Minor != 28_000 || got.Current.TransactionCount != 3 {
		t.Fatalf("current=%#v", got.Current)
	}
	if want := []string{"food", "housing"}; !reflect.DeepEqual(spendingCategoryRefs(got.Current.Categories), want) {
		t.Fatalf("current categories=%#v want %#v", spendingCategoryRefs(got.Current.Categories), want)
	}
	if got.Current.Categories[0].Name != "餐饮" || got.Current.Categories[0].Amount.Minor != 8_000 || got.Current.Categories[0].TransactionCount != 2 {
		t.Fatalf("food=%#v", got.Current.Categories[0])
	}
	if got.Current.Categories[1].Name != "住房" || got.Current.Categories[1].Amount.Minor != 20_000 || got.Current.Categories[1].TransactionCount != 1 {
		t.Fatalf("housing=%#v", got.Current.Categories[1])
	}

	if len(got.Comparisons) != 2 || got.Comparisons[0].Period != "2026-07" || got.Comparisons[1].Period != "2026-06" {
		t.Fatalf("comparisons=%#v", got.Comparisons)
	}
	if got.Comparisons[0].Total.Minor != 7_500 || got.Comparisons[0].TransactionCount != 2 {
		t.Fatalf("july=%#v", got.Comparisons[0])
	}
	if want := []string{"food", "legacy"}; !reflect.DeepEqual(spendingCategoryRefs(got.Comparisons[0].Categories), want) {
		t.Fatalf("july categories=%#v want %#v", spendingCategoryRefs(got.Comparisons[0].Categories), want)
	}
	if got.Comparisons[0].Categories[1].Name != "" {
		t.Fatalf("missing category metadata was invented: %#v", got.Comparisons[0].Categories[1])
	}
	if got.Comparisons[1].Total.Minor != 4_000 || got.Comparisons[1].TransactionCount != 1 {
		t.Fatalf("june=%#v", got.Comparisons[1])
	}

	for _, part := range []string{"currency USD differs", "unknown cashflow semantics", "category legacy metadata unavailable"} {
		if !containsWarning(got.Warnings, part) {
			t.Fatalf("warnings=%#v missing %q", got.Warnings, part)
		}
	}
	if book.accountsCalls != 1 || book.categoriesCalls != 1 || book.transactionsCalls != 1 {
		t.Fatalf("ledger calls accounts/categories/transactions=%d/%d/%d want 1/1/1", book.accountsCalls, book.categoriesCalls, book.transactionsCalls)
	}
	if planner.profileCalls != 1 || planner.otherCalls != 0 {
		t.Fatalf("planner calls profile/other=%d/%d want 1/0", planner.profileCalls, planner.otherCalls)
	}
}

func TestSpendingAnalysisRejectsInvalidPeriodAndCompareCountBeforeLedgerRead(t *testing.T) {
	book := &spendingLedger{}
	planner := &spendingPlanner{profile: household.Profile{Household: household.Household{ID: 42, BaseCurrency: "CNY", Timezone: "Asia/Shanghai"}}}
	api, err := New(Dependencies{Ledger: book, Planner: planner})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, tc := range []struct {
		period  string
		compare int
	}{
		{"2026-13", 0},
		{"2026-08", -1},
		{"2026-08", 13},
	} {
		if _, err := api.SpendingAnalysis(context.Background(), 42, tc.period, tc.compare); err == nil {
			t.Fatalf("SpendingAnalysis accepted period=%q compare=%d", tc.period, tc.compare)
		}
	}
	if book.accountsCalls != 0 || book.categoriesCalls != 0 || book.transactionsCalls != 0 {
		t.Fatalf("ledger was called for rejected input: %d/%d/%d", book.accountsCalls, book.categoriesCalls, book.transactionsCalls)
	}
}

func spendingCategoryRefs(values []struct {
	CategoryRef      string
	Name             string
	Amount           struct {
		Minor    int64
		Currency string
	}
	TransactionCount int
}) []string {
	refs := make([]string, len(values))
	for i, value := range values {
		refs[i] = value.CategoryRef
	}
	return refs
}

type spendingLedger struct {
	accounts     []ledger.Account
	categories   []ledger.Category
	transactions []ledger.Transaction

	accountsCalls     int
	categoriesCalls   int
	transactionsCalls int
}

func (l *spendingLedger) ListAccounts(context.Context) ([]ledger.Account, error) {
	l.accountsCalls++
	return append([]ledger.Account(nil), l.accounts...), nil
}
func (l *spendingLedger) ListCategories(context.Context) ([]ledger.Category, error) {
	l.categoriesCalls++
	return append([]ledger.Category(nil), l.categories...), nil
}
func (l *spendingLedger) ListTransactions(context.Context, ledger.TransactionQuery) ([]ledger.Transaction, error) {
	l.transactionsCalls++
	return append([]ledger.Transaction(nil), l.transactions...), nil
}

type spendingPlanner struct {
	profile      household.Profile
	profileCalls int
	otherCalls   int
}

func (p *spendingPlanner) Profile(context.Context, int64) (household.Profile, error) {
	p.profileCalls++
	return p.profile, nil
}
func (p *spendingPlanner) BudgetPlan(context.Context, int64, string) (budget.BudgetPlan, error) {
	p.otherCalls++
	return budget.BudgetPlan{}, errors.New("BudgetPlan must not be called by SpendingAnalysis")
}
func (p *spendingPlanner) Debts(context.Context, int64) ([]DebtSnapshot, error) {
	p.otherCalls++
	return nil, errors.New("Debts must not be called by SpendingAnalysis")
}
func (p *spendingPlanner) Goals(context.Context, int64) ([]goals.FinancialGoal, error) {
	p.otherCalls++
	return nil, errors.New("Goals must not be called by SpendingAnalysis")
}

func containsWarning(values []string, part string) bool {
	for _, value := range values {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}
