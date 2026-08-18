package mcpadapter

import (
	"context"

	appserver "github.com/shawnwu2022/family-finance-os/internal/server"
)

func (*stubFinanceBackend) SimulateExtraDebtPayment(context.Context, int64, int64, int64) (appserver.DebtExtraPaymentSimulationResponse, error) {
	return appserver.DebtExtraPaymentSimulationResponse{}, nil
}
