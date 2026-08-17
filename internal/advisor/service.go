package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/analytics"
	"github.com/shawnwu2022/family-finance-os/internal/audit"
	"github.com/shawnwu2022/family-finance-os/internal/llm"
)

var (
	ErrInvalidAdvisorService = errors.New("invalid advisor service")
	ErrAdvisorResponse       = errors.New("invalid advisor response")
)

type BlockReason string

const (
	BlockReasonRequiredToolNotCalled BlockReason = "required_tool_not_called"
	BlockReasonToolExecutionFailed   BlockReason = "tool_execution_failed"
	BlockReasonUnexpectedToolCall    BlockReason = "unexpected_tool_call"
)

type AdviceResult struct {
	Text        string
	Review      string
	Reviewed    bool
	ToolResults []ToolResult
	Blocked     bool
	BlockReason BlockReason
}

type Service struct {
	provider llm.Provider
	registry *Registry
	recorder audit.Recorder
	policy   Policy
	now      func() time.Time
}

func NewService(provider llm.Provider, registry *Registry, recorder audit.Recorder, policy Policy) (*Service, error) {
	if provider == nil || registry == nil || recorder == nil || strings.TrimSpace(policy.PromptTemplateVersion) == "" {
		return nil, ErrInvalidAdvisorService
	}
	return &Service{
		provider: provider,
		registry: registry,
		recorder: recorder,
		policy:   policy,
		now:      time.Now,
	}, nil
}

func (s *Service) Advise(ctx context.Context, request AdviceRequest) (AdviceResult, error) {
	definitions := s.registry.Definitions()
	initialRequest, err := s.policy.InitialRequest(request, definitions)
	if err != nil {
		return AdviceResult{}, err
	}
	requestHash, err := hashAdviceRequest(request)
	if err != nil {
		return AdviceResult{}, err
	}

	initialResponse, err := s.provider.Respond(ctx, initialRequest)
	if err != nil {
		return AdviceResult{}, s.finishError(ctx, request, requestHash, nil, false, "provider_initial_failed", err)
	}

	if len(initialResponse.ToolCalls) == 0 {
		if request.RequireTool {
			result := AdviceResult{Blocked: true, BlockReason: BlockReasonRequiredToolNotCalled}
			if err := s.record(ctx, request, requestHash, result, audit.AdviceStatusBlocked, nil, false); err != nil {
				return AdviceResult{}, err
			}
			return result, nil
		}
		candidate := strings.TrimSpace(initialResponse.Text)
		if candidate == "" {
			return AdviceResult{}, s.finishError(ctx, request, requestHash, nil, false, "provider_empty_response", ErrAdvisorResponse)
		}
		result := AdviceResult{Text: prefixQualityNotice(s.policy.QualityNotice(request.DataQuality), candidate)}
		if request.RequireReview {
			if err := s.applyReview(ctx, request, nil, &result); err != nil {
				return AdviceResult{}, s.finishError(ctx, request, requestHash, nil, true, "review_failed", err)
			}
		}
		if err := s.record(ctx, request, requestHash, result, audit.AdviceStatusSuccess, nil, result.Reviewed); err != nil {
			return AdviceResult{}, err
		}
		return result, nil
	}

	toolResults := make([]ToolResult, 0, len(initialResponse.ToolCalls))
	toolExecutions := make([]audit.ToolExecution, 0, len(initialResponse.ToolCalls))
	for i, call := range initialResponse.ToolCalls {
		name := ToolName(call.Name)
		output, invokeErr := s.registry.Invoke(ctx, name, call.Arguments)
		if invokeErr != nil {
			toolExecutions = append(toolExecutions, audit.NewFailedToolExecution(i, call.Name, call.Arguments, "tool_execution_failed"))
			result := AdviceResult{Blocked: true, BlockReason: BlockReasonToolExecutionFailed}
			if err := s.record(ctx, request, requestHash, result, audit.AdviceStatusBlocked, toolExecutions, false); err != nil {
				return AdviceResult{}, err
			}
			return result, nil
		}
		toolExecutions = append(toolExecutions, audit.NewToolExecution(i, call.Name, call.Arguments, output, ""))
		toolResults = append(toolResults, ToolResult{Name: name, Result: cloneRaw(output)})
	}

	explanationRequest, err := s.policy.ExplanationRequest(request, toolResults)
	if err != nil {
		return AdviceResult{}, s.finishError(ctx, request, requestHash, toolExecutions, false, "explanation_policy_failed", err)
	}
	explanationResponse, err := s.provider.Respond(ctx, explanationRequest)
	if err != nil {
		return AdviceResult{}, s.finishError(ctx, request, requestHash, toolExecutions, false, "provider_explanation_failed", err)
	}
	if len(explanationResponse.ToolCalls) != 0 {
		result := AdviceResult{
			ToolResults: cloneToolResults(toolResults),
			Blocked:     true,
			BlockReason: BlockReasonUnexpectedToolCall,
		}
		if err := s.record(ctx, request, requestHash, result, audit.AdviceStatusBlocked, toolExecutions, false); err != nil {
			return AdviceResult{}, err
		}
		return result, nil
	}
	candidate := strings.TrimSpace(explanationResponse.Text)
	if candidate == "" {
		return AdviceResult{}, s.finishError(ctx, request, requestHash, toolExecutions, false, "provider_empty_explanation", ErrAdvisorResponse)
	}

	result := AdviceResult{
		Text:        prefixQualityNotice(s.policy.QualityNotice(request.DataQuality), candidate),
		ToolResults: cloneToolResults(toolResults),
	}
	if request.RequireReview {
		if err := s.applyReview(ctx, request, toolResults, &result); err != nil {
			return AdviceResult{}, s.finishError(ctx, request, requestHash, toolExecutions, true, "review_failed", err)
		}
	}
	if err := s.record(ctx, request, requestHash, result, audit.AdviceStatusSuccess, toolExecutions, result.Reviewed); err != nil {
		return AdviceResult{}, err
	}
	return result, nil
}

