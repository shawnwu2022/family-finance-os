package appapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/internal/advisor"
	"github.com/shawnwu2022/family-finance-os/internal/analytics"
	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/llm"
	"github.com/shawnwu2022/family-finance-os/internal/scenario"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var (
	ErrInvalidDependencies = errors.New("invalid app API dependencies")
	ErrAdvisorUnavailable  = errors.New("finance advisor is not configured")
	ErrUnsupportedScenario = errors.New("scenario is not exposed by the V1 HTTP API")
)

type DebtSnapshot struct {
	ID                  int64
	Name                string
	Type                string
	Balance             money.Money
	APR                 string
	RepaymentType       string
	MinimumPayment      money.Money
	ScheduledPayment    money.Money
	TermRemainingMonths int32
	DueDay              int32
	Active              bool
}

type Planner interface {
	Profile(ctx context.Context, householdID int64) (household.Profile, error)
	BudgetPlan(ctx context.Context, householdID int64, period string) (budget.BudgetPlan, error)
	Debts(ctx context.Context, householdID int64) ([]DebtSnapshot, error)
	Goals(ctx context.Context, householdID int64) ([]goals.FinancialGoal, error)
}

type AdvisorRunner interface {
	Advise(ctx context.Context, request advisor.AdviceRequest) (advisor.AdviceResult, error)
}

type Dependencies struct {
	Ledger  ledger.Ledger
	Planner Planner
	Advisor AdvisorRunner
	Now     func() time.Time
}

type API struct {
	ledger  ledger.Ledger
	planner Planner
	advisor AdvisorRunner
	now     func() time.Time
}

func New(deps Dependencies) (*API, error) {
	if deps.Ledger == nil || deps.Planner == nil {
		return nil, ErrInvalidDependencies
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &API{ledger: deps.Ledger, planner: deps.Planner, advisor: deps.Advisor, now: deps.Now}, nil
}

func (a *API) SetAdvisor(runner AdvisorRunner) {
	a.advisor = runner
}

func (a *API) Overview(ctx context.Context, householdID int64) (server.OverviewResponse, error) {
	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return server.OverviewResponse{}, fmt.Errorf("load household profile: %w", err)
	}
	period, err := periodAt(a.now(), profile.Household.Timezone)
	if err != nil {
		return server.OverviewResponse{}, err
	}
	snapshot, err := a.snapshot(ctx, profile, period)
	if err != nil {
		return server.OverviewResponse{}, err
	}

	return server.OverviewResponse{
		DataAsOf:        snapshot.asOf,
		Quality:         qualityString(snapshot.quality),
		NetWorth:        moneyDTO(snapshot.netWorth.NetWorth),
		Income:          moneyDTO(snapshot.cashflow.RecognizedIncome),
		Expense:         moneyDTO(snapshot.cashflow.RecognizedExpense),
		NetCashflow:     moneyDTO(snapshot.cashflow.NetCashflow),
		SavingsRate:     savingsRateString(snapshot.cashflow),
		SafeToSpend:     moneyDTO(snapshot.safeToSpend.Amount),
		EmergencyMonths: decimalResultString(snapshot.emergencyMonths),
		TotalDebt:       moneyDTO(snapshot.totalDebt),
		GoalCount:       len(snapshot.goals),
		Warnings:        cloneWarnings(snapshot.warnings),
	}, nil
}

func (a *API) Cashflow(ctx context.Context, householdID int64, period string) (server.CashflowResponse, error) {
	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return server.CashflowResponse{}, fmt.Errorf("load household profile: %w", err)
	}
	if _, _, err := periodBounds(period, profile.Household.Timezone); err != nil {
		return server.CashflowResponse{}, err
	}
	snapshot, err := a.snapshot(ctx, profile, period)
	if err != nil {
		return server.CashflowResponse{}, err
	}
	return server.CashflowResponse{
		DataAsOf:    snapshot.asOf,
		Quality:     qualityString(snapshot.quality),
		Period:      period,
		Income:      moneyDTO(snapshot.cashflow.RecognizedIncome),
		Expense:     moneyDTO(snapshot.cashflow.RecognizedExpense),
		NetCashflow: moneyDTO(snapshot.cashflow.NetCashflow),
		SavingsRate: savingsRateString(snapshot.cashflow),
		Warnings:    cloneWarnings(snapshot.warnings),
	}, nil
}

