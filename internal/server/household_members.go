package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	financeauth "github.com/shawnwu2022/family-finance-os/internal/auth"
	"github.com/shawnwu2022/family-finance-os/internal/requestscope"
)

type HouseholdMemberAuth interface {
	ListHouseholdMembers(context.Context, int64) ([]financeauth.HouseholdMember, error)
	CreateHouseholdMember(context.Context, int64, financeauth.CreateHouseholdMemberInput) (financeauth.HouseholdMember, error)
	UpdateHouseholdMemberRole(context.Context, int64, int64, financeauth.Role, time.Time) (financeauth.HouseholdMember, error)
	DisableHouseholdMember(context.Context, int64, int64, time.Time) (financeauth.HouseholdMember, error)
}

type householdMemberResponse struct {
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	Disabled     bool   `json:"disabled"`
	TOTPEnrolled bool   `json:"totp_enrolled"`
}

type createHouseholdMemberRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type updateHouseholdMemberRequest struct {
	Role string `json:"role"`
}

func registerHouseholdMembers(mux *http.ServeMux, auth BrowserAuth) {
	memberAuth, ok := auth.(HouseholdMemberAuth)
	if mux == nil || !ok {
		return
	}

	listOrCreate := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireOwnerRole(w, r.Context()) {
			return
		}
		householdID, ok := requestscope.HouseholdID(r.Context())
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		switch r.Method {
		case http.MethodGet:
			members, err := memberAuth.ListHouseholdMembers(r.Context(), householdID)
			if err != nil {
				writeHouseholdMemberError(w, err)
				return
			}
			response := make([]householdMemberResponse, 0, len(members))
			for _, member := range members {
				response = append(response, householdMemberDTO(member))
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": response})
		case http.MethodPost:
			var request createHouseholdMemberRequest
			if !decodeStrictJSON(w, r, &request) {
				return
			}
			role, err := financeauth.ParseRole(request.Role)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
				return
			}
			member, err := memberAuth.CreateHouseholdMember(r.Context(), householdID, financeauth.CreateHouseholdMemberInput{
				Username: request.Username,
				Password: request.Password,
				Role:     role,
			})
			if err != nil {
				writeHouseholdMemberError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, householdMemberDTO(member))
		default:
			w.Header().Set("Allow", "GET, POST")
			writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})
	mux.Handle("/api/v1/household/members", requireBrowserAuth(auth, listOrCreate))

	memberMutation := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireOwnerRole(w, r.Context()) {
			return
		}
		householdID, ok := requestscope.HouseholdID(r.Context())
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		userID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("user_id")), 10, 64)
		if err != nil || userID <= 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
			return
		}
		switch r.Method {
		case http.MethodPatch:
			var request updateHouseholdMemberRequest
			if !decodeStrictJSON(w, r, &request) {
				return
			}
			role, err := financeauth.ParseRole(request.Role)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
				return
			}
			member, err := memberAuth.UpdateHouseholdMemberRole(r.Context(), householdID, userID, role, time.Now().UTC())
			if err != nil {
				writeHouseholdMemberError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, householdMemberDTO(member))
		case http.MethodDelete:
			if _, err := memberAuth.DisableHouseholdMember(r.Context(), householdID, userID, time.Now().UTC()); err != nil {
				writeHouseholdMemberError(w, err)
				return
			}
			writeNoContent(w)
		default:
			w.Header().Set("Allow", "PATCH, DELETE")
			writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})
	mux.Handle("/api/v1/household/members/{user_id}", requireBrowserAuth(auth, memberMutation))
}

func requireOwnerRole(w http.ResponseWriter, ctx context.Context) bool {
	raw, ok := requestscope.Role(ctx)
	if !ok {
		writeAPIError(w, http.StatusForbidden, "insufficient_role", "insufficient role")
		return false
	}
	role, err := financeauth.ParseRole(raw)
	if err != nil || !role.IsOwner() {
		writeAPIError(w, http.StatusForbidden, "insufficient_role", "insufficient role")
		return false
	}
	return true
}

func householdMemberDTO(member financeauth.HouseholdMember) householdMemberResponse {
	return householdMemberResponse{
		UserID:       member.UserID,
		Username:     member.Username,
		Role:         string(member.Role),
		Disabled:     member.Disabled,
		TOTPEnrolled: member.TOTPEnrolled,
	}
}

func writeHouseholdMemberError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, financeauth.ErrInvalidMemberInput), errors.Is(err, financeauth.ErrInvalidRole):
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
	case errors.Is(err, financeauth.ErrUsernameExists):
		writeAPIError(w, http.StatusConflict, "username_exists", "username already exists")
	case errors.Is(err, financeauth.ErrLastOwner):
		writeAPIError(w, http.StatusConflict, "last_owner_required", "household must retain an active owner")
	case errors.Is(err, financeauth.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "member not found")
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
