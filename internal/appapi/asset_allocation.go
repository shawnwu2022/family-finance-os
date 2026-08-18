package appapi

import (
	"context"
	"fmt"
	"sort"

	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/portfolio"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func (a *API) AssetAllocation(ctx context.Context, householdID int64) (server.AssetAllocationResponse, error) {
	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return server.AssetAllocationResponse{}, fmt.Errorf("load household profile: %w", err)
	}
	currency := profile.Household.BaseCurrency
	asOf := a.now().UTC()

	accounts, err := a.ledger.ListAccounts(ctx)
	if err != nil {
		return server.AssetAllocationResponse{}, fmt.Errorf("list ledger accounts: %w", err)
	}

	valuations := make([]portfolio.Valuation, 0, len(accounts))
	warnings := make([]string, 0)
	partial := false
	for _, account := range accounts {
		if account.Hidden || account.Structure == ledger.AccountStructureMultipleSubAccounts || !account.IsAsset || account.IsLiability {
			continue
		}
		if account.Balance.Currency != currency {
			partial = true
			warnings = appendWarning(warnings, fmt.Sprintf("account %s skipped: currency %s differs from household currency %s", account.ID, account.Balance.Currency, currency))
			continue
		}
		value, err := magnitude(account.Balance)
		if err != nil {
			return server.AssetAllocationResponse{}, err
		}
		class, warning := portfolioClassForAccount(account)
		if warning != "" {
			partial = true
			warnings = appendWarning(warnings, warning)
		}
		valuations = append(valuations, portfolio.Valuation{
			ID:             account.ID,
			Name:           account.Name,
			Class:          class,
			Value:          value,
			SourceCurrency: currency,
			ValuationAsOf:  asOf,
		})
	}

	summary, err := portfolio.Summarize(portfolio.SummaryInput{
		ReportingCurrency: currency,
		AsOf:              asOf,
		Valuations:        valuations,
	})
	if err != nil {
		return server.AssetAllocationResponse{}, err
	}
	for _, warning := range summary.Warnings {
		partial = true
		warnings = appendWarning(warnings, fmt.Sprintf("portfolio warning %s for valuation %s", warning.Code, warning.ValuationID))
	}

	classes := make([]portfolio.AssetClass, 0, len(summary.ByClass))
	for class := range summary.ByClass {
		classes = append(classes, class)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i] < classes[j] })
	items := make([]server.AssetAllocationItemResponse, 0, len(classes))
	for _, class := range classes {
		allocation := summary.ByClass[class]
		share := ""
		if allocation.Share != nil {
			share = allocation.Share.String()
		}
		items = append(items, server.AssetAllocationItemResponse{
			Class: string(class),
			Value: moneyDTO(allocation.Value),
			Share: share,
		})
	}

	quality := "good"
	if partial {
		quality = "partial"
	}
	return server.AssetAllocationResponse{
		DataAsOf: asOf,
		Quality:  quality,
		Currency: currency,
		Total:    moneyDTO(summary.Total),
		Items:    items,
		Warnings: warnings,
	}, nil
}

func portfolioClassForAccount(account ledger.Account) (portfolio.AssetClass, string) {
	switch account.Category {
	case ledger.AccountCategoryCash, ledger.AccountCategoryChecking, ledger.AccountCategoryVirtual:
		return portfolio.AssetClassCash, ""
	case ledger.AccountCategorySavings, ledger.AccountCategoryCertificateOfDeposit:
		return portfolio.AssetClassDeposit, ""
	case ledger.AccountCategoryInvestment:
		return portfolio.AssetClassOther, fmt.Sprintf("account %s classified as other: investment holdings unavailable", account.ID)
	case ledger.AccountCategoryReceivables:
		return portfolio.AssetClassOther, fmt.Sprintf("account %s classified as other: receivables are not a market asset class", account.ID)
	case ledger.AccountCategoryUnknown:
		return portfolio.AssetClassOther, fmt.Sprintf("account %s classified as other: unknown account category", account.ID)
	default:
		return portfolio.AssetClassOther, fmt.Sprintf("account %s classified as other: account category %d has no finer allocation mapping", account.ID, account.Category)
	}
}
