package main

import (
	"context"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

var _ server.DataQualityAPI = householdScopedAPI{}

func (a householdScopedAPI) DataQuality(ctx context.Context, householdID int64, period string) (server.DataQualityResponse, error) {
	return a.advisor.DataQuality(ctx, householdID, period)
}
