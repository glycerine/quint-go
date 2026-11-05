package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// terminalKnownDeviations lists whole-command fixtures (by name) that the Go port
// documents as deliberate differences. Each case still runs; it fails if it
// starts matching the fixture, so the entry is removed once it matches.
var terminalKnownDeviations = map[string]string{
	// The fixtures were generated with the TypeScript backend pinned, which prints
	// no reproduce hint on a runtime error. The Rust default (rust/run.md) prints
	// "Use --seed=... --backend=rust to reproduce."; Go prints the same hint with
	// --backend=go.
	"run_runtime_error":        "generator pins TypeScript backend (no hint on error); Go prints the Rust-style hint with --backend=go",
	"run_nested_runtime_error": "generator pins TypeScript backend (no hint on error); Go prints the Rust-style hint with --backend=go",
}

func TestUpstreamWholeTerminalCommandTranscripts(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Cases []struct {
			Name, Command, Source, Stdout, Stderr string
			Flags                                 []string
			Level, Status                         int
		}
	}
	data, err := os.ReadFile("../testdata/parser/cli_terminal_transcripts.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Cases) != 22 {
		t.Fatalf("expected twenty-two whole command transcripts, got %d", len(fixtures.Cases))
	}
	runTiming := regexp.MustCompile(`\(\d+ms at [^)]* traces/second\)`)
	testTiming := regexp.MustCompile(`\(\d+ms\)`)
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name+"/"+string(rune('0'+fixture.Level)), func(t *testing.T) {
			t.Parallel()
			input := filepath.Join(t.TempDir(), "fixture.qnt")
			if err := os.WriteFile(input, []byte(fixture.Source), 0600); err != nil {
				t.Fatal(err)
			}
			ctx := WithTerminalOptions(context.Background(), TerminalOptions{ColorLevel: &fixture.Level})
			args := append([]string{fixture.Command, input}, fixture.Flags...)
			var stdout, stderr bytes.Buffer
			status := Execute(ctx, args, &stdout, &stderr)
			normalize := func(text string) string {
				// The established Go replay hint uses an anchored, unqualified
				// selector and explicit Go backend; keep that correction visible
				// in its dedicated tests while comparing the upstream transcript.
				text = testReferenceNormalizeText(text, input)
				text = runTiming.ReplaceAllString(text, "(<elapsed>ms at <rate> traces/second)")
				return testTiming.ReplaceAllString(text, "(<elapsed>ms)")
			}
			actualOut, actualErr := normalize(stdout.String()), normalize(stderr.String())
			matches := status == fixture.Status && actualOut == fixture.Stdout && actualErr == fixture.Stderr
			if reason, known := terminalKnownDeviations[fixture.Name]; known {
				// A known deviation must still deviate. If it now matches, remove its entry.
				if matches {
					t.Errorf("known deviation now matches upstream; remove it from terminalKnownDeviations (%s)", reason)
				}
				t.Skip("known deviation: " + reason)
			}
			if !matches {
				t.Fatalf("status=%d want=%d\nstdout=%q\nwant=%q\nstderr=%q\nwant=%q", status, fixture.Status, actualOut, fixture.Stdout, actualErr, fixture.Stderr)
			}
		})
	}
}
