package appapi

import (
	"context"
	"fmt"

	"github.com/shawnwu2022/family-finance-os/internal/dataquality"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func (a *API) DataQuality(ctx context.Context, householdID int64, period string) (server.DataQualityResponse, error) {
	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return server.DataQualityResponse{}, fmt.Errorf("load household profile: %w", err)
	}
	start, end, err := periodBounds(period, profile.Household.Timezone)
	if err != nil {
		return server.DataQualityResponse{}, err
	}
	accounts, err := a.ledger.ListAccounts(ctx)
	if err != nil {
		return server.DataQualityResponse{}, fmt.Errorf("list ledger accounts: %w", err)
	}
	categories, err := a.ledger.ListCategories(ctx)
	if err != nil {
		return server.DataQualityResponse{}, fmt.Errorf("list ledger categories: %w", err)
	}
	transactions, err := a.ledger.ListTransactions(ctx, ledger.TransactionQuery{Start: start, End: end})
	if err != nil {
		return server.DataQualityResponse{}, fmt.Errorf("list ledger transactions: %w", err)
	}

	periodTransactions := make([]ledger.Transaction, 0, len(transactions))
	for _, tx := range transactions {
		if tx.OccurredAt.Before(start) || !tx.OccurredAt.Before(end) {
			continue
		}
		periodTransactions = append(periodTransactions, tx)
	}
	report := dataquality.Analyze(accounts, categories, periodTransactions, dataquality.Options{})
	response := server.DataQualityResponse{
		Period:              period,
		Quality:             string(report.Quality),
		CheckedTransactions: report.CheckedTransactions,
		IssueCount:          len(report.Issues),
		DuplicateGroupCount: len(report.DuplicateGroups),
		Issues:              make([]server.DataQualityIssueResponse, 0, len(report.Issues)),
		DuplicateCandidates: make([]server.DuplicateCandidateResponse, 0, len(report.DuplicateGroups)),
	}
	for _, issue := range report.Issues {
		response.Issues = append(response.Issues, server.DataQualityIssueResponse{
			Kind:          string(issue.Kind),
			TransactionID: issue.TransactionID,
			Reference:     issue.Reference,
		})
	}
	for _, group := range report.DuplicateGroups {
		response.DuplicateCandidates = append(response.DuplicateCandidates, server.DuplicateCandidateResponse{
			TransactionIDs:  append([]string(nil), group.TransactionIDs...),
			Type:            transactionTypeString(group.Type),
			Amount:          moneyDTO(group.Amount),
			FirstOccurredAt: group.FirstOccurredAt,
			LastOccurredAt:  group.LastOccurredAt,
		})
	}
	return response, nil
}

func transactionTypeString(value ledger.TransactionType) string {
	switch value {
	case ledger.TransactionTypeBalanceModification:
		return "balance_modification"
	case ledger.TransactionTypeIncome:
		return "income"
	case ledger.TransactionTypeExpense:
		return "expense"
	case ledger.TransactionTypeTransfer:
		return "transfer"
	default:
		return "unknown"
	}
}
