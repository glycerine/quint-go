//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
)

func TestREPLFileInputPlatformFallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	if input, supported, err := newREPLFileInput(ctx, read); input != nil || supported || err != nil {
		t.Fatal("platform advertised an unavailable interruptible adapter", input, supported, err)
	}
	if _, err := io.WriteString(write, "42\n"); err != nil {
		t.Fatal(err)
	}
	write.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	host := &replSignalHost{input: read, cancel: cancel, interactive: true}
	ctx = context.WithValue(ctx, replSignalHostKey{}, host)
	var out, stderr bytes.Buffer
	if status := runREPLCommandInput(ctx, &out, &stderr, REPLLoopOptions{Quiet: true}); status != 0 || out.String() != "42\n" || stderr.Len() != 0 {
		t.Fatal(status, out.String(), stderr.String())
	}
	if host.active != nil {
		t.Fatal("fallback registered terminal keepalive")
	}
	host.interrupt()
	if ctx.Err() != context.Canceled {
		t.Fatal("unsupported terminal Ctrl-C must retain cancellation")
	}
}
