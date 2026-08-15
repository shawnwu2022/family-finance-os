package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr string
	Timezone   string
	Database   DatabaseConfig
	Ledger     LedgerConfig
	LLM        LLMConfig
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
	}, nil
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
