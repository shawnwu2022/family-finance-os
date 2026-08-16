package debt

import (
	"fmt"
	"sort"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func BuildPayoffPlan(debts []DebtContract, liquid, floor money.Money, strategy PayoffStrategy) (PayoffPlan, error) {
	if liquid.Currency != floor.Currency {
		return PayoffPlan{}, money.ErrCurrencyMismatch
	}
	if liquid.Minor < 0 || floor.Minor < 0 {
		return PayoffPlan{}, ErrInvalidDebt
	}
	if strategy != PayoffStrategyAvalanche && strategy != PayoffStrategySnowball {
		return PayoffPlan{}, ErrInvalidStrategy
	}

	currency := liquid.Currency
	plan := PayoffPlan{
		Strategy:           strategy,
		AvailableExtra:     money.Money{Currency: currency},
		LiquidityShortfall: money.Money{Currency: currency},
		Allocations:        []DebtExtraAllocation{},
	}

	if liquid.Minor <= floor.Minor {
		if liquid.Minor < floor.Minor {
			plan.LiquidityShortfall = money.Money{Minor: floor.Minor - liquid.Minor, Currency: currency}
		}
		return plan, nil
	}
	plan.AvailableExtra = money.Money{Minor: liquid.Minor - floor.Minor, Currency: currency}

	candidates := make([]DebtContract, 0, len(debts))
	for _, debt := range debts {
		if !debt.Active || debt.Balance.Minor <= 0 {
			continue
		}
		if debt.Balance.Currency != currency {
			return PayoffPlan{}, money.ErrCurrencyMismatch
		}
		if strategy == PayoffStrategyAvalanche {
			if debt.APR == nil {
				return PayoffPlan{}, ErrAPRRequired
			}
			if debt.APR.Cmp(apd.New(0, 0)) < 0 {
				return PayoffPlan{}, ErrInvalidDebt
			}
		}
		candidates = append(candidates, debt)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if strategy == PayoffStrategyAvalanche {
			cmp := a.APR.Cmp(b.APR)
			if cmp != 0 {
				return cmp > 0
			}
		} else if a.Balance.Minor != b.Balance.Minor {
			return a.Balance.Minor < b.Balance.Minor
		}
		return a.ID < b.ID
	})

	remaining := plan.AvailableExtra.Minor
	for _, debt := range candidates {
		if remaining == 0 {
			break
		}
		amount := minInt64(remaining, debt.Balance.Minor)
		if amount <= 0 {
			continue
		}
		plan.Allocations = append(plan.Allocations, DebtExtraAllocation{
			DebtID: debt.ID,
			Amount: money.Money{Minor: amount, Currency: currency},
		})
		remaining -= amount
	}
	if remaining < 0 {
		return PayoffPlan{}, fmt.Errorf("%w: negative allocation remainder", ErrInvalidDebt)
	}
	return plan, nil
}
