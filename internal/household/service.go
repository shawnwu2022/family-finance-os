package household

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type Service struct {
	pool    *pgxpool.Pool
	queries *storesqlc.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, queries: storesqlc.New(pool)}
}

func (s *Service) CreateHousehold(ctx context.Context, input NewHousehold) (Household, error) {
	input, err := validateNewHousehold(input)
	if err != nil {
		return Household{}, err
	}
	row, err := s.queries.CreateHousehold(ctx, storesqlc.CreateHouseholdParams{
		Name:         input.Name,
		BaseCurrency: input.BaseCurrency,
		Timezone:     input.Timezone,
	})
	if err != nil {
		return Household{}, fmt.Errorf("create household: %w", err)
	}
	return householdFromRow(row.ID, row.Name, row.BaseCurrency, row.Timezone), nil
}

func (s *Service) AddMember(ctx context.Context, householdID int64, input NewMember) (Member, error) {
	if householdID <= 0 {
		return Member{}, errors.New("household ID must be positive")
	}
	name, err := validateName(input.Name, "member")
	if err != nil {
		return Member{}, err
	}
	if !input.Kind.valid() {
		return Member{}, fmt.Errorf("invalid member kind %q", input.Kind)
	}
	row, err := s.queries.CreateHouseholdMember(ctx, storesqlc.CreateHouseholdMemberParams{
		HouseholdID: householdID,
		Name:        name,
		Kind:        string(input.Kind),
		Active:      input.Active,
	})
	if err != nil {
		return Member{}, fmt.Errorf("create household member: %w", err)
	}
	return memberFromRow(row.ID, row.HouseholdID, row.Name, row.Kind, row.Active), nil
}

func (s *Service) AddIncomeSource(ctx context.Context, householdID int64, input NewIncomeSource) (IncomeSource, error) {
	home, err := s.getHousehold(ctx, s.queries, householdID)
	if err != nil {
		return IncomeSource{}, err
	}
	name, err := validateName(input.Name, "income source")
	if err != nil {
		return IncomeSource{}, err
	}
	if err := validateMoneyForBase(input.Amount, home.BaseCurrency); err != nil {
		return IncomeSource{}, err
	}
	if !input.Cadence.valid() {
		return IncomeSource{}, fmt.Errorf("invalid income cadence %q", input.Cadence)
	}
	if !input.Stability.valid() {
		return IncomeSource{}, fmt.Errorf("invalid income stability %q", input.Stability)
	}

	memberID := pgtype.Int8{}
	if input.MemberID != nil {
		if *input.MemberID <= 0 {
			return IncomeSource{}, errors.New("member ID must be positive")
		}
		memberID = pgtype.Int8{Int64: *input.MemberID, Valid: true}
	}
	row, err := s.queries.CreateIncomeSource(ctx, storesqlc.CreateIncomeSourceParams{
		HouseholdID: householdID,
		MemberID:    memberID,
		Name:        name,
		AmountMinor: input.Amount.Minor,
		Currency:    home.BaseCurrency,
		Cadence:     string(input.Cadence),
		Stability:   string(input.Stability),
		Active:      input.Active,
	})
	if err != nil {
		return IncomeSource{}, fmt.Errorf("create income source: %w", err)
	}
	return incomeSourceFromValues(row.ID, row.HouseholdID, row.MemberID, row.Name, row.AmountMinor, row.Currency, row.Cadence, row.Stability, row.Active), nil
}

func (s *Service) AddExpenseBaseline(ctx context.Context, householdID int64, input NewExpenseBaseline) (ExpenseBaseline, error) {
	home, err := s.getHousehold(ctx, s.queries, householdID)
	if err != nil {
		return ExpenseBaseline{}, err
	}
	name, err := validateName(input.Name, "expense baseline")
	if err != nil {
		return ExpenseBaseline{}, err
	}
	if err := validateMoneyForBase(input.Amount, home.BaseCurrency); err != nil {
		return ExpenseBaseline{}, err
	}
	if !input.Cadence.valid() {
		return ExpenseBaseline{}, fmt.Errorf("invalid expense cadence %q", input.Cadence)
	}
	row, err := s.queries.CreateExpenseBaseline(ctx, storesqlc.CreateExpenseBaselineParams{
		HouseholdID: householdID,
		Name:        name,
		AmountMinor: input.Amount.Minor,
		Currency:    home.BaseCurrency,
		Cadence:     string(input.Cadence),
		Essential:   input.Essential,
		Active:      input.Active,
	})
	if err != nil {
		return ExpenseBaseline{}, fmt.Errorf("create expense baseline: %w", err)
	}
	return expenseBaselineFromValues(row.ID, row.HouseholdID, row.Name, row.AmountMinor, row.Currency, row.Cadence, row.Essential, row.Active), nil
}

