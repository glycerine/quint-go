package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type frontendStageFixture struct {
	Name, Summary, Stdout, Stderr string
	Files                         []struct{ Name, Source string }
	Directory                     bool
	Status                        int
	Stage                         json.RawMessage
}

func frontendStageFixtures(t *testing.T) []frontendStageFixture {
	t.Helper()
	data, err := os.ReadFile("../testdata/parser/cli_frontend_stages.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct{ Cases []frontendStageFixture }
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures.Cases
}

func TestUpstreamRunTestFrontendFailureStages(t *testing.T) {
	t.Parallel()
	for _, fixture := range frontendStageFixtures(t) {
		for _, command := range []string{"run", "test"} {
			t.Run(command+"/"+fixture.Name, func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				for _, file := range fixture.Files {
					if err := os.WriteFile(filepath.Join(directory, file.Name), []byte(file.Source), 0600); err != nil {
						t.Fatal(err)
					}
				}
				input := filepath.Join(directory, "M.qnt")
				if fixture.Directory {
					if err := os.Mkdir(input, 0700); err != nil {
						t.Fatal(err)
					}
				}
				output := filepath.Join(directory, "stage.json")
				args := []string{command, input, "--seed=1", "--verbosity=0", "--out=" + output}
				var stdout, stderr bytes.Buffer
				if code := Execute(context.Background(), args, &stdout, &stderr); code != fixture.Status || stdout.Len() != 0 || stderr.Len() != 0 {
					t.Fatal(code, stdout.String(), stderr.String())
				}
				encoded, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				// The exact upstream stage includes field presence, all
				// diagnostics, recovered modules, IDs, table, types, and effects.
				if got, want := coreParityJSON(t, encoded, directory), coreParityJSON(t, fixture.Stage, directory); !reflect.DeepEqual(got, want) {
					gotJSON, _ := json.MarshalIndent(got, "", "  ")
					wantJSON, _ := json.MarshalIndent(want, "", "  ")
					t.Fatalf("Go %s\nupstream %s", gotJSON, wantJSON)
				}
				stdout.Reset()
				stderr.Reset()
				if code := Execute(context.Background(), args[:len(args)-1], &stdout, &stderr); code != fixture.Status {
					t.Fatal(code, stdout.String(), stderr.String())
				}
				if fixture.Directory {
					// OS-specific read errors differ between Node and Go;
					// the loading stage and empty diagnostic list are exact.
					if stdout.Len() != 0 || !strings.Contains(stderr.String(), "file "+input+" could not be opened") {
						t.Fatal(stdout.String(), stderr.String())
					}
				} else if gotOut, gotErr := strings.ReplaceAll(stdout.String(), directory, "<fixture>"), strings.ReplaceAll(stderr.String(), directory, "<fixture>"); gotOut != fixture.Stdout || gotErr != fixture.Stderr {
					t.Fatalf("stdout %q, want %q\nstderr %q, want %q", gotOut, fixture.Stdout, gotErr, fixture.Stderr)
				}
			})
		}
	}
}
