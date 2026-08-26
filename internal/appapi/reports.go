package appapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/shawnwu2022/family-finance-os/internal/report"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

var ErrInvalidReportingAPI = errors.New("invalid reporting API dependencies")

type ReportingAPI struct {
	*API
}

func NewReportingAPI(core *API, reports report.Store) (*ReportingAPI, error) {
	if core == nil || reports == nil {
		return nil, ErrInvalidReportingAPI
	}
	core.reports = reports
	return &ReportingAPI{API: core}, nil
}

func (a *API) MonthlyReport(ctx context.Context, householdID int64, period string) (report.MonthlyReport, error) {
	if a.reports != nil {
		stored, err := a.reports.Get(ctx, householdID, period)
		if err == nil {
			return stored.Report, nil
		}
		if !errors.Is(err, report.ErrNotFound) {
			return report.MonthlyReport{}, fmt.Errorf("load monthly report: %w", err)
		}
	}
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
	monthly, err := report.GenerateMonthly(report.MonthlyInput{
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
	if err != nil {
		return report.MonthlyReport{}, err
	}
	if a.reports == nil {
		return monthly, nil
	}
	stored, err := a.reports.Save(ctx, householdID, monthly)
	if err != nil {
		return report.MonthlyReport{}, fmt.Errorf("persist monthly report: %w", err)
	}
	return stored.Report, nil
}

func (a *ReportingAPI) Reports(ctx context.Context, householdID int64) (server.ReportsResponse, error) {
	stored, err := a.reports.List(ctx, householdID)
	if err != nil {
		return server.ReportsResponse{}, fmt.Errorf("list monthly reports: %w", err)
	}
	items := make([]server.ReportSummary, 0, len(stored))
	for _, artifact := range stored {
		items = append(items, server.ReportSummary{
			ID:          artifact.ID,
			Period:      artifact.Report.Period,
			Kind:        artifact.Report.Kind,
			Status:      "ready",
			ContentHash: artifact.ContentHash,
			CreatedAt:   artifact.CreatedAt,
		})
	}
	return server.ReportsResponse{Items: items}, nil
}
