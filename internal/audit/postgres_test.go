package audit

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/llm"
	"github.com/shawnwu2022/family-finance-os/internal/store"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
)

func TestPostgresRecorderRoundTripIntegration(t *testing.T) {
	pool := openAuditIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reviewer := llm.ModelRoleReviewer
	secretInput := []byte(`{"household_id":1,"secret":"DO_NOT_STORE_ME"}`)
	secretResult := []byte(`{"safe_to_spend_minor":350000,"secret":"RESULT_SECRET"}`)
	record := AdviceRecord{
		CreatedAt:             time.Date(2026, 8, 17, 3, 0, 0, 0, time.UTC),
		ModelRole:             llm.ModelRolePlanner,
		ReviewerRole:          &reviewer,
		DataAsOf:              time.Date(2026, 8, 17, 2, 59, 0, 0, time.UTC),
		PromptTemplateVersion: "finance-advisor-v1",
		RequestSHA256:         SHA256Hex([]byte("raw question must not be stored")),
		AdviceSHA256:          SHA256Hex([]byte("raw advice must not be stored")),
		QualityLevel:          "partial",
		Status:                AdviceStatusSuccess,
		Tools: []ToolExecution{
			NewToolExecution(0, "get_overview", secretInput, secretResult, ""),
			NewFailedToolExecution(1, "get_goal_status", []byte(`{"household_id":1}`), "tool_execution_failed"),
		},
	}

	recorder := NewPostgresRecorder(pool)
	id, err := recorder.Record(ctx, record)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if id <= 0 {
		t.Fatalf("id=%d want positive", id)
	}

	queries := storesqlc.New(pool)
	stored, err := queries.GetAdviceAudit(ctx, id)
	if err != nil {
		t.Fatalf("GetAdviceAudit: %v", err)
	}
	if stored.ModelRole != string(record.ModelRole) || !stored.ReviewerRole.Valid || stored.ReviewerRole.String != string(reviewer) {
		t.Fatalf("stored roles = %#v", stored)
	}
	if stored.RequestSha256 != record.RequestSHA256 || stored.AdviceSha256 != record.AdviceSHA256 || stored.QualityLevel != record.QualityLevel || stored.Status != string(record.Status) {
		t.Fatalf("stored metadata = %#v", stored)
	}

	tools, err := queries.ListAdviceAuditTools(ctx, id)
	if err != nil {
		t.Fatalf("ListAdviceAuditTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("tools=%#v", tools)
	}
	if !tools[0].Success || !tools[0].ResultSha256.Valid || tools[0].ResultSha256.String != record.Tools[0].ResultSHA256 || tools[0].ErrorCode.Valid {
		t.Fatalf("successful tool row=%#v", tools[0])
	}
	if tools[1].Success || tools[1].ResultSha256.Valid || !tools[1].ErrorCode.Valid || tools[1].ErrorCode.String != "tool_execution_failed" {
		t.Fatalf("failed tool row=%#v", tools[1])
	}

	forbiddenColumns := map[string]struct{}{
		"question": {}, "content": {}, "payload": {}, "tool_input": {}, "tool_result": {},
		"error_message": {}, "advice_text": {}, "raw_input": {}, "raw_result": {},
	}
	rows, err := pool.Query(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name IN ('advice_audits', 'advice_audit_tools')
	`)
	if err != nil {
		t.Fatalf("query audit columns: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatalf("scan audit column: %v", err)
		}
		if _, forbidden := forbiddenColumns[strings.ToLower(column)]; forbidden {
			t.Fatalf("raw-content column must not exist: %q", column)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("audit column rows: %v", err)
	}
}

func openAuditIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}
	portRaw := os.Getenv("TEST_POSTGRES_PORT")
	if portRaw == "" {
		portRaw = "5432"
	}
	port, err := strconv.ParseUint(portRaw, 10, 16)
	if err != nil {
		t.Fatalf("parse TEST_POSTGRES_PORT: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := store.OpenPostgres(ctx, config.DatabaseConfig{
		Host: host, Port: uint16(port), Name: os.Getenv("TEST_POSTGRES_DB"),
		User: os.Getenv("TEST_POSTGRES_USER"), Password: os.Getenv("TEST_POSTGRES_PASSWORD"), SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
