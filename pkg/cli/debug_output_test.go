package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamDebugCommandTranscripts(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Command, Source, Input string
		Verbosity, MaxSamples        int
		Out                          bool
		Status                       int
		Stdout, Stderr               string
		Passed, Failed, Ignored      []string
	}
	data, err := os.ReadFile("testdata/debug_output.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			var status int
			if fixture.Command == "repl" {
				status = RunREPL(context.Background(), strings.NewReader(fixture.Input), &stdout, &stderr, REPLLoopOptions{Verbosity: fixture.Verbosity})
			} else {
				input := filepath.Join(t.TempDir(), "fixture.qnt")
				if err := os.WriteFile(input, []byte(fixture.Source), 0600); err != nil {
					t.Fatal(err)
				}
				args := []string{input, "--seed=1", fmt.Sprintf("--max-samples=%d", fixture.MaxSamples), fmt.Sprintf("--verbosity=%d", fixture.Verbosity)}
				output := filepath.Join(t.TempDir(), "stage.json")
				if fixture.Out {
					args = append(args, "--out="+output)
				}
				status = commandTest(context.Background(), args, &stdout, &stderr)
				if fixture.Out {
					data, err := os.ReadFile(output)
					if err != nil {
						t.Fatal(err)
					}
					var stage struct{ Passed, Failed, Ignored []string }
					if err = json.Unmarshal(data, &stage); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(stage.Passed, fixture.Passed) || !reflect.DeepEqual(stage.Failed, fixture.Failed) || !reflect.DeepEqual(stage.Ignored, fixture.Ignored) {
						t.Errorf("stage results: got %+v; want passed=%v failed=%v ignored=%v", stage, fixture.Passed, fixture.Failed, fixture.Ignored)
					}
				}
			}
			if status != fixture.Status || testReferenceDuration.ReplaceAllString(stdout.String(), "(0ms)") != fixture.Stdout || stderr.String() != fixture.Stderr {
				t.Fatalf("status=%d stdout=%q stderr=%q; want status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String(), fixture.Status, fixture.Stdout, fixture.Stderr)
			}
		})
	}
}

func TestDebugCommandWriterFailures(t *testing.T) {
	t.Parallel()
	input := filepath.Join(t.TempDir(), "M.qnt")
	if err := os.WriteFile(input, []byte(`module M { val sampleTest=q::debug("sample",true) }`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, short := range []bool{false, true} {
		t.Run(fmt.Sprint("short=", short), func(t *testing.T) {
			t.Parallel()
			writer := &debugFailWriter{short: short}
			var stderr bytes.Buffer
			if status := commandTest(context.Background(), []string{input, "--verbosity=0", "--seed=1"}, writer, &stderr); status != 1 || writer.calls != 1 || !strings.Contains(stderr.String(), "error: writing test output:") {
				t.Fatalf("test status=%d calls=%d stderr=%q", status, writer.calls, stderr.String())
			}
			writer = &debugFailWriter{short: short}
			stderr.Reset()
			options := REPLLoopOptions{Quiet: true, Commands: []string{`q::debug("first",1)`, `q::debug("second",2)`}}
			if status := RunREPL(context.Background(), strings.NewReader(""), writer, &stderr, options); status != 1 || writer.calls != 1 {
				t.Fatalf("repl status=%d calls=%d stderr=%q", status, writer.calls, stderr.String())
			}
		})
	}
}

func TestDebugREPLCancellationKeepsContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderr bytes.Buffer
	writer := debugWriteFunc(func(data []byte) (int, error) {
		n, err := stdout.Write(data)
		if strings.HasPrefix(string(data), "> first ") {
			cancel()
		}
		return n, err
	})
	options := REPLLoopOptions{Quiet: true, Commands: []string{`q::debug("first",1)`, `q::debug("second",2)`}}
	if status := RunREPL(ctx, strings.NewReader(""), writer, &stderr, options); status != 1 || !strings.Contains(stdout.String(), "> first 1\n") || strings.Contains(stdout.String(), "second") || !strings.Contains(stderr.String(), "context canceled") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}

type debugFailWriter struct {
	short bool
	calls int
}

func (w *debugFailWriter) Write(data []byte) (int, error) {
	w.calls++
	if w.short {
		return len(data) - 1, nil
	}
	return 0, errors.New("debug output unavailable")
}

type debugWriteFunc func([]byte) (int, error)

func (f debugWriteFunc) Write(data []byte) (int, error) { return f(data) }

var _ io.Writer = debugWriteFunc(nil)