func (s *Service) applyReview(ctx context.Context, request AdviceRequest, toolResults []ToolResult, result *AdviceResult) error {
	candidate := strings.TrimSpace(result.Text)
	if candidate == "" {
		return ErrAdvisorResponse
	}
	reviewRequest, err := s.policy.ReviewRequest(request, candidate, toolResults)
	if err != nil {
		return err
	}
	reviewResponse, err := s.provider.Respond(ctx, reviewRequest)
	if err != nil {
		return err
	}
	if len(reviewResponse.ToolCalls) != 0 {
		return fmt.Errorf("%w: reviewer returned a tool call", ErrAdvisorResponse)
	}
	review := strings.TrimSpace(reviewResponse.Text)
	if review == "" {
		return fmt.Errorf("%w: reviewer returned empty text", ErrAdvisorResponse)
	}
	result.Reviewed = true
	result.Review = review
	return nil
}

func (s *Service) finishError(
	ctx context.Context,
	request AdviceRequest,
	requestHash string,
	tools []audit.ToolExecution,
	reviewerAttempted bool,
	errorCode string,
	cause error,
) error {
	if err := s.recordWithAdviceHash(ctx, request, requestHash, audit.SHA256Hex([]byte(errorCode)), audit.AdviceStatusError, tools, reviewerAttempted); err != nil {
		return fmt.Errorf("advisor failure: %v; record audit: %w", cause, err)
	}
	return cause
}

func (s *Service) record(
	ctx context.Context,
	request AdviceRequest,
	requestHash string,
	result AdviceResult,
	status audit.AdviceStatus,
	tools []audit.ToolExecution,
	reviewed bool,
) error {
	return s.recordWithAdviceHash(ctx, request, requestHash, hashAdviceResult(result), status, tools, reviewed)
}

func (s *Service) recordWithAdviceHash(
	ctx context.Context,
	request AdviceRequest,
	requestHash string,
	adviceHash string,
	status audit.AdviceStatus,
	tools []audit.ToolExecution,
	reviewed bool,
) error {
	record := audit.AdviceRecord{
		CreatedAt:             s.now().UTC(),
		ModelRole:             request.Role,
		DataAsOf:              request.DataQuality.AsOf,
		PromptTemplateVersion: s.policy.PromptTemplateVersion,
		RequestSHA256:         requestHash,
		AdviceSHA256:          adviceHash,
		QualityLevel:          qualityLevelString(request.DataQuality),
		Status:                status,
		Tools:                 append([]audit.ToolExecution(nil), tools...),
	}
	if reviewed {
		role := llm.ModelRoleReviewer
		record.ReviewerRole = &role
	}
	if err := record.Validate(); err != nil {
		return fmt.Errorf("validate advice audit: %w", err)
	}
	if _, err := s.recorder.Record(ctx, record); err != nil {
		return fmt.Errorf("record advice audit: %w", err)
	}
	return nil
}

func hashAdviceRequest(request AdviceRequest) (string, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encode advice request for audit: %w", err)
	}
	return audit.SHA256Hex(encoded), nil
}

func hashAdviceResult(result AdviceResult) string {
	if result.Blocked {
		return audit.SHA256Hex([]byte("blocked:" + string(result.BlockReason)))
	}
	return audit.SHA256Hex([]byte(result.Text + "\nreview:" + result.Review))
}

func qualityLevelString(quality analytics.DataQuality) string {
	switch quality.Level {
	case analytics.QualityGood:
		return "good"
	case analytics.QualityPartial:
		return "partial"
	case analytics.QualityStale:
		return "stale"
	default:
		return "unknown"
	}
}

func prefixQualityNotice(notice, text string) string {
	notice = strings.TrimSpace(notice)
	text = strings.TrimSpace(text)
	if notice == "" {
		return text
	}
	if text == "" {
		return notice
	}
	return notice + "\n\n" + text
}
