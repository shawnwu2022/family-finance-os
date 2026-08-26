package ezbookkeeping

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/ledger"
)

const (
	testToken    = "test-api-token"
	testTimezone = "Asia/Shanghai"
)

func TestClientListAccountsNormalizesAndFlattensHierarchy(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestHeaders(t, r)
		if r.URL.Path != "/api/v1/accounts/list.json" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		serveFixture(t, w, "accounts.json")
	}))
	defer server.Close()

	client := mustClient(t, server.URL+"/api/v1", server.Client())
	accounts, err := client.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 3 {
		t.Fatalf("len(accounts) = %d, want 3", len(accounts))
	}

	bank := findAccount(t, accounts, "101")
	if bank.ParentID != "100" || bank.Category != ledger.AccountCategoryChecking || bank.Structure != ledger.AccountStructureSingle {
		t.Fatalf("bank normalized incorrectly: %#v", bank)
	}
	if bank.Balance.Minor != 123456 || bank.Balance.Currency != "CNY" {
		t.Fatalf("bank balance = %#v", bank.Balance)
	}

	card := findAccount(t, accounts, "200")
	if card.Category != ledger.AccountCategoryCreditCard || !card.IsLiability || card.IsAsset {
		t.Fatalf("credit card normalized incorrectly: %#v", card)
	}
}

func TestClientListTransactionsSendsExclusiveTimeBounds(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestHeaders(t, r)
		if got := r.URL.Query().Get("min_time"); got != "1698768000000" {
			t.Fatalf("min_time = %q, want 1698768000000", got)
		}
		if got := r.URL.Query().Get("max_time"); got != "1701446399999" {
			t.Fatalf("max_time = %q, want 1701446399999", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":{"items":[],"nextTimeSequenceId":0,"totalCount":0}}`))
	}))
	defer server.Close()

	client := mustClient(t, server.URL+"/api/v1", server.Client())
	_, err := client.ListTransactions(context.Background(), ledger.TransactionQuery{
		Start: time.Unix(1698768000, 0).UTC(),
		End:   time.Unix(1701446400, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
}

func TestClientListCategoriesNormalizesAndFlattensHierarchy(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestHeaders(t, r)
		if r.URL.Path != "/api/v1/transaction/categories/list.json" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		serveFixture(t, w, "categories.json")
	}))
	defer server.Close()

	client := mustClient(t, server.URL+"/api/v1", server.Client())
	categories, err := client.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(categories) != 4 {
		t.Fatalf("len(categories) = %d, want 4", len(categories))
	}

	meal := findCategory(t, categories, "401")
	if meal.ParentID != "400" || meal.Type != ledger.CategoryTypeExpense {
		t.Fatalf("meal category normalized incorrectly: %#v", meal)
	}
}

func TestClientListTransactionsPaginatesAndNormalizesAmounts(t *testing.T) {
	t.Parallel()

	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestHeaders(t, r)
		if r.URL.Path != "/api/v1/transactions/list.json" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("count"); got != "50" {
			t.Fatalf("count = %q, want 50", got)
		}
		if got := r.URL.Query().Get("account_ids"); got != "101,200" {
			t.Fatalf("account_ids = %q, want 101,200", got)
		}

		call++
		switch call {
		case 1:
			if got := r.URL.Query().Get("max_time"); got != "0" {
				t.Fatalf("first max_time = %q, want 0", got)
			}
			serveFixture(t, w, "transactions-page-1.json")
		case 2:
			if got := r.URL.Query().Get("max_time"); got != "1699999800000000" {
				t.Fatalf("second max_time = %q", got)
			}
			serveFixture(t, w, "transactions-page-2.json")
		default:
			t.Fatalf("unexpected pagination request %d", call)
		}
	}))
	defer server.Close()

	client := mustClient(t, server.URL+"/api/v1", server.Client())
	transactions, err := client.ListTransactions(context.Background(), ledger.TransactionQuery{
		AccountIDs: []string{"101", "200"},
	})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if call != 2 {
		t.Fatalf("requests = %d, want 2", call)
	}
	if len(transactions) != 3 {
		t.Fatalf("len(transactions) = %d, want 3", len(transactions))
	}

	expense := findTransaction(t, transactions, "tx-expense")
	if expense.Type != ledger.TransactionTypeExpense {
		t.Fatalf("expense type = %v", expense.Type)
	}
	if expense.SourceAmount.Minor != 12800 || expense.SourceAmount.Currency != "CNY" {
		t.Fatalf("expense amount = %#v", expense.SourceAmount)
	}
	if expense.DestinationAmount != nil {
		t.Fatalf("expense destination amount = %#v, want nil", expense.DestinationAmount)
	}

	transfer := findTransaction(t, transactions, "tx-transfer")
	if transfer.Type != ledger.TransactionTypeTransfer || transfer.DestinationAmount == nil {
		t.Fatalf("transfer normalized incorrectly: %#v", transfer)
	}
	if transfer.DestinationAmount.Minor != 50000 || transfer.DestinationAmount.Currency != "CNY" {
		t.Fatalf("transfer destination amount = %#v", transfer.DestinationAmount)
	}

	income := findTransaction(t, transactions, "tx-income")
	if income.Type != ledger.TransactionTypeIncome || income.SourceAmount.Minor != 3000000 {
		t.Fatalf("income normalized incorrectly: %#v", income)
	}
}

