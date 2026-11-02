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

func TestUpstreamCoreCLIRecoveryStagesAndSourceMaps(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name                               string
		Files                              []struct{ Name, Source string }
		Status                             int
		Summary                            string
		Stage, SourceMap                   json.RawMessage
		IncidentalStdout, IncidentalStderr string
	}
	data, err := os.ReadFile("../testdata/parser/cli_recovery.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 10 {
		t.Fatalf("CLI recovery fixture inventory: got %d, want 10", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			for _, file := range fixture.Files {
				if err := os.WriteFile(filepath.Join(directory, file.Name), []byte(file.Source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			out, sourceMap := filepath.Join(directory, "stage.json"), filepath.Join(directory, "source-map.json")
			var stdout, stderr bytes.Buffer
			status := Execute(context.Background(), []string{"parse", filepath.Join(directory, "M.qnt"), "--out", out, "--source-map", sourceMap}, &stdout, &stderr)
			if status != fixture.Status || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("CLI output: status %d want %d, stdout=%q stderr=%q", status, fixture.Status, stdout.String(), stderr.String())
			}
			for _, output := range []struct {
				name, path string
				want       json.RawMessage
			}{{"stage", out, fixture.Stage}, {"source-map", sourceMap, fixture.SourceMap}} {
				got, err := os.ReadFile(output.path)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(coreParityJSON(t, got, directory), coreParityJSON(t, output.want, directory)) {
					t.Errorf("raw recovery %s differs:\n got %s\nwant %s", output.name, got, output.want)
				}
			}
			if fixture.Status == 1 && fixture.Summary != "parsing failed" {
				t.Fatalf("unexpected upstream recovery summary %q", fixture.Summary)
			}
			// The oracle also retains incidental upstream console.debug output.
			// This slice compares status, stage and source-map artifacts; the Go
			// command keeps those internal listener debug messages off stdout.
		})
	}
}
