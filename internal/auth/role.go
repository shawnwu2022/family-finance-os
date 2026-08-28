package auth

import (
	"errors"
	"strings"
)

type Role string

const (
	RoleOwner  Role = "owner"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

var ErrInvalidRole = errors.New("invalid household role")

func ParseRole(raw string) (Role, error) {
	role := Role(strings.ToLower(strings.TrimSpace(raw)))
	switch role {
	case RoleOwner, RoleEditor, RoleViewer:
		return role, nil
	default:
		return "", ErrInvalidRole
	}
}

func (r Role) CanEditFinance() bool {
	return r == RoleOwner || r == RoleEditor
}

func (r Role) IsOwner() bool {
	return r == RoleOwner
}
