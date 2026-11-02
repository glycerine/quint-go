package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

var runElapsedTranscript = regexp.MustCompile(`\(\d+ms at (?:Infinity|NaN|\d+) traces/second\)`)

func runTranscriptJSON(t *testing.T, data []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err, string(data))
	}
	return value
}

func TestUpstreamRunTranscripts(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source   string
		Args           []string
		ExitCode       int
		Stdout, Stderr string
		Stage          json.RawMessage
		Traces         []struct {
			Filename string
			Trace    json.RawMessage
		}
	}
	data, err := os.ReadFile("testdata/run_transcripts.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			source := filepath.Join(directory, "Counter.qnt")
			stage := filepath.Join(directory, "stage.json")
			trace := filepath.Join(directory, "trace-{seq}.itf.json")
			if err := os.WriteFile(source, []byte(fixture.Source+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{source}
			for _, argument := range fixture.Args {
				argument = strings.ReplaceAll(argument, "${STAGE}", stage)
				argument = strings.ReplaceAll(argument, "${TRACE}", trace)
				args = append(args, argument)
			}
			var stdout, stderr bytes.Buffer
			status := commandRun(context.Background(), args, &stdout, &stderr)
			normalize := func(value string) string {
				value = strings.ReplaceAll(value, source, "${SOURCE}")
				value = strings.ReplaceAll(value, directory, "${DIRECTORY}")
				return runElapsedTranscript.ReplaceAllString(value, "(<elapsed>ms at <rate> traces/second)")
			}
			if status != fixture.ExitCode || normalize(stdout.String()) != fixture.Stdout || normalize(stderr.String()) != fixture.Stderr {
				t.Fatalf("status %d want %d\nstdout: %q\nwant:   %q\nstderr: %q\nwant:   %q", status, fixture.ExitCode, normalize(stdout.String()), fixture.Stdout, normalize(stderr.String()), fixture.Stderr)
			}
			if fixture.Stage != nil {
				data, err := os.ReadFile(stage)
				if err != nil {
					t.Fatal(err)
				}
				actual := runTranscriptJSON(t, []byte(normalize(string(data)))).(map[string]any)
				projection := map[string]any{}
				for _, key := range []string{"stage", "status", "trace", "seed", "errors"} {
					if value, present := actual[key]; present {
						projection[key] = value
					}
				}
				if want := runTranscriptJSON(t, fixture.Stage); !reflect.DeepEqual(projection, want) {
					t.Fatalf("stage got %#v\nwant %#v", projection, want)
				}
			}
			files, err := filepath.Glob(filepath.Join(directory, "trace-*.itf.json"))
			if err != nil || len(files) != len(fixture.Traces) {
				t.Fatal("ITF trace count", files, len(fixture.Traces), err)
			}
			for _, expected := range fixture.Traces {
				data, err := os.ReadFile(filepath.Join(directory, expected.Filename))
				if err != nil {
					t.Fatal(err)
				}
				actual := runTranscriptJSON(t, data).(map[string]any)
				metadata := actual["#meta"].(map[string]any)
				metadata["source"] = "${SOURCE}"
				metadata["description"] = "Created by Quint on ${CREATED}"
				metadata["timestamp"] = json.Number("0")
				if want := runTranscriptJSON(t, expected.Trace); !reflect.DeepEqual(actual, want) {
					t.Fatalf("ITF got %#v\nwant %#v", actual, want)
				}
			}
		})
	}
}

func TestRunIndividualInvariantIsolationAndRuntimeFailure(t *testing.T) {
	t.Parallel()
	parsed, err := quint.ParseModulesWithLocations("report.qnt", "module M { var x:int val positive=x>0 val small=x<2 val broken=assert(false) }")
	if err != nil {
		t.Fatal(err)
	}
	state, err := quint.ParseExpression(`Rec("x",2)`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := runViolatedInvariants(context.Background(), parsed.Modules, parsed.IDs, "M", []string{"positive", "small", "broken"}, state)
	if err != nil || !reflect.DeepEqual(got, []int{1}) {
		t.Fatal(got, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = runViolatedInvariants(canceled, parsed.Modules, parsed.IDs, "M", []string{"small"}, state); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestRunOutputNumbersAndDiagnosticDeduplication(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		samples  int
		elapsed  int64
		expected string
	}{{1, 0, "Infinity"}, {0, 0, "NaN"}, {1, 3, "333"}, {2, 16, "125"}} {
		if got := runThroughput(test.samples, test.elapsed); got != test.expected {
			t.Fatal(got, test.expected)
		}
	}
	messages := []quint.ErrorMessage{{Explanation: "first", Locations: []quint.Location{}}, {Explanation: "first", Locations: []quint.Location{}}, {Explanation: "second", Locations: []quint.Location{}}}
	got := runUniqueMessages(messages)
	if len(got) != 2 || got[0].Explanation != "first" || got[1].Explanation != "second" {
		t.Fatal(got)
	}
	if _, err := runExpressions(context.Background(), []quint.RuntimeValue{{}}, &quint.RuntimeObjectHashState{}); err == nil {
		t.Fatal("invalid state serialization accepted")
	}
}
