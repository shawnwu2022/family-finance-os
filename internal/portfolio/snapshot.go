package portfolio

import (
	"errors"
	"strings"
	"time"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var ErrInvalidAssetSnapshot = errors.New("invalid portfolio asset snapshot")

type SnapshotSourceKind string

const (
	SnapshotSourceManual SnapshotSourceKind = "manual"
	SnapshotSourceImport SnapshotSourceKind = "import"
)

type AssetSnapshot struct {
	AssetRef         string
	Name             string
	Class            AssetClass
	Value            money.Money
	SourceCurrency   string
	ValuationAsOf    time.Time
	FXAsOf           *time.Time
	SourceAccountRef string
	SourceKind       SnapshotSourceKind
}

func ValidateAssetSnapshot(snapshot AssetSnapshot) error {
	if !canonicalRequiredText(snapshot.AssetRef) || !canonicalRequiredText(snapshot.Name) {
		return ErrInvalidAssetSnapshot
	}
	if !snapshot.Class.valid() || snapshot.Value.Minor < 0 {
		return ErrInvalidAssetSnapshot
	}
	if !validCurrencyCode(snapshot.Value.Currency) || !validCurrencyCode(snapshot.SourceCurrency) {
		return ErrInvalidAssetSnapshot
	}
	if snapshot.ValuationAsOf.IsZero() {
		return ErrInvalidAssetSnapshot
	}
	if snapshot.FXAsOf != nil && snapshot.FXAsOf.IsZero() {
		return ErrInvalidAssetSnapshot
	}
	if snapshot.SourceAccountRef != "" && !canonicalRequiredText(snapshot.SourceAccountRef) {
		return ErrInvalidAssetSnapshot
	}
	switch snapshot.SourceKind {
	case SnapshotSourceManual, SnapshotSourceImport:
	default:
		return ErrInvalidAssetSnapshot
	}
	if snapshot.SourceCurrency != snapshot.Value.Currency && snapshot.FXAsOf == nil {
		return ErrInvalidAssetSnapshot
	}
	return nil
}

func canonicalRequiredText(value string) bool {
	return value != "" && value == strings.TrimSpace(value)
}

func validCurrencyCode(value string) bool {
	if len(value) != 3 {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 'A' || value[i] > 'Z' {
			return false
		}
	}
	return true
}
