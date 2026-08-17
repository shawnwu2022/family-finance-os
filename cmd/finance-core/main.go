package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

type serveFunc func(addr string, handler http.Handler) error
type checkFunc func(ctx context.Context, url string) error
type buildHandlerFunc func(ctx context.Context, cfg config.Config) (http.Handler, func(), error)

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
	return runWithBuilder(args, getenv, serve, check, func(context.Context, config.Config) (http.Handler, func(), error) {
		return server.NewHandler(), func() {}, nil
	})
}

func runWithBuilder(args []string, getenv func(string) string, serve serveFunc, check checkFunc, build buildHandlerFunc) error {
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
