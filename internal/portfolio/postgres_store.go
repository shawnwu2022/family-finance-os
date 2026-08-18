package portfolio

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type PostgresStore struct {
	queries *storesqlc.Queries
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{queries: storesqlc.New(pool)}
}

func (s *PostgresStore) ListAssetSnapshots(ctx context.Context, householdID int64) ([]AssetSnapshot, error) {
	if householdID <= 0 {
		return nil, errors.New("household ID must be positive")
	}
	rows, err := s.queries.ListPortfolioAssetSnapshotsByHousehold(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("list portfolio asset snapshots: %w", err)
	}
	snapshots := make([]AssetSnapshot, 0, len(rows))
	for _, row := range rows {
		snapshot, err := assetSnapshotFromValues(
			row.AssetRef,
			row.Name,
			row.AssetClass,
			row.ValueMinor,
			row.Currency,
			row.SourceCurrency,
			row.ValuationAsOf,
			row.FxAsOf,
			row.SourceAccountRef,
			row.SourceKind,
		)
		if err != nil {
			return nil, fmt.Errorf("decode portfolio asset snapshot %q: %w", row.AssetRef, err)
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func (s *PostgresStore) UpsertAssetSnapshot(ctx context.Context, householdID int64, snapshot AssetSnapshot) (AssetSnapshot, error) {
	if householdID <= 0 {
		return AssetSnapshot{}, errors.New("household ID must be positive")
	}
	if err := ValidateAssetSnapshot(snapshot); err != nil {
		return AssetSnapshot{}, err
	}

	row, err := s.queries.UpsertPortfolioAssetSnapshot(ctx, storesqlc.UpsertPortfolioAssetSnapshotParams{
		HouseholdID:      householdID,
		AssetRef:         snapshot.AssetRef,
		Name:             snapshot.Name,
		AssetClass:       string(snapshot.Class),
		ValueMinor:       snapshot.Value.Minor,
		Currency:         snapshot.Value.Currency,
		SourceCurrency:   snapshot.SourceCurrency,
		ValuationAsOf:    pgtype.Timestamptz{Time: snapshot.ValuationAsOf, Valid: true},
		FxAsOf:           optionalTimestamptz(snapshot.FXAsOf),
		SourceAccountRef: optionalText(snapshot.SourceAccountRef),
		SourceKind:       string(snapshot.SourceKind),
	})
	if err != nil {
		return AssetSnapshot{}, fmt.Errorf("upsert portfolio asset snapshot: %w", err)
	}
	return assetSnapshotFromValues(
		row.AssetRef,
		row.Name,
		row.AssetClass,
		row.ValueMinor,
		row.Currency,
		row.SourceCurrency,
		row.ValuationAsOf,
		row.FxAsOf,
		row.SourceAccountRef,
		row.SourceKind,
	)
}

func (s *PostgresStore) DeleteAssetSnapshot(ctx context.Context, householdID int64, assetRef string) error {
	if householdID <= 0 {
		return errors.New("household ID must be positive")
	}
	if !canonicalRequiredText(assetRef) {
		return ErrInvalidAssetSnapshot
	}
	if err := s.queries.DeletePortfolioAssetSnapshot(ctx, storesqlc.DeletePortfolioAssetSnapshotParams{
		HouseholdID: householdID,
		AssetRef:    assetRef,
	}); err != nil {
		return fmt.Errorf("delete portfolio asset snapshot: %w", err)
	}
	return nil
}

func assetSnapshotFromValues(
	assetRef string,
	name string,
	assetClass string,
	valueMinor int64,
	currency string,
	sourceCurrency string,
	valuationAsOf pgtype.Timestamptz,
	fxAsOf pgtype.Timestamptz,
	sourceAccountRef pgtype.Text,
	sourceKind string,
) (AssetSnapshot, error) {
	if !valuationAsOf.Valid {
		return AssetSnapshot{}, ErrInvalidAssetSnapshot
	}
	var fxTime *time.Time
	if fxAsOf.Valid {
		value := fxAsOf.Time
		fxTime = &value
	}
	snapshot := AssetSnapshot{
		AssetRef:         assetRef,
		Name:             name,
		Class:            AssetClass(assetClass),
		Value:            money.Money{Minor: valueMinor, Currency: currency},
		SourceCurrency:   sourceCurrency,
		ValuationAsOf:    valuationAsOf.Time,
		FXAsOf:           fxTime,
		SourceAccountRef: textValue(sourceAccountRef),
		SourceKind:       SnapshotSourceKind(sourceKind),
	}
	if err := ValidateAssetSnapshot(snapshot); err != nil {
		return AssetSnapshot{}, err
	}
	return snapshot, nil
}

func optionalTimestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func optionalText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func textValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
