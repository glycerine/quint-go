package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/term"
)

type replFileInput interface {
	Next(context.Context) (REPLInputEvent, error)
	Interrupt()
	Close() error
}

type replSignalHostKey struct{}
type replSignalHost struct {
	input       *os.File
	interactive bool
	cancel      context.CancelFunc
	mu          sync.Mutex
	active      replFileInput
}

// ExecuteWithSignals is the process entry point. It owns signal notification
// and its signal worker, while stdin remains borrowed. Unix REPL input uses an
// interruptible file adapter: Ctrl-C at a terminal abandons pending input and
// keeps the session; other commands and SIGTERM cancel their execution.
// Platforms without that adapter retain cancellation on Ctrl-C.
func ExecuteWithSignals(ctx context.Context, args []string, stdin *os.File, stdout, stderr io.Writer) int {
	signals := make(chan os.Signal, 16)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	return executeWithSignalEvents(ctx, args, stdin, stdout, stderr, signals)
}

func executeWithSignalEvents(parent context.Context, args []string, stdin *os.File, stdout, stderr io.Writer, signals <-chan os.Signal) int {
	ctx, cancel := context.WithCancel(parent)
	host := &replSignalHost{input: stdin, cancel: cancel}
	if stdin != nil {
		if fd, err := replInputFD(stdin); err == nil {
			host.interactive = term.IsTerminal(fd)
		}
	}
	ctx = context.WithValue(ctx, replSignalHostKey{}, host)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case sig, open := <-signals:
				if !open {
					return
				}
				if sig == os.Interrupt {
					host.interrupt()
				} else {
					cancel()
				}
			}
		}
	}()
	code := Execute(ctx, args, stdout, stderr)
	cancel()
	<-done
	return code
}

// SyscallConn avoids File.Fd's conversion of a borrowed descriptor to blocking
// mode. The process host keeps the file alive and is its sole reader.
func replInputFD(input *os.File) (int, error) {
	connection, err := input.SyscallConn()
	if err != nil {
		return 0, err
	}
	var fd int
	err = connection.Control(func(descriptor uintptr) { fd = int(descriptor) })
	return fd, err
}

func (h *replSignalHost) interrupt() {
	h.mu.Lock()
	if h.interactive && h.active != nil {
		h.active.Interrupt()
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	h.cancel()
}

func runREPLCommandInput(ctx context.Context, stdout, stderr io.Writer, options REPLLoopOptions) int {
	host, _ := ctx.Value(replSignalHostKey{}).(*replSignalHost)
	if host == nil {
		return RunREPL(ctx, os.Stdin, stdout, stderr, options)
	}
	if host.input == nil {
		fmt.Fprintln(stderr, "repl: process input is unavailable")
		return 2
	}
	input, supported, err := newREPLFileInput(ctx, host.input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !supported {
		return RunREPL(ctx, host.input, stdout, stderr, options)
	}
	host.mu.Lock()
	host.active = input
	host.mu.Unlock()
	defer func() {
		host.mu.Lock()
		host.active = nil
		host.mu.Unlock()
		input.Close()
	}()
	return runREPL(ctx, input.Next, stdout, stderr, options)
}
