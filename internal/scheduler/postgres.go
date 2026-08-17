package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
)

var ErrRunNotClaimed = errors.New("job run is not currently claimed")

const ErrorCodeProcessRestarted = "process_restarted"

type PostgresRunStore struct {
	queries *storesqlc.Queries
}

func NewPostgresRunStore(pool *pgxpool.Pool) *PostgresRunStore {
	return &PostgresRunStore{queries: storesqlc.New(pool)}
}

func (s *PostgresRunStore) Claim(ctx context.Context, key RunKey, startedAt time.Time) (bool, error) {
	_, err := s.queries.ClaimJobRun(ctx, storesqlc.ClaimJobRunParams{
		HouseholdID:  key.HouseholdID,
		JobName:      strings.TrimSpace(key.JobName),
		ScheduledFor: postgresTime(key.ScheduledFor),
		Period:       strings.TrimSpace(key.Period),
		StartedAt:    postgresTime(startedAt),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim persisted job run: %w", err)
	}
	return true, nil
}

func (s *PostgresRunStore) Finish(ctx context.Context, key RunKey, finishedAt time.Time, outcome RunOutcome) error {
	errorCode := pgtype.Text{}
	switch outcome.Status {
	case RunSucceeded:
		if outcome.ErrorCode != "" {
			return fmt.Errorf("%w: successful run cannot have an error code", ErrInvalidJob)
		}
	case RunFailed:
		if strings.TrimSpace(outcome.ErrorCode) == "" {
			return fmt.Errorf("%w: failed run requires an error code", ErrInvalidJob)
		}
		errorCode = pgtype.Text{String: strings.TrimSpace(outcome.ErrorCode), Valid: true}
	default:
		return fmt.Errorf("%w: unsupported run status %q", ErrInvalidJob, outcome.Status)
	}

	rows, err := s.queries.FinishJobRun(ctx, storesqlc.FinishJobRunParams{
		Status:       string(outcome.Status),
		FinishedAt:   postgresTime(finishedAt),
		ErrorCode:    errorCode,
		HouseholdID:  key.HouseholdID,
		JobName:      strings.TrimSpace(key.JobName),
		ScheduledFor: postgresTime(key.ScheduledFor),
	})
	if err != nil {
		return fmt.Errorf("finish persisted job run: %w", err)
	}
	if rows != 1 {
		return ErrRunNotClaimed
	}
	return nil
}

func (s *PostgresRunStore) RecoverInterrupted(ctx context.Context, recoveredAt time.Time) error {
	if _, err := s.queries.RecoverInterruptedJobRuns(ctx, postgresTime(recoveredAt)); err != nil {
		return fmt.Errorf("recover interrupted job runs: %w", err)
	}
	return nil
}

func (s *PostgresRunStore) List(ctx context.Context, householdID int64, jobName string) ([]RunRecord, error) {
	rows, err := s.queries.ListJobRuns(ctx, storesqlc.ListJobRunsParams{
		HouseholdID: householdID,
		JobName:     strings.TrimSpace(jobName),
	})
	if err != nil {
		return nil, fmt.Errorf("list persisted job runs: %w", err)
	}
	records := make([]RunRecord, 0, len(rows))
	for _, row := range rows {
		record := RunRecord{
			ID: row.ID,
			Key: RunKey{
				HouseholdID:  row.HouseholdID,
				JobName:      row.JobName,
				ScheduledFor: pgTime(row.ScheduledFor),
				Period:       row.Period,
			},
			Status:    RunStatus(row.Status),
			StartedAt: pgTime(row.StartedAt),
		}
		if row.FinishedAt.Valid {
			record.FinishedAt = row.FinishedAt.Time
		}
		if row.ErrorCode.Valid {
			record.ErrorCode = row.ErrorCode.String
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *PostgresRunStore) ListHouseholds(ctx context.Context) ([]HouseholdScope, error) {
	rows, err := s.queries.ListSchedulerHouseholds(ctx)
	if err != nil {
		return nil, fmt.Errorf("list scheduler households: %w", err)
	}
	scopes := make([]HouseholdScope, 0, len(rows))
	for _, row := range rows {
		scopes = append(scopes, HouseholdScope{HouseholdID: row.ID, Timezone: row.Timezone})
	}
	return scopes, nil
}

func postgresTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func pgTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}
