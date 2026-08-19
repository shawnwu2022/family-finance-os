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
		cfg.api = scopedFinanceAPI{next: api}
	}
}

type scopedFinanceAPI struct {
	next FinanceAPI
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

func (s scopedFinanceAPI) ListPortfolioAssets(ctx context.Context, householdID int64) (PortfolioAssetsResponse, error) {
	api, ok := s.next.(PortfolioFinanceAPI)
	if !ok {
		return PortfolioAssetsResponse{}, errPortfolioFinanceAPIUnavailable
	}
	return api.ListPortfolioAssets(ctx, householdID)
}

func (s scopedFinanceAPI) UpsertPortfolioAsset(ctx context.Context, householdID int64, assetRef string, request PortfolioAssetUpsertRequest) (PortfolioAssetResponse, error) {
	api, ok := s.next.(PortfolioFinanceAPI)
	if !ok {
		return PortfolioAssetResponse{}, errPortfolioFinanceAPIUnavailable
	}
	return api.UpsertPortfolioAsset(ctx, householdID, assetRef, request)
}

func (s scopedFinanceAPI) DeletePortfolioAsset(ctx context.Context, householdID int64, assetRef string) error {
	api, ok := s.next.(PortfolioFinanceAPI)
	if !ok {
		return errPortfolioFinanceAPIUnavailable
	}
	return api.DeletePortfolioAsset(ctx, householdID, assetRef)
}
