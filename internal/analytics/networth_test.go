package analytics

import (
	"errors"
	"math"
	"testing"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestCalculateNetWorthAssetsMinusLiabilities(t *testing.T) {
	got, err := CalculateNetWorth([]Valuation{
		{Kind: ValuationAsset, Value: money.Money{Minor: 1200000, Currency: "CNY"}},
		{Kind: ValuationAsset, Value: money.Money{Minor: 300000, Currency: "CNY"}},
		{Kind: ValuationLiability, Value: money.Money{Minor: 450000, Currency: "CNY"}},
	}, "CNY")
	if err != nil {
		t.Fatalf("CalculateNetWorth: %v", err)
	}
	if got.TotalAssets.Minor != 1500000 {
		t.Fatalf("assets = %d", got.TotalAssets.Minor)
	}
	if got.TotalLiabilities.Minor != 450000 {
		t.Fatalf("liabilities = %d", got.TotalLiabilities.Minor)
	}
	if got.NetWorth.Minor != 1050000 {
		t.Fatalf("net worth = %d", got.NetWorth.Minor)
	}
}

func TestCalculateNetWorthRequiresNonNegativeExplicitValuations(t *testing.T) {
	_, err := CalculateNetWorth([]Valuation{{Kind: ValuationLiability, Value: money.Money{Minor: -1, Currency: "CNY"}}}, "CNY")
	if !errors.Is(err, ErrNegativeValuation) {
		t.Fatalf("error = %v, want ErrNegativeValuation", err)
	}
}

func TestCalculateNetWorthRejectsCurrencyMismatchOverflowAndUnknownKind(t *testing.T) {
	_, err := CalculateNetWorth([]Valuation{{Kind: ValuationAsset, Value: money.Money{Minor: 1, Currency: "USD"}}}, "CNY")
	if !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("currency error = %v", err)
	}

	_, err = CalculateNetWorth([]Valuation{
		{Kind: ValuationAsset, Value: money.Money{Minor: math.MaxInt64, Currency: "CNY"}},
		{Kind: ValuationAsset, Value: money.Money{Minor: 1, Currency: "CNY"}},
	}, "CNY")
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("overflow error = %v", err)
	}

	_, err = CalculateNetWorth([]Valuation{{Kind: ValuationKind(99), Value: money.Money{Minor: 1, Currency: "CNY"}}}, "CNY")
	if !errors.Is(err, ErrUnknownValuationKind) {
		t.Fatalf("kind error = %v", err)
	}
}

func FuzzCalculateNetWorthBalancedIncreaseInvariant(f *testing.F) {
	f.Add(uint32(1000000), uint32(300000), uint32(50000))
	f.Fuzz(func(t *testing.T, assetMinor, liabilityMinor, delta uint32) {
		before, err := CalculateNetWorth([]Valuation{
			{Kind: ValuationAsset, Value: money.Money{Minor: int64(assetMinor), Currency: "CNY"}},
			{Kind: ValuationLiability, Value: money.Money{Minor: int64(liabilityMinor), Currency: "CNY"}},
		}, "CNY")
		if err != nil {
			t.Fatalf("before: %v", err)
		}
		after, err := CalculateNetWorth([]Valuation{
			{Kind: ValuationAsset, Value: money.Money{Minor: int64(assetMinor) + int64(delta), Currency: "CNY"}},
			{Kind: ValuationLiability, Value: money.Money{Minor: int64(liabilityMinor) + int64(delta), Currency: "CNY"}},
		}, "CNY")
		if err != nil {
			t.Fatalf("after: %v", err)
		}
		if after.NetWorth != before.NetWorth {
			t.Fatalf("balanced increase changed net worth: before=%+v after=%+v", before.NetWorth, after.NetWorth)
		}
	})
}
