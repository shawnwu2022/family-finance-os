package server

import "time"

type DataQualityIssueResponse struct {
	Kind          string `json:"kind"`
	TransactionID string `json:"transaction_id"`
	Reference     string `json:"reference,omitempty"`
}

type DuplicateCandidateResponse struct {
	TransactionIDs  []string  `json:"transaction_ids"`
	Type            string    `json:"type"`
	Amount          MoneyDTO  `json:"amount"`
	FirstOccurredAt time.Time `json:"first_occurred_at"`
	LastOccurredAt  time.Time `json:"last_occurred_at"`
}

type DataQualityResponse struct {
	Period              string                       `json:"period"`
	Quality             string                       `json:"quality"`
	CheckedTransactions int                          `json:"checked_transactions"`
	IssueCount          int                          `json:"issue_count"`
	DuplicateGroupCount int                          `json:"duplicate_group_count"`
	Issues              []DataQualityIssueResponse   `json:"issues"`
	DuplicateCandidates []DuplicateCandidateResponse `json:"duplicate_candidates"`
}