func (a *API) Budget(ctx context.Context, householdID int64, period string) (server.BudgetResponse, error) {
	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return server.BudgetResponse{}, fmt.Errorf("load household profile: %w", err)
	}
	snapshot, err := a.snapshot(ctx, profile, period)
	if err != nil {
		return server.BudgetResponse{}, err
	}
	return server.BudgetResponse{
		DataAsOf: snapshot.asOf,
		Quality:  qualityString(snapshot.quality),
		Period:   period,
		Currency: profile.Household.BaseCurrency,
		Lines:    append([]server.BudgetLineResponse(nil), snapshot.budgetLines...),
		Warnings: cloneWarnings(snapshot.warnings),
	}, nil
}

func (a *API) Debts(ctx context.Context, householdID int64) (server.DebtsResponse, error) {
	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return server.DebtsResponse{}, fmt.Errorf("load household profile: %w", err)
	}
	period, err := periodAt(a.now(), profile.Household.Timezone)
	if err != nil {
		return server.DebtsResponse{}, err
	}
	snapshot, err := a.snapshot(ctx, profile, period)
	if err != nil {
		return server.DebtsResponse{}, err
	}
	items := make([]server.DebtResponse, 0, len(snapshot.debts))
	for _, debt := range snapshot.debts {
		items = append(items, server.DebtResponse{
			ID:                  debt.ID,
			Name:                debt.Name,
			Type:                debt.Type,
			Balance:             moneyDTO(debt.Balance),
			APR:                 debt.APR,
			RepaymentType:       debt.RepaymentType,
			MinimumPayment:      moneyDTO(debt.MinimumPayment),
			ScheduledPayment:    moneyDTO(debt.ScheduledPayment),
			TermRemainingMonths: debt.TermRemainingMonths,
			DueDay:              debt.DueDay,
		})
	}
	return server.DebtsResponse{
		DataAsOf: snapshot.asOf,
		Quality:  qualityString(snapshot.quality),
		Currency: profile.Household.BaseCurrency,
		Total:    moneyDTO(snapshot.totalDebt),
		Items:    items,
		Warnings: cloneWarnings(snapshot.warnings),
	}, nil
}

func (a *API) Goals(ctx context.Context, householdID int64) (server.GoalsResponse, error) {
	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return server.GoalsResponse{}, fmt.Errorf("load household profile: %w", err)
	}
	period, err := periodAt(a.now(), profile.Household.Timezone)
	if err != nil {
		return server.GoalsResponse{}, err
	}
	snapshot, err := a.snapshot(ctx, profile, period)
	if err != nil {
		return server.GoalsResponse{}, err
	}
	items, err := projectGoalDTOs(snapshot.goals, snapshot.cashflow, snapshot.asOf)
	if err != nil {
		return server.GoalsResponse{}, err
	}
	return server.GoalsResponse{
		DataAsOf: snapshot.asOf,
		Quality:  qualityString(snapshot.quality),
		Items:    items,
		Warnings: cloneWarnings(snapshot.warnings),
	}, nil
}

