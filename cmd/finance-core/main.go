package main

import (
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
type checkFunc func(url string) error

func main() {
	if err := run(os.Args[1:], os.Getenv, serveHTTP, checkHTTP); err != nil {
		slog.Error("finance-core failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, serve serveFunc, check checkFunc) error {
	if len(args) == 0 {
		return errors.New("command is required: serve or healthcheck")
	}

	command := args[0]

	switch command {
	case "serve":
		cfg, err := config.Load(getenv)
		if err != nil {
			return fmt.Errorf("load runtime config: %w", err)
		}
		if serve == nil {
			return errors.New("serve function is required")
		}
		slog.Info("starting finance-core", "addr", cfg.ListenAddr, "timezone", cfg.Timezone)
		return serve(cfg.ListenAddr, server.NewHandler())
	case "healthcheck":
		url := getenv("FINANCE_HEALTHCHECK_URL")
		if url == "" {
			url = "http://127.0.0.1:8000/healthz"
		}
		if check == nil {
			return errors.New("check function is required")
		}
		return check(url)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func serveHTTP(addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server.ListenAndServe()
}

func checkHTTP(url string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("healthcheck request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned status %d", resp.StatusCode)
	}
	return nil
}
