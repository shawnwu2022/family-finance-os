package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr string
	Timezone   string
	Database   DatabaseConfig
	Ledger     LedgerConfig
	LLM        LLMConfig
	MCP        MCPConfig
	Portfolio  PortfolioConfig
}

type DatabaseConfig struct {
	Host     string
	Port     uint16
	Name     string
	User     string
	Password string
	SSLMode  string
}

type LedgerConfig struct {
	BaseURL  string
	APIToken string
}

type LLMConfig struct {
	BaseURL       string
	APIKey        string
	FastModel     string
	PlannerModel  string
	ReviewerModel string
}

type MCPConfig struct {
	Enabled           bool
	TokenFile         string
	HouseholdID       int64
	AllowedOrigins    []string
	RequestTimeout    time.Duration
	MaxConcurrent     int
	RequestsPerMinute int
	MaxBodyBytes      int64
}

type PortfolioConfig struct {
	ValuationStaleAfter time.Duration
	FXStaleAfter        time.Duration
}

func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, errors.New("getenv function is required")
	}

	port, err := parsePort(valueOrDefault(getenv("DB_PORT"), "5432"))
	if err != nil {
		return Config{}, err
	}

	db := DatabaseConfig{
		Host:     valueOrDefault(getenv("DB_HOST"), "postgres"),
		Port:     port,
		Name:     strings.TrimSpace(getenv("DB_NAME")),
		User:     strings.TrimSpace(getenv("DB_USER")),
		Password: getenv("DB_PASSWORD"),
		SSLMode:  valueOrDefault(getenv("DB_SSLMODE"), "disable"),
	}
	if err := validateDatabaseConfig(db); err != nil {
		return Config{}, err
	}

	mcp, err := parseMCPConfig(getenv)
	if err != nil {
		return Config{}, err
	}
	portfolioConfig, err := parsePortfolioConfig(getenv)
	if err != nil {
		return Config{}, err
	}

	return Config{
		ListenAddr: valueOrDefault(getenv("FINANCE_LISTEN_ADDR"), ":8000"),
		Timezone:   valueOrDefault(getenv("APP_TIMEZONE"), "Asia/Shanghai"),
		Database:   db,
		Ledger: LedgerConfig{
			BaseURL:  valueOrDefault(getenv("EBK_BASE_URL"), "http://ezbookkeeping:8080/api/v1"),
			APIToken: getenv("EBK_API_TOKEN"),
		},
		LLM: LLMConfig{
			BaseURL:       getenv("LLM_BASE_URL"),
			APIKey:        getenv("LLM_API_KEY"),
			FastModel:     getenv("LLM_FAST_MODEL"),
			PlannerModel:  getenv("LLM_PLANNER_MODEL"),
			ReviewerModel: getenv("LLM_REVIEWER_MODEL"),
		},
		MCP:       mcp,
		Portfolio: portfolioConfig,
	}, nil
}

func parsePortfolioConfig(getenv func(string) string) (PortfolioConfig, error) {
	valuationStaleAfter, err := parsePositiveDuration("PORTFOLIO_VALUATION_STALE_AFTER", valueOrDefault(getenv("PORTFOLIO_VALUATION_STALE_AFTER"), "720h"))
	if err != nil {
		return PortfolioConfig{}, err
	}
	fxStaleAfter, err := parsePositiveDuration("PORTFOLIO_FX_STALE_AFTER", valueOrDefault(getenv("PORTFOLIO_FX_STALE_AFTER"), "72h"))
	if err != nil {
		return PortfolioConfig{}, err
	}
	return PortfolioConfig{ValuationStaleAfter: valuationStaleAfter, FXStaleAfter: fxStaleAfter}, nil
}

func (c DatabaseConfig) URL() *url.URL {
	query := url.Values{}
	query.Set("sslmode", c.SSLMode)

	return &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(int(c.Port))),
		Path:     "/" + c.Name,
		RawQuery: query.Encode(),
	}
}

func parseMCPConfig(getenv func(string) string) (MCPConfig, error) {
	rawEnabled := strings.TrimSpace(getenv("MCP_ENABLED"))
	if rawEnabled == "" {
		return MCPConfig{}, nil
	}
	enabled, err := strconv.ParseBool(rawEnabled)
	if err != nil {
		return MCPConfig{}, fmt.Errorf("MCP_ENABLED must be a boolean")
	}
	if !enabled {
		return MCPConfig{}, nil
	}

	householdID, err := parsePositiveInt64("MCP_HOUSEHOLD_ID", getenv("MCP_HOUSEHOLD_ID"))
	if err != nil {
		return MCPConfig{}, err
	}
	allowedOrigins, err := parseMCPOrigins(getenv("MCP_ALLOWED_ORIGINS"))
	if err != nil {
		return MCPConfig{}, err
	}
	requestTimeout, err := time.ParseDuration(valueOrDefault(getenv("MCP_REQUEST_TIMEOUT"), "15s"))
	if err != nil || requestTimeout <= 0 {
		return MCPConfig{}, fmt.Errorf("MCP_REQUEST_TIMEOUT must be a positive duration")
	}
	maxConcurrent, err := parsePositiveInt("MCP_MAX_CONCURRENT", valueOrDefault(getenv("MCP_MAX_CONCURRENT"), "4"))
	if err != nil {
		return MCPConfig{}, err
	}
	requestsPerMinute, err := parsePositiveInt("MCP_REQUESTS_PER_MINUTE", valueOrDefault(getenv("MCP_REQUESTS_PER_MINUTE"), "60"))
	if err != nil {
		return MCPConfig{}, err
	}
	maxBodyBytes, err := parsePositiveInt64("MCP_MAX_BODY_BYTES", valueOrDefault(getenv("MCP_MAX_BODY_BYTES"), "262144"))
	if err != nil {
		return MCPConfig{}, err
	}

	return MCPConfig{
		Enabled:           true,
		TokenFile:         valueOrDefault(getenv("MCP_TOKEN_FILE"), "/run/secrets/finance-mcp-token"),
		HouseholdID:       householdID,
		AllowedOrigins:    allowedOrigins,
		RequestTimeout:    requestTimeout,
		MaxConcurrent:     maxConcurrent,
		RequestsPerMinute: requestsPerMinute,
		MaxBodyBytes:      maxBodyBytes,
	}, nil
}

func parseMCPOrigins(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin, err := canonicalMCPOrigin(part)
		if err != nil {
			return nil, fmt.Errorf("MCP_ALLOWED_ORIGINS contains an invalid origin: %w", err)
		}
		origins = append(origins, origin)
	}
	return origins, nil
}

func canonicalMCPOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "null") {
		return "", fmt.Errorf("origin is empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse origin: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("origin scheme must be http or https")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin must contain only scheme and authority")
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func parsePositiveInt(key, raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

func parsePositiveDuration(key, raw string) (time.Duration, error) {
	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return value, nil
}

func parsePositiveInt64(key, raw string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

func parsePort(raw string) (uint16, error) {
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 16)
	if err != nil || value == 0 {
		return 0, fmt.Errorf("DB_PORT must be an integer between 1 and 65535")
	}
	return uint16(value), nil
}

func validateDatabaseConfig(cfg DatabaseConfig) error {
	required := []struct {
		key   string
		value string
	}{
		{key: "DB_NAME", value: cfg.Name},
		{key: "DB_USER", value: cfg.User},
		{key: "DB_PASSWORD", value: cfg.Password},
	}
	for _, item := range required {
		if strings.TrimSpace(item.value) == "" {
			return fmt.Errorf("%s is required", item.key)
		}
	}
	return nil
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