func (a *API) Scenario(ctx context.Context, request server.ScenarioRequest) (server.ScenarioResponse, error) {
	if request.Kind != "purchase" {
		return server.ScenarioResponse{}, ErrUnsupportedScenario
	}
	var input purchaseRequest
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return server.ScenarioResponse{}, fmt.Errorf("decode purchase scenario: %w", err)
	}
	profile, err := a.planner.Profile(ctx, request.HouseholdID)
	if err != nil {
		return server.ScenarioResponse{}, err
	}
	period, err := periodAt(a.now(), profile.Household.Timezone)
	if err != nil {
		return server.ScenarioResponse{}, err
	}
	snapshot, err := a.snapshot(ctx, profile, period)
	if err != nil {
		return server.ScenarioResponse{}, err
	}
	purchase, err := input.Money(profile.Household.BaseCurrency)
	if err != nil {
		return server.ScenarioResponse{}, err
	}
	result, err := scenario.SimulatePurchase(scenario.PurchaseInput{
		Purchase:       purchase,
		Cashflow:       snapshot.cashflow,
		SafeToSpend:    snapshot.safeInput,
		LiquidBalance:  snapshot.liquid,
		LiquidityFloor: profile.Policy.LiquidityFloor,
	})
	if err != nil {
		return server.ScenarioResponse{}, err
	}
	encoded, err := json.Marshal(purchaseResultDTO(result))
	if err != nil {
		return server.ScenarioResponse{}, err
	}
	return server.ScenarioResponse{Kind: request.Kind, Result: encoded, Warnings: cloneWarnings(snapshot.warnings)}, nil
}

func (a *API) Advisor(ctx context.Context, request server.AdvisorRequest) (server.AdvisorResponse, error) {
	if a.advisor == nil {
		return server.AdvisorResponse{}, ErrAdvisorUnavailable
	}
	profile, err := a.planner.Profile(ctx, request.HouseholdID)
	if err != nil {
		return server.AdvisorResponse{}, err
	}
	period, err := periodAt(a.now(), profile.Household.Timezone)
	if err != nil {
		return server.AdvisorResponse{}, err
	}
	snapshot, err := a.snapshot(ctx, profile, period)
	if err != nil {
		return server.AdvisorResponse{}, err
	}
	result, err := a.advisor.Advise(ctx, advisor.AdviceRequest{
		Question:      request.Question,
		Role:          llm.ModelRolePlanner,
		DataQuality:   snapshot.quality,
		RequireTool:   request.RequireTool,
		RequireReview: request.RequireReview,
	})
	if err != nil {
		return server.AdvisorResponse{}, err
	}
	return server.AdvisorResponse{
		Text:        result.Text,
		Reviewed:    result.Reviewed,
		Review:      result.Review,
		Blocked:     result.Blocked,
		BlockReason: string(result.BlockReason),
		Warnings:    cloneWarnings(snapshot.warnings),
	}, nil
}

func (a *API) Reports(context.Context, int64) (server.ReportsResponse, error) {
	return server.ReportsResponse{Items: []server.ReportSummary{}}, nil
}

type snapshot struct {
	asOf            time.Time
	profile         household.Profile
	cashflow        analytics.CashflowResult
	netWorth        analytics.NetWorthResult
	liquid          money.Money
	quality         analytics.DataQuality
	warnings        []string
	plan            budget.BudgetPlan
	budgetLines     []server.BudgetLineResponse
	debts           []DebtSnapshot
	goals           []goals.FinancialGoal
	totalDebt       money.Money
	safeInput       budget.SafeToSpendInput
	safeToSpend     budget.SafeToSpendResult
	emergencyMonths budget.DecimalResult
}

