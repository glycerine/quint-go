package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	cmd "github.com/glycerine/apalache-go/pkg/shai/cmdexecutor"
	quint "github.com/glycerine/quint-go/pkg"
)

// TestPublicEntryPointsRejectCancelledContext pins that every exported
// context-taking entry point in this package returns the caller's context
// error, unchanged, before it prepares input or starts the checker service.
func TestPublicEntryPointsRejectCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ex := New()
	calls := []struct {
		name string
		run  func() error
	}{
		{"Check", func() error {
			_, err := ex.Check(ctx, quint.CompilationResult{}, json.RawMessage(`{}`), CheckOptions{})
			return err
		}},
		{"Run", func() error {
			_, err := ex.Run(ctx, cmd.Cmd_CHECK, json.RawMessage(`{}`))
			return err
		}},
		{"FindViolatedInvariants", func() error {
			_, err := FindViolatedInvariants(ctx, quint.CompilationResult{}, []string{"inv"}, json.RawMessage(`{}`))
			return err
		}},
	}
	for _, c := range calls {
		if err := c.run(); !errors.Is(err, context.Canceled) {
			t.Errorf("%s: err=%v, want context.Canceled", c.name, err)
		}
	}
}
