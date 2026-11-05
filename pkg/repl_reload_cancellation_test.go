package quint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestREPLSessionReloadRejectsCancelledContext pins that Reload returns the
// caller's cancellation before it reloads the loaded source, and that the
// session keeps its current declarations.
func TestREPLSessionReloadRejectsCancelledContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.qnt")
	if err := os.WriteFile(path, []byte("module M { pure val result = 1 }"), 0600); err != nil {
		t.Fatal(err)
	}
	session := NewREPLSession()
	if err := session.Load(context.Background(), path, "M"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := session.Reload(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Reload err=%v, want context.Canceled", err)
	}
	replEvaluate(t, session, "result", "1")
}
