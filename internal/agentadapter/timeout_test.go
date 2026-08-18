package agentadapter

import (
	"context"
	"errors"
	"testing"
)

func TestEncodeBackendResultMapsDeadlineToTimeout(t *testing.T) {
	_, err := encodeBackendResult(struct{}{}, context.DeadlineExceeded, nil, "", nil)
	var adapterErr *Error
	if !errors.As(err, &adapterErr) || adapterErr.Code != CodeTimeout {
		t.Fatalf("error=%v want timeout", err)
	}
}

func TestEncodeBackendResultMapsCancellationToTimeout(t *testing.T) {
	_, err := encodeBackendResult(struct{}{}, context.Canceled, nil, "", nil)
	var adapterErr *Error
	if !errors.As(err, &adapterErr) || adapterErr.Code != CodeTimeout {
		t.Fatalf("error=%v want timeout", err)
	}
}
