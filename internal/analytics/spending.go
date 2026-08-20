package analytics

import (
	"fmt"
	"sort"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type SpendingCategoryTotal struct {
	CategoryID       string
	Amount           money.Money
	TransactionCount int
}

type SpendingResult struct {
	Total            money.Money
	TransactionCount int
	Categories       []SpendingCategoryTotal
}

func AggregateSpending(events []CashflowEvent, currency string) (SpendingResult, error) {
	total := money.Money{Currency: currency}
	amountByCategory := make(map[string]money.Money)
	countByCategory := make(map[string]int)
	transactionCount := 0

	for _, event := range events {
		if event.Type != CashflowEventExpense && event.Type != CashflowEventRefund {
			continue
		}
		var err error
		total, err = total.Add(event.Amount)
		if err != nil {
			return SpendingResult{}, fmt.Errorf("aggregate spending total: %w", err)
		}
		current := amountByCategory[event.CategoryID]
		if current.Currency == "" {
			current.Currency = currency
		}
		current, err = current.Add(event.Amount)
		if err != nil {
			return SpendingResult{}, fmt.Errorf("aggregate spending category %q: %w", event.CategoryID, err)
		}
		amountByCategory[event.CategoryID] = current
		countByCategory[event.CategoryID]++
		transactionCount++
	}

	categoryIDs := make([]string, 0, len(amountByCategory))
	for categoryID := range amountByCategory {
		categoryIDs = append(categoryIDs, categoryID)
	}
	sort.Strings(categoryIDs)

	categories := make([]SpendingCategoryTotal, 0, len(categoryIDs))
	for _, categoryID := range categoryIDs {
		categories = append(categories, SpendingCategoryTotal{
			CategoryID:       categoryID,
			Amount:           amountByCategory[categoryID],
			TransactionCount: countByCategory[categoryID],
		})
	}

	return SpendingResult{
		Total:            total,
		TransactionCount: transactionCount,
		Categories:       categories,
	}, nil
}
