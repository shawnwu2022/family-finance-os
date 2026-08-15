package ledger

import "context"

type Ledger interface {
	ListAccounts(ctx context.Context) ([]Account, error)
	ListCategories(ctx context.Context) ([]Category, error)
	ListTransactions(ctx context.Context, q TransactionQuery) ([]Transaction, error)
}