func TestClientRejectsUnknownTransactionType(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":{"items":[{"id":"bad","timeSequenceId":"1","type":99,"categoryId":"401","time":1700000000,"utcOffset":480,"sourceAccountId":"101","sourceAccount":{"id":"101","name":"bank","parentId":"0","category":2,"type":1,"currency":"CNY","balance":0,"isAsset":true,"isLiability":false,"hidden":false,"subAccounts":[]},"destinationAccountId":"0","sourceAmount":100,"destinationAmount":0,"hideAmount":false,"tagIds":[],"tags":[],"pictures":[],"comment":"","editable":true}],"nextTimeSequenceId":0,"totalCount":1}}`))
	}))
	defer server.Close()

	client := mustClient(t, server.URL+"/api/v1", server.Client())
	_, err := client.ListTransactions(context.Background(), ledger.TransactionQuery{})
	if !errors.Is(err, ledger.ErrUnknownEnum) {
		t.Fatalf("error = %v, want ErrUnknownEnum", err)
	}
}

func TestClientReturnsAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errorCode":123456,"errorMessage":"denied","path":"/api/v1/accounts/list.json","success":false}`))
	}))
	defer server.Close()

	client := mustClient(t, server.URL+"/api/v1", server.Client())
	_, err := client.ListAccounts(context.Background())
	if err == nil {
		t.Fatal("ListAccounts returned nil error for API failure")
	}
}

func mustClient(t *testing.T, baseURL string, httpClient *http.Client) *Client {
	t.Helper()
	client, err := NewClient(baseURL, testToken, testTimezone, httpClient)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func assertRequestHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
		t.Fatalf("Authorization = %q", got)
	}
	if got := r.Header.Get("X-Timezone-Name"); got != testTimezone {
		t.Fatalf("X-Timezone-Name = %q", got)
	}
}

func serveFixture(t *testing.T, w http.ResponseWriter, name string) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "tests", "fixtures", "ezbookkeeping", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func findAccount(t *testing.T, items []ledger.Account, id string) ledger.Account {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("account %s not found", id)
	return ledger.Account{}
}

func findCategory(t *testing.T, items []ledger.Category, id string) ledger.Category {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("category %s not found", id)
	return ledger.Category{}
}

func findTransaction(t *testing.T, items []ledger.Transaction, id string) ledger.Transaction {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("transaction %s not found", id)
	return ledger.Transaction{}
}

func TestClientRejectsUnknownAccountCategory(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"bad","name":"future","parentId":"0","category":99,"type":1,"currency":"CNY","balance":0,"isAsset":true,"isLiability":false,"hidden":false,"subAccounts":[]}]}`))
	}))
	defer server.Close()

	client := mustClient(t, server.URL+"/api/v1", server.Client())
	_, err := client.ListAccounts(context.Background())
	if !errors.Is(err, ledger.ErrUnknownEnum) {
		t.Fatalf("error = %v, want ErrUnknownEnum", err)
	}
}

func TestClientRejectsCategoryGroupTypeMismatch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":{"2":[{"id":"bad","name":"mismatch","parentId":"0","type":1,"hidden":false,"subCategories":[]}]}}`))
	}))
	defer server.Close()

	client := mustClient(t, server.URL+"/api/v1", server.Client())
	_, err := client.ListCategories(context.Background())
	if err == nil {
		t.Fatal("ListCategories returned nil for response group/type mismatch")
	}
}

func TestClientRejectsRepeatedPaginationCursor(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequestHeaders(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":{"items":[{"id":"tx","timeSequenceId":"1","type":3,"categoryId":"401","time":1700000000,"utcOffset":480,"sourceAccountId":"101","sourceAccount":{"id":"101","name":"bank","parentId":"0","category":2,"type":1,"currency":"CNY","balance":0,"isAsset":true,"isLiability":false,"hidden":false,"subAccounts":[]},"destinationAccountId":"0","sourceAmount":100,"destinationAmount":0,"hideAmount":false,"comment":"","editable":true}],"nextTimeSequenceId":7,"totalCount":2}}`))
	}))
	defer server.Close()

	client := mustClient(t, server.URL+"/api/v1", server.Client())
	_, err := client.ListTransactions(context.Background(), ledger.TransactionQuery{})
	if err == nil {
		t.Fatal("ListTransactions returned nil for repeated pagination cursor")
	}
}
