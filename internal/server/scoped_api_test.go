package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shawnwu2022/family-finance-os/internal/requestscope"
)

func TestWithScopedAPIBindsAdvisorHouseholdInContext(t *testing.T) {
	capture := &scopedCaptureAPI{}
	handler := newFinanceAPIUnitHandler(scopedFinanceAPI{next: capture})

	body := `{"household_id":77,"question":"现在的现金流安全吗？","require_tool":true}`
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/v1/advisor", strings.NewReader(body)))

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if capture.householdID != 77 {
		t.Fatalf("context household id=%d want 77", capture.householdID)
	}
}

type scopedCaptureAPI struct {
	fakeFinanceAPI
	householdID int64
}

func (s *scopedCaptureAPI) Advisor(ctx context.Context, request AdvisorRequest) (AdvisorResponse, error) {
	if householdID, ok := requestscope.HouseholdID(ctx); ok {
		s.householdID = householdID
	}
	return AdvisorResponse{Text: "scoped"}, nil
}
