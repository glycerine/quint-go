package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var testReferenceDuration = regexp.MustCompile(`\([0-9]+ms\)`)
var testReferenceReplay = regexp.MustCompile(`--match=([^ ]+?)(?: --backend=go)? to repeat\.`)

func testReferenceNormalizeText(text, input string) string {
	text = strings.ReplaceAll(text, input, "fixture.qnt")
	text = testReferenceDuration.ReplaceAllString(text, "(0ms)")
	// The Go hint uses the unqualified declaration name, because discovery
	// matches that name even when its displayed result includes namespaces.
	// The separately tested improvement still points at the same named test.
	return testReferenceReplay.ReplaceAllStringFunc(text, func(line string) string {
		match := testReferenceReplay.FindStringSubmatch(line)
		name := strings.TrimSuffix(strings.TrimPrefix(match[1], "^"), "$")
		if index := strings.LastIndex(name, "::"); index >= 0 {
			name = name[index+2:]
		}
		return "--match=" + name + " to repeat."
	})
}

func testReferenceJSON(t *testing.T, data []byte, input string, trace bool) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err, string(data))
	}
	var normalize func(any) any
	normalize = func(value any) any {
		switch value := value.(type) {
		case string:
			return strings.ReplaceAll(value, input, "fixture.qnt")
		case []any:
			for i, element := range value {
				value[i] = normalize(element)
			}
		case map[string]any:
			for key, element := range value {
				value[key] = normalize(element)
			}
		}
		return value
	}
	value = normalize(value)
	if trace {
		metadata := value.(map[string]any)["#meta"].(map[string]any)
		delete(metadata, "description")
		delete(metadata, "timestamp")
	}
	return value
}

func TestUpstreamTestCommandTranscripts(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source, Main, Match, Seed string
		Verbosity, MaxSamples, Status   int
		ITF                             bool
		Stdout, Stderr                  string
		Passed, Failed, Ignored         []string
		Errors                          json.RawMessage
		Traces                          []struct {
			Index int
			Name  string
			Trace json.RawMessage
		}
	}
	data, err := os.ReadFile("testdata/test_command.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			input := filepath.Join(t.TempDir(), "fixture.qnt")
			if err := os.WriteFile(input, []byte(fixture.Source), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{input, "--seed=" + fixture.Seed, fmt.Sprintf("--max-samples=%d", fixture.MaxSamples), fmt.Sprintf("--verbosity=%d", fixture.Verbosity)}
			if fixture.Main != "" {
				args = append(args, "--main="+fixture.Main)
			}
			if fixture.Match != "" {
				args = append(args, "--match="+fixture.Match)
			}
			traceDirectory := t.TempDir()
			transcriptArgs := append([]string{}, args...)
			if fixture.ITF {
				transcriptArgs = append(transcriptArgs, "--out-itf="+filepath.Join(traceDirectory, "trace_{seq}_{test}.itf.json"))
			}
			var stdout, stderr bytes.Buffer
			status := commandTest(context.Background(), transcriptArgs, &stdout, &stderr)
			if status != fixture.Status {
				t.Errorf("status: got %d, want %d; stderr=%q", status, fixture.Status, stderr.String())
			}
			gotOut := testReferenceNormalizeText(stdout.String(), input)
			wantOut := testReferenceNormalizeText(fixture.Stdout, input)
			if gotOut != wantOut {
				t.Errorf("stdout:\nGOT:\n%s\nWANT:\n%s", gotOut, wantOut)
			}
			gotErr := testReferenceNormalizeText(stderr.String(), input)
			wantErr := testReferenceNormalizeText(fixture.Stderr, input)
			if gotErr != wantErr {
				t.Errorf("stderr: got %q, want %q", gotErr, wantErr)
			}
			files, err := os.ReadDir(traceDirectory)
			if err != nil || len(files) != len(fixture.Traces) {
				t.Errorf("trace count: got %d, want %d; error=%v", len(files), len(fixture.Traces), err)
			}
			for _, trace := range fixture.Traces {
				filename := filepath.Join(traceDirectory, fmt.Sprintf("trace_%d_%s.itf.json", trace.Index, trace.Name))
				actual, err := os.ReadFile(filename)
				if err != nil {
					t.Error(err)
					continue
				}
				got := testReferenceJSON(t, actual, input, true)
				want := testReferenceJSON(t, trace.Trace, input, true)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("trace %s: got %s, want %s", trace.Name, actual, trace.Trace)
				}
			}
			// --out suppresses reporting and preserves the same result arrays and
			// diagnostics. Run it separately so it does not hide the transcript.
			output := filepath.Join(t.TempDir(), "stage.json")
			stdout.Reset()
			stderr.Reset()
			status = commandTest(context.Background(), append(args, "--out="+output), &stdout, &stderr)
			if status != fixture.Status || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("stage invocation: status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
			actual, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var stage struct {
				Passed, Failed, Ignored []string
				Errors                  json.RawMessage
			}
			if err = json.Unmarshal(actual, &stage); err != nil {
				t.Fatal(err, string(actual))
			}
			if !reflect.DeepEqual(stage.Passed, fixture.Passed) || !reflect.DeepEqual(stage.Failed, fixture.Failed) || !reflect.DeepEqual(stage.Ignored, fixture.Ignored) {
				t.Errorf("results: got %v/%v/%v, want %v/%v/%v", stage.Passed, stage.Failed, stage.Ignored, fixture.Passed, fixture.Failed, fixture.Ignored)
			}
			if got, want := testReferenceJSON(t, stage.Errors, input, false), testReferenceJSON(t, fixture.Errors, input, false); !reflect.DeepEqual(got, want) {
				t.Errorf("errors: got %s, want %s", stage.Errors, fixture.Errors)
			}
		})
	}
}
