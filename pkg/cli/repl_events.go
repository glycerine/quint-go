package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

// REPLInputKind identifies a completed input line, an interactive interruption,
// or end of input. An interrupt abandons unfinished multiline input while
// preserving declarations, runtime state, and accepted expression history.
type REPLInputKind uint8

const (
	REPLInputLine REPLInputKind = iota
	REPLInputInterrupt
	REPLInputEOF
)

// REPLInputEvent carries one input event. Line excludes its final newline. Err
// reports an input failure and takes precedence over Kind and Line. Callers own
// event production and the channel's lifetime; the REPL never closes or drains
// that channel after returning.
type REPLInputEvent struct {
	Kind REPLInputKind
	Line string
	Err  error
}

type replNextInput func(context.Context) (REPLInputEvent, error)

// RunREPL evaluates a sequential input stream. An expression error is reported
// and leaves the session available for the next input. The input reader belongs
// to the caller and is never closed. Cancellation is checked between reads and
// evaluations; callers control interruption of a blocked Read. RunREPLEvents
// supports cancellation while awaiting input without a reader goroutine.
func RunREPL(ctx context.Context, input io.Reader, stdout, stderr io.Writer, options REPLLoopOptions) int {
	if input == nil {
		return 2
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)
	return runREPL(ctx, func(context.Context) (REPLInputEvent, error) {
		if !scanner.Scan() {
			return REPLInputEvent{Kind: REPLInputEOF}, scanner.Err()
		}
		return REPLInputEvent{Line: scanner.Text()}, nil
	}, stdout, stderr, options)
}

// RunREPLEvents evaluates completed lines and interrupts from a caller-owned
// event channel. A closed channel is end of input. Context cancellation stops
// an idle session immediately. No goroutine reads input on the caller's behalf.
func RunREPLEvents(ctx context.Context, input <-chan REPLInputEvent, stdout, stderr io.Writer, options REPLLoopOptions) int {
	if input == nil {
		return 2
	}
	return runREPL(ctx, func(ctx context.Context) (REPLInputEvent, error) {
		if err := ctx.Err(); err != nil {
			return REPLInputEvent{}, err
		}
		var event REPLInputEvent
		select {
		case <-ctx.Done():
			return REPLInputEvent{}, ctx.Err()
		case received, open := <-input:
			if !open {
				return REPLInputEvent{Kind: REPLInputEOF}, nil
			}
			event = received
		}
		if err := ctx.Err(); err != nil {
			return REPLInputEvent{}, err
		}
		if event.Err != nil {
			return REPLInputEvent{}, event.Err
		}
		if event.Kind > REPLInputEOF {
			return REPLInputEvent{}, fmt.Errorf("unknown REPL input event kind %d", event.Kind)
		}
		return event, nil
	}, stdout, stderr, options)
}
