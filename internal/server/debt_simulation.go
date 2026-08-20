package server

import "time"

type DebtExtraPaymentSimulationResponse struct {
	DataAsOf              time.Time `json:"data_as_of"`
	Quality               string    `json:"quality"`
	DebtID                int64     `json:"debt_id"`
	RepaymentAssumption   string    `json:"repayment_assumption"`
	RequestedExtra        MoneyDTO  `json:"requested_extra"`
	AppliedExtra          MoneyDTO  `json:"applied_extra"`
	AppliedMonth          int       `json:"applied_month"`
	BaselinePayoffMonths  int       `json:"baseline_payoff_months"`
	SimulatedPayoffMonths int       `json:"simulated_payoff_months"`
	MonthsSaved           int       `json:"months_saved"`
	BaselineInterest      MoneyDTO  `json:"baseline_interest"`
	SimulatedInterest     MoneyDTO  `json:"simulated_interest"`
	InterestSaved         MoneyDTO  `json:"interest_saved"`
	PrepaymentFees        MoneyDTO  `json:"prepayment_fees"`
	NetSavings            MoneyDTO  `json:"net_savings"`
	Warnings              []string  `json:"warnings,omitempty"`
}
