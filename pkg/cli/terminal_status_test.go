package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/glycerine/quint-go/pkg/terminal"
)

func TestUpstreamTerminalStatusSegments(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Kind, Output string
		Level              int
		Input              struct{ Status string }
	}
	data, err := os.ReadFile("../testdata/parser/terminal_status.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 10 {
		t.Fatal("expected ten actual upstream status snapshots", len(fixtures))
	}
	stripANSI := regexp.MustCompile("\x1b\\[[0-9;]*m")
	runTiming := regexp.MustCompile(`\([0-9]+ms at [^)]* traces/second\)\.`)
	testTiming := regexp.MustCompile(`\([0-9]+ms\)`)
	for _, fixture := range fixtures {
		t.Run(fixture.Name+"/"+string(rune('0'+fixture.Level)), func(t *testing.T) {
			level := fixture.Level
			ctx := terminal.WithOptions(context.Background(), terminal.Options{ColorLevel: &level})
			var stdout, stderr bytes.Buffer
			var status int
			if fixture.Kind == "test" {
				input := testCommandFile(t, `module M { pure val passingTest=true pure val failingTest=false pure val ignoredTest=7 }`)
				status = commandTest(ctx, []string{input, "--seed=17", "--max-samples=1", "--verbosity=1"}, &stdout, &stderr)
				if status != 1 || !strings.Contains(stderr.String(), "Tests failed") {
					t.Fatal("unexpected test command result", status, stdout.String(), stderr.String())
				}
				// The module header is printed outside outputTestResults and
				// retains its independent existing CLI transcript contract.
				actual := strings.TrimPrefix(stdout.String(), "\n  M\n")
				actual = testTiming.ReplaceAllString(actual, "(17ms)")
				if actual != fixture.Output {
					t.Fatalf("Go %q\nupstream %q", actual, fixture.Output)
				}
				return
			}
			step, invariant := "x'=x+1", "x>=0"
			if fixture.Input.Status == "violation" {
				invariant = "x<1"
			} else if fixture.Input.Status == "error" {
				step = "x'=1/0"
			}
			input := testCommandFile(t, "module M { var x:int action init=x'=0 action step="+step+" val safe="+invariant+" }")
			args := []string{input, "--seed=17", "--max-samples=1", "--max-steps=1", "--verbosity=1", "--invariant=safe"}
			if fixture.Kind == "witnesses" {
				args = append(args, "--witnesses=x >= 0")
			}
			status = commandRun(ctx, args, &stdout, &stderr)
			wantStatus := 0
			if fixture.Input.Status == "violation" || fixture.Input.Status == "error" {
				wantStatus = 1
			}
			if status != wantStatus {
				t.Fatal("unexpected run command result", status, stdout.String(), stderr.String())
			}
			// Compare only upstream status/witness helpers. Backend-specific
			// replay hints retain their established Go text and separate tests.
			var actual strings.Builder
			for _, line := range strings.Split(stdout.String(), "\n") {
				plain := stripANSI.ReplaceAllString(line, "")
				selected := strings.HasPrefix(plain, "["+fixture.Input.Status+"] ")
				if fixture.Kind == "witnesses" {
					selected = plain == "Witnesses:" || strings.Contains(plain, " was witnessed in ")
				}
				if selected {
					actual.WriteString(runTiming.ReplaceAllString(line, "(17ms at 59 traces/second)."))
					actual.WriteByte('\n')
				}
			}
			if actual.String() != fixture.Output {
				t.Fatalf("Go %q\nupstream %q\nfull Go output %q", actual.String(), fixture.Output, stdout.String())
			}
		})
	}
}
