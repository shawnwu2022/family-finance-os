package appapi

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/shawnwu2022/family-finance-os/internal/portfolio"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func (a *API) ListPortfolioAssets(ctx context.Context, householdID int64) (server.PortfolioAssetsResponse, error) {
	if a.assetSnapshots == nil {
		return server.PortfolioAssetsResponse{}, ErrPortfolioUnavailable
	}
	snapshots, err := a.assetSnapshots.ListAssetSnapshots(ctx, householdID)
	if err != nil {
		return server.PortfolioAssetsResponse{}, fmt.Errorf("list portfolio assets: %w", err)
	}
	ordered := append([]portfolio.AssetSnapshot(nil), snapshots...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].AssetRef < ordered[j].AssetRef })
	items := make([]server.PortfolioAssetResponse, 0, len(ordered))
	for _, snapshot := range ordered {
		items = append(items, portfolioAssetDTO(snapshot))
	}
	return server.PortfolioAssetsResponse{Items: items}, nil
}

func (a *API) UpsertPortfolioAsset(ctx context.Context, householdID int64, assetRef string, request server.PortfolioAssetUpsertRequest) (server.PortfolioAssetResponse, error) {
	if a.assetSnapshots == nil {
		return server.PortfolioAssetResponse{}, ErrPortfolioUnavailable
	}
	snapshot := portfolio.AssetSnapshot{
		AssetRef:         strings.TrimSpace(assetRef),
		Name:             strings.TrimSpace(request.Name),
		Class:            portfolio.AssetClass(strings.ToLower(strings.TrimSpace(request.AssetClass))),
		Value:            money.Money{Minor: request.ValueMinor, Currency: strings.ToUpper(strings.TrimSpace(request.Currency))},
		SourceCurrency:   strings.ToUpper(strings.TrimSpace(request.SourceCurrency)),
		ValuationAsOf:    request.ValuationAsOf,
		FXAsOf:           request.FXAsOf,
		SourceAccountRef: strings.TrimSpace(request.SourceAccountRef),
		SourceKind:       portfolio.SnapshotSourceKind(strings.ToLower(strings.TrimSpace(request.SourceKind))),
	}
	if err := portfolio.ValidateAssetSnapshot(snapshot); err != nil {
		return server.PortfolioAssetResponse{}, fmt.Errorf("validate portfolio asset: %w", err)
	}
	stored, err := a.assetSnapshots.UpsertAssetSnapshot(ctx, householdID, snapshot)
	if err != nil {
		return server.PortfolioAssetResponse{}, fmt.Errorf("upsert portfolio asset: %w", err)
	}
	return portfolioAssetDTO(stored), nil
}

func (a *API) DeletePortfolioAsset(ctx context.Context, householdID int64, assetRef string) error {
	if a.assetSnapshots == nil {
		return ErrPortfolioUnavailable
	}
	assetRef = strings.TrimSpace(assetRef)
	if assetRef == "" {
		return portfolio.ErrInvalidAssetSnapshot
	}
	if err := a.assetSnapshots.DeleteAssetSnapshot(ctx, householdID, assetRef); err != nil {
		return fmt.Errorf("delete portfolio asset: %w", err)
	}
	return nil
}

func portfolioAssetDTO(snapshot portfolio.AssetSnapshot) server.PortfolioAssetResponse {
	return server.PortfolioAssetResponse{
		AssetRef:         snapshot.AssetRef,
		Name:             snapshot.Name,
		AssetClass:       string(snapshot.Class),
		ValueMinor:       snapshot.Value.Minor,
		Currency:         snapshot.Value.Currency,
		SourceCurrency:   snapshot.SourceCurrency,
		ValuationAsOf:    snapshot.ValuationAsOf,
		FXAsOf:           snapshot.FXAsOf,
		SourceAccountRef: snapshot.SourceAccountRef,
		SourceKind:       string(snapshot.SourceKind),
	}
}
