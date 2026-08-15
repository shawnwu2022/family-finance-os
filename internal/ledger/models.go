package ledger

import (
	"errors"
	"time"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var ErrUnknownEnum = errors.New("unknown ledger enum")

type AccountCategory uint8

const (
	AccountCategoryUnknown AccountCategory = iota
	AccountCategoryCash
	AccountCategoryChecking
	AccountCategoryCreditCard
	AccountCategoryVirtual
	AccountCategoryDebt
	AccountCategoryReceivables
	AccountCategoryInvestment
	AccountCategorySavings
	AccountCategoryCertificateOfDeposit
)

type AccountStructure uint8

const (
	AccountStructureUnknown AccountStructure = iota
	AccountStructureSingle
	AccountStructureMultipleSubAccounts
)

type CategoryType uint8

const (
	CategoryTypeUnknown CategoryType = iota
	CategoryTypeIncome
	CategoryTypeExpense
	CategoryTypeTransfer
)

type TransactionType uint8

const (
	TransactionTypeUnknown TransactionType = iota
	TransactionTypeBalanceModification
	TransactionTypeIncome
	TransactionTypeExpense
	TransactionTypeTransfer
)

type Account struct {
	ID          string
	Name        string
	ParentID    string
	Category    AccountCategory
	Structure   AccountStructure
	Balance     money.Money
	IsAsset     bool
	IsLiability bool
	Hidden      bool
}

type Category struct {
	ID       string
	Name     string
	ParentID string
	Type     CategoryType
	Hidden   bool
}

type Transaction struct {
	ID                   string
	TimeSequenceID       string
	Type                 TransactionType
	CategoryID           string
	OccurredAt           time.Time
	UTCOffsetMinutes     int
	SourceAccountID      string
	DestinationAccountID string
	SourceAmount         money.Money
	DestinationAmount    *money.Money
	AmountHidden         bool
	Comment              string
	Editable             bool
}

type TransactionQuery struct {
	Type        TransactionType
	CategoryIDs []string
	AccountIDs  []string
}
