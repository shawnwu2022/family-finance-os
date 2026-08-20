package agentadapter

import (
	"context"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func (f *fakeBackend) AssetAllocation(context.Context, int64) (server.AssetAllocationResponse, error) {
	return server.AssetAllocationResponse{}, f.err
}

func (b *cancellationBackend) AssetAllocation(ctx context.Context, _ int64) (server.AssetAllocationResponse, error) {
	return server.AssetAllocationResponse{}, ctx.Err()
}

func (b *concurrentBackend) AssetAllocation(context.Context, int64) (server.AssetAllocationResponse, error) {
	return server.AssetAllocationResponse{}, nil
}
