package portfolio

import (
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestValidateAssetSnapshot(t *testing.T) {
	asOf := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	fxAsOf := asOf.Add(-time.Hour)
	valid := AssetSnapshot{
		AssetRef:         "brokerage-1:510300",
		Name:             "沪深300 ETF",
		Class:            AssetClassFund,
		Value:            money.Money{Minor: 1_250_000, Currency: "CNY"},
		SourceCurrency:   "CNY",
		ValuationAsOf:    asOf,
		SourceAccountRef: "brokerage-1",
		SourceKind:       SnapshotSourceManual,
	}

	cases := []struct {
		name    string
		mutate  func(*AssetSnapshot)
		wantErr bool
	}{
		{name: "valid manual"},
		{name: "valid import", mutate: func(s *AssetSnapshot) { s.SourceKind = SnapshotSourceImport }},
		{name: "valid zero value", mutate: func(s *AssetSnapshot) { s.Value.Minor = 0 }},
		{name: "valid foreign source with fx timestamp", mutate: func(s *AssetSnapshot) { s.SourceCurrency = "USD"; s.FXAsOf = &fxAsOf }},
		{name: "blank asset ref", mutate: func(s *AssetSnapshot) { s.AssetRef = "" }, wantErr: true},
		{name: "whitespace asset ref", mutate: func(s *AssetSnapshot) { s.AssetRef = "   " }, wantErr: true},
		{name: "non canonical asset ref", mutate: func(s *AssetSnapshot) { s.AssetRef = " asset-1 " }, wantErr: true},
		{name: "blank name", mutate: func(s *AssetSnapshot) { s.Name = "" }, wantErr: true},
		{name: "whitespace name", mutate: func(s *AssetSnapshot) { s.Name = "   " }, wantErr: true},
		{name: "non canonical name", mutate: func(s *AssetSnapshot) { s.Name = " Fund " }, wantErr: true},
		{name: "invalid class", mutate: func(s *AssetSnapshot) { s.Class = AssetClass("crypto") }, wantErr: true},
		{name: "negative value", mutate: func(s *AssetSnapshot) { s.Value.Minor = -1 }, wantErr: true},
		{name: "blank reporting currency", mutate: func(s *AssetSnapshot) { s.Value.Currency = "" }, wantErr: true},
		{name: "lowercase reporting currency", mutate: func(s *AssetSnapshot) { s.Value.Currency = "cny" }, wantErr: true},
		{name: "long reporting currency", mutate: func(s *AssetSnapshot) { s.Value.Currency = "USDT" }, wantErr: true},
		{name: "blank source currency", mutate: func(s *AssetSnapshot) { s.SourceCurrency = "" }, wantErr: true},
		{name: "lowercase source currency", mutate: func(s *AssetSnapshot) { s.SourceCurrency = "usd" }, wantErr: true},
		{name: "zero valuation time", mutate: func(s *AssetSnapshot) { s.ValuationAsOf = time.Time{} }, wantErr: true},
		{name: "unsupported source kind", mutate: func(s *AssetSnapshot) { s.SourceKind = SnapshotSourceKind("market") }, wantErr: true},
		{name: "foreign source without fx timestamp", mutate: func(s *AssetSnapshot) { s.SourceCurrency = "USD"; s.FXAsOf = nil }, wantErr: true},
		{name: "non canonical source account ref", mutate: func(s *AssetSnapshot) { s.SourceAccountRef = " brokerage-1 " }, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := valid
			if tc.mutate != nil {
				tc.mutate(&snapshot)
			}
			err := ValidateAssetSnapshot(snapshot)
			if tc.wantErr && err == nil {
				t.Fatalf("ValidateAssetSnapshot(%#v) error=nil, want error", snapshot)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("ValidateAssetSnapshot(%#v) error=%v, want nil", snapshot, err)
			}
		})
	}
}
