package requestscope

import "context"

type householdIDKey struct{}

func WithHouseholdID(ctx context.Context, householdID int64) context.Context {
	if householdID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, householdIDKey{}, householdID)
}

func HouseholdID(ctx context.Context) (int64, bool) {
	value, ok := ctx.Value(householdIDKey{}).(int64)
	return value, ok && value > 0
}
