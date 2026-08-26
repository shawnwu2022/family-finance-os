package server

import (
	"context"

	"github.com/shawnwu2022/family-finance-os/internal/requestscope"
)

func WithScopedAPI(api FinanceAPI) HandlerOption {
	return func(cfg *handlerConfig) {
		if api == nil {
			cfg.api = nil
			return
		}
		base := scopedFinanceAPI{next: api}
		if portfolioAPI, ok := api.(PortfolioFinanceAPI); ok {
			cfg.api = scopedPortfolioFinanceAPI{scopedFinanceAPI: base, portfolio: portfolioAPI}
			return
		}
		cfg.api = base
	}
}

type scopedFinanceAPI struct {
	next FinanceAPI
}

func (s scopedFinanceAPI) Dashboard(ctx context.Context, householdID int64, period string) (DashboardResponse, error) {
	return s.next.Dashboard(ctx, householdID, period)
}

func (s scopedFinanceAPI) Overview(ctx context.Context, householdID int64) (OverviewResponse, error) {
	return s.next.Overview(ctx, householdID)
}

func (s scopedFinanceAPI) Cashflow(ctx context.Context, householdID int64, period string) (CashflowResponse, error) {
	return s.next.Cashflow(ctx, householdID, period)
}

func (s scopedFinanceAPI) Budget(ctx context.Context, householdID int64, period string) (BudgetResponse, error) {
	return s.next.Budget(ctx, householdID, period)
}

func (s scopedFinanceAPI) Debts(ctx context.Context, householdID int64) (DebtsResponse, error) {
	return s.next.Debts(ctx, householdID)
}

func (s scopedFinanceAPI) Goals(ctx context.Context, householdID int64) (GoalsResponse, error) {
	return s.next.Goals(ctx, householdID)
}

func (s scopedFinanceAPI) Scenario(ctx context.Context, request ScenarioRequest) (ScenarioResponse, error) {
	return s.next.Scenario(ctx, request)
}

func (s scopedFinanceAPI) Advisor(ctx context.Context, request AdvisorRequest) (AdvisorResponse, error) {
	return s.next.Advisor(requestscope.WithHouseholdID(ctx, request.HouseholdID), request)
}

func (s scopedFinanceAPI) Reports(ctx context.Context, householdID int64) (ReportsResponse, error) {
	return s.next.Reports(ctx, householdID)
}

type scopedPortfolioFinanceAPI struct {
	scopedFinanceAPI
	portfolio PortfolioFinanceAPI
}

func (s scopedPortfolioFinanceAPI) ListPortfolioAssets(ctx context.Context, householdID int64) (PortfolioAssetsResponse, error) {
	return s.portfolio.ListPortfolioAssets(ctx, householdID)
}

func (s scopedPortfolioFinanceAPI) UpsertPortfolioAsset(ctx context.Context, householdID int64, assetRef string, request PortfolioAssetUpsertRequest) (PortfolioAssetResponse, error) {
	return s.portfolio.UpsertPortfolioAsset(ctx, householdID, assetRef, request)
}

func (s scopedPortfolioFinanceAPI) DeletePortfolioAsset(ctx context.Context, householdID int64, assetRef string) error {
	return s.portfolio.DeletePortfolioAsset(ctx, householdID, assetRef)
}
