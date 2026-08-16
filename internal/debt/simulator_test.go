package debt

import (
	"errors"
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestSimulateAnnuityGolden(t *testing.T) {
	debt := DebtContract{
		ID:                         1,
		Name:                       "三期等额本息",
		OriginalPrincipal:          money.Money{Minor: 300_000, Currency: "CNY"},
		Balance:                    money.Money{Minor: 300_000, Currency: "CNY"},
		APR:                        apd.New(12, -2),
		RateType:                   DebtRateFixed,
		TermRemainingMonths:        3,
		DueDay:                     15,
		RepaymentType:              DebtRepaymentAnnuity,
		MinimumPayment:             money.Money{Currency: "CNY"},
		ScheduledPayment:           money.Money{Currency: "CNY"},
		PrepaymentFeeRate:          apd.New(0, 0),
		PrepaymentRestrictedMonths: 0,
		Active:                     true,
	}

	got, err := SimulateDebt(debt, money.Money{Currency: "CNY"})
	if err != nil {
		t.Fatalf("SimulateDebt: %v", err)
	}
	want := []DebtPayment{
		{Month: 1, OpeningBalance: money.Money{Minor: 300_000, Currency: "CNY"}, Interest: money.Money{Minor: 3_000, Currency: "CNY"}, ScheduledPayment: money.Money{Minor: 102_007, Currency: "CNY"}, ScheduledPrincipal: money.Money{Minor: 99_007, Currency: "CNY"}, ExtraPrincipal: money.Money{Currency: "CNY"}, PrepaymentFee: money.Money{Currency: "CNY"}, ClosingBalance: money.Money{Minor: 200_993, Currency: "CNY"}},
		{Month: 2, OpeningBalance: money.Money{Minor: 200_993, Currency: "CNY"}, Interest: money.Money{Minor: 2_010, Currency: "CNY"}, ScheduledPayment: money.Money{Minor: 102_007, Currency: "CNY"}, ScheduledPrincipal: money.Money{Minor: 99_997, Currency: "CNY"}, ExtraPrincipal: money.Money{Currency: "CNY"}, PrepaymentFee: money.Money{Currency: "CNY"}, ClosingBalance: money.Money{Minor: 100_996, Currency: "CNY"}},
		{Month: 3, OpeningBalance: money.Money{Minor: 100_996, Currency: "CNY"}, Interest: money.Money{Minor: 1_010, Currency: "CNY"}, ScheduledPayment: money.Money{Minor: 102_006, Currency: "CNY"}, ScheduledPrincipal: money.Money{Minor: 100_996, Currency: "CNY"}, ExtraPrincipal: money.Money{Currency: "CNY"}, PrepaymentFee: money.Money{Currency: "CNY"}, ClosingBalance: money.Money{Currency: "CNY"}},
	}
	if len(got.Payments) != len(want) {
		t.Fatalf("payments=%d want %d", len(got.Payments), len(want))
	}
	for i := range want {
		if got.Payments[i] != want[i] {
			t.Fatalf("payment[%d]=%#v want %#v", i, got.Payments[i], want[i])
		}
	}
	if got.TotalInterest.Minor != 6_020 || got.PayoffMonths != 3 {
		t.Fatalf("result=%#v", got)
	}
}

func TestSimulateEqualPrincipalGolden(t *testing.T) {
	debt := baseDebtForTest()
	debt.RepaymentType = DebtRepaymentEqualPrincipal
	got, err := SimulateDebt(debt, money.Money{Currency: "CNY"})
	if err != nil {
		t.Fatalf("SimulateDebt: %v", err)
	}
	wantInterest := []int64{3_000, 2_000, 1_000}
	wantPayment := []int64{103_000, 102_000, 101_000}
	for i := range wantInterest {
		if got.Payments[i].Interest.Minor != wantInterest[i] || got.Payments[i].ScheduledPayment.Minor != wantPayment[i] {
			t.Fatalf("payment[%d]=%#v", i, got.Payments[i])
		}
	}
	if got.TotalInterest.Minor != 6_000 || got.PayoffMonths != 3 {
		t.Fatalf("result=%#v", got)
	}
}

func TestSimulateRevolvingGolden(t *testing.T) {
	debt := baseDebtForTest()
	debt.OriginalPrincipal = money.Money{Minor: 10_000, Currency: "CNY"}
	debt.Balance = money.Money{Minor: 10_000, Currency: "CNY"}
	debt.APR = apd.New(12, -2)
	debt.TermRemainingMonths = 0
	debt.RepaymentType = DebtRepaymentRevolving
	debt.Revolving = true
	debt.MinimumPayment = money.Money{Minor: 4_000, Currency: "CNY"}

	got, err := SimulateDebt(debt, money.Money{Currency: "CNY"})
	if err != nil {
		t.Fatalf("SimulateDebt: %v", err)
	}
	want := [][4]int64{
		{100, 4_000, 3_900, 6_100},
		{61, 4_000, 3_939, 2_161},
		{22, 2_183, 2_161, 0},
	}
	if len(got.Payments) != len(want) {
		t.Fatalf("payments=%d want %d", len(got.Payments), len(want))
	}
	for i, row := range want {
		p := got.Payments[i]
		if p.Interest.Minor != row[0] || p.ScheduledPayment.Minor != row[1] || p.ScheduledPrincipal.Minor != row[2] || p.ClosingBalance.Minor != row[3] {
			t.Fatalf("payment[%d]=%#v", i, p)
		}
	}
	if got.TotalInterest.Minor != 183 || got.PayoffMonths != 3 {
		t.Fatalf("result=%#v", got)
	}
}

func TestBuildPayoffPlanAvalancheAndSnowballRespectLiquidityFloor(t *testing.T) {
	a := baseDebtForTest()
	a.ID = 1
	a.Balance = money.Money{Minor: 100_000, Currency: "CNY"}
	a.APR = apd.New(20, -2)
	b := baseDebtForTest()
	b.ID = 2
	b.Balance = money.Money{Minor: 50_000, Currency: "CNY"}
	b.APR = apd.New(10, -2)

	liquid := money.Money{Minor: 80_000, Currency: "CNY"}
	floor := money.Money{Minor: 30_000, Currency: "CNY"}

	avalanche, err := BuildPayoffPlan([]DebtContract{a, b}, liquid, floor, PayoffStrategyAvalanche)
	if err != nil {
		t.Fatalf("BuildPayoffPlan avalanche: %v", err)
	}
	if avalanche.AvailableExtra.Minor != 50_000 || len(avalanche.Allocations) != 1 || avalanche.Allocations[0].DebtID != 1 || avalanche.Allocations[0].Amount.Minor != 50_000 {
		t.Fatalf("avalanche=%#v", avalanche)
	}

	snowball, err := BuildPayoffPlan([]DebtContract{a, b}, liquid, floor, PayoffStrategySnowball)
	if err != nil {
		t.Fatalf("BuildPayoffPlan snowball: %v", err)
	}
	if len(snowball.Allocations) != 1 || snowball.Allocations[0].DebtID != 2 || snowball.Allocations[0].Amount.Minor != 50_000 {
		t.Fatalf("snowball=%#v", snowball)
	}

	noExtra, err := BuildPayoffPlan([]DebtContract{a, b}, money.Money{Minor: 20_000, Currency: "CNY"}, floor, PayoffStrategyAvalanche)
	if err != nil {
		t.Fatalf("BuildPayoffPlan below floor: %v", err)
	}
	if noExtra.AvailableExtra.Minor != 0 || noExtra.LiquidityShortfall.Minor != 10_000 || len(noExtra.Allocations) != 0 {
		t.Fatalf("below floor=%#v", noExtra)
	}
}

func TestBuildAvalancheRequiresAPR(t *testing.T) {
	debt := baseDebtForTest()
	debt.APR = nil
	_, err := BuildPayoffPlan([]DebtContract{debt}, money.Money{Minor: 100_000, Currency: "CNY"}, money.Money{Currency: "CNY"}, PayoffStrategyAvalanche)
	if !errors.Is(err, ErrAPRRequired) {
		t.Fatalf("error=%v want ErrAPRRequired", err)
	}
}

func baseDebtForTest() DebtContract {
	return DebtContract{
		ID:                         1,
		Name:                       "测试债务",
		OriginalPrincipal:          money.Money{Minor: 300_000, Currency: "CNY"},
		Balance:                    money.Money{Minor: 300_000, Currency: "CNY"},
		APR:                        apd.New(12, -2),
		RateType:                   DebtRateFixed,
		TermRemainingMonths:        3,
		DueDay:                     15,
		RepaymentType:              DebtRepaymentAnnuity,
		MinimumPayment:             money.Money{Currency: "CNY"},
		ScheduledPayment:           money.Money{Currency: "CNY"},
		PrepaymentFeeRate:          apd.New(0, 0),
		PrepaymentRestrictedMonths: 0,
		Active:                     true,
	}
}
