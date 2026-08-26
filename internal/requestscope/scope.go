package requestscope

import "context"

type householdIDKey struct{}
type userIDKey struct{}

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

func WithUserID(ctx context.Context, userID int64) context.Context {
	if userID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, userIDKey{}, userID)
}

func UserID(ctx context.Context) (int64, bool) {
	value, ok := ctx.Value(userIDKey{}).(int64)
	return value, ok && value > 0
}
