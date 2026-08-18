package main

import (
	"context"

	appserver "github.com/shawnwu2022/family-finance-os/internal/server"
)

func (*mcpIntegrationBackend) SpendingAnalysis(context.Context, int64, string, int) (appserver.SpendingAnalysisResponse, error) {
	return appserver.SpendingAnalysisResponse{}, nil
}