func (a *API) snapshot(ctx context.Context, profile household.Profile, period string) (snapshot, error) {
	start, end, err := periodBounds(period, profile.Household.Timezone)
	if err != nil {
		return snapshot{}, err
	}
	currency := profile.Household.BaseCurrency
	asOf := a.now().UTC()
	warnings := []string{}
	partial := false

	accounts, err := a.ledger.ListAccounts(ctx)
	if err != nil {
		return snapshot{}, fmt.Errorf("list ledger accounts: %w", err)
	}
	accountMap := make(map[string]ledger.Account, len(accounts))
	valuations := make([]analytics.Valuation, 0, len(accounts))
	liquid := money.Money{Currency: currency}
	for _, account := range accounts {
		accountMap[account.ID] = account
		if account.Hidden || account.Structure == ledger.AccountStructureMultipleSubAccounts || (!account.IsAsset && !account.IsLiability) {
			continue
		}
		if account.Balance.Currency != currency {
			partial = true
			warnings = appendWarning(warnings, fmt.Sprintf("account %s skipped: currency %s differs from household currency %s", account.ID, account.Balance.Currency, currency))
			continue
		}
		value, err := magnitude(account.Balance)
		if err != nil {
			return snapshot{}, err
		}
		kind := analytics.ValuationAsset
		if account.IsLiability {
			kind = analytics.ValuationLiability
		}
		valuations = append(valuations, analytics.Valuation{Kind: kind, Value: value})
		if account.IsAsset && isLiquidCategory(account.Category) {
			liquid, err = liquid.Add(value)
			if err != nil {
				return snapshot{}, err
			}
		}
	}
	netWorth, err := analytics.CalculateNetWorth(valuations, currency)
	if err != nil {
		return snapshot{}, err
	}

	transactions, err := a.ledger.ListTransactions(ctx, ledger.TransactionQuery{})
	if err != nil {
		return snapshot{}, fmt.Errorf("list ledger transactions: %w", err)
	}
	events := make([]analytics.CashflowEvent, 0, len(transactions))
	for _, tx := range transactions {
		if tx.OccurredAt.Before(start) || !tx.OccurredAt.Before(end) {
			continue
		}
		if tx.SourceAmount.Currency != currency {
			partial = true
			warnings = appendWarning(warnings, fmt.Sprintf("transaction %s skipped: currency %s differs from household currency %s", tx.ID, tx.SourceAmount.Currency, currency))
			continue
		}
		event := analytics.NormalizeTransaction(tx, accountMap)
		if event.Type == analytics.CashflowEventUnknown {
			partial = true
			warnings = appendWarning(warnings, fmt.Sprintf("transaction %s has unknown cashflow semantics", tx.ID))
			continue
		}
		events = append(events, event)
	}
	cashflow, err := analytics.CalculateCashflow(events, currency)
	if err != nil {
		return snapshot{}, err
	}

	plan, err := a.planner.BudgetPlan(ctx, profile.Household.ID, period)
	if err != nil {
		return snapshot{}, fmt.Errorf("load budget plan: %w", err)
	}
	budgetLines, reserves, lineWarnings, err := calculateBudgetStatus(plan, events)
	if err != nil {
		return snapshot{}, err
	}
	if len(lineWarnings) != 0 {
		partial = true
		warnings = append(warnings, lineWarnings...)
	}

	debts, err := a.planner.Debts(ctx, profile.Household.ID)
	if err != nil {
		return snapshot{}, fmt.Errorf("load debts: %w", err)
	}
	activeDebts := make([]DebtSnapshot, 0, len(debts))
	totalDebt := money.Money{Currency: currency}
	debtCommitment := money.Money{Currency: currency}
	for _, debt := range debts {
		if debt.Balance.Currency != currency || debt.MinimumPayment.Currency != currency || debt.ScheduledPayment.Currency != currency {
			partial = true
			warnings = appendWarning(warnings, fmt.Sprintf("debt %d skipped: currency differs from household currency %s", debt.ID, currency))
			continue
		}
		activeDebts = append(activeDebts, debt)
		totalDebt, err = totalDebt.Add(debt.Balance)
		if err != nil {
			return snapshot{}, err
		}
		commitment := debt.ScheduledPayment
		if commitment.Minor <= 0 {
			commitment = debt.MinimumPayment
		}
		debtCommitment, err = debtCommitment.Add(commitment)
		if err != nil {
			return snapshot{}, err
		}
	}

	goalList, err := a.planner.Goals(ctx, profile.Household.ID)
	if err != nil {
		return snapshot{}, fmt.Errorf("load goals: %w", err)
	}
	activeGoals := make([]goals.FinancialGoal, 0, len(goalList))
	hardGoalCommitment := money.Money{Currency: currency}
	for _, goal := range goalList {
		if !goal.Active {
			continue
		}
		if goal.Target.Currency != currency || goal.MonthlyContribution.Currency != currency {
			partial = true
			warnings = appendWarning(warnings, fmt.Sprintf("goal %d skipped: currency differs from household currency %s", goal.ID, currency))
			continue
		}
		activeGoals = append(activeGoals, goal)
		if goal.Flexibility == goals.GoalFlexibilityHard {
			hardGoalCommitment, err = hardGoalCommitment.Add(goal.MonthlyContribution)
			if err != nil {
				return snapshot{}, err
			}
		}
	}

	remainingDebtCommitment, err := residualCommitment(debtCommitment, reserves.debt)
	if err != nil {
		return snapshot{}, err
	}
	remainingGoalCommitment, err := residualCommitment(hardGoalCommitment, reserves.goal)
	if err != nil {
		return snapshot{}, err
	}
	upcomingMandatory, err := reserves.debt.Add(reserves.goal)
	if err != nil {
		return snapshot{}, err
	}
	upcomingMandatory, err = upcomingMandatory.Add(reserves.savingInvestment)
	if err != nil {
		return snapshot{}, err
	}
	safeInput := budget.SafeToSpendInput{
		LiquidDiscretionaryPool:        liquid,
		UpcomingMandatoryExpenses:      upcomingMandatory,
		DebtCommitments:                remainingDebtCommitment,
		EssentialReserveUntilPeriodEnd: reserves.essential,
		EmergencyFundGapReserved:       profile.Policy.LiquidityFloor,
		HardGoalContributions:          remainingGoalCommitment,
	}
	safe, err := budget.CalculateSafeToSpend(safeInput)
	if err != nil {
		return snapshot{}, err
	}
	emergency, err := budget.CalculateEmergencyMonths(liquid, reserves.plannedEssential)
	if err != nil {
		return snapshot{}, err
	}

	level := analytics.QualityGood
	if partial {
		level = analytics.QualityPartial
	}
	quality := analytics.DataQuality{
		AsOf:           asOf,
		LedgerSyncedAt: asOf,
		UnknownAmount:  money.Money{Currency: currency},
		Level:          level,
	}
	return snapshot{
		asOf:            asOf,
		profile:         profile,
		cashflow:        cashflow,
		netWorth:        netWorth,
		liquid:          liquid,
		quality:         quality,
		warnings:        warnings,
		plan:            plan,
		budgetLines:     budgetLines,
		debts:           activeDebts,
		goals:           activeGoals,
		totalDebt:       totalDebt,
		safeInput:       safeInput,
		safeToSpend:     safe,
		emergencyMonths: emergency,
	}, nil
}

