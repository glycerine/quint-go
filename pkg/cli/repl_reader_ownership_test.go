package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
)

// Generic reader ownership is explicit: cancellation is observed when an
// ongoing Read returns. RunREPL must neither close that reader nor abandon a
// blocked helper goroutine. Hosts needing an interruptible prompt must supply
// an input adapter that can itself stop its read.
func TestREPLBlockedInputRemainsCallerOwned(t *testing.T) {
	t.Parallel()
	input := &replOwnershipReader{started: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var release sync.Once
	defer release.Do(func() { close(input.release) })
	type completed struct {
		status int
		stderr string
	}
	done := make(chan completed, 1)
	go func() {
		var stderr bytes.Buffer
		status := RunREPL(ctx, input, io.Discard, &stderr, REPLLoopOptions{Quiet: true})
		done <- completed{status, stderr.String()}
	}()
	<-input.started
	cancel()
	select {
	case result := <-done:
		t.Fatal("RunREPL returned while caller-owned input was still blocked", result)
	default:
	}
	release.Do(func() { close(input.release) })
	result := <-done
	if result.status != 1 || strings.Count(result.stderr, "context canceled") != 1 || input.closed {
		t.Fatal("reader ownership/cancellation changed", result, input.closed)
	}
}

type replOwnershipReader struct {
	started, release chan struct{}
	closed           bool
}

func (r *replOwnershipReader) Read([]byte) (int, error) {
	close(r.started)
	<-r.release
	return 0, io.EOF
}

func (r *replOwnershipReader) Close() error {
	r.closed = true
	return nil
}
