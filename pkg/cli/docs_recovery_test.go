package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestUpstreamDocumentationCLIRecovery(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name                    string
		Files                   []struct{ Name, Source string }
		Status                  int
		Summary, Stdout, Stderr string
		IncidentalDebug         string
		Stage                   json.RawMessage
		Modules                 []quint.Module
		Diagnostics             []quint.Diagnostic
		UpstreamSources         quint.UpstreamSourceMap
		NextID                  quint.ID
	}
	data, err := os.ReadFile("../testdata/parser/cli_docs_recovery.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 10 {
		t.Fatalf("documentation recovery fixture inventory: got %d, want 10", len(fixtures))
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
			out := filepath.Join(directory, "stage.json")
			var stdout, stderr bytes.Buffer
			status := Execute(context.Background(), []string{"docs", filepath.Join(directory, "M.qnt"), "--out", out}, &stdout, &stderr)
			if status != fixture.Status || stdout.String() != fixture.Stdout || stderr.String() != fixture.Stderr {
				t.Fatalf("documentation output: status %d want %d\nstdout=%q want %q\nstderr=%q want %q", status, fixture.Status, stdout.String(), fixture.Stdout, stderr.String(), fixture.Stderr)
			}
			got, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(coreParityJSON(t, got, directory), coreParityJSON(t, fixture.Stage, directory)) {
				t.Errorf("documentation recovery stage:\n got %s\nwant %s", got, fixture.Stage)
			}
			if fixture.Status == 1 && fixture.Summary != "parsing failed" {
				t.Fatalf("unexpected upstream recovery summary %q", fixture.Summary)
			}
			// Pin the raw partial modules behind the Markdown projection, including
			// IDs that would be hidden by matching rendered signatures alone.
			parsed, err := quint.ParseModulesRecovering("<fixture>/M.qnt", fixture.Files[0].Source)
			if err != nil {
				t.Fatal(err)
			}
			for _, snapshot := range []struct {
				name      string
				got, want any
			}{{"modules", parsed.Modules, fixture.Modules}, {"diagnostics", parsed.Diagnostics, fixture.Diagnostics}, {"raw spans", parsed.UpstreamSources, fixture.UpstreamSources}} {
				if !reflect.DeepEqual(snapshot.got, snapshot.want) {
					a, _ := json.Marshal(snapshot.got)
					b, _ := json.Marshal(snapshot.want)
					t.Errorf("documentation %s:\n got %s\nwant %s", snapshot.name, a, b)
				}
			}
			if next := parsed.IDs.Next(); next != fixture.NextID {
				t.Errorf("continuing ID: got %s want %s", next, fixture.NextID)
			}
			// Listener console.debug text is saved for audit; it is not part of
			// the documentation output written by the Go command.
		})
	}
}
