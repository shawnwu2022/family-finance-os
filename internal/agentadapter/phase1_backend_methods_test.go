package agentadapter

import (
	"context"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func (f *fakeBackend) SafeToSpend(_ context.Context, _ int64) (server.SafeToSpendResponse, error) {
	return server.SafeToSpendResponse{}, f.err
}

func (f *fakeBackend) SimulateGoal(_ context.Context, _, _, _ int64) (server.GoalSimulationResponse, error) {
	return server.GoalSimulationResponse{}, f.err
}

func (b *cancellationBackend) SafeToSpend(ctx context.Context, _ int64) (server.SafeToSpendResponse, error) {
	return server.SafeToSpendResponse{}, ctx.Err()
}

func (b *cancellationBackend) SimulateGoal(ctx context.Context, _, _, _ int64) (server.GoalSimulationResponse, error) {
	return server.GoalSimulationResponse{}, ctx.Err()
}

func (b *concurrentBackend) SafeToSpend(context.Context, int64) (server.SafeToSpendResponse, error) {
	return server.SafeToSpendResponse{}, nil
}

func (b *concurrentBackend) SimulateGoal(context.Context, int64, int64, int64) (server.GoalSimulationResponse, error) {
	return server.GoalSimulationResponse{}, nil
}
