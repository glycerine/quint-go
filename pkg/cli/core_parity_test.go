package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

type coreParityFixtures struct {
	Cases []struct {
		Name, Command string
		Files         []struct{ Name, Source string }
		Options       struct {
			Flatten                *bool
			Main, Invariant, Input string
		}
		Status        int
		Summary       string
		Stdout, Stage json.RawMessage
	}
	Options []struct {
		Command        string
		Args           []string
		Out, SourceMap string
		Error          string
		Flatten        bool
		Verbosity      json.RawMessage
	}
	Informational []struct {
		Args          []string
		Help, Version bool
	}
}

func coreReadParityFixtures(t *testing.T) coreParityFixtures {
	t.Helper()
	data, err := os.ReadFile("../testdata/parser/cli_core.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures coreParityFixtures
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

// Only temporary source paths differ from the saved upstream transcript.
// IDs, lookup references, table keys, and complete analysis payloads remain.
func coreParityJSON(t *testing.T, encoded []byte, directory string) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err, string(encoded))
	}
	var normalize func(any) any
	normalize = func(value any) any {
		switch node := value.(type) {
		case map[string]any:
			result := map[string]any{}
			for key, value := range node {
				result[key] = normalize(value)
			}
			return result
		case []any:
			result := make([]any, len(node))
			for i, child := range node {
				result[i] = normalize(child)
			}
			return result
		case string:
			return strings.ReplaceAll(node, directory, "<fixture>")
		default:
			return value
		}
	}
	return normalize(value)
}

func TestUpstreamCoreCLIStagesAndTargets(t *testing.T) {
	t.Parallel()
	for _, fixture := range coreReadParityFixtures(t).Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			directory := t.TempDir()
			for _, file := range fixture.Files {
				if err := os.WriteFile(filepath.Join(directory, file.Name), []byte(file.Source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			out := filepath.Join(directory, "stage.json")
			input := "M.qnt"
			if fixture.Options.Input != "" {
				input = fixture.Options.Input
			}
			args := []string{fixture.Command, filepath.Join(directory, input), "--out", out}
			if fixture.Options.Flatten != nil {
				args = append(args, "--flatten="+map[bool]string{true: "true", false: "false"}[*fixture.Options.Flatten])
			}
			if fixture.Options.Main != "" {
				args = append(args, "--main="+fixture.Options.Main)
			}
			if fixture.Options.Invariant != "" {
				args = append(args, "--invariant="+fixture.Options.Invariant)
			}
			var stdout, stderr bytes.Buffer
			status := Execute(context.Background(), args, &stdout, &stderr)
			if status != fixture.Status || stderr.Len() != 0 {
				t.Fatal(status, fixture.Status, stderr.String())
			}
			stage, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			got, want := coreParityJSON(t, stage, directory), coreParityJSON(t, fixture.Stage, directory)
			if !reflect.DeepEqual(got, want) {
				gotMap, wantMap := got.(map[string]any), want.(map[string]any)
				for _, key := range []string{"stage", "modules", "table", "types", "effects", "errors", "main", "warnings"} {
					if !reflect.DeepEqual(gotMap[key], wantMap[key]) {
						gotJSON, _ := json.Marshal(gotMap[key])
						wantJSON, _ := json.Marshal(wantMap[key])
						t.Errorf("stage.%s\ngot %s\nwant %s", key, gotJSON, wantJSON)
					}
				}
				t.FailNow()
			}
			if string(fixture.Stdout) == "null" {
				if stdout.Len() != 0 {
					t.Fatal("unexpected target output", stdout.String())
				}
			} else if got, want := coreParityJSON(t, stdout.Bytes(), directory), coreParityJSON(t, fixture.Stdout, directory); !reflect.DeepEqual(got, want) {
				t.Fatal("stdout target differs from upstream", stdout.String())
			}
		})
	}
}

func TestUpstreamCoreCLIOptionCoercion(t *testing.T) {
	t.Parallel()
	for _, fixture := range coreReadParityFixtures(t).Options {
		args := append([]string{"M.qnt"}, fixture.Args...)
		options, err := coreParseOptions(fixture.Command, args)
		if fixture.Error != "" {
			if err == nil || !strings.Contains(err.Error(), fixture.Error) {
				t.Fatal(fixture.Args, err, fixture.Error)
			}
			continue
		}
		if err != nil {
			t.Fatal(fixture.Args, err)
		}
		if options.out != fixture.Out || options.sourceMap != fixture.SourceMap || options.flatten != fixture.Flatten {
			t.Fatal(fixture.Args, options, fixture)
		}
		switch string(fixture.Verbosity) {
		case `"NaN"`:
			if !math.IsNaN(options.verbosity) {
				t.Fatal(fixture.Args, options.verbosity)
			}
		case `"Infinity"`:
			if !math.IsInf(options.verbosity, 1) {
				t.Fatal(fixture.Args, options.verbosity)
			}
		default:
			var expected float64
			if err := json.Unmarshal(fixture.Verbosity, &expected); err != nil || options.verbosity != expected {
				t.Fatal(fixture.Args, options.verbosity, expected, err)
			}
		}
	}
	for _, args := range [][]string{{"compile", "--out=a", "--out=b", "--version"}, {"parse", "--version"}, {"--version", "extra"}} {
		status, stdout, stderr := coreInvoke(t, args...)
		if status != 0 || stdout != Version+"\n" || stderr != "" {
			t.Fatal(args, status, stdout, stderr)
		}
	}
	status, stdout, stderr := coreInvoke(t, "compile", "--out=a", "--out=b", "--help")
	if status != 0 || !strings.Contains(stdout, "compile") || stderr != "" {
		t.Fatal(status, stdout, stderr)
	}
}

