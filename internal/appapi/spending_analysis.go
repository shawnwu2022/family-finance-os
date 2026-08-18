package appapi

import (
	"context"
	"fmt"

	"github.com/shawnwu2022/family-finance-os/internal/analytics"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func (a *API) SpendingAnalysis(ctx context.Context, householdID int64, period string, comparePeriods int) (server.SpendingAnalysisResponse, error) {
	if comparePeriods < 0 || comparePeriods > 12 {
		return server.SpendingAnalysisResponse{}, fmt.Errorf("compare_periods must be between 0 and 12")
	}
	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return server.SpendingAnalysisResponse{}, fmt.Errorf("load household profile: %w", err)
	}
	start, end, err := periodBounds(period, profile.Household.Timezone)
	if err != nil {
		return server.SpendingAnalysisResponse{}, err
	}
	currency := profile.Household.BaseCurrency
	earliest := start.AddDate(0, -comparePeriods, 0)

	accounts, err := a.ledger.ListAccounts(ctx)
	if err != nil {
		return server.SpendingAnalysisResponse{}, fmt.Errorf("list ledger accounts: %w", err)
	}
	accountMap := make(map[string]ledger.Account, len(accounts))
	for _, account := range accounts {
		accountMap[account.ID] = account
	}

	categories, err := a.ledger.ListCategories(ctx)
	if err != nil {
		return server.SpendingAnalysisResponse{}, fmt.Errorf("list ledger categories: %w", err)
	}
	categoryMap := make(map[string]ledger.Category, len(categories))
	for _, category := range categories {
		categoryMap[category.ID] = category
	}

	transactions, err := a.ledger.ListTransactions(ctx, ledger.TransactionQuery{})
	if err != nil {
		return server.SpendingAnalysisResponse{}, fmt.Errorf("list ledger transactions: %w", err)
	}

	periods := make([]string, comparePeriods+1)
	eventsByPeriod := make(map[string][]analytics.CashflowEvent, comparePeriods+1)
	for i := 0; i <= comparePeriods; i++ {
		periods[i] = start.AddDate(0, -i, 0).Format("2006-01")
	}

	warnings := []string{}
	partial := false
	for _, tx := range transactions {
		if tx.OccurredAt.Before(earliest) || !tx.OccurredAt.Before(end) {
			continue
		}
		if tx.SourceAmount.Currency != currency {
			partial = true
			warnings = appendWarning(warnings, fmt.Sprintf("transaction %s skipped: currency %s differs from household currency %s", tx.ID, tx.SourceAmount.Currency, currency))
			continue
		}
		event := analytics.NormalizeTransaction(tx, accountMap)
		if event.Type == analytics.CashflowEventUnknown {
			partial = true
			warnings = appendWarning(warnings, fmt.Sprintf("transaction %s has unknown cashflow semantics", tx.ID))
			continue
		}
		if event.Type != analytics.CashflowEventExpense && event.Type != analytics.CashflowEventRefund {
			continue
		}
		localPeriod := tx.OccurredAt.In(start.Location()).Format("2006-01")
		eventsByPeriod[localPeriod] = append(eventsByPeriod[localPeriod], event)
	}

	responses := make([]server.SpendingPeriodResponse, len(periods))
	for i, selectedPeriod := range periods {
		result, err := analytics.AggregateSpending(eventsByPeriod[selectedPeriod], currency)
		if err != nil {
			return server.SpendingAnalysisResponse{}, err
		}
		categoryResponses := make([]server.SpendingCategoryResponse, 0, len(result.Categories))
		for _, categoryTotal := range result.Categories {
			category := categoryMap[categoryTotal.CategoryID]
			name := category.Name
			if _, ok := categoryMap[categoryTotal.CategoryID]; !ok {
				partial = true
				warnings = appendWarning(warnings, fmt.Sprintf("category %s metadata unavailable", categoryTotal.CategoryID))
				name = ""
			}
			categoryResponses = append(categoryResponses, server.SpendingCategoryResponse{
				CategoryRef:      categoryTotal.CategoryID,
				Name:             name,
				Amount:           moneyDTO(categoryTotal.Amount),
				TransactionCount: categoryTotal.TransactionCount,
			})
		}
		responses[i] = server.SpendingPeriodResponse{
			Period:           selectedPeriod,
			Total:            moneyDTO(result.Total),
			TransactionCount: result.TransactionCount,
			Categories:       categoryResponses,
		}
	}

	quality := "good"
	if partial {
		quality = "partial"
	}
	return server.SpendingAnalysisResponse{
		DataAsOf:    a.now().UTC(),
		Quality:     quality,
		Currency:    currency,
		Current:     responses[0],
		Comparisons: append([]server.SpendingPeriodResponse(nil), responses[1:]...),
		Warnings:    warnings,
	}, nil
}
