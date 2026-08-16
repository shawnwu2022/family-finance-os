package analytics

import (
	"fmt"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type CashflowResult struct {
	RecognizedIncome  money.Money
	RecognizedExpense money.Money
	NetCashflow       money.Money
}

func CalculateCashflow(events []CashflowEvent, currency string) (CashflowResult, error) {
	income := money.Money{Currency: currency}
	expense := money.Money{Currency: currency}

	for _, event := range events {
		var err error
		switch event.Type {
		case CashflowEventIncome:
			income, err = income.Add(event.Amount)
		case CashflowEventExpense, CashflowEventRefund:
			expense, err = expense.Add(event.Amount)
		default:
			continue
		}
		if err != nil {
			return CashflowResult{}, fmt.Errorf("calculate cashflow: %w", err)
		}
	}

	net, err := income.Sub(expense)
	if err != nil {
		return CashflowResult{}, fmt.Errorf("calculate net cashflow: %w", err)
	}
	return CashflowResult{
		RecognizedIncome:  income,
		RecognizedExpense: expense,
		NetCashflow:       net,
	}, nil
}
