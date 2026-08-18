package agentadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const auditFinalizeTimeout = 2 * time.Second

type CallMetadata struct {
	Protocol        string
	ProtocolVersion string
	ClientName      string
	ClientVersion   string
}

type AuditAttempt struct {
	CreatedAt       time.Time
	PrincipalKind   string
	HouseholdID     int64
	Protocol        string
	ProtocolVersion string
	ClientName      string
	ClientVersion   string
	ToolName        ToolName
	InputSHA256     string
}

type AuditSuccess struct {
	OutputSHA256 string
	DataAsOf     *time.Time
	DurationMS   int64
}

type AuditFailure struct {
	ErrorCode  ErrorCode
	DurationMS int64
}

type AuditRecorder interface {
	Start(context.Context, AuditAttempt) (int64, error)
	CompleteSuccess(context.Context, int64, AuditSuccess) error
	CompleteFailure(context.Context, int64, AuditFailure) error
}

type AuditedService struct {
	service  *Service
	recorder AuditRecorder
	now      func() time.Time
}

func NewAudited(service *Service, recorder AuditRecorder, now func() time.Time) (*AuditedService, error) {
	if service == nil {
		return nil, fmt.Errorf("agentadapter: service is required")
	}
	if recorder == nil {
		return nil, fmt.Errorf("agentadapter: audit recorder is required")
	}
	if now == nil {
		return nil, fmt.Errorf("agentadapter: audit clock is required")
	}
	return &AuditedService{service: service, recorder: recorder, now: now}, nil
}

func (s *AuditedService) Definitions() []ToolDefinition {
	if s == nil || s.service == nil {
		return nil
	}
	return s.service.Definitions()
}

func (s *AuditedService) Call(ctx context.Context, principal Principal, metadata CallMetadata, name ToolName, arguments json.RawMessage) (Result, error) {
	if strings.TrimSpace(metadata.Protocol) == "" || strings.TrimSpace(metadata.ProtocolVersion) == "" {
		return Result{}, adapterError(CodeInvalidArgument, "audit protocol metadata is required", nil)
	}

	startedAt := s.now().UTC()
	auditID, err := s.recorder.Start(ctx, AuditAttempt{
		CreatedAt:       startedAt,
		PrincipalKind:   principal.Kind,
		HouseholdID:     principal.HouseholdID,
		Protocol:        metadata.Protocol,
		ProtocolVersion: metadata.ProtocolVersion,
		ClientName:      metadata.ClientName,
		ClientVersion:   metadata.ClientVersion,
		ToolName:        name,
		InputSHA256:     auditInputSHA256(arguments),
	})
	if err != nil || auditID <= 0 {
		return Result{}, adapterError(CodeAuditUnavailable, "agent tool audit is unavailable", err)
	}

	result, callErr := s.service.Call(ctx, principal, name, arguments)
	durationMS := s.now().UTC().Sub(startedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}

	auditCtx, cancelAudit := context.WithTimeout(context.WithoutCancel(ctx), auditFinalizeTimeout)
	defer cancelAudit()

	if callErr != nil {
		code := CodeInternal
		var adapterErr *Error
		if errors.As(callErr, &adapterErr) {
			code = adapterErr.Code
		}
		if err := s.recorder.CompleteFailure(auditCtx, auditID, AuditFailure{ErrorCode: code, DurationMS: durationMS}); err != nil {
			return Result{}, adapterError(CodeAuditUnavailable, "agent tool audit is unavailable", err)
		}
		return Result{}, callErr
	}

	if err := s.recorder.CompleteSuccess(auditCtx, auditID, AuditSuccess{
		OutputSHA256: sha256Hex(result.Data),
		DataAsOf:     cloneTime(result.AsOf),
		DurationMS:   durationMS,
	}); err != nil {
		return Result{}, adapterError(CodeAuditUnavailable, "agent tool audit is unavailable", err)
	}

	result.AuditID = "audit_" + strconv.FormatInt(auditID, 36)
	return result, nil
}

func auditInputSHA256(raw json.RawMessage) string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err == nil {
		var trailing any
		if err := decoder.Decode(&trailing); err == io.EOF {
			if canonical, err := json.Marshal(value); err == nil {
				return sha256Hex(canonical)
			}
		}
	}
	invalid := append([]byte("invalid-json\x00"), raw...)
	return sha256Hex(invalid)
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
