package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUpstreamVariableHeaders(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Command, Name, Source, Main, Invariant string
		Witnesses                              []string
		Metadata                               bool
		Headers                                []struct {
			Filename string
			Vars     []string
		}
	}
	data, err := os.ReadFile("testdata/variable_headers.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Command+"_"+fixture.Name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			input := filepath.Join(directory, "fixture.qnt")
			if err := os.WriteFile(input, []byte(fixture.Source), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{input, "--seed=1", "--verbosity=2", "--max-samples=1", "--out-itf=" + filepath.Join(directory, "trace_{seq}_{test}.itf.json")}
			if fixture.Main != "" {
				args = append(args, "--main="+fixture.Main)
			}
			var stdout, stderr bytes.Buffer
			var status int
			if fixture.Command == "run" {
				args = append(args, "--max-steps=0")
				if fixture.Metadata {
					args = append(args, "--mbt")
				}
				if fixture.Invariant != "" {
					args = append(args, "--invariant="+fixture.Invariant)
				}
				if len(fixture.Witnesses) > 0 {
					args = append(args, "--witnesses")
					args = append(args, fixture.Witnesses...)
				}
				status = commandRun(context.Background(), args, &stdout, &stderr)
			} else {
				status = commandTest(context.Background(), args, &stdout, &stderr)
			}
			if status != 0 {
				t.Fatal(status, stdout.String(), stderr.String())
			}
			for _, header := range fixture.Headers {
				data, err := os.ReadFile(filepath.Join(directory, header.Filename))
				if err != nil {
					t.Fatal(err)
				}
				var actual struct{ Vars []string }
				if err := json.Unmarshal(data, &actual); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual.Vars, header.Vars) {
					t.Errorf("%s vars = %v, want %v", header.Filename, actual.Vars, header.Vars)
				}
			}
		})
	}
}
