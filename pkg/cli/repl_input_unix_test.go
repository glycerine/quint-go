//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func replInputPipe(t *testing.T, ctx context.Context) (*os.File, *os.File, replFileInput) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { read.Close(); write.Close() })
	input, supported, err := newREPLFileInput(ctx, read)
	if !supported || err != nil {
		t.Fatal(supported, err)
	}
	t.Cleanup(func() { input.Close() })
	return read, write, input
}

func TestREPLFileInputLinesEOFAndBorrowedFlags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	fd, err := replInputFD(read)
	if err != nil {
		t.Fatal(err)
	}
	input, supported, err := newREPLFileInput(ctx, read)
	if err != nil || !supported {
		t.Fatal(supported, err)
	}
	defer input.Close()
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(write, "1\r\n\n42"); err != nil {
		t.Fatal(err)
	}
	if err = write.Close(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []REPLInputEvent{{Line: "1"}, {Line: ""}, {Line: "42"}, {Kind: REPLInputEOF}} {
		got, err := input.Next(ctx)
		if err != nil || got != want {
			t.Fatal(got, err, "want", want)
		}
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	if err := input.Close(); err != nil {
		t.Fatal("close must be idempotent", err)
	}
	input.Interrupt() // A late signal cannot write a reused/closed descriptor.
	if _, err := input.Next(ctx); !errors.Is(err, os.ErrClosed) {
		t.Fatal("closed adapter accepted input", err)
	}
	after, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil || flags != after {
		t.Fatal("host changed/closed borrowed input", flags, after, err)
	}
}

func TestREPLFileInterruptDiscardsOnlyUnfinishedInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, write, input := replInputPipe(t, ctx)
	if _, err := io.WriteString(write, "1\n2\npartial"); err != nil {
		t.Fatal(err)
	}
	if got, err := input.Next(ctx); err != nil || got.Line != "1" {
		t.Fatal(got, err)
	}
	input.Interrupt()
	if got, err := input.Next(ctx); err != nil || got.Kind != REPLInputInterrupt {
		t.Fatal(got, err)
	}
	if got, err := input.Next(ctx); err != nil || got.Line != "2" {
		t.Fatal("interrupt discarded an already completed line", got, err)
	}
	if _, err := io.WriteString(write, "42\n"); err != nil {
		t.Fatal(err)
	}
	write.Close()
	if got, err := input.Next(ctx); err != nil || got.Line != "42" {
		t.Fatal("unfinished input survived interruption", got, err)
	}
	if got, err := input.Next(ctx); err != nil || got.Kind != REPLInputEOF {
		t.Fatal(got, err)
	}
}

func TestREPLFileInputBoundsLongLines(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, write, input := replInputPipe(t, ctx)
	done := make(chan error, 1)
	go func() {
		_, err := write.Write(bytes.Repeat([]byte{'x'}, 16*1024*1024))
		write.Close()
		done <- err
	}()
	if _, err := input.Next(ctx); !errors.Is(err, bufio.ErrTooLong) {
		t.Fatal("file host lost the reader's line bound", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestREPLFileInputWakesOnInterruptAndCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	read, write, input := replInputPipe(t, ctx)
	type result struct {
		event REPLInputEvent
		err   error
	}
	done := make(chan result, 1)
	next := func() { event, err := input.Next(ctx); done <- result{event, err} }
	go next()
	input.Interrupt()
	got := <-done
	if got.err != nil || got.event.Kind != REPLInputInterrupt {
		t.Fatal(got)
	}
	// Multiple interrupts remain events even when their wake bytes coalesce.
	input.Interrupt()
	input.Interrupt()
	for range 2 {
		if got, err := input.Next(ctx); err != nil || got.Kind != REPLInputInterrupt {
			t.Fatal(got, err)
		}
	}
	if _, err := io.WriteString(write, "42\n"); err != nil {
		t.Fatal(err)
	}
	if got, err := input.Next(ctx); err != nil || got.Line != "42" {
		t.Fatal(got, err)
	}
	go next()
	cancel()
	if got := <-done; !errors.Is(got.err, context.Canceled) {
		t.Fatal("blocked poll ignored context", got)
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := read.Stat(); err != nil {
		t.Fatal("context cancellation closed borrowed input", err)
	}
}

func TestREPLFileHostSignalCancellationAndJoinedWorker(t *testing.T) {
	t.Parallel()
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			t.Parallel()
			read, write, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer read.Close()
			defer write.Close()
			signals := make(chan os.Signal, 1)
			signals <- signal
			var out, stderr bytes.Buffer
			status := executeWithSignalEvents(context.Background(), []string{"repl", "-q", "--seed=17"}, read, &out, &stderr, signals)
			if status != 1 || !strings.Contains(stderr.String(), "context canceled") {
				t.Fatal(status, out.String(), stderr.String())
			}
			if _, err := read.Stat(); err != nil {
				t.Fatal("process signal closed borrowed input", err)
			}
		})
	}
}

func TestREPLFileHostUsesSharedStateLoop(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	read, write, input := replInputPipe(t, ctx)
	if _, err := io.WriteString(write, "var x:int\nx'=1\nx\n"); err != nil {
		t.Fatal(err)
	}
	write.Close()
	var out, stderr bytes.Buffer
	status := runREPL(ctx, input.Next, &out, &stderr, REPLLoopOptions{Quiet: true})
	if status != 0 || out.String() != "\ntrue\n1\n\n" || stderr.Len() != 0 {
		t.Fatal(status, out.String(), stderr.String())
	}
	if _, err := read.Stat(); err != nil {
		t.Fatal("REPL closed borrowed input", err)
	}
}

func TestREPLProcessHostReadsBorrowedFile(t *testing.T) {
	t.Parallel()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	if _, err := io.WriteString(write, "pure val x=40\nx+2\n"); err != nil {
		t.Fatal(err)
	}
	write.Close()
	var out, stderr bytes.Buffer
	signals := make(chan os.Signal)
	close(signals)
	status := executeWithSignalEvents(context.Background(), []string{"repl", "-q", "--seed=17"}, read, &out, &stderr, signals)
	if status != 0 || out.String() != "\n42\n\n" || stderr.Len() != 0 {
		t.Fatal(status, out.String(), stderr.String())
	}
	if _, err := read.Stat(); err != nil {
		t.Fatal("process host closed borrowed input", err)
	}
	read.Close()
	if _, _, err := newREPLFileInput(context.Background(), read); err == nil {
		t.Fatal("closed descriptor accepted an input adapter")
	}
}
