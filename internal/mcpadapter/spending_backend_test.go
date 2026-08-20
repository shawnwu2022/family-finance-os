package mcpadapter

import (
	"context"

	appserver "github.com/shawnwu2022/family-finance-os/internal/server"
)

func (*stubFinanceBackend) SpendingAnalysis(context.Context, int64, string, int) (appserver.SpendingAnalysisResponse, error) {
	return appserver.SpendingAnalysisResponse{}, nil
}
