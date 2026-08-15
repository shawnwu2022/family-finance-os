package household

import "github.com/shawnwu2022/family-finance-os/pkg/money"

type MemberKind string

const (
	MemberKindAdult     MemberKind = "adult"
	MemberKindChild     MemberKind = "child"
	MemberKindDependent MemberKind = "dependent"
)

func (k MemberKind) valid() bool {
	switch k {
	case MemberKindAdult, MemberKindChild, MemberKindDependent:
		return true
	default:
		return false
	}
}

type Cadence string

const (
	CadenceMonthly   Cadence = "monthly"
	CadenceAnnual    Cadence = "annual"
	CadenceIrregular Cadence = "irregular"
)

func (c Cadence) valid() bool {
	switch c {
	case CadenceMonthly, CadenceAnnual, CadenceIrregular:
		return true
	default:
		return false
	}
}

type IncomeStability string

const (
	IncomeStabilityStable    IncomeStability = "stable"
	IncomeStabilityVariable  IncomeStability = "variable"
	IncomeStabilityIrregular IncomeStability = "irregular"
)

func (s IncomeStability) valid() bool {
	switch s {
	case IncomeStabilityStable, IncomeStabilityVariable, IncomeStabilityIrregular:
		return true
	default:
		return false
	}
}

type Household struct {
	ID           int64
	Name         string
	BaseCurrency string
	Timezone     string
}

type NewHousehold struct {
	Name         string
	BaseCurrency string
	Timezone     string
}

type Member struct {
	ID          int64
	HouseholdID int64
	Name        string
	Kind        MemberKind
	Active      bool
}

type NewMember struct {
	Name   string
	Kind   MemberKind
	Active bool
}

type IncomeSource struct {
	ID          int64
	HouseholdID int64
	MemberID    *int64
	Name        string
	Amount      money.Money
	Cadence     Cadence
	Stability   IncomeStability
	Active      bool
}

type NewIncomeSource struct {
	MemberID  *int64
	Name      string
	Amount    money.Money
	Cadence   Cadence
	Stability IncomeStability
	Active    bool
}

type ExpenseBaseline struct {
	ID          int64
	HouseholdID int64
	Name        string
	Amount      money.Money
	Cadence     Cadence
	Essential   bool
	Active      bool
}

type NewExpenseBaseline struct {
	Name      string
	Amount    money.Money
	Cadence   Cadence
	Essential bool
	Active    bool
}

type HouseholdPolicy struct {
	HouseholdID    int64
	LiquidityFloor money.Money
}

type NewHouseholdPolicy struct {
	LiquidityFloor money.Money
}

type Profile struct {
	Household        Household
	Members          []Member
	IncomeSources    []IncomeSource
	ExpenseBaselines []ExpenseBaseline
	Policy           HouseholdPolicy
}
