package dataquality

import (
	"sort"
	"strings"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type Quality string

const (
	QualityClean  Quality = "clean"
	QualityReview Quality = "review"
)

type IssueKind string

const (
	IssueMissingSourceAccount       IssueKind = "missing_source_account"
	IssueMissingDestinationAccount  IssueKind = "missing_destination_account"
	IssueMissingCategory            IssueKind = "missing_category"
	IssueCategoryTypeMismatch       IssueKind = "category_type_mismatch"
	IssueTransferMissingDestination IssueKind = "transfer_missing_destination"
	IssueHiddenAmount               IssueKind = "hidden_amount"
	IssueMissingCurrency            IssueKind = "missing_currency"
)

type Issue struct {
	Kind          IssueKind
	TransactionID string
	Reference     string
}

type DuplicateGroup struct {
	TransactionIDs  []string
	Type            ledger.TransactionType
	Amount          money.Money
	FirstOccurredAt time.Time
	LastOccurredAt  time.Time
}

type Report struct {
	Quality             Quality
	CheckedTransactions int
	Issues              []Issue
	DuplicateGroups     []DuplicateGroup
}

type Options struct {
	DuplicateWindow time.Duration
}

const defaultDuplicateWindow = 5 * time.Minute

func Analyze(accounts []ledger.Account, categories []ledger.Category, transactions []ledger.Transaction, options Options) Report {
	window := options.DuplicateWindow
	if window <= 0 {
		window = defaultDuplicateWindow
	}

	accountIDs := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		if strings.TrimSpace(account.ID) != "" {
			accountIDs[account.ID] = struct{}{}
		}
	}
	categoryByID := make(map[string]ledger.Category, len(categories))
	for _, category := range categories {
		if strings.TrimSpace(category.ID) != "" {
			categoryByID[category.ID] = category
		}
	}

	report := Report{Quality: QualityClean, CheckedTransactions: len(transactions)}
	for _, tx := range transactions {
		report.Issues = append(report.Issues, transactionIssues(tx, accountIDs, categoryByID)...)
	}
	report.DuplicateGroups = duplicateGroups(transactions, window)

	sort.Slice(report.Issues, func(i, j int) bool {
		if report.Issues[i].TransactionID != report.Issues[j].TransactionID {
			return report.Issues[i].TransactionID < report.Issues[j].TransactionID
		}
		if report.Issues[i].Kind != report.Issues[j].Kind {
			return report.Issues[i].Kind < report.Issues[j].Kind
		}
		return report.Issues[i].Reference < report.Issues[j].Reference
	})
	if len(report.Issues) != 0 || len(report.DuplicateGroups) != 0 {
		report.Quality = QualityReview
	}
	return report
}

func transactionIssues(tx ledger.Transaction, accountIDs map[string]struct{}, categoryByID map[string]ledger.Category) []Issue {
	issues := []Issue{}
	if tx.AmountHidden {
		issues = append(issues, Issue{Kind: IssueHiddenAmount, TransactionID: tx.ID})
	}
	if strings.TrimSpace(tx.SourceAmount.Currency) == "" {
		issues = append(issues, Issue{Kind: IssueMissingCurrency, TransactionID: tx.ID})
	}
	if tx.SourceAccountID == "" {
		issues = append(issues, Issue{Kind: IssueMissingSourceAccount, TransactionID: tx.ID})
	} else if _, ok := accountIDs[tx.SourceAccountID]; !ok {
		issues = append(issues, Issue{Kind: IssueMissingSourceAccount, TransactionID: tx.ID, Reference: tx.SourceAccountID})
	}

	switch tx.Type {
	case ledger.TransactionTypeIncome, ledger.TransactionTypeExpense:
		category, ok := categoryByID[tx.CategoryID]
		if tx.CategoryID == "" || !ok {
			issues = append(issues, Issue{Kind: IssueMissingCategory, TransactionID: tx.ID, Reference: tx.CategoryID})
		} else if !categoryMatchesTransaction(category.Type, tx.Type) {
			issues = append(issues, Issue{Kind: IssueCategoryTypeMismatch, TransactionID: tx.ID, Reference: tx.CategoryID})
		}
	case ledger.TransactionTypeTransfer:
		if tx.DestinationAccountID == "" {
			issues = append(issues, Issue{Kind: IssueTransferMissingDestination, TransactionID: tx.ID})
		} else if _, ok := accountIDs[tx.DestinationAccountID]; !ok {
			issues = append(issues, Issue{Kind: IssueMissingDestinationAccount, TransactionID: tx.ID, Reference: tx.DestinationAccountID})
		}
	}
	return issues
}

