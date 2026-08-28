package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidMemberInput = errors.New("invalid household member input")

type CreateHouseholdMemberInput struct {
	Username string
	Password string
	Role     Role
}

func (s *Service) ListHouseholdMembers(ctx context.Context, householdID int64) ([]HouseholdMember, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("auth service is required")
	}
	return s.store.ListHouseholdMembers(ctx, householdID)
}

func (s *Service) CreateHouseholdMember(ctx context.Context, householdID int64, input CreateHouseholdMemberInput) (HouseholdMember, error) {
	if s == nil || s.store == nil {
		return HouseholdMember{}, errors.New("auth service is required")
	}
	username := strings.TrimSpace(input.Username)
	normalized := normalizeUsername(username)
	if householdID <= 0 || normalized == "" || len([]byte(normalized)) > 128 {
		return HouseholdMember{}, ErrInvalidMemberInput
	}
	role, err := ParseRole(string(input.Role))
	if err != nil {
		return HouseholdMember{}, ErrInvalidMemberInput
	}
	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return HouseholdMember{}, fmt.Errorf("%w: invalid password: %v", ErrInvalidMemberInput, err)
	}
	return s.store.CreateHouseholdMember(ctx, householdID, username, normalized, passwordHash, role)
}

func (s *Service) UpdateHouseholdMemberRole(ctx context.Context, householdID, userID int64, role Role, now time.Time) (HouseholdMember, error) {
	if s == nil || s.store == nil {
		return HouseholdMember{}, errors.New("auth service is required")
	}
	parsed, err := ParseRole(string(role))
	if err != nil {
		return HouseholdMember{}, ErrInvalidMemberInput
	}
	return s.store.UpdateHouseholdMemberRole(ctx, householdID, userID, parsed, now)
}

func (s *Service) DisableHouseholdMember(ctx context.Context, householdID, userID int64, now time.Time) (HouseholdMember, error) {
	if s == nil || s.store == nil {
		return HouseholdMember{}, errors.New("auth service is required")
	}
	return s.store.DisableHouseholdMember(ctx, householdID, userID, now)
}
