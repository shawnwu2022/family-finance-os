package agentadapter

import (
	"context"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func (f *fakeBackend) SimulateExtraDebtPayment(context.Context, int64, int64, int64) (server.DebtExtraPaymentSimulationResponse, error) {
	return server.DebtExtraPaymentSimulationResponse{}, f.err
}

func (b *cancellationBackend) SimulateExtraDebtPayment(ctx context.Context, _, _, _ int64) (server.DebtExtraPaymentSimulationResponse, error) {
	return server.DebtExtraPaymentSimulationResponse{}, ctx.Err()
}

func (b *concurrentBackend) SimulateExtraDebtPayment(context.Context, int64, int64, int64) (server.DebtExtraPaymentSimulationResponse, error) {
	return server.DebtExtraPaymentSimulationResponse{}, nil
}
