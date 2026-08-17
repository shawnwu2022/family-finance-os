package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/shawnwu2022/family-finance-os/internal/advisor"
	"github.com/shawnwu2022/family-finance-os/internal/appapi"
	"github.com/shawnwu2022/family-finance-os/internal/audit"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/ledger/ezbookkeeping"
	"github.com/shawnwu2022/family-finance-os/internal/llm"
	"github.com/shawnwu2022/family-finance-os/internal/requestscope"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/internal/store"
	"github.com/shawnwu2022/family-finance-os/internal/webassets"
)

func buildApplicationHandler(ctx context.Context, cfg config.Config) (http.Handler, func(), error) {
	pool, err := store.OpenPostgres(ctx, cfg.Database)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { pool.Close() }
	fail := func(err error) (http.Handler, func(), error) {
		cleanup()
		return nil, nil, err
	}

	ledgerClient, err := ezbookkeeping.NewClient(cfg.Ledger.BaseURL, cfg.Ledger.APIToken, cfg.Timezone, nil)
	if err != nil {
		return fail(fmt.Errorf("configure ezbookkeeping ledger: %w", err))
	}
	financeAPI, err := appapi.New(appapi.Dependencies{
		Ledger:  ledgerClient,
		Planner: appapi.NewPostgresPlanner(pool),
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

	handler := server.NewHandler(
		server.WithAPI(householdScopedAPI{API: financeAPI}),
		server.WithWeb(webassets.Handler()),
		server.WithReady(pool.Ping),
	)
	return handler, cleanup, nil
}

type householdScopedAPI struct {
	*appapi.API
}

func (a householdScopedAPI) Advisor(ctx context.Context, request server.AdvisorRequest) (server.AdvisorResponse, error) {
	return a.API.Advisor(requestscope.WithHouseholdID(ctx, request.HouseholdID), request)
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
