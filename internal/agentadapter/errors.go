package agentadapter

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeForbidden        ErrorCode = "forbidden"
	CodeInvalidArgument  ErrorCode = "invalid_argument"
	CodeToolNotFound     ErrorCode = "tool_not_found"
	CodeDataUnavailable  ErrorCode = "data_unavailable"
	CodeDataPartial      ErrorCode = "data_partial"
	CodeAuditUnavailable ErrorCode = "audit_unavailable"
	CodeTimeout          ErrorCode = "timeout"
	CodeBusy             ErrorCode = "busy"
	CodeInternal         ErrorCode = "internal"
)

type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsCode(err error, code ErrorCode) bool {
	var target *Error
	return errors.As(err, &target) && target.Code == code
}
