package audit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
)

func TestAgentPostgresRecorderStateMachineIntegration(t *testing.T) {
	pool := openAuditIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var householdID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO households (name, base_currency, timezone)
		VALUES ('agent-audit-test', 'CNY', 'Asia/Shanghai')
		RETURNING id
	`).Scan(&householdID); err != nil {
		t.Fatalf("create household: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM households WHERE id = $1`, householdID)
	})

	recorder := NewAgentPostgresRecorder(pool)
	attempt := agentadapter.AuditAttempt{
		CreatedAt:       time.Date(2026, 8, 18, 6, 0, 0, 0, time.UTC),
		PrincipalKind:   "mcp",
		HouseholdID:     householdID,
		Protocol:        "mcp",
		ProtocolVersion: "test-version",
		ClientName:      "test-client",
		ClientVersion:   "1",
		ToolName:        agentadapter.ToolGetHouseholdOverview,
		InputSHA256:     strings.Repeat("a", 64),
	}

	successID, err := recorder.Start(ctx, attempt)
	if err != nil {
		t.Fatalf("Start success row: %v", err)
	}
	assertAgentAuditRunning(t, ctx, pool, successID, householdID, attempt)

	dataAsOf := time.Date(2026, 8, 18, 5, 59, 0, 0, time.UTC)
	success := agentadapter.AuditSuccess{
		OutputSHA256: strings.Repeat("b", 64),
		DataAsOf:     &dataAsOf,
		DurationMS:   25,
	}
	if err := recorder.CompleteSuccess(ctx, successID, success); err != nil {
		t.Fatalf("CompleteSuccess: %v", err)
	}
	assertAgentAuditSuccess(t, ctx, pool, successID, success)
	if err := recorder.CompleteSuccess(ctx, successID, success); err == nil {
		t.Fatal("second success completion unexpectedly succeeded")
	}

	failureID, err := recorder.Start(ctx, attempt)
	if err != nil {
		t.Fatalf("Start failure row: %v", err)
	}
	failure := agentadapter.AuditFailure{ErrorCode: agentadapter.CodeInvalidArgument, DurationMS: 7}
	if err := recorder.CompleteFailure(ctx, failureID, failure); err != nil {
		t.Fatalf("CompleteFailure: %v", err)
	}
	assertAgentAuditFailure(t, ctx, pool, failureID, failure)
	if err := recorder.CompleteFailure(ctx, failureID, failure); err == nil {
		t.Fatal("second failure completion unexpectedly succeeded")
	}

	assertAgentAuditHasNoRawColumns(t, ctx, pool)

	bad := attempt
	bad.HouseholdID = householdID + 1_000_000_000
	if _, err := recorder.Start(ctx, bad); err == nil {
		t.Fatal("audit accepted nonexistent household")
	}
}

func assertAgentAuditRunning(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, householdID int64, attempt agentadapter.AuditAttempt) {
	t.Helper()
	var (
		storedHouseholdID int64
		principalKind     string
		protocol          string
		protocolVersion   string
		toolName          string
		inputSHA256       string
		status            string
		outputSHA256      *string
		dataAsOf          *time.Time
		errorCode         *string
		durationMS        *int64
	)
	if err := pool.QueryRow(ctx, `
		SELECT household_id, principal_kind, protocol, protocol_version, tool_name,
		       input_sha256, status, output_sha256, data_as_of, error_code, duration_ms
		FROM agent_tool_audits
		WHERE id = $1
	`, id).Scan(&storedHouseholdID, &principalKind, &protocol, &protocolVersion, &toolName, &inputSHA256, &status, &outputSHA256, &dataAsOf, &errorCode, &durationMS); err != nil {
		t.Fatalf("query running audit: %v", err)
	}
	if storedHouseholdID != householdID || principalKind != attempt.PrincipalKind || protocol != attempt.Protocol || protocolVersion != attempt.ProtocolVersion || toolName != string(attempt.ToolName) || inputSHA256 != attempt.InputSHA256 {
		t.Fatalf("running metadata mismatch: household=%d principal=%q protocol=%q version=%q tool=%q input=%q", storedHouseholdID, principalKind, protocol, protocolVersion, toolName, inputSHA256)
	}
	if status != "running" || outputSHA256 != nil || dataAsOf != nil || errorCode != nil || durationMS != nil {
		t.Fatalf("running state invalid: status=%q output=%v asOf=%v error=%v duration=%v", status, outputSHA256, dataAsOf, errorCode, durationMS)
	}
}

func assertAgentAuditSuccess(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id int64, success agentadapter.AuditSuccess) {
	t.Helper()
	var (
		status       string
		outputSHA256 *string
		dataAsOf     *time.Time
		errorCode    *string
		durationMS   *int64
	)
	if err := pool.QueryRow(ctx, `SELECT status, output_sha256, data_as_of, error_code, duration_ms FROM agent_tool_audits WHERE id = $1`, id).Scan(&status, &outputSHA256, &dataAsOf, &errorCode, &durationMS); err != nil {
		t.Fatalf("query success audit: %v", err)
	}
	if status != "success" || outputSHA256 == nil || *outputSHA256 != success.OutputSHA256 || dataAsOf == nil || !dataAsOf.Equal(*success.DataAsOf) || errorCode != nil || durationMS == nil || *durationMS != success.DurationMS {
		t.Fatalf("success state invalid: status=%q output=%v asOf=%v error=%v duration=%v", status, outputSHA256, dataAsOf, errorCode, durationMS)
	}
}

func assertAgentAuditFailure(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id int64, failure agentadapter.AuditFailure) {
	t.Helper()
	var (
		status       string
		outputSHA256 *string
		dataAsOf     *time.Time
		errorCode    *string
		durationMS   *int64
	)
	if err := pool.QueryRow(ctx, `SELECT status, output_sha256, data_as_of, error_code, duration_ms FROM agent_tool_audits WHERE id = $1`, id).Scan(&status, &outputSHA256, &dataAsOf, &errorCode, &durationMS); err != nil {
		t.Fatalf("query failure audit: %v", err)
	}
	if status != "error" || outputSHA256 != nil || dataAsOf != nil || errorCode == nil || *errorCode != string(failure.ErrorCode) || durationMS == nil || *durationMS != failure.DurationMS {
		t.Fatalf("failure state invalid: status=%q output=%v asOf=%v error=%v duration=%v", status, outputSHA256, dataAsOf, errorCode, durationMS)
	}
}

func assertAgentAuditHasNoRawColumns(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name='agent_tool_audits'`)
	if err != nil {
		t.Fatalf("query columns: %v", err)
	}
	defer rows.Close()
	forbidden := map[string]struct{}{
		"payload": {}, "input": {}, "output": {}, "raw_input": {}, "raw_output": {},
		"error_message": {}, "request_body": {}, "response_body": {},
	}
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		if _, found := forbidden[strings.ToLower(column)]; found {
			t.Fatalf("raw-content column must not exist: %q", column)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("column rows: %v", err)
	}
}