type budgetReserves struct {
	essential        money.Money
	plannedEssential money.Money
	debt             money.Money
	goal             money.Money
	savingInvestment money.Money
}

func calculateBudgetStatus(plan budget.BudgetPlan, events []analytics.CashflowEvent) ([]server.BudgetLineResponse, budgetReserves, []string, error) {
	currency := plan.Currency
	reserves := budgetReserves{
		essential:        money.Money{Currency: currency},
		plannedEssential: money.Money{Currency: currency},
		debt:             money.Money{Currency: currency},
		goal:             money.Money{Currency: currency},
		savingInvestment: money.Money{Currency: currency},
	}
	actualByCategory := map[string]money.Money{}
	for _, event := range events {
		if event.Type != analytics.CashflowEventExpense && event.Type != analytics.CashflowEventRefund {
			continue
		}
		current := actualByCategory[event.CategoryID]
		if current.Currency == "" {
			current.Currency = currency
		}
		var err error
		current, err = current.Add(event.Amount)
		if err != nil {
			return nil, budgetReserves{}, nil, err
		}
		actualByCategory[event.CategoryID] = current
	}

	responses := make([]server.BudgetLineResponse, 0, len(plan.Lines))
	warnings := []string{}
	for _, line := range plan.Lines {
		actual := money.Money{Currency: currency}
		if line.ExternalCategoryRef != "" {
			actual = actualByCategory[line.ExternalCategoryRef]
			if actual.Currency == "" {
				actual.Currency = currency
			}
		} else if line.SemanticGroup != "" {
			warnings = appendWarning(warnings, fmt.Sprintf("budget semantic group %s actual is unavailable without an explicit category mapping", line.SemanticGroup))
		}
		metrics, err := budget.CalculateBudgetLine(line, actual)
		if err != nil {
			return nil, budgetReserves{}, nil, err
		}
		remaining := positiveMoney(metrics.Remaining)
		switch line.Kind {
		case budget.BudgetKindEssential:
			reserves.essential, err = reserves.essential.Add(remaining)
			if err == nil {
				reserves.plannedEssential, err = reserves.plannedEssential.Add(line.Planned)
			}
		case budget.BudgetKindDebt:
			reserves.debt, err = reserves.debt.Add(remaining)
		case budget.BudgetKindGoal:
			reserves.goal, err = reserves.goal.Add(remaining)
		case budget.BudgetKindSaving, budget.BudgetKindInvestment:
			reserves.savingInvestment, err = reserves.savingInvestment.Add(remaining)
		}
		if err != nil {
			return nil, budgetReserves{}, nil, err
		}
		responses = append(responses, server.BudgetLineResponse{
			Kind:                string(line.Kind),
			ExternalCategoryRef: line.ExternalCategoryRef,
			SemanticGroup:       line.SemanticGroup,
			Planned:             moneyDTO(metrics.Planned),
			Actual:              moneyDTO(metrics.Actual),
			Remaining:           moneyDTO(metrics.Remaining),
			Utilization:         decimalString(metrics.Utilization),
		})
	}
	return responses, reserves, warnings, nil
}

