package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/advisor"
	"github.com/shawnwu2022/family-finance-os/internal/appapi"
	"github.com/shawnwu2022/family-finance-os/internal/audit"
	financeauth "github.com/shawnwu2022/family-finance-os/internal/auth"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/ledger/ezbookkeeping"
	"github.com/shawnwu2022/family-finance-os/internal/llm"
	"github.com/shawnwu2022/family-finance-os/internal/portfolio"
	"github.com/shawnwu2022/family-finance-os/internal/report"
	"github.com/shawnwu2022/family-finance-os/internal/requestscope"
	"github.com/shawnwu2022/family-finance-os/internal/scheduler"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/internal/store"
	"github.com/shawnwu2022/family-finance-os/internal/webassets"
)

func buildApplicationHandler(ctx context.Context, cfg config.Config) (http.Handler, func(), error) {
	secretBox, err := loadBrowserAuthSecretBox(cfg.Auth)
	if err != nil {
		return nil, nil, err
	}
	ledgerToken, err := loadLedgerAPIToken(cfg.Ledger)
	if err != nil {
		return nil, nil, err
	}
	defer clear(ledgerToken)

	pool, err := store.OpenPostgres(ctx, cfg.Database)
	if err != nil {
		return nil, nil, err
	}
	schedulerCtx, cancelScheduler := context.WithCancel(ctx)
	cleanup := func() {
		cancelScheduler()
		pool.Close()
	}
	fail := func(err error) (http.Handler, func(), error) {
		cleanup()
		return nil, nil, err
	}

	browserAuth, err := financeauth.NewService(financeauth.NewPostgresStore(pool), secretBox)
	if err != nil {
		return fail(fmt.Errorf("configure browser authentication: %w", err))
	}
	ledgerClient, err := ezbookkeeping.NewClient(cfg.Ledger.BaseURL, string(ledgerToken), cfg.Timezone, nil)
	if err != nil {
		return fail(fmt.Errorf("configure ezbookkeeping ledger: %w", err))
	}
	financeAPI, err := appapi.New(appapi.Dependencies{
		Ledger:              ledgerClient,
		Planner:             appapi.NewPostgresPlanner(pool),
		Portfolio:           portfolio.NewPostgresStore(pool),
		ValuationStaleAfter: cfg.Portfolio.ValuationStaleAfter,
		FXStaleAfter:        cfg.Portfolio.FXStaleAfter,
	})
	if err != nil {
		return fail(fmt.Errorf("configure finance API: %w", err))
	}

	aiEnabled, err := validateRuntimeAIConfig(cfg.LLM)
	if err != nil {
		return fail(err)
	}
	if aiEnabled {
		provider, err := llm.NewOpenAICompatibleProvider(llm.OpenAICompatibleConfig{
			BaseURL: cfg.LLM.BaseURL,
			APIKey:  cfg.LLM.APIKey,
			Models: llm.ModelSet{
				Fast:     cfg.LLM.FastModel,
				Planner:  cfg.LLM.PlannerModel,
				Reviewer: cfg.LLM.ReviewerModel,
			},
		})
		if err != nil {
			return fail(fmt.Errorf("configure LLM provider: %w", err))
		}
		registry, err := financeAPI.AdvisorRegistry()
		if err != nil {
			return fail(fmt.Errorf("configure advisor tools: %w", err))
		}
		advisorService, err := advisor.NewService(
			provider,
			registry,
			audit.NewPostgresRecorder(pool),
			advisor.DefaultPolicy(),
		)
		if err != nil {
			return fail(fmt.Errorf("configure advisor service: %w", err))
		}
		financeAPI.SetAdvisor(advisorService)
	}

	runStore := scheduler.NewPostgresRunStore(pool)
	if err := runStore.RecoverInterrupted(ctx, time.Now().UTC()); err != nil {
		return fail(fmt.Errorf("recover interrupted scheduled jobs: %w", err))
	}
	reportingAPI, err := appapi.NewReportingAPI(financeAPI, report.NewPostgresStore(pool))
	if err != nil {
		return fail(fmt.Errorf("configure reporting API: %w", err))
	}
	reportScheduler, err := scheduler.New(runStore, nil)
	if err != nil {
		return fail(fmt.Errorf("configure report scheduler: %w", err))
	}
	scopes, err := runStore.ListHouseholds(ctx)
	if err != nil {
		return fail(fmt.Errorf("load scheduler households: %w", err))
	}
	jobs := make([]scheduler.Job, 0, len(scopes))
	for _, scope := range scopes {
		job, err := newMonthlyReportJob(scope, financeAPI)
		if err != nil {
			return fail(fmt.Errorf("configure monthly report job for household %d: %w", scope.HouseholdID, err))
		}
		jobs = append(jobs, job)
	}

	handlerOptions := []server.HandlerOption{
		server.WithAPI(householdScopedAPI{FinanceAPI: reportingAPI, advisor: financeAPI}),
		server.WithWeb(webassets.Handler()),
		server.WithReady(pool.Ping),
		server.WithBrowserAuth(browserAuth),
	}
	if cfg.MCP.Enabled {
		mcpHandler, err := buildMCPHandler(ctx, cfg.MCP, pool, financeAPI)
		if err != nil {
			return fail(fmt.Errorf("configure MCP handler: %w", err))
		}
		handlerOptions = append(handlerOptions, server.WithMCP(mcpHandler))
	}
	handler := server.NewHandler(handlerOptions...)
	for _, job := range jobs {
		job := job
		go func() {
			if err := reportScheduler.Run(schedulerCtx, job); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("monthly report scheduler stopped", "household_id", job.HouseholdID, "error", err)
			}
		}()
	}
	return handler, cleanup, nil
}

