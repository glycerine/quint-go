package apalache

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckerTuningErrorExplainsG3UnknownKeys(t *testing.T) {
	base := errors.New("invalid g3 tuning: set g3 symbol option encoding: unknown parameter 'encoding'")
	got := checkerTuningError(base)
	if !strings.Contains(got.Error(), "g3 does not expose this checker tuning key") || !errors.Is(got, base) {
		t.Fatalf("g3-unknown message not explained or not wrapped: %v", got)
	}
	other := errors.New("invalid g3 tuning: g3: AST source precondition")
	if checkerTuningError(other) != other {
		t.Fatal("non-unknown g3 rejection was rewritten")
	}
	if checkerTuningError(nil) != nil {
		t.Fatal("nil error was rewritten")
	}
}
