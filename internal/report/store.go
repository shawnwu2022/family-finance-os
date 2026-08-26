package report

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound     = errors.New("report artifact not found")
	ErrHashMismatch = errors.New("report artifact hash mismatch")
)

type StoredMonthlyReport struct {
	ID          int64
	HouseholdID int64
	ContentHash string
	CreatedAt   time.Time
	Report      MonthlyReport
}

type Store interface {
	Get(ctx context.Context, householdID int64, period string) (StoredMonthlyReport, error)
	Save(ctx context.Context, householdID int64, monthly MonthlyReport) (StoredMonthlyReport, error)
	List(ctx context.Context, householdID int64) ([]StoredMonthlyReport, error)
}

func ContentHash(monthly MonthlyReport) (string, error) {
	payload, err := json.Marshal(monthly)
	if err != nil {
		return "", fmt.Errorf("encode monthly report: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}