func projectGoalDTOs(goalList []goals.FinancialGoal, cashflow analytics.CashflowResult, asOf time.Time) ([]server.GoalResponse, error) {
	available := cashflow.NetCashflow
	if available.Minor < 0 {
		available.Minor = 0
	}
	items := make([]server.GoalResponse, 0, len(goalList))
	for _, goal := range goalList {
		projection, err := goals.ProjectGoal(goals.GoalProjectionInput{Goal: goal, AsOf: asOf, AvailableMonthly: available})
		if err != nil {
			return nil, err
		}
		items = append(items, server.GoalResponse{
			ID:                  goal.ID,
			Name:                goal.Name,
			Target:              moneyDTO(goal.Target),
			Funded:              moneyDTO(goal.Funded),
			TargetDate:          goal.TargetDate.Format("2006-01-02"),
			Priority:            goal.Priority,
			Flexibility:         string(goal.Flexibility),
			MonthlyContribution: moneyDTO(goal.MonthlyContribution),
			RequiredMonthly:     moneyDTO(projection.RequiredMonthly),
			Status:              string(projection.Status),
		})
	}
	return items, nil
}

func periodAt(now time.Time, timezone string) (string, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return "", fmt.Errorf("load household timezone: %w", err)
	}
	return now.In(location).Format("2006-01"), nil
}

func periodBounds(period, timezone string) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("load household timezone: %w", err)
	}
	start, err := time.ParseInLocation("2006-01", period, location)
	if err != nil || start.Format("2006-01") != period {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period %q", period)
	}
	return start, start.AddDate(0, 1, 0), nil
}

func magnitude(value money.Money) (money.Money, error) {
	if value.Minor == math.MinInt64 {
		return money.Money{}, money.ErrOverflow
	}
	if value.Minor < 0 {
		value.Minor = -value.Minor
	}
	return value, nil
}

func isLiquidCategory(category ledger.AccountCategory) bool {
	switch category {
	case ledger.AccountCategoryCash, ledger.AccountCategoryChecking, ledger.AccountCategoryVirtual, ledger.AccountCategorySavings:
		return true
	default:
		return false
	}
}

func moneyDTO(value money.Money) server.MoneyDTO {
	return server.MoneyDTO{Minor: value.Minor, Currency: value.Currency}
}

func positiveMoney(value money.Money) money.Money {
	if value.Minor < 0 {
		value.Minor = 0
	}
	return value
}

func residualCommitment(total, reserved money.Money) (money.Money, error) {
	remaining, err := total.Sub(reserved)
	if err != nil {
		return money.Money{}, err
	}
	if remaining.Minor < 0 {
		remaining.Minor = 0
	}
	return remaining, nil
}

