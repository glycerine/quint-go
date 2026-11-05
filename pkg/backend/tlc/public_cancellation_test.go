package tlc

import (
	"context"
	"errors"
	"testing"
)

// TestCheckTLARejectsCancelledContext pins that the raw-TLA entry point returns
// the caller's cancellation before it starts the TLC process.
func TestCheckTLARejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := CheckTLA(ctx, "Spec.tla", "---- MODULE Spec ----\n====", Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
	if result.Complete || result.Outcome != "" {
		t.Fatalf("cancelled CheckTLA produced a result: %+v", result)
	}
}
