package audit

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
)

type PostgresRecorder struct {
	pool *pgxpool.Pool
}

func NewPostgresRecorder(pool *pgxpool.Pool) *PostgresRecorder {
	return &PostgresRecorder{pool: pool}
}

func (r *PostgresRecorder) Record(ctx context.Context, record AdviceRecord) (int64, error) {
	if r == nil || r.pool == nil {
		return 0, fmt.Errorf("%w: postgres pool is required", ErrInvalidAdviceAudit)
	}
	if err := record.Validate(); err != nil {
		return 0, err
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin advice audit transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := storesqlc.New(tx)

	reviewerRole := pgtype.Text{}
	if record.ReviewerRole != nil {
		reviewerRole = pgtype.Text{String: string(*record.ReviewerRole), Valid: true}
	}
	parent, err := queries.CreateAdviceAudit(ctx, storesqlc.CreateAdviceAuditParams{
		CreatedAt:             pgtype.Timestamptz{Time: record.CreatedAt, Valid: true},
		ModelRole:             string(record.ModelRole),
		ReviewerRole:          reviewerRole,
		DataAsOf:              pgtype.Timestamptz{Time: record.DataAsOf, Valid: true},
		PromptTemplateVersion: record.PromptTemplateVersion,
		RequestSha256:         record.RequestSHA256,
		AdviceSha256:          record.AdviceSHA256,
		QualityLevel:          record.QualityLevel,
		Status:                string(record.Status),
	})
	if err != nil {
		return 0, fmt.Errorf("insert advice audit: %w", err)
	}

	for _, execution := range record.Tools {
		if execution.Sequence > math.MaxInt32 {
			return 0, fmt.Errorf("%w: tool sequence exceeds int32", ErrInvalidAdviceAudit)
		}
		resultHash := pgtype.Text{}
		errorCode := pgtype.Text{}
		if execution.Success {
			resultHash = pgtype.Text{String: execution.ResultSHA256, Valid: true}
		} else {
			errorCode = pgtype.Text{String: execution.ErrorCode, Valid: true}
		}
		_, err := queries.CreateAdviceAuditTool(ctx, storesqlc.CreateAdviceAuditToolParams{
			AdviceAuditID: parent.ID,
			Sequence:      int32(execution.Sequence),
			ToolName:      execution.ToolName,
			InputSha256:   execution.InputSHA256,
			ResultSha256:  resultHash,
			Success:       execution.Success,
			ErrorCode:     errorCode,
		})
		if err != nil {
			return 0, fmt.Errorf("insert advice audit tool sequence %d: %w", execution.Sequence, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit advice audit transaction: %w", err)
	}
	return parent.ID, nil
}
