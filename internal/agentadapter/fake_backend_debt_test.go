package agentadapter

import (
	"context"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func (f *fakeBackend) SimulateExtraDebtPayment(context.Context, int64, int64, int64) (server.DebtExtraPaymentSimulationResponse, error) {
	return server.DebtExtraPaymentSimulationResponse{}, f.err
}
