package main

import (
	"testing"

	"github.com/shawnwu2022/family-finance-os/internal/config"
)

func TestValidateRuntimeAIConfigModes(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.LLMConfig
		want bool
		err  bool
	}{
		{name: "disabled", cfg: config.LLMConfig{Mode: config.LLMModeDisabled}, want: false},
		{name: "implicit disabled remains compatible", cfg: config.LLMConfig{}, want: false},
		{name: "local loopback without API key", cfg: config.LLMConfig{Mode: config.LLMModeLocal, BaseURL: "http://127.0.0.1:11434/v1", FastModel: "local-fast", PlannerModel: "local-planner", ReviewerModel: "local-reviewer"}, want: true},
		{name: "local cannot carry a remote HTTPS endpoint", cfg: config.LLMConfig{Mode: config.LLMModeLocal, BaseURL: "https://llm.example/v1", FastModel: "fast", PlannerModel: "planner", ReviewerModel: "reviewer"}, err: true},
		{name: "external requires API key", cfg: config.LLMConfig{Mode: config.LLMModeExternal, BaseURL: "https://llm.example/v1", FastModel: "fast", PlannerModel: "planner", ReviewerModel: "reviewer"}, err: true},
		{name: "external complete", cfg: config.LLMConfig{Mode: config.LLMModeExternal, BaseURL: "https://llm.example/v1", APIKey: "secret", FastModel: "fast", PlannerModel: "planner", ReviewerModel: "reviewer"}, want: true},
		{name: "legacy complete config remains external-compatible", cfg: config.LLMConfig{BaseURL: "https://llm.example/v1", APIKey: "secret", FastModel: "fast", PlannerModel: "planner", ReviewerModel: "reviewer"}, want: true},
		{name: "invalid mode", cfg: config.LLMConfig{Mode: "sideways", BaseURL: "https://llm.example/v1", APIKey: "secret", FastModel: "fast", PlannerModel: "planner", ReviewerModel: "reviewer"}, err: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled, err := validateRuntimeAIConfig(tt.cfg)
			if tt.err {
				if err == nil {
					t.Fatal("expected configuration error")
				}
				return
			}
			if err != nil || enabled != tt.want {
				t.Fatalf("enabled=%v error=%v want=%v", enabled, err, tt.want)
			}
		})
	}
}
