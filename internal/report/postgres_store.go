package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Get(ctx context.Context, householdID int64, period string) (StoredMonthlyReport, error) {
	if s == nil || s.pool == nil {
		return StoredMonthlyReport{}, errors.New("report store is not configured")
	}
	var stored StoredMonthlyReport
	var payload []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, household_id, content_hash, payload, created_at
		FROM monthly_reports
		WHERE household_id = $1 AND period = $2 AND kind = 'monthly'
	`, householdID, period).Scan(&stored.ID, &stored.HouseholdID, &stored.ContentHash, &payload, &stored.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return StoredMonthlyReport{}, ErrNotFound
	}
	if err != nil {
		return StoredMonthlyReport{}, fmt.Errorf("get monthly report: %w", err)
	}
	return decodeStored(stored, payload)
}

func (s *PostgresStore) Save(ctx context.Context, householdID int64, monthly MonthlyReport) (StoredMonthlyReport, error) {
	if s == nil || s.pool == nil {
		return StoredMonthlyReport{}, errors.New("report store is not configured")
	}
	payload, err := json.Marshal(monthly)
	if err != nil {
		return StoredMonthlyReport{}, fmt.Errorf("encode monthly report: %w", err)
	}
	hash, err := ContentHash(monthly)
	if err != nil {
		return StoredMonthlyReport{}, err
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO monthly_reports (
			household_id, period, kind, data_as_of, generated_at, quality, content_hash, payload
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (household_id, period, kind) DO NOTHING
	`, householdID, monthly.Period, monthly.Kind, monthly.DataAsOf, monthly.GeneratedAt, monthly.Quality, hash, payload); err != nil {
		return StoredMonthlyReport{}, fmt.Errorf("save monthly report: %w", err)
	}
	return s.Get(ctx, householdID, monthly.Period)
}

func (s *PostgresStore) List(ctx context.Context, householdID int64) ([]StoredMonthlyReport, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("report store is not configured")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, household_id, content_hash, payload, created_at
		FROM monthly_reports
		WHERE household_id = $1 AND kind = 'monthly'
		ORDER BY period DESC, id DESC
	`, householdID)
	if err != nil {
		return nil, fmt.Errorf("list monthly reports: %w", err)
	}
	defer rows.Close()
	items := []StoredMonthlyReport{}
	for rows.Next() {
		var stored StoredMonthlyReport
		var payload []byte
		if err := rows.Scan(&stored.ID, &stored.HouseholdID, &stored.ContentHash, &payload, &stored.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan monthly report: %w", err)
		}
		stored, err = decodeStored(stored, payload)
		if err != nil {
			return nil, err
		}
		items = append(items, stored)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list monthly reports: %w", err)
	}
	return items, nil
}

func decodeStored(stored StoredMonthlyReport, payload []byte) (StoredMonthlyReport, error) {
	if err := json.Unmarshal(payload, &stored.Report); err != nil {
		return StoredMonthlyReport{}, fmt.Errorf("decode monthly report %d: %w", stored.ID, err)
	}
	hash, err := ContentHash(stored.Report)
	if err != nil {
		return StoredMonthlyReport{}, err
	}
	if hash != stored.ContentHash {
		return StoredMonthlyReport{}, fmt.Errorf("%w: report %d", ErrHashMismatch, stored.ID)
	}
	return stored, nil
}
