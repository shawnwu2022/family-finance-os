package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
)

type AgentPostgresRecorder struct {
	pool *pgxpool.Pool
}

var _ agentadapter.AuditRecorder = (*AgentPostgresRecorder)(nil)

func NewAgentPostgresRecorder(pool *pgxpool.Pool) *AgentPostgresRecorder {
	return &AgentPostgresRecorder{pool: pool}
}

func (r *AgentPostgresRecorder) Start(ctx context.Context, attempt agentadapter.AuditAttempt) (int64, error) {
	if r == nil || r.pool == nil {
		return 0, fmt.Errorf("agent tool audit postgres pool is required")
	}
	if err := validateAgentAuditAttempt(attempt); err != nil {
		return 0, err
	}

	id, err := storesqlc.New(r.pool).CreateAgentToolAuditAttempt(ctx, storesqlc.CreateAgentToolAuditAttemptParams{
		CreatedAt:       pgtype.Timestamptz{Time: attempt.CreatedAt.UTC(), Valid: true},
		PrincipalKind:   attempt.PrincipalKind,
		HouseholdID:     attempt.HouseholdID,
		Protocol:        attempt.Protocol,
		ProtocolVersion: attempt.ProtocolVersion,
		ClientName:      nullableText(attempt.ClientName),
		ClientVersion:   nullableText(attempt.ClientVersion),
		ToolName:        string(attempt.ToolName),
		InputSha256:     attempt.InputSHA256,
	})
	if err != nil {
		return 0, fmt.Errorf("create agent tool audit attempt: %w", err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("create agent tool audit attempt returned invalid id %d", id)
	}
	return id, nil
}

func (r *AgentPostgresRecorder) CompleteSuccess(ctx context.Context, id int64, success agentadapter.AuditSuccess) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("agent tool audit postgres pool is required")
	}
	if id <= 0 {
		return fmt.Errorf("agent tool audit id must be positive")
	}
	if !isLowerSHA256(success.OutputSHA256) {
		return fmt.Errorf("agent tool audit output hash must be 64 lowercase hex characters")
	}
	if success.DurationMS < 0 {
		return fmt.Errorf("agent tool audit duration must not be negative")
	}

	dataAsOf := pgtype.Timestamptz{}
	if success.DataAsOf != nil {
		dataAsOf = pgtype.Timestamptz{Time: success.DataAsOf.UTC(), Valid: true}
	}
	completedID, err := storesqlc.New(r.pool).CompleteAgentToolAuditSuccess(ctx, storesqlc.CompleteAgentToolAuditSuccessParams{
		OutputSha256: pgtype.Text{String: success.OutputSHA256, Valid: true},
		DataAsOf:     dataAsOf,
		DurationMs:   pgtype.Int8{Int64: success.DurationMS, Valid: true},
		ID:           id,
	})
	if err != nil {
		return fmt.Errorf("complete agent tool audit success: %w", err)
	}
	if completedID != id {
		return fmt.Errorf("complete agent tool audit success returned id %d, want %d", completedID, id)
	}
	return nil
}

func (r *AgentPostgresRecorder) CompleteFailure(ctx context.Context, id int64, failure agentadapter.AuditFailure) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("agent tool audit postgres pool is required")
	}
	if id <= 0 {
		return fmt.Errorf("agent tool audit id must be positive")
	}
	if strings.TrimSpace(string(failure.ErrorCode)) == "" {
		return fmt.Errorf("agent tool audit error code is required")
	}
	if failure.DurationMS < 0 {
		return fmt.Errorf("agent tool audit duration must not be negative")
	}

	completedID, err := storesqlc.New(r.pool).CompleteAgentToolAuditFailure(ctx, storesqlc.CompleteAgentToolAuditFailureParams{
		ErrorCode:  pgtype.Text{String: string(failure.ErrorCode), Valid: true},
		DurationMs: pgtype.Int8{Int64: failure.DurationMS, Valid: true},
		ID:         id,
	})
	if err != nil {
		return fmt.Errorf("complete agent tool audit failure: %w", err)
	}
	if completedID != id {
		return fmt.Errorf("complete agent tool audit failure returned id %d, want %d", completedID, id)
	}
	return nil
}

func validateAgentAuditAttempt(attempt agentadapter.AuditAttempt) error {
	if attempt.CreatedAt.IsZero() {
		return fmt.Errorf("agent tool audit created_at is required")
	}
	if strings.TrimSpace(attempt.PrincipalKind) == "" {
		return fmt.Errorf("agent tool audit principal kind is required")
	}
	if attempt.HouseholdID <= 0 {
		return fmt.Errorf("agent tool audit household id must be positive")
	}
	if strings.TrimSpace(attempt.Protocol) == "" {
		return fmt.Errorf("agent tool audit protocol is required")
	}
	if strings.TrimSpace(attempt.ProtocolVersion) == "" {
		return fmt.Errorf("agent tool audit protocol version is required")
	}
	if strings.TrimSpace(string(attempt.ToolName)) == "" {
		return fmt.Errorf("agent tool audit tool name is required")
	}
	if !isLowerSHA256(attempt.InputSHA256) {
		return fmt.Errorf("agent tool audit input hash must be 64 lowercase hex characters")
	}
	return nil
}

func nullableText(value string) pgtype.Text {
	if strings.TrimSpace(value) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func isLowerSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
