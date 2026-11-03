package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func TestUpstreamREPLInterruptEvents(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Cases []struct {
			Name, Output, Stderr string
			Verbosity            int
			Steps                []struct {
				Input     string
				Interrupt bool
			}
		}
	}
	data, err := os.ReadFile("../testdata/parser/repl_interrupt.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	prompts := regexp.MustCompile(`(?m)^(?:(?:>>> |\.\.\. ))+`)
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			var events []REPLInputEvent
			for _, step := range fixture.Steps {
				if step.Interrupt {
					events = append(events, REPLInputEvent{Kind: REPLInputInterrupt})
				} else {
					for _, line := range strings.Split(strings.TrimSuffix(step.Input, "\n"), "\n") {
						events = append(events, REPLInputEvent{Line: line})
					}
				}
			}
			events = append(events, REPLInputEvent{Line: ".exit"})
			input := replEventChannel(events...)
			var out, stderr bytes.Buffer
			status := RunREPLEvents(context.Background(), input, &out, &stderr, REPLLoopOptions{Verbosity: fixture.Verbosity})
			text := strings.TrimPrefix(out.String(), "Quint Go REPL\nType \".exit\" to exit, or \".help\" for more information\n")
			text = prompts.ReplaceAllString(text, "")
			if status != 0 || text != fixture.Output || stderr.String() != fixture.Stderr {
				t.Fatalf("status=%d stdout=%q want=%q stderr=%q want=%q", status, text, fixture.Output, stderr.String(), fixture.Stderr)
			}
		})
	}
}

func replEventChannel(events ...REPLInputEvent) <-chan REPLInputEvent {
	input := make(chan REPLInputEvent, len(events))
	for _, event := range events {
		input <- event
	}
	close(input)
	return input
}

func TestREPLEventBoundaries(t *testing.T) {
	t.Parallel()
	failure := errors.New("input failed")
	for _, test := range []struct {
		name         string
		events       []REPLInputEvent
		status       int
		out, message string
	}{
		{"closed", nil, 0, "", ""},
		{"eof", []REPLInputEvent{{Kind: REPLInputEOF}, {Line: "42"}}, 0, "", ""},
		{"pending_eof", []REPLInputEvent{{Line: "{"}, {Kind: REPLInputEOF}}, 1, "", "Unexpected end of input"},
		{"pending_interrupt_eof", []REPLInputEvent{{Line: "{"}, {Kind: REPLInputInterrupt}, {Kind: REPLInputEOF}}, 0, " <cancelled>\n", ""},
		{"invalid", []REPLInputEvent{{Kind: REPLInputKind(255)}}, 1, "", "unknown REPL input event kind 255"},
		{"input_error", []REPLInputEvent{{Err: failure, Line: "42"}}, 1, "", "input failed"},
		{"interrupt_error", []REPLInputEvent{{Kind: REPLInputInterrupt, Err: failure}}, 1, "", "input failed"},
		{"exit", []REPLInputEvent{{Line: ".exit"}, {Line: "42"}}, 0, "", ""},
		{"abandoned_definition", []REPLInputEvent{{Line: "{"}, {Line: "pure val forgotten=1"}, {Kind: REPLInputInterrupt}, {Line: "forgotten"}, {Line: "42"}}, 0, " <cancelled>\n42\n", "Name 'forgotten' not found"},
		{"paste_interrupt", []REPLInputEvent{{Line: ">>> {"}, {Line: "... 1"}, {Kind: REPLInputInterrupt}, {Line: "42"}}, 0, " <cancelled>\n42\n", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var out, stderr bytes.Buffer
			status := RunREPLEvents(context.Background(), replEventChannel(test.events...), &out, &stderr, REPLLoopOptions{Quiet: true})
			if status != test.status || out.String() != test.out || (test.message == "" && stderr.Len() != 0) || (test.message != "" && !strings.Contains(stderr.String(), test.message)) {
				t.Fatal(status, out.String(), stderr.String())
			}
		})
	}
	if status := RunREPLEvents(context.Background(), nil, io.Discard, io.Discard, REPLLoopOptions{}); status != 2 {
		t.Fatal("nil event channel accepted", status)
	}
	if status := RunREPLEvents(context.Background(), replEventChannel(), nil, io.Discard, REPLLoopOptions{}); status != 2 {
		t.Fatal("nil writer accepted", status)
	}
	if status := RunREPLEvents(context.Background(), replEventChannel(REPLInputEvent{Kind: REPLInputInterrupt}), &executionFailWriter{failAt: 1, err: failure}, io.Discard, REPLLoopOptions{Quiet: true}); status != 1 {
		t.Fatal("interrupt writer failure was lost", status)
	}
}

func TestREPLEventsCancellationDoesNotOwnProducer(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input := make(chan REPLInputEvent, 1)
	ready := make(chan struct{})
	var readyOnce sync.Once
	type completed struct {
		status int
		stderr string
	}
	done := make(chan completed, 1)
	go func() {
		var stderr bytes.Buffer
		status := RunREPLEvents(ctx, input, replEventReadyWriter{ready, &readyOnce}, &stderr, REPLLoopOptions{Verbosity: 1})
		done <- completed{status, stderr.String()}
	}()
	<-ready
	cancel()
	result := <-done
	if result.status != 1 || strings.Count(result.stderr, "context canceled") != 1 {
		t.Fatal("idle event loop did not cancel", result)
	}
	// A returned loop must not close or drain a channel owned by its producer.
	input <- REPLInputEvent{Line: "42"}
	if len(input) != 1 {
		t.Fatal("returned event loop consumed producer input")
	}
	close(input)
}

type replEventReadyWriter struct {
	ready chan struct{}
	once  *sync.Once
}

func (w replEventReadyWriter) Write(data []byte) (int, error) {
	if bytes.Contains(data, []byte(">>> ")) {
		w.once.Do(func() { close(w.ready) })
	}
	return len(data), nil
}
