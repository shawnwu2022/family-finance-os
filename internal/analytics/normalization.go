package analytics

import (
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type CashflowEventType uint8

const (
	CashflowEventUnknown CashflowEventType = iota
	CashflowEventIncome
	CashflowEventExpense
	CashflowEventRefund
	CashflowEventTransfer
	CashflowEventCreditCardRepayment
	CashflowEventBalanceAdjustment
)

type CashflowEvent struct {
	TransactionID        string
	Type                 CashflowEventType
	Amount               money.Money
	OccurredAt           time.Time
	CategoryID           string
	SourceAccountID      string
	DestinationAccountID string
}

func NormalizeTransaction(tx ledger.Transaction, accounts map[string]ledger.Account) CashflowEvent {
	event := CashflowEvent{
		TransactionID:        tx.ID,
		Type:                 CashflowEventUnknown,
		Amount:               tx.SourceAmount,
		OccurredAt:           tx.OccurredAt,
		CategoryID:           tx.CategoryID,
		SourceAccountID:      tx.SourceAccountID,
		DestinationAccountID: tx.DestinationAccountID,
	}

	switch tx.Type {
	case ledger.TransactionTypeIncome:
		event.Type = CashflowEventIncome
	case ledger.TransactionTypeExpense:
		if tx.SourceAmount.Minor < 0 {
			event.Type = CashflowEventRefund
		} else {
			event.Type = CashflowEventExpense
		}
	case ledger.TransactionTypeTransfer:
		if isCreditCardRepayment(tx, accounts) {
			event.Type = CashflowEventCreditCardRepayment
		} else {
			event.Type = CashflowEventTransfer
		}
	case ledger.TransactionTypeBalanceModification:
		event.Type = CashflowEventBalanceAdjustment
	default:
		event.Type = CashflowEventUnknown
	}

	return event
}

func isCreditCardRepayment(tx ledger.Transaction, accounts map[string]ledger.Account) bool {
	source, sourceOK := accounts[tx.SourceAccountID]
	destination, destinationOK := accounts[tx.DestinationAccountID]
	if !sourceOK || !destinationOK {
		return false
	}
	return source.IsAsset &&
		destination.Category == ledger.AccountCategoryCreditCard &&
		destination.IsLiability
}