func savingsRateString(cashflow analytics.CashflowResult) string {
	if cashflow.RecognizedIncome.Minor == 0 {
		return ""
	}
	return ratString(new(big.Rat).SetFrac(big.NewInt(cashflow.NetCashflow.Minor), big.NewInt(cashflow.RecognizedIncome.Minor)), 34)
}

func ratString(value *big.Rat, precision int) string {
	text := value.FloatString(precision)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	}
	if text == "-0" {
		return "0"
	}
	return text
}

func decimalString(value *apd.Decimal) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func decimalResultString(result budget.DecimalResult) string {
	if !result.Applicable || result.Value == nil {
		return ""
	}
	return result.Value.String()
}

func qualityString(value analytics.DataQuality) string {
	switch value.Level {
	case analytics.QualityGood:
		return "good"
	case analytics.QualityPartial:
		return "partial"
	case analytics.QualityStale:
		return "stale"
	default:
		return "unknown"
	}
}

func appendWarning(warnings []string, warning string) []string {
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return warnings
	}
	for _, existing := range warnings {
		if existing == warning {
			return warnings
		}
	}
	return append(warnings, warning)
}

func cloneWarnings(values []string) []string {
	return append([]string(nil), values...)
}

type minorInput int64

func (m *minorInput) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	parsed := new(big.Int)
	if _, ok := parsed.SetString(text, 10); !ok || !parsed.IsInt64() {
		return errors.New("invalid minor amount")
	}
	*m = minorInput(parsed.Int64())
	return nil
}

type purchaseRequest struct {
	AmountMinor minorInput `json:"amount_minor"`
	Currency    string     `json:"currency"`
}

func (p purchaseRequest) Money(baseCurrency string) (money.Money, error) {
	currency := strings.TrimSpace(p.Currency)
	if currency == "" {
		currency = baseCurrency
	}
	if currency != baseCurrency || int64(p.AmountMinor) < 0 {
		return money.Money{}, money.ErrCurrencyMismatch
	}
	return money.Money{Minor: int64(p.AmountMinor), Currency: baseCurrency}, nil
}

type purchaseMetricsDTO struct {
	SafeToSpend   server.MoneyDTO `json:"safe_to_spend"`
	SavingsRate   string          `json:"savings_rate,omitempty"`
	LiquidBalance server.MoneyDTO `json:"liquid_balance"`
	NetCashflow   server.MoneyDTO `json:"net_cashflow"`
}

type purchaseResultResponse struct {
	Before           purchaseMetricsDTO `json:"before"`
	After            purchaseMetricsDTO `json:"after"`
	SafeToSpendDelta server.MoneyDTO    `json:"safe_to_spend_delta"`
	NetCashflowDelta server.MoneyDTO    `json:"net_cashflow_delta"`
	Violations       []string           `json:"violations"`
}

func purchaseResultDTO(result scenario.PurchaseResult) purchaseResultResponse {
	violations := make([]string, len(result.Violations))
	for i, violation := range result.Violations {
		violations[i] = string(violation)
	}
	return purchaseResultResponse{
		Before: purchaseMetricsDTO{
			SafeToSpend:   moneyDTO(result.Before.SafeToSpend.Amount),
			SavingsRate:   decimalString(result.Before.SavingsRate),
			LiquidBalance: moneyDTO(result.Before.LiquidBalance),
			NetCashflow:   moneyDTO(result.Before.NetCashflow),
		},
		After: purchaseMetricsDTO{
			SafeToSpend:   moneyDTO(result.After.SafeToSpend.Amount),
			SavingsRate:   decimalString(result.After.SavingsRate),
			LiquidBalance: moneyDTO(result.After.LiquidBalance),
			NetCashflow:   moneyDTO(result.After.NetCashflow),
		},
		SafeToSpendDelta: moneyDTO(result.SafeToSpendDelta),
		NetCashflowDelta: moneyDTO(result.NetCashflowDelta),
		Violations:       violations,
	}
}
