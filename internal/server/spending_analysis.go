package server

import "time"

type SpendingCategoryResponse struct {
	CategoryRef      string   `json:"category_ref"`
	Name             string   `json:"name,omitempty"`
	Amount           MoneyDTO `json:"amount"`
	TransactionCount int      `json:"transaction_count"`
}

type SpendingPeriodResponse struct {
	Period           string                     `json:"period"`
	Total            MoneyDTO                   `json:"total"`
	TransactionCount int                        `json:"transaction_count"`
	Categories       []SpendingCategoryResponse `json:"categories"`
}

type SpendingAnalysisResponse struct {
	DataAsOf    time.Time                `json:"data_as_of"`
	Quality     string                   `json:"quality"`
	Currency    string                   `json:"currency"`
	Current     SpendingPeriodResponse   `json:"current"`
	Comparisons []SpendingPeriodResponse `json:"comparisons"`
	Warnings    []string                 `json:"warnings,omitempty"`
}
