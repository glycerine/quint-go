package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func docsSourceFile(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.qnt")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDocsCommandUpstreamSources(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../testdata/parser/documentation.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Source  string
			Modules []struct{ Markdown string }
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, example := range fixture.Cases {
		var stdout, stderr bytes.Buffer
		status := commandDocs(context.Background(), []string{docsSourceFile(t, example.Source)}, &stdout, &stderr)
		want := ""
		for _, module := range example.Modules {
			want += module.Markdown
		}
		if status != 0 || stderr.Len() != 0 || stdout.String() != want {
			t.Fatalf("status=%d stderr=%q\ngot %q\nwant %q", status, stderr.String(), stdout.String(), want)
		}
	}
}

func TestDocsCommandStageAndNoImportResolution(t *testing.T) {
	t.Parallel()
	source := "module M { import Missing.* from \"not-present\"\n/// docs\nval second=unknown\nval first=3 }"
	input := docsSourceFile(t, source)
	out := filepath.Join(t.TempDir(), "output.json")
	var stdout, stderr bytes.Buffer
	status := commandDocs(context.Background(), []string{input, "--out=" + out}, &stdout, &stderr)
	if status != 0 || stderr.Len() != 0 || !strings.HasPrefix(stdout.String(), "# Documentation for M\n\n") {
		t.Fatal(status, stdout.String(), stderr.String())
	}
	encoded, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"stage":"documentation","warnings":[],"documentation":{"M":{"second":{"name":"second","signature":"val second","documentation":"docs"},"first":{"name":"first","signature":"val first"}}}}`
	if string(encoded) != want {
		t.Fatalf("stage order/optional fields\ngot  %s\nwant %s", encoded, want)
	}
	// Both module-level and definition-level duplicates follow JavaScript Map:
	// replace the last value without moving the first insertion's position.
	input = docsSourceFile(t, "module Z {val x=1} module A {val a=1} module Z {val y=2}")
	stdout.Reset()
	status = commandDocs(context.Background(), []string{"--out", out, input}, &stdout, &stderr)
	if status != 0 || strings.Count(stdout.String(), "# Documentation for Z") != 2 {
		t.Fatal(status, stdout.String(), stderr.String())
	}
	encoded, err = os.ReadFile(out)
	if err != nil || !strings.Contains(string(encoded), `"documentation":{"Z":{"y"`) || !strings.Contains(string(encoded), `},"A":{"a"`) {
		t.Fatal("duplicate module order", string(encoded), err)
	}
}

func TestDocsCommandOptionsAndFailures(t *testing.T) {
	t.Parallel()
	input := docsSourceFile(t, "module M {}")
	for _, args := range [][]string{
		nil, {"--unknown"}, {input, input}, {input, "--out"}, {input, "--out="},
		{input, "--out=x", "--out=y"}, {input, "--out", "--help"},
	} {
		var stdout, stderr bytes.Buffer
		if status := commandDocs(context.Background(), args, &stdout, &stderr); status != 1 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatal(args, status, stdout.String(), stderr.String())
		}
	}
	for _, args := range [][]string{{"--help"}, {"-h"}, {input, "--help"}} {
		var stdout, stderr bytes.Buffer
		if status := commandDocs(context.Background(), args, &stdout, &stderr); status != 0 || stdout.String() != docsHelp || stderr.Len() != 0 {
			t.Fatal(args, status, stdout.String(), stderr.String())
		}
	}
	var positionOut, positionErr bytes.Buffer
	if status := commandDocs(context.Background(), []string{"--", input}, &positionOut, &positionErr); status != 0 || positionErr.Len() != 0 {
		t.Fatal("positional terminator", status, positionOut.String(), positionErr.String())
	}
	for _, path := range []string{filepath.Join(t.TempDir(), "absent.qnt"), t.TempDir(), docsSourceFile(t, "module M {"), docsSourceFile(t, string([]byte{0xff}))} {
		var stdout, stderr bytes.Buffer
		if status := commandDocs(context.Background(), []string{path}, &stdout, &stderr); status != 1 || stderr.Len() == 0 {
			t.Fatal(path, status, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if status := commandDocs(context.Background(), []string{input, "--out", t.TempDir()}, &stdout, &stderr); status != 1 || stderr.Len() == 0 {
		t.Fatal("output failure", status, stdout.String(), stderr.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stdout.Reset()
	stderr.Reset()
	if status := commandDocs(ctx, []string{input}, &stdout, &stderr); status != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "context canceled") {
		t.Fatal("cancellation", status, stdout.String(), stderr.String())
	}
}

func TestDocsCommandErrorStages(t *testing.T) {
	t.Parallel()
	for _, example := range []struct{ source, stage string }{{"module M {", "documentation"}, {"module M { type lower = int }", "documentation"}} {
		input := docsSourceFile(t, example.source)
		out := filepath.Join(t.TempDir(), "errors.json")
		var stdout, stderr bytes.Buffer
		if status := commandDocs(context.Background(), []string{input, "--out", out}, &stdout, &stderr); status != 1 || stderr.Len() != 0 {
			t.Fatal(status, stdout.String(), stderr.String())
		}
		encoded, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var stage struct {
			Stage         string
			Errors        []quint.ErrorMessage
			Warnings      []any
			Documentation json.RawMessage
		}
		if err = json.Unmarshal(encoded, &stage); err != nil || stage.Stage != example.stage || len(stage.Errors) == 0 || stage.Warnings == nil || stage.Documentation != nil {
			t.Fatal(string(encoded), err)
		}
		if len(stage.Errors[0].Locations) == 0 || stage.Errors[0].Locations[0].Source != input || !strings.HasPrefix(stage.Errors[0].Explanation, "[QNT") {
			t.Fatal("error location", string(encoded))
		}
	}
	var stdout, stderr bytes.Buffer
	out := filepath.Join(t.TempDir(), "missing.json")
	if status := commandDocs(context.Background(), []string{filepath.Join(t.TempDir(), "missing"), "--out", out}, &stdout, &stderr); status != 1 || stderr.Len() != 0 {
		t.Fatal(status, stdout.String(), stderr.String())
	}
	encoded, err := os.ReadFile(out)
	if err != nil || string(encoded) != `{"stage":"loading","errors":[]}` {
		t.Fatal(string(encoded), err)
	}
}

type docsFailedWriter struct{}

func (docsFailedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDocsCommandWriterAndDiagnosticErrors(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	if status := commandDocs(context.Background(), []string{"--help"}, docsFailedWriter{}, &stderr); status != 1 || !strings.Contains(stderr.String(), io.ErrClosedPipe.Error()) {
		t.Fatal(status, stderr.String())
	}
	stderr.Reset()
	if status := commandDocs(context.Background(), []string{docsSourceFile(t, "module M {}")}, docsFailedWriter{}, &stderr); status != 1 || !strings.Contains(stderr.String(), io.ErrClosedPipe.Error()) {
		t.Fatal(status, stderr.String())
	}
	diagnostics := docsParseDiagnostics(&quint.SyntaxErrors{Issues: []quint.SyntaxIssue{{Code: "QNT006", Message: "custom", Position: quint.Position{Line: 2, Col: 4}}, {Code: "QNT006", Message: "[QNT006] already prefixed"}}}, nil, "source")
	if diagnostics[0].Explanation != "[QNT006] custom" || diagnostics[1].Explanation != "[QNT006] already prefixed" {
		t.Fatal(diagnostics)
	}
	diagnostics = docsParseDiagnostics(errors.New("failure"), nil, "source")
	if len(diagnostics) != 1 || diagnostics[0].Explanation != "failure" || diagnostics[0].Locations == nil {
		t.Fatal(diagnostics)
	}
}
