package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/bootstrap"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/migration"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

type serveFunc func(addr string, handler http.Handler) error
type checkFunc func(ctx context.Context, url string) error
type buildHandlerFunc func(ctx context.Context, cfg config.Config) (http.Handler, func(), error)

type databaseCommandHandlers struct {
	migrate   func(context.Context, config.DatabaseConfig) error
	bootstrap func(context.Context, config.DatabaseConfig, bootstrap.Input) (bootstrap.Result, error)
}

func main() {
	check := func(ctx context.Context, url string) error {
		client := &http.Client{Timeout: 3 * time.Second}
		return checkHealth(ctx, client, url)
	}
	if err := runWithBuilder(os.Args[1:], os.Getenv, http.ListenAndServe, check, buildApplicationHandler); err != nil {
		slog.Error("finance-core exited", "error", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, serve serveFunc, check checkFunc) error {
	return runWithCommands(args, getenv, serve, check, func(context.Context, config.Config) (http.Handler, func(), error) {
		return server.NewHandler(), func() {}, nil
	}, io.Discard, defaultDatabaseCommandHandlers())
}

func runWithBuilder(args []string, getenv func(string) string, serve serveFunc, check checkFunc, build buildHandlerFunc) error {
	return runWithCommands(args, getenv, serve, check, build, os.Stdout, defaultDatabaseCommandHandlers())
}

func defaultDatabaseCommandHandlers() databaseCommandHandlers {
	return databaseCommandHandlers{migrate: migration.Run, bootstrap: bootstrap.Run}
}

func runWithCommands(args []string, getenv func(string) string, serve serveFunc, check checkFunc, build buildHandlerFunc, output io.Writer, databaseHandlers databaseCommandHandlers) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "serve":
		cfg, err := config.Load(getenv)
		if err != nil {
			return fmt.Errorf("load runtime config: %w", err)
		}
		if serve == nil {
			return errors.New("serve function is required")
		}
		if build == nil {
			return errors.New("application builder is required")
		}
		handler, cleanup, err := build(context.Background(), cfg)
		if err != nil {
			return fmt.Errorf("build application: %w", err)
		}
		if handler == nil {
			return errors.New("application builder returned a nil handler")
		}
		if cleanup != nil {
			defer cleanup()
		}
		slog.Info("starting finance-core", "addr", cfg.ListenAddr, "timezone", cfg.Timezone)
		return serve(cfg.ListenAddr, handler)
	case "healthcheck":
		url := getenv("FINANCE_HEALTHCHECK_URL")
		if url == "" {
			url = "http://127.0.0.1:8000/healthz"
		}
		if check == nil {
			return errors.New("healthcheck function is required")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return check(ctx, url)
	case "migrate":
		if len(args) != 1 {
			return errors.New("migrate does not accept arguments")
		}
		cfg, err := config.Load(getenv)
		if err != nil {
			return fmt.Errorf("load runtime config: %w", err)
		}
		if databaseHandlers.migrate == nil {
			return errors.New("migration handler is required")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		return databaseHandlers.migrate(ctx, cfg.Database)
	case "bootstrap":
		cfg, err := config.Load(getenv)
		if err != nil {
			return fmt.Errorf("load runtime config: %w", err)
		}
		if databaseHandlers.bootstrap == nil {
			return errors.New("bootstrap handler is required")
		}
		if output == nil {
			output = io.Discard
		}
		flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
		flags.SetOutput(output)
		name := flags.String("name", "家庭", "household name")
		currency := flags.String("currency", "CNY", "household base currency")
		timezone := flags.String("timezone", cfg.Timezone, "household timezone")
		period := flags.String("period", "", "initial budget period (defaults to current month in household timezone)")
		liquidityFloor := flags.Int64("liquidity-floor-minor", 0, "liquidity floor in minor units")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected bootstrap argument %q", flags.Arg(0))
		}
		if *period == "" {
			location, err := time.LoadLocation(*timezone)
			if err != nil {
				return fmt.Errorf("load bootstrap timezone: %w", err)
			}
			*period = time.Now().In(location).Format("2006-01")
		}
		input, err := bootstrap.Validate(bootstrap.Input{
			Name: *name, Currency: *currency, Timezone: *timezone, Period: *period, LiquidityFloorMinor: *liquidityFloor,
		})
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := databaseHandlers.bootstrap(ctx, cfg.Database, input)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "household_id=%d budget_plan_id=%d\n", result.HouseholdID, result.BudgetPlanID)
		return err
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func checkHealth(ctx context.Context, client *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build health request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health status %d", resp.StatusCode)
	}
	return nil
}
