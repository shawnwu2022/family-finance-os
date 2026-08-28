package server

import "net/http"

// newFinanceAPIUnitHandler tests the raw API/request-security contract without
// going through the public NewHandler authentication boundary. Public handler
// tests must use NewHandler and prove fail-closed browser authentication.
func newFinanceAPIUnitHandler(api FinanceAPI) http.Handler {
	mux := http.NewServeMux()
	registerFinanceAPI(mux, api)
	registerPortfolioFinanceAPI(mux, api)
	return applicationSecurityHeaders(secureAPIRequests(mux))
}
