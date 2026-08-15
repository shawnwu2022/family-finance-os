package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

type serveFunc func(addr string, handler http.Handler) error
type checkFunc func(ctx context.Context, url string) error

func main() {
	if err := run(os.Args[1:], os.Getenv, http.ListenAndServe, func(ctx context.Context, url string) error {
		client := &http.Client{Timeout: 3 * time.Second}
		return checkHealth(ctx, client, url)
	}); err != nil {
		slog.Error("finance-core exited", "error", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, serve serveFunc, check checkFunc) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "serve":
		addr := getenv("FINANCE_LISTEN_ADDR")
		if addr == "" {
			addr = ":8000"
		}
		if serve == nil {
			return errors.New("serve function is required")
		}
		slog.Info("starting finance-core", "addr", addr)
		return serve(addr, server.NewHandler())
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
