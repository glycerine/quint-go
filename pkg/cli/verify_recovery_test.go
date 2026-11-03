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
	"github.com/glycerine/quint-go/pkg/backend/apalache"
	gotlc "github.com/glycerine/quint-go/pkg/backend/tlc"
)

// Reuse the exact upstream parse stages: verify traverses the same load/parse
// procedures before selecting a checker. No duplicate oracle corpus is needed.
func TestVerifyUpstreamRecoveringDiagnostics(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name   string
		Files  []struct{ Name, Source string }
		Status int
		Stage  struct {
			Errors json.RawMessage `json:"errors"`
		}
	}
	data, err := os.ReadFile("../testdata/parser/cli_recovery.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, fixture := range fixtures {
		if fixture.Status != 1 {
			continue
		}
		checked++
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			for _, file := range fixture.Files {
				if err := os.WriteFile(filepath.Join(directory, file.Name), []byte(file.Source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, backend := range []string{"apalache-go", "tlc-go"} {
				output := filepath.Join(directory, backend+"-stage.json")
				called := false
				check := verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
					called = true
					return apalache.VerificationResult{}, nil
				})
				tlcCheck := verifyTLCBackend(func(context.Context, quint.CompilationResult, gotlc.Options) (gotlc.Result, error) {
					called = true
					return gotlc.Result{}, nil
				})
				var stdout, stderr bytes.Buffer
				code := verifyWithCheckers(context.Background(), []string{filepath.Join(directory, "M.qnt"), "--backend=" + backend, "--out", output}, &stdout, &stderr, check, tlcCheck)
				encoded, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				var stage verifyStage
				if err := json.Unmarshal(encoded, &stage); err != nil || code != 1 || called || stdout.Len() != 0 || stderr.Len() != 0 || stage.Status != "failure" || stage.Explanation != "parsing failed" || stage.Outcome != "" || stage.TLC != nil {
					t.Fatal(code, called, string(encoded), stdout.String(), stderr.String(), err)
				}
				got, err := json.Marshal(stage.Errors)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(coreParityJSON(t, got, directory), coreParityJSON(t, fixture.Stage.Errors, directory)) {
					t.Fatalf("verify diagnostics differ from upstream parse stage:\ngot %s\nwant %s", got, fixture.Stage.Errors)
				}
			}
		})
	}
	if checked != 9 {
		t.Fatalf("malformed upstream stage inventory: got %d, want 9", checked)
	}
}
