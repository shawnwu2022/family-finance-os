package server

import "time"

type AssetAllocationItemResponse struct {
	Class string   `json:"class"`
	Value MoneyDTO `json:"value"`
	Share string   `json:"share"`
}

type AssetAllocationResponse struct {
	DataAsOf time.Time                     `json:"data_as_of"`
	Quality  string                        `json:"quality"`
	Currency string                        `json:"currency"`
	Total    MoneyDTO                      `json:"total"`
	Items    []AssetAllocationItemResponse `json:"items"`
	Warnings []string                      `json:"warnings,omitempty"`
}
