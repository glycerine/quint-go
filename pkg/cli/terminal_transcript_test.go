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
			if actualOut, actualErr := normalize(stdout.String()), normalize(stderr.String()); status != fixture.Status || actualOut != fixture.Stdout || actualErr != fixture.Stderr {
				t.Fatalf("status=%d want=%d\nstdout=%q\nwant=%q\nstderr=%q\nwant=%q", status, fixture.Status, actualOut, fixture.Stdout, actualErr, fixture.Stderr)
			}
		})
	}
}
