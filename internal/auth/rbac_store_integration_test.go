package auth

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/store"
)

func TestHouseholdRBACLastOwnerInvariantIntegration(t *testing.T) {
	cfg := authIntegrationDatabaseConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := store.OpenPostgres(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	defer pool.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	var householdID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO households (name, base_currency, timezone)
		VALUES ($1, 'CNY', 'Asia/Shanghai')
		RETURNING id
	`, "rbac-integration-"+suffix).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}

	authStore := NewPostgresStore(pool)
	owner, created, err := authStore.CreateOrGetAdminUser(ctx, CreateAdminUserParams{
		Username:           "Owner-" + suffix,
		NormalizedUsername: "owner-" + suffix,
		PasswordHash:       "$argon2id$test",
		HouseholdID:        householdID,
	})
	if err != nil || !created {
		t.Fatalf("CreateOrGetAdminUser: created=%v err=%v", created, err)
	}
	role, err := authStore.GetUserRole(ctx, owner.ID)
	if err != nil || role != RoleOwner {
		t.Fatalf("initial owner role=%q err=%v", role, err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	if _, err := authStore.UpdateHouseholdMemberRole(ctx, householdID, owner.ID, RoleEditor, now); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("demoting sole owner error=%v want ErrLastOwner", err)
	}

	secondOwner, err := authStore.CreateHouseholdMember(ctx, householdID, "Second-"+suffix, "second-"+suffix, "$argon2id$test", RoleOwner)
	if err != nil {
		t.Fatalf("CreateHouseholdMember second owner: %v", err)
	}
	updated, err := authStore.UpdateHouseholdMemberRole(ctx, householdID, owner.ID, RoleEditor, now.Add(time.Second))
	if err != nil || updated.Role != RoleEditor {
		t.Fatalf("demote owner with backup = %#v err=%v", updated, err)
	}
	if _, err := authStore.DisableHouseholdMember(ctx, householdID, secondOwner.UserID, now.Add(2*time.Second)); !errors.Is(err, ErrLastOwner) {
		t.Fatalf("disable last owner error=%v want ErrLastOwner", err)
	}

	updated, err = authStore.UpdateHouseholdMemberRole(ctx, householdID, owner.ID, RoleOwner, now.Add(3*time.Second))
	if err != nil || updated.Role != RoleOwner {
		t.Fatalf("restore owner = %#v err=%v", updated, err)
	}
	disabled, err := authStore.DisableHouseholdMember(ctx, householdID, secondOwner.UserID, now.Add(4*time.Second))
	if err != nil || !disabled.Disabled {
		t.Fatalf("disable redundant owner = %#v err=%v", disabled, err)
	}
	if _, err := authStore.GetUserRole(ctx, secondOwner.UserID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disabled member role lookup error=%v want ErrNotFound", err)
	}
}
