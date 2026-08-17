package scheduler

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/store"
)

func TestPostgresRunStoreIdempotencyAndRestartRecoveryIntegration(t *testing.T) {
	pool := openSchedulerIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var householdID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO households (name, base_currency, timezone)
		VALUES ($1, 'CNY', 'Asia/Shanghai')
		RETURNING id
	`, "scheduler-integration-"+strconv.FormatInt(time.Now().UnixNano(), 10)).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM households WHERE id = $1`, householdID)
	})

	runs := NewPostgresRunStore(pool)
	key := RunKey{
		HouseholdID:  householdID,
		JobName:      "monthly_report",
		ScheduledFor: time.Date(2026, time.August, 1, 3, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
		Period:       "2026-07",
	}
	startedAt := time.Date(2026, time.August, 1, 3, 0, 1, 0, time.UTC)

	claimed, err := runs.Claim(ctx, key, startedAt)
	if err != nil || !claimed {
		t.Fatalf("first Claim() = %v, %v; want true, nil", claimed, err)
	}
	claimed, err = runs.Claim(ctx, key, startedAt.Add(time.Second))
	if err != nil || claimed {
		t.Fatalf("duplicate Claim() = %v, %v; want false, nil", claimed, err)
	}
	if err := runs.Finish(ctx, key, startedAt.Add(2*time.Second), RunOutcome{Status: RunSucceeded}); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	records, err := runs.List(ctx, householdID, key.JobName)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("List() returned %d records, want 1", len(records))
	}
	if records[0].ID <= 0 || records[0].Key.Period != key.Period || records[0].Status != RunSucceeded {
		t.Fatalf("stored record = %#v", records[0])
	}

	interrupted := RunKey{
		HouseholdID:  householdID,
		JobName:      key.JobName,
		ScheduledFor: key.ScheduledFor.AddDate(0, 1, 0),
		Period:       "2026-08",
	}
	claimed, err = runs.Claim(ctx, interrupted, startedAt.Add(time.Hour))
	if err != nil || !claimed {
		t.Fatalf("interrupted Claim() = %v, %v", claimed, err)
	}
	recoveredAt := startedAt.Add(2 * time.Hour)
	if err := runs.RecoverInterrupted(ctx, recoveredAt); err != nil {
		t.Fatalf("RecoverInterrupted() error = %v", err)
	}
	claimed, err = runs.Claim(ctx, interrupted, recoveredAt.Add(time.Second))
	if err != nil || !claimed {
		t.Fatalf("Claim() after recovery = %v, %v; want true, nil", claimed, err)
	}
}

func TestPostgresRunStoreListsSchedulerHouseholdsIntegration(t *testing.T) {
	pool := openSchedulerIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var householdID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO households (name, base_currency, timezone)
		VALUES ($1, 'CNY', 'Asia/Shanghai')
		RETURNING id
	`, "scheduler-household-"+strconv.FormatInt(time.Now().UnixNano(), 10)).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM households WHERE id = $1`, householdID)
	})

	scopes, err := NewPostgresRunStore(pool).ListHouseholds(ctx)
	if err != nil {
		t.Fatalf("ListHouseholds() error = %v", err)
	}
	for _, scope := range scopes {
		if scope.HouseholdID == householdID {
			if scope.Timezone != "Asia/Shanghai" {
				t.Fatalf("timezone = %q, want Asia/Shanghai", scope.Timezone)
			}
			return
		}
	}
	t.Fatalf("household %d not returned: %#v", householdID, scopes)
}

func openSchedulerIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}
	portRaw := os.Getenv("TEST_POSTGRES_PORT")
	if portRaw == "" {
		portRaw = "5432"
	}
	port, err := strconv.ParseUint(portRaw, 10, 16)
	if err != nil {
		t.Fatalf("parse TEST_POSTGRES_PORT: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := store.OpenPostgres(ctx, config.DatabaseConfig{
		Host: host, Port: uint16(port), Name: os.Getenv("TEST_POSTGRES_DB"),
		User: os.Getenv("TEST_POSTGRES_USER"), Password: os.Getenv("TEST_POSTGRES_PASSWORD"), SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