type householdScopedAPI struct {
	server.FinanceAPI
	advisor *appapi.API
}

func (a householdScopedAPI) Advisor(ctx context.Context, request server.AdvisorRequest) (server.AdvisorResponse, error) {
	return a.advisor.Advisor(requestscope.WithHouseholdID(ctx, request.HouseholdID), request)
}

func (a householdScopedAPI) ListPortfolioAssets(ctx context.Context, householdID int64) (server.PortfolioAssetsResponse, error) {
	return a.advisor.ListPortfolioAssets(ctx, householdID)
}

func (a householdScopedAPI) UpsertPortfolioAsset(ctx context.Context, householdID int64, assetRef string, request server.PortfolioAssetUpsertRequest) (server.PortfolioAssetResponse, error) {
	return a.advisor.UpsertPortfolioAsset(ctx, householdID, assetRef, request)
}

func (a householdScopedAPI) DeletePortfolioAsset(ctx context.Context, householdID int64, assetRef string) error {
	return a.advisor.DeletePortfolioAsset(ctx, householdID, assetRef)
}

type monthlyReporter interface {
	MonthlyReport(ctx context.Context, householdID int64, period string) (report.MonthlyReport, error)
}

func newMonthlyReportJob(scope scheduler.HouseholdScope, reporter monthlyReporter) (scheduler.Job, error) {
	if scope.HouseholdID <= 0 {
		return scheduler.Job{}, errors.New("household ID must be positive")
	}
	if reporter == nil {
		return scheduler.Job{}, errors.New("monthly reporter is required")
	}
	location, err := time.LoadLocation(strings.TrimSpace(scope.Timezone))
	if err != nil {
		return scheduler.Job{}, fmt.Errorf("load household timezone: %w", err)
	}
	schedule, err := scheduler.NewMonthlySchedule(location, 3, 0)
	if err != nil {
		return scheduler.Job{}, err
	}
	period := func(trigger time.Time) string {
		return trigger.In(location).AddDate(0, -1, 0).Format("2006-01")
	}
	return scheduler.Job{
		HouseholdID: scope.HouseholdID,
		Name:        report.JobNameMonthly,
		Schedule:    schedule,
		CatchUp:     scheduler.CatchUpLatestOnly,
		Period:      period,
		Run: func(ctx context.Context, scheduledFor time.Time) error {
			_, err := reporter.MonthlyReport(ctx, scope.HouseholdID, period(scheduledFor))
			return err
		},
	}, nil
}

func loadBrowserAuthSecretBox(cfg config.AuthConfig) (*financeauth.SecretBox, error) {
	key, err := readRequiredSecretFile(strings.TrimSpace(cfg.KeyFile), 32)
	if err != nil {
		return nil, fmt.Errorf("read finance auth key: %w", err)
	}
	defer clear(key)
	secretBox, err := financeauth.NewSecretBox(key)
	if err != nil {
		return nil, fmt.Errorf("configure finance auth key: %w", err)
	}
	return secretBox, nil
}

func loadLedgerAPIToken(cfg config.LedgerConfig) ([]byte, error) {
	token, err := readRequiredSecretFile(strings.TrimSpace(cfg.APITokenFile), 4096)
	if err != nil {
		return nil, fmt.Errorf("read ezbookkeeping API token: %w", err)
	}
	return token, nil
}

func validateRuntimeAIConfig(cfg config.LLMConfig) (bool, error) {
	values := []string{
		cfg.BaseURL,
		cfg.APIKey,
		cfg.FastModel,
		cfg.PlannerModel,
		cfg.ReviewerModel,
	}
	configured := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			configured++
		}
	}
	if configured == 0 {
		return false, nil
	}
	if configured != len(values) {
		return false, errors.New("LLM configuration must be fully specified or fully disabled")
	}
	return true, nil
}
