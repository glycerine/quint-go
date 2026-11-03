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

// Upstream reads the main CLI input verbatim while its resolver normalizes
// imported files to LF. Pin both halves, including absolute source-map indices.
func TestUpstreamMainSourceLineEndings(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Cases []struct {
			Name                             string
			Files                            []struct{ Name, Source string }
			Parsing, Typechecking, SourceMap json.RawMessage
		}
	}
	data, err := os.ReadFile("../testdata/parser/cli_main_sources.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Cases) != 2 {
		t.Fatalf("expected main-only and imported CRLF witnesses, got %d", len(fixtures.Cases))
	}
	for _, fixture := range fixtures.Cases {
		for _, command := range []string{"parse", "typecheck", "compile", "verify-apalache", "verify-tlc"} {
			t.Run(command+"/"+fixture.Name, func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				for _, file := range fixture.Files {
					if err := os.WriteFile(filepath.Join(directory, file.Name), []byte(file.Source), 0600); err != nil {
						t.Fatal(err)
					}
				}
				input, output := filepath.Join(directory, "M.qnt"), filepath.Join(directory, "stage.json")
				var stdout, stderr bytes.Buffer
				var code int
				sourceMap := filepath.Join(directory, "source-map.json")
				switch command {
				case "verify-apalache", "verify-tlc":
					called := false
					backend := verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
						called = true
						return apalache.VerificationResult{}, nil
					})
					tlc := func(context.Context, quint.CompilationResult, gotlc.Options) (gotlc.Result, error) {
						called = true
						return gotlc.Result{}, nil
					}
					args := []string{input, "--out=" + output, "--verbosity=0"}
					if command == "verify-tlc" {
						args = append(args, "--backend=tlc")
					}
					code = verifyWithCheckers(context.Background(), args, &stdout, &stderr, backend, tlc)
					if called {
						t.Fatal("type errors must stop before either checker")
					}
				default:
					args := []string{command, input, "--out=" + output}
					if command == "parse" {
						args = append(args, "--source-map="+sourceMap)
					}
					code = Execute(context.Background(), args, &stdout, &stderr)
					if command == "parse" {
						encoded, err := os.ReadFile(sourceMap)
						if err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(coreParityJSON(t, encoded, directory), coreParityJSON(t, fixture.SourceMap, directory)) {
							t.Fatalf("source-map coordinates differ:\ngot %s\nwant %s", encoded, fixture.SourceMap)
						}
					}
				}
				wantStatus, wantStage := 1, fixture.Typechecking
				if command == "parse" {
					wantStatus, wantStage = 0, fixture.Parsing
				}
				if code != wantStatus || stdout.Len() != 0 || stderr.Len() != 0 {
					t.Fatal(code, stdout.String(), stderr.String())
				}
				encoded, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(coreParityJSON(t, encoded, directory), coreParityJSON(t, wantStage, directory)) {
					t.Fatalf("frontend stage differs:\ngot %s\nwant %s", encoded, wantStage)
				}
			})
		}
	}
}
