package appapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/shawnwu2022/family-finance-os/internal/report"
	"github.com/shawnwu2022/family-finance-os/internal/scheduler"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

var ErrInvalidReportingAPI = errors.New("invalid reporting API dependencies")

type ReportRunReader interface {
	List(ctx context.Context, householdID int64, jobName string) ([]scheduler.RunRecord, error)
}

type ReportingAPI struct {
	*API
	runs ReportRunReader
}

func NewReportingAPI(core *API, runs ReportRunReader) (*ReportingAPI, error) {
	if core == nil || runs == nil {
		return nil, ErrInvalidReportingAPI
	}
	return &ReportingAPI{API: core, runs: runs}, nil
}

func (a *API) MonthlyReport(ctx context.Context, householdID int64, period string) (report.MonthlyReport, error) {
	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return report.MonthlyReport{}, fmt.Errorf("load household profile: %w", err)
	}
	if _, _, err := periodBounds(period, profile.Household.Timezone); err != nil {
		return report.MonthlyReport{}, err
	}
	snapshot, err := a.snapshot(ctx, profile, period)
	if err != nil {
		return report.MonthlyReport{}, err
	}
	return report.GenerateMonthly(report.MonthlyInput{
		Period:      period,
		DataAsOf:    snapshot.asOf,
		GeneratedAt: a.now().UTC(),
		Quality:     qualityString(snapshot.quality),
		Metrics: report.MonthlyMetrics{
			Income:          snapshot.cashflow.RecognizedIncome,
			Expense:         snapshot.cashflow.RecognizedExpense,
			NetCashflow:     snapshot.cashflow.NetCashflow,
			NetWorth:        snapshot.netWorth.NetWorth,
			SafeToSpend:     snapshot.safeToSpend.Amount,
			TotalDebt:       snapshot.totalDebt,
			SavingsRate:     savingsRateString(snapshot.cashflow),
			EmergencyMonths: decimalResultString(snapshot.emergencyMonths),
		},
		Warnings: cloneWarnings(snapshot.warnings),
	})
}

func (a *ReportingAPI) Reports(ctx context.Context, householdID int64) (server.ReportsResponse, error) {
	runs, err := a.runs.List(ctx, householdID, report.JobNameMonthly)
	if err != nil {
		return server.ReportsResponse{}, fmt.Errorf("list monthly report runs: %w", err)
	}
	items := make([]server.ReportSummary, 0, len(runs))
	for _, run := range runs {
		items = append(items, server.ReportSummary{
			ID:        run.ID,
			Period:    run.Key.Period,
			Kind:      report.KindMonthly,
			Status:    string(run.Status),
			CreatedAt: run.StartedAt,
		})
	}
	return server.ReportsResponse{Items: items}, nil
}
