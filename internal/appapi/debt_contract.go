package appapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shawnwu2022/family-finance-os/internal/debt"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var ErrDebtNotFound = errors.New("debt not found")

func (p *PostgresPlanner) DebtContract(ctx context.Context, householdID, debtID int64) (debt.DebtContract, error) {
	if p == nil || p.queries == nil || householdID <= 0 || debtID <= 0 {
		return debt.DebtContract{}, ErrDebtNotFound
	}
	row, err := p.queries.GetDebt(ctx, debtID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return debt.DebtContract{}, ErrDebtNotFound
		}
		return debt.DebtContract{}, fmt.Errorf("get debt contract: %w", err)
	}
	if row.HouseholdID != householdID || !row.Active {
		return debt.DebtContract{}, ErrDebtNotFound
	}

	apr, err := decimalFromNumeric(row.Apr)
	if err != nil {
		return debt.DebtContract{}, fmt.Errorf("decode debt %d APR: %w", row.ID, err)
	}
	prepaymentFeeRate, err := decimalFromNumeric(row.PrepaymentFeeRate)
	if err != nil {
		return debt.DebtContract{}, fmt.Errorf("decode debt %d prepayment fee rate: %w", row.ID, err)
	}

	return debt.DebtContract{
		ID:                         row.ID,
		Name:                       row.Name,
		OriginalPrincipal:          money.Money{Minor: row.OriginalPrincipalMinor, Currency: row.Currency},
		Balance:                    money.Money{Minor: row.BalanceMinor, Currency: row.Currency},
		APR:                        apr,
		RateType:                   debt.DebtRateType(row.RateType),
		TermRemainingMonths:        int(row.TermRemainingMonths),
		DueDay:                     int(row.DueDay),
		RepaymentType:              debt.DebtRepaymentType(row.RepaymentType),
		MinimumPayment:             money.Money{Minor: row.MinimumPaymentMinor, Currency: row.Currency},
		ScheduledPayment:           money.Money{Minor: row.ScheduledPaymentMinor, Currency: row.Currency},
		PrepaymentFeeRate:          prepaymentFeeRate,
		PrepaymentRestrictedMonths: int(row.PrepaymentRestrictedMonths),
		Revolving:                  row.Revolving,
		Active:                     true,
	}, nil
}

func decimalFromNumeric(value pgtype.Numeric) (*apd.Decimal, error) {
	raw, err := numericString(value)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	decimal, _, err := apd.NewFromString(raw)
	if err != nil {
		return nil, err
	}
	return decimal, nil
}
