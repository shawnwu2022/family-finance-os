package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/llm"
)

var ErrInvalidAdviceAudit = errors.New("invalid advice audit metadata")

type AdviceStatus string

const (
	AdviceStatusSuccess AdviceStatus = "success"
	AdviceStatusBlocked AdviceStatus = "blocked"
	AdviceStatusError   AdviceStatus = "error"
)

type Recorder interface {
	Record(ctx context.Context, record AdviceRecord) (int64, error)
}

type ToolExecution struct {
	Sequence     int    `json:"sequence"`
	ToolName     string `json:"tool_name"`
	InputSHA256  string `json:"input_sha256"`
	ResultSHA256 string `json:"result_sha256,omitempty"`
	Success      bool   `json:"success"`
	ErrorCode    string `json:"error_code,omitempty"`
}

type AdviceRecord struct {
	CreatedAt             time.Time       `json:"created_at"`
	ModelRole             llm.ModelRole   `json:"model_role"`
	ReviewerRole          *llm.ModelRole  `json:"reviewer_role,omitempty"`
	DataAsOf              time.Time       `json:"data_as_of"`
	PromptTemplateVersion string          `json:"prompt_template_version"`
	RequestSHA256         string          `json:"request_sha256"`
	AdviceSHA256          string          `json:"advice_sha256"`
	QualityLevel          string          `json:"quality_level"`
	Status                AdviceStatus    `json:"status"`
	Tools                 []ToolExecution `json:"tools,omitempty"`
}

func SHA256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func NewToolExecution(sequence int, toolName string, input, result []byte, errorCode string) ToolExecution {
	execution := ToolExecution{
		Sequence:    sequence,
		ToolName:    strings.TrimSpace(toolName),
		InputSHA256: SHA256Hex(input),
		Success:     strings.TrimSpace(errorCode) == "",
		ErrorCode:   strings.TrimSpace(errorCode),
	}
	if execution.Success {
		execution.ResultSHA256 = SHA256Hex(result)
	}
	return execution
}

func NewFailedToolExecution(sequence int, toolName string, input []byte, errorCode string) ToolExecution {
	return NewToolExecution(sequence, toolName, input, nil, errorCode)
}

func (r AdviceRecord) Validate() error {
	if strings.TrimSpace(r.PromptTemplateVersion) == "" {
		return fmt.Errorf("%w: prompt template version is required", ErrInvalidAdviceAudit)
	}
	if !validModelRole(r.ModelRole) {
		return fmt.Errorf("%w: invalid model role %q", ErrInvalidAdviceAudit, r.ModelRole)
	}
	if r.ReviewerRole != nil && *r.ReviewerRole != llm.ModelRoleReviewer {
		return fmt.Errorf("%w: invalid reviewer role %q", ErrInvalidAdviceAudit, *r.ReviewerRole)
	}
	if !validSHA256(r.RequestSHA256) || !validSHA256(r.AdviceSHA256) {
		return fmt.Errorf("%w: invalid request/advice SHA-256", ErrInvalidAdviceAudit)
	}
	if !validQualityLevel(r.QualityLevel) {
		return fmt.Errorf("%w: invalid quality level %q", ErrInvalidAdviceAudit, r.QualityLevel)
	}
	if !validAdviceStatus(r.Status) {
		return fmt.Errorf("%w: invalid advice status %q", ErrInvalidAdviceAudit, r.Status)
	}
	for i, execution := range r.Tools {
		if err := execution.Validate(); err != nil {
			return fmt.Errorf("%w: tool execution %d: %v", ErrInvalidAdviceAudit, i, err)
		}
	}
	return nil
}

func (e ToolExecution) Validate() error {
	if e.Sequence < 0 || strings.TrimSpace(e.ToolName) == "" || !validSHA256(e.InputSHA256) {
		return ErrInvalidAdviceAudit
	}
	if e.Success {
		if !validSHA256(e.ResultSHA256) || e.ErrorCode != "" {
			return ErrInvalidAdviceAudit
		}
	} else {
		if e.ResultSHA256 != "" || strings.TrimSpace(e.ErrorCode) == "" {
			return ErrInvalidAdviceAudit
		}
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func validModelRole(role llm.ModelRole) bool {
	switch role {
	case llm.ModelRoleFast, llm.ModelRolePlanner, llm.ModelRoleReviewer:
		return true
	default:
		return false
	}
}

func validQualityLevel(level string) bool {
	switch level {
	case "good", "partial", "stale", "unknown":
		return true
	default:
		return false
	}
}

func validAdviceStatus(status AdviceStatus) bool {
	switch status {
	case AdviceStatusSuccess, AdviceStatusBlocked, AdviceStatusError:
		return true
	default:
		return false
	}
}