func (s *Service) SetPolicy(ctx context.Context, householdID int64, input NewHouseholdPolicy) (HouseholdPolicy, error) {
	home, err := s.getHousehold(ctx, s.queries, householdID)
	if err != nil {
		return HouseholdPolicy{}, err
	}
	if err := validateMoneyForBase(input.LiquidityFloor, home.BaseCurrency); err != nil {
		return HouseholdPolicy{}, fmt.Errorf("liquidity floor: %w", err)
	}
	row, err := s.queries.UpsertHouseholdPolicy(ctx, storesqlc.UpsertHouseholdPolicyParams{
		HouseholdID:         householdID,
		LiquidityFloorMinor: input.LiquidityFloor.Minor,
		Currency:            home.BaseCurrency,
	})
	if err != nil {
		return HouseholdPolicy{}, fmt.Errorf("upsert household policy: %w", err)
	}
	return HouseholdPolicy{
		HouseholdID: row.HouseholdID,
		LiquidityFloor: money.Money{
			Minor:    row.LiquidityFloorMinor,
			Currency: row.Currency,
		},
	}, nil
}

func (s *Service) GetProfile(ctx context.Context, householdID int64) (Profile, error) {
	if householdID <= 0 {
		return Profile{}, errors.New("household ID must be positive")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Profile{}, fmt.Errorf("begin household profile transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := storesqlc.New(tx)

	home, err := s.getHousehold(ctx, queries, householdID)
	if err != nil {
		return Profile{}, err
	}
	memberRows, err := queries.ListHouseholdMembers(ctx, householdID)
	if err != nil {
		return Profile{}, fmt.Errorf("list household members: %w", err)
	}
	incomeRows, err := queries.ListIncomeSources(ctx, householdID)
	if err != nil {
		return Profile{}, fmt.Errorf("list income sources: %w", err)
	}
	expenseRows, err := queries.ListExpenseBaselines(ctx, householdID)
	if err != nil {
		return Profile{}, fmt.Errorf("list expense baselines: %w", err)
	}
	policyRow, err := queries.GetHouseholdPolicy(ctx, householdID)
	if err != nil {
		return Profile{}, fmt.Errorf("get household policy: %w", err)
	}
	policy := HouseholdPolicy{
		HouseholdID: policyRow.HouseholdID,
		LiquidityFloor: money.Money{
			Minor:    policyRow.LiquidityFloorMinor,
			Currency: policyRow.Currency,
		},
	}

	profile := Profile{
		Household:        home,
		Members:          make([]Member, 0, len(memberRows)),
		IncomeSources:    make([]IncomeSource, 0, len(incomeRows)),
		ExpenseBaselines: make([]ExpenseBaseline, 0, len(expenseRows)),
		Policy:           policy,
	}
	for _, row := range memberRows {
		profile.Members = append(profile.Members, memberFromRow(row.ID, row.HouseholdID, row.Name, row.Kind, row.Active))
	}
	for _, row := range incomeRows {
		profile.IncomeSources = append(profile.IncomeSources, incomeSourceFromValues(row.ID, row.HouseholdID, row.MemberID, row.Name, row.AmountMinor, row.Currency, row.Cadence, row.Stability, row.Active))
	}
	for _, row := range expenseRows {
		profile.ExpenseBaselines = append(profile.ExpenseBaselines, expenseBaselineFromValues(row.ID, row.HouseholdID, row.Name, row.AmountMinor, row.Currency, row.Cadence, row.Essential, row.Active))
	}
	if err := tx.Commit(ctx); err != nil {
		return Profile{}, fmt.Errorf("commit household profile transaction: %w", err)
	}
	return profile, nil
}

type householdGetter interface {
	GetHousehold(context.Context, int64) (storesqlc.GetHouseholdRow, error)
}

func (s *Service) getHousehold(ctx context.Context, q householdGetter, householdID int64) (Household, error) {
	if householdID <= 0 {
		return Household{}, errors.New("household ID must be positive")
	}
	row, err := q.GetHousehold(ctx, householdID)
	if err != nil {
		return Household{}, fmt.Errorf("get household: %w", err)
	}
	return householdFromRow(row.ID, row.Name, row.BaseCurrency, row.Timezone), nil
}

func householdFromRow(id int64, name, currency, timezone string) Household {
	return Household{ID: id, Name: name, BaseCurrency: currency, Timezone: timezone}
}

func memberFromRow(id, householdID int64, name, kind string, active bool) Member {
	return Member{ID: id, HouseholdID: householdID, Name: name, Kind: MemberKind(kind), Active: active}
}

func incomeSourceFromValues(id, householdID int64, memberID pgtype.Int8, name string, amountMinor int64, currency, cadence, stability string, active bool) IncomeSource {
	var member *int64
	if memberID.Valid {
		value := memberID.Int64
		member = &value
	}
	return IncomeSource{
		ID:          id,
		HouseholdID: householdID,
		MemberID:    member,
		Name:        name,
		Amount:      money.Money{Minor: amountMinor, Currency: currency},
		Cadence:     Cadence(cadence),
		Stability:   IncomeStability(stability),
		Active:      active,
	}
}

func expenseBaselineFromValues(id, householdID int64, name string, amountMinor int64, currency, cadence string, essential, active bool) ExpenseBaseline {
	return ExpenseBaseline{
		ID:          id,
		HouseholdID: householdID,
		Name:        name,
		Amount:      money.Money{Minor: amountMinor, Currency: currency},
		Cadence:     Cadence(cadence),
		Essential:   essential,
		Active:      active,
	}
}
