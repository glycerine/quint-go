package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type symbolicCLITraceDrive struct {
	Thrown *struct{ Name, Message string }
	Events []struct {
		States json.RawMessage
		ITF    struct {
			Value json.RawMessage
			Error string
		}
	}
	Result *struct{ Status string }
}

func TestUpstreamSymbolicSourceCLITraceExports(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Cases []struct {
			Name, Source, Seed            string
			Metadata                      bool
			MaxSamples, MaxSteps          int
			Simulation, ZeroUIDSimulation *symbolicCLITraceDrive
			Test, ZeroUIDTest             *symbolicCLITraceDrive
		}
	}
	data, err := os.ReadFile("../testdata/parser/runtime_symbolic_trace.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	runCases := map[string]bool{"interval_metadata": true, "powerset": true, "nested_list_record": true, "compact_map_values": true, "two_sample_tie_map": true, "infinite_integer": true, "infinite_powerset": true}
	testCases := map[string]bool{"interval": true, "powerset": true, "compact_map_values": true, "two_sample_tie_map": true, "infinite_integer": true, "infinite_powerset": true, "test_state_serialization_before_false": true, "test_state_serialization_before_diagnostic": true, "test_result_serialization_before_ignored_powerset": true, "test_result_serialization_ignored_tie_map": true}
	for _, fixture := range fixtures.Cases {
		for _, mode := range []string{"run", "test"} {
			if mode == "run" && !runCases[fixture.Name] || mode == "test" && !testCases[fixture.Name] {
				continue
			}
			t.Run(fixture.Name+"/"+mode, func(t *testing.T) {
				t.Parallel()
				want := fixture.Test
				if fixture.ZeroUIDTest != nil {
					want = fixture.ZeroUIDTest
				}
				if mode == "run" {
					want = fixture.Simulation
					if fixture.ZeroUIDSimulation != nil {
						want = fixture.ZeroUIDSimulation
					}
				}
				directory := t.TempDir()
				input := filepath.Join(directory, "M.qnt")
				stage := filepath.Join(directory, "stage.json")
				trace := filepath.Join(directory, "trace.itf.json")
				if err := os.WriteFile(input, []byte(fixture.Source), 0600); err != nil {
					t.Fatal(err)
				}
				args := []string{input, "--main=M", "--seed=" + fixture.Seed, fmt.Sprintf("--max-samples=%d", fixture.MaxSamples), "--verbosity=0", "--out=" + stage, "--out-itf=" + trace}
				var stdout, stderr bytes.Buffer
				var status int
				if mode == "run" {
					args = append(args, "--invariant=inv", fmt.Sprintf("--max-steps=%d", fixture.MaxSteps))
					if fixture.Metadata {
						args = append(args, "--mbt")
					}
					status = commandRun(context.Background(), args, &stdout, &stderr)
				} else {
					args = append(args, "--match=^example$")
					status = commandTest(context.Background(), args, &stdout, &stderr)
				}
				data, err := os.ReadFile(stage)
				if err != nil {
					t.Fatal("missing stage", status, stdout.String(), stderr.String(), err)
				}
				var result struct {
					Errors []struct{ Explanation string }
					Trace  json.RawMessage
				}
				if err = json.Unmarshal(data, &result); err != nil {
					t.Fatal(err)
				}
				message := ""
				if want.Thrown != nil {
					message = want.Thrown.Message
				} else if len(want.Events) > 0 && want.Events[0].ITF.Error != "" {
					message = "ITF conversion failed on 0: " + want.Events[0].ITF.Error
				}
				if message != "" {
					if status != 1 || len(result.Errors) != 1 || result.Errors[0].Explanation != message {
						t.Fatal("export error", status, string(data), stdout.String(), stderr.String(), message)
					}
					if _, err = os.Stat(trace); !os.IsNotExist(err) {
						t.Fatal("failed conversion wrote a trace", err)
					}
					return
				}
				if status != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
					t.Fatal("command output", status, stdout.String(), stderr.String(), string(data))
				}
				if len(want.Events) == 0 {
					if _, err = os.Stat(trace); !os.IsNotExist(err) {
						t.Fatal("ignored result wrote a trace", err)
					}
					return
				}
				actual, err := os.ReadFile(trace)
				if err != nil {
					t.Fatal(err)
				}
				var traceFields map[string]json.RawMessage
				if err = json.Unmarshal(actual, &traceFields); err != nil {
					t.Fatal(err)
				}
				delete(traceFields, "#meta")
				actual, err = json.Marshal(traceFields)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(runTranscriptJSON(t, actual), runTranscriptJSON(t, want.Events[0].ITF.Value)) {
					t.Fatalf("ITF\ngot  %s\nwant %s", actual, want.Events[0].ITF.Value)
				}
				if mode == "run" && !reflect.DeepEqual(runTranscriptJSON(t, result.Trace), runTranscriptJSON(t, want.Events[0].States)) {
					t.Fatalf("stage trace\ngot  %s\nwant %s", result.Trace, want.Events[0].States)
				}
			})
		}
	}
}

func TestSymbolicStateErrorsSurfaceWithoutITFExport(t *testing.T) {
	t.Parallel()
	for _, last := range []string{"false", "1/0==0"} {
		t.Run(last, func(t *testing.T) {
			t.Parallel()
			source := "module M { var x:Set[Set[int]] action example=(x'=Int.powerset()).then(all { x'=x," + last + " }) }"
			input := testCommandFile(t, source)
			status, _, stderr := testCommandCall(t, context.Background(), input, "--match=^example$", "--seed=0", "--verbosity=0")
			if status != 1 || !strings.Contains(stderr, "Either state is Left") || strings.Contains(stderr, "Tests failed") {
				t.Fatal("trace serialization must precede test classification", status, stderr)
			}
		})
	}
}
