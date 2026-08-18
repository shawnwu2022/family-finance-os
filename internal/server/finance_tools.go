package server

import "time"

type SafeToSpendComponentsResponse struct {
	LiquidDiscretionaryPool        MoneyDTO `json:"liquid_discretionary_pool"`
	UpcomingMandatoryExpenses      MoneyDTO `json:"upcoming_mandatory_expenses"`
	DebtCommitments                MoneyDTO `json:"debt_commitments"`
	EssentialReserveUntilPeriodEnd MoneyDTO `json:"essential_reserve_until_period_end"`
	EmergencyFundGapReserved       MoneyDTO `json:"emergency_fund_gap_reserved"`
	HardGoalContributions          MoneyDTO `json:"hard_goal_contributions"`
}

type SafeToSpendResponse struct {
	DataAsOf   time.Time                     `json:"data_as_of"`
	Quality    string                        `json:"quality"`
	Period     string                        `json:"period"`
	Amount     MoneyDTO                      `json:"amount"`
	IsDeficit  bool                          `json:"is_deficit"`
	Components SafeToSpendComponentsResponse `json:"components"`
	Warnings   []string                      `json:"warnings,omitempty"`
}
