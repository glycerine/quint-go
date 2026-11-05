package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

type replSignalTestInput struct{ interrupts int }

func (*replSignalTestInput) Next(context.Context) (REPLInputEvent, error) {
	return REPLInputEvent{Kind: REPLInputEOF}, nil
}
func (i *replSignalTestInput) Interrupt() { i.interrupts++ }
func (*replSignalTestInput) Close() error { return nil }

func TestREPLSignalRouting(t *testing.T) {
	t.Parallel()
	for _, active := range []bool{false, true} {
		for _, interactive := range []bool{false, true} {
			ctx, cancel := context.WithCancel(context.Background())
			input := &replSignalTestInput{}
			host := &replSignalHost{interactive: interactive, cancel: cancel}
			if active {
				host.active = input
			}
			host.interrupt()
			if active && interactive {
				if ctx.Err() != nil || input.interrupts != 1 {
					t.Fatal("terminal SIGINT lost its session", ctx.Err(), input.interrupts)
				}
			} else if ctx.Err() != context.Canceled || input.interrupts != 0 {
				t.Fatal("command SIGINT did not cancel", active, interactive, ctx.Err(), input.interrupts)
			}
			cancel()
		}
	}
}

func TestREPLProcessEntryAndInputErrors(t *testing.T) {
	t.Parallel()
	var out, stderr bytes.Buffer
	if status := ExecuteWithSignals(context.Background(), []string{"--version"}, nil, &out, &stderr); status != 0 || out.Len() == 0 || stderr.Len() != 0 {
		t.Fatal(status, out.String(), stderr.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host := &replSignalHost{cancel: cancel}
	bound := context.WithValue(ctx, replSignalHostKey{}, host)
	if status := runREPLCommandInput(bound, io.Discard, io.Discard, REPLLoopOptions{}); status != 2 {
		t.Fatal("missing host input accepted", status)
	}
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	read.Close()
	write.Close()
	if _, err := replInputFD(read); err == nil {
		t.Fatal("closed process input accepted")
	}
}

// ExecuteWithSignals must report the caller's cancellation before running the
// command, so a pre-cancelled context produces no command output.
func TestExecuteWithSignalsRejectsCancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	status := ExecuteWithSignals(ctx, []string{"--version"}, nil, &stdout, &stderr)
	if status != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "context canceled") {
		t.Fatal(status, stdout.String(), stderr.String())
	}
}
