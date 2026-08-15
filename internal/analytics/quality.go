package analytics

import (
	"time"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type QualityLevel uint8

const (
	QualityGood QualityLevel = iota + 1
	QualityPartial
	QualityStale
)

type DataQuality struct {
	AsOf           time.Time
	LedgerSyncedAt time.Time
	UnknownAmount  money.Money
	Level          QualityLevel
}

func EvaluateDataQuality(asOf, ledgerSyncedAt time.Time, unknownAmount money.Money, staleAfter time.Duration) DataQuality {
	level := QualityGood
	if ledgerSyncedAt.IsZero() || (staleAfter > 0 && asOf.Sub(ledgerSyncedAt) > staleAfter) {
		level = QualityStale
	} else if unknownAmount.Minor != 0 {
		level = QualityPartial
	}
	return DataQuality{
		AsOf:           asOf,
		LedgerSyncedAt: ledgerSyncedAt,
		UnknownAmount:  unknownAmount,
		Level:          level,
	}
}
