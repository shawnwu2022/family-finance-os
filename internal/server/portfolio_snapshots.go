package server

import "time"

type PortfolioAssetUpsertRequest struct {
	Name             string     `json:"name"`
	AssetClass       string     `json:"asset_class"`
	ValueMinor       int64      `json:"value_minor,string"`
	Currency         string     `json:"currency"`
	SourceCurrency   string     `json:"source_currency"`
	ValuationAsOf    time.Time  `json:"valuation_as_of"`
	FXAsOf           *time.Time `json:"fx_as_of,omitempty"`
	SourceAccountRef string     `json:"source_account_ref,omitempty"`
	SourceKind       string     `json:"source_kind"`
}

type PortfolioAssetResponse struct {
	AssetRef         string     `json:"asset_ref"`
	Name             string     `json:"name"`
	AssetClass       string     `json:"asset_class"`
	ValueMinor       int64      `json:"value_minor,string"`
	Currency         string     `json:"currency"`
	SourceCurrency   string     `json:"source_currency"`
	ValuationAsOf    time.Time  `json:"valuation_as_of"`
	FXAsOf           *time.Time `json:"fx_as_of,omitempty"`
	SourceAccountRef string     `json:"source_account_ref,omitempty"`
	SourceKind       string     `json:"source_kind"`
}

type PortfolioAssetsResponse struct {
	Items []PortfolioAssetResponse `json:"items"`
}
