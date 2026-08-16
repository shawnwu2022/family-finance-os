package budget

import "github.com/shawnwu2022/family-finance-os/pkg/money"

type BudgetKind string

const (
	BudgetKindEssential  BudgetKind = "essential"
	BudgetKindFlexible   BudgetKind = "flexible"
	BudgetKindDebt       BudgetKind = "debt"
	BudgetKindSaving     BudgetKind = "saving"
	BudgetKindInvestment BudgetKind = "investment"
	BudgetKindGoal       BudgetKind = "goal"
)

type BudgetPlan struct {
	ID          int64
	HouseholdID int64
	Period      string
	Currency    string
	Lines       []BudgetLine
}

type NewBudgetPlan struct {
	HouseholdID int64
	Period      string
	Currency    string
}

type BudgetLine struct {
	ID                  int64
	BudgetPlanID        int64
	ExternalCategoryRef string
	SemanticGroup       string
	Planned             money.Money
	Kind                BudgetKind
}

type NewBudgetLine struct {
	ExternalCategoryRef string
	SemanticGroup       string
	Planned             money.Money
	Kind                BudgetKind
}