func TestCoreJSONPropertyEnumerationAndSourceIndexes(t *testing.T) {
	fields := insdict.NewDict[string, any]()
	for _, key := range []string{"other", "4294967295", "10", "02", "2", "4294967294", "0", "last"} {
		fields.Set(key, key)
	}
	encoded, err := coreMarshalStage(fields)
	want := `{"0":"0","2":"2","10":"10","4294967294":"4294967294","other":"other","4294967295":"4294967295","02":"02","last":"last"}`
	if err != nil || string(encoded) != want {
		t.Fatal(string(encoded), want, err)
	}
	sources := quint.UpstreamSourceMap{}
	for i, source := range []string{"first", "first", "first", "second", "second"} {
		id, _ := quint.ParseID(string(rune('1' + i)))
		sources[id] = quint.UpstreamLocation{Source: source}
	}
	encoded, err = coreCompactSourceMap(sources)
	if err != nil || !strings.Contains(string(encoded), `"sourceIndex":{"0":"first","3":"second"}`) {
		t.Fatal(string(encoded), err)
	}
}

func TestCoreLoadingStageAndCompatibilityCopies(t *testing.T) {
	out := filepath.Join(t.TempDir(), "missing.json")
	status, stdout, stderr := coreInvoke(t, "parse", filepath.Join(t.TempDir(), "absent.qnt"), "--out", out)
	encoded, err := os.ReadFile(out)
	if status != 1 || stdout != "" || stderr != "" || err != nil || string(encoded) != `{"stage":"loading","errors":[]}` {
		t.Fatal(status, stdout, stderr, string(encoded), err)
	}
	modules, err := quint.ParseModules("module M { var x: int val y=1 }")
	if err != nil {
		t.Fatal(err)
	}
	copy := coreModules(modules)
	if copy[0].Declarations[0].Depth == nil || *copy[0].Declarations[0].Depth != 0 || copy[0].Declarations[1].Depth != nil || modules[0].Declarations[0].Depth != nil {
		t.Fatal("metadata projection changed IR")
	}
	copy[0].Declarations[0].Name = "changed"
	if modules[0].Declarations[0].Name != "x" {
		t.Fatal("metadata projection shares declaration slice")
	}
}

func TestUpstreamCoreInformationalOptions(t *testing.T) {
	for _, fixture := range coreReadParityFixtures(t).Informational {
		help, version, _ := coreInformationalOptions(fixture.Args)
		if help != fixture.Help || version != fixture.Version {
			t.Fatal(fixture.Args, help, version, fixture)
		}
	}
	for _, flag := range []string{"--help=false", "--version=false", "--no-help", "--no-version"} {
		status, _, stderr := coreInvoke(t, "parse", filepath.Join(t.TempDir(), "missing.qnt"), flag)
		if status != 1 || !strings.Contains(stderr, "does not exist") {
			t.Fatal(flag, status, stderr)
		}
	}
}

func TestCoreParityNormalizationPreservesIRIDs(t *testing.T) {
	input := []byte(`{"source":"/temporary/file.qnt","modules":[{"id":1,"typeAnnotation":{"kind":"int","id":2}}],"table":{"3":{"id":4}},"types":{"5":{"type":{"kind":"int","id":6}}},"effects":{"7":{"reference":8}}}`)
	want := []byte(`{"source":"<fixture>/file.qnt","modules":[{"id":1,"typeAnnotation":{"kind":"int","id":2}}],"table":{"3":{"id":4}},"types":{"5":{"type":{"kind":"int","id":6}}},"effects":{"7":{"reference":8}}}`)
	decoder := json.NewDecoder(bytes.NewReader(want))
	decoder.UseNumber()
	var expected any
	if err := decoder.Decode(&expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(coreParityJSON(t, input, "/temporary"), expected) {
		t.Fatal("source path normalization changed IR identities")
	}
}
