package requestscope

import (
	"context"
	"strings"
)

type householdIDKey struct{}
type userIDKey struct{}
type roleKey struct{}

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

func WithRole(ctx context.Context, role string) context.Context {
	role = strings.TrimSpace(role)
	if role == "" {
		return ctx
	}
	return context.WithValue(ctx, roleKey{}, role)
}

func Role(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(roleKey{}).(string)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}