func categoryMatchesTransaction(categoryType ledger.CategoryType, transactionType ledger.TransactionType) bool {
	switch transactionType {
	case ledger.TransactionTypeIncome:
		return categoryType == ledger.CategoryTypeIncome
	case ledger.TransactionTypeExpense:
		return categoryType == ledger.CategoryTypeExpense
	default:
		return true
	}
}

type duplicateSignature struct {
	typeID               ledger.TransactionType
	categoryID           string
	sourceID             string
	destinationID        string
	sourceMinor          int64
	sourceCurrency       string
	destinationMinor     int64
	destinationCurrency  string
	hasDestinationAmount bool
	comment              string
}

func duplicateGroups(transactions []ledger.Transaction, window time.Duration) []DuplicateGroup {
	bySignature := map[duplicateSignature][]ledger.Transaction{}
	for _, tx := range transactions {
		if tx.Type != ledger.TransactionTypeIncome && tx.Type != ledger.TransactionTypeExpense && tx.Type != ledger.TransactionTypeTransfer {
			continue
		}
		sig := signature(tx)
		bySignature[sig] = append(bySignature[sig], tx)
	}

	groups := []DuplicateGroup{}
	for _, txs := range bySignature {
		if len(txs) < 2 {
			continue
		}
		sort.Slice(txs, func(i, j int) bool {
			if !txs[i].OccurredAt.Equal(txs[j].OccurredAt) {
				return txs[i].OccurredAt.Before(txs[j].OccurredAt)
			}
			return txs[i].ID < txs[j].ID
		})
		start := 0
		for start < len(txs) {
			end := start + 1
			for end < len(txs) && txs[end].OccurredAt.Sub(txs[start].OccurredAt) <= window {
				end++
			}
			if end-start >= 2 {
				ids := make([]string, 0, end-start)
				for _, tx := range txs[start:end] {
					ids = append(ids, tx.ID)
				}
				sort.Strings(ids)
				groups = append(groups, DuplicateGroup{
					TransactionIDs:  ids,
					Type:            txs[start].Type,
					Amount:          txs[start].SourceAmount,
					FirstOccurredAt: txs[start].OccurredAt,
					LastOccurredAt:  txs[end-1].OccurredAt,
				})
			}
			start = end
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		if !groups[i].FirstOccurredAt.Equal(groups[j].FirstOccurredAt) {
			return groups[i].FirstOccurredAt.Before(groups[j].FirstOccurredAt)
		}
		left, right := "", ""
		if len(groups[i].TransactionIDs) > 0 {
			left = groups[i].TransactionIDs[0]
		}
		if len(groups[j].TransactionIDs) > 0 {
			right = groups[j].TransactionIDs[0]
		}
		return left < right
	})
	return groups
}

func signature(tx ledger.Transaction) duplicateSignature {
	sig := duplicateSignature{
		typeID:         tx.Type,
		categoryID:     tx.CategoryID,
		sourceID:       tx.SourceAccountID,
		destinationID:  tx.DestinationAccountID,
		sourceMinor:    tx.SourceAmount.Minor,
		sourceCurrency: tx.SourceAmount.Currency,
		comment:        strings.ToLower(strings.TrimSpace(tx.Comment)),
	}
	if tx.DestinationAmount != nil {
		sig.hasDestinationAmount = true
		sig.destinationMinor = tx.DestinationAmount.Minor
		sig.destinationCurrency = tx.DestinationAmount.Currency
	}
	return sig
}
