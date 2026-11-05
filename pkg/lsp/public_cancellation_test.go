package lsp

import (
	"context"
	"errors"
	"testing"
)

// TestPublicEntryPointsRejectCancelledContext pins that Serve and Wait return
// the caller's cancellation before they read input or wait on work.
//
// Handle is deliberately not in this set: a request whose context is already
// cancelled is answered with a JSON-RPC RequestCancelled (-32800) response, as
// the LSP protocol requires. TestServerCancelledRequestAndWorkerShutdown and
// TestSignatureHelpCurrentBuffersAndMissingAnalysis pin that behavior.
func TestPublicEntryPointsRejectCancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	server := NewServer(Options{})
	defer server.Close()
	if err := server.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait err=%v, want context.Canceled", err)
	}
	if err := server.Serve(ctx, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Serve err=%v, want context.Canceled", err)
	}
}
