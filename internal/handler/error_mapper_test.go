package handler

import (
	"errors"
	"testing"

	connectrpc "connectrpc.com/connect"
)

func TestToErrorWithClassifier(t *testing.T) {
	err := toError(errors.New("invalid"), func(err error) (connectrpc.Code, bool) {
		return connectrpc.CodeInvalidArgument, true
	})
	if got := connectrpc.CodeOf(err); got != connectrpc.CodeInvalidArgument {
		t.Fatalf("expected %v, got %v", connectrpc.CodeInvalidArgument, got)
	}
}

func TestToErrorDefaultInternal(t *testing.T) {
	err := toError(errors.New("boom"))
	if got := connectrpc.CodeOf(err); got != connectrpc.CodeInternal {
		t.Fatalf("expected %v, got %v", connectrpc.CodeInternal, got)
	}
}
