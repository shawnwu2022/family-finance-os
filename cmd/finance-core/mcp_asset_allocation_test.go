package main

import (
	"context"

	appserver "github.com/shawnwu2022/family-finance-os/internal/server"
)

func (*mcpIntegrationBackend) AssetAllocation(context.Context, int64) (appserver.AssetAllocationResponse, error) {
	return appserver.AssetAllocationResponse{}, nil
}
