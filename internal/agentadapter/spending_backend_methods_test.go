package agentadapter

import (
	"context"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func (f *fakeBackend) SpendingAnalysis(context.Context, int64, string, int) (server.SpendingAnalysisResponse, error) {
	return server.SpendingAnalysisResponse{}, f.err
}

func (b *cancellationBackend) SpendingAnalysis(ctx context.Context, _ int64, _ string, _ int) (server.SpendingAnalysisResponse, error) {
	return server.SpendingAnalysisResponse{}, ctx.Err()
}

func (b *concurrentBackend) SpendingAnalysis(context.Context, int64, string, int) (server.SpendingAnalysisResponse, error) {
	return server.SpendingAnalysisResponse{}, nil
}
