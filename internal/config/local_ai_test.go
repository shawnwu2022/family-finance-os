package config

import "testing"

func TestLoadLLMMode(t *testing.T) {
	values := map[string]string{
		"DB_NAME":            "finance",
		"DB_USER":            "finance",
		"DB_PASSWORD":        "secret",
		"LLM_MODE":           "local",
		"LLM_BASE_URL":       "http://127.0.0.1:11434/v1",
		"LLM_FAST_MODEL":     "fast",
		"LLM_PLANNER_MODEL":  "planner",
		"LLM_REVIEWER_MODEL": "reviewer",
	}
	cfg, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Mode != LLMModeLocal {
		t.Fatalf("LLM mode=%q want %q", cfg.LLM.Mode, LLMModeLocal)
	}
}

func TestLoadRejectsUnknownLLMMode(t *testing.T) {
	values := map[string]string{
		"DB_NAME":     "finance",
		"DB_USER":     "finance",
		"DB_PASSWORD": "secret",
		"LLM_MODE":    "sideways",
	}
	if _, err := Load(func(key string) string { return values[key] }); err == nil {
		t.Fatal("Load accepted unknown LLM_MODE")
	}
}
