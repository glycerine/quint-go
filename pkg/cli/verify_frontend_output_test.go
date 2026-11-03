package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/backend/apalache"
	gotlc "github.com/glycerine/quint-go/pkg/backend/tlc"
)

func TestVerifyUpstreamFrontendConsoleFailures(t *testing.T) {
	t.Parallel()
	for _, fixture := range frontendStageFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
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
			backend := verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
				t.Fatal("frontend failure reached Apalache")
				return apalache.VerificationResult{}, nil
			})
			tlc := func(context.Context, quint.CompilationResult, gotlc.Options) (gotlc.Result, error) {
				t.Fatal("frontend failure reached TLC")
				return gotlc.Result{}, nil
			}
			for _, checker := range []string{"apalache", "tlc"} {
				var stdout, stderr bytes.Buffer
				args := []string{input, "--backend=" + checker, "--verbosity=0"}
				status := verifyWithCheckers(context.Background(), args, &stdout, &stderr, backend, tlc)
				if status != fixture.Status || stdout.String() != fixture.Stdout {
					t.Fatal(checker, status, stdout.String(), stderr.String())
				}
				if fixture.Directory {
					// The native read error text differs between Node and Go.
					if !strings.HasPrefix(stderr.String(), "error: file "+input+" could not be opened due to ") {
						t.Fatal(checker, stderr.String())
					}
				} else if got := strings.ReplaceAll(stderr.String(), directory, "<fixture>"); got != fixture.Stderr {
					t.Fatalf("%s stderr %q\nwant %q", checker, got, fixture.Stderr)
				}
				stdout.Reset()
				stderr.Reset()
				output := filepath.Join(directory, checker+"-stage.json")
				if status = verifyWithCheckers(context.Background(), append(args, "--out="+output), &stdout, &stderr, backend, tlc); status != fixture.Status || stdout.Len() != 0 || stderr.Len() != 0 {
					t.Fatal(checker, status, stdout.String(), stderr.String())
				}
				encoded, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				if got, want := coreParityJSON(t, encoded, directory), coreParityJSON(t, fixture.Stage, directory); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s source metadata changed stage JSON\ngot %#v\nwant %#v", checker, got, want)
				}
			}
		})
	}
}

func TestVerifyFrontendReporterDeduplicationAndSuppression(t *testing.T) {
	t.Parallel()
	message := quint.ErrorMessage{Explanation: "loading failed", Locations: []quint.Location{}}
	stage := verifyStage{Frontend: verifyFrontendSources(nil), Status: "failure", Explanation: message.Explanation, Errors: []quint.ErrorMessage{message, message}}
	for _, options := range []verifyOptions{{verbosity: 0}, {verbosity: 5}} {
		var stderr bytes.Buffer
		handled, code := verifyFrontendOutputContext(context.Background(), options, stage, &stderr)
		if !handled || code != 1 || stderr.String() != "error: loading failed\n" || len(stage.Errors) != 2 {
			t.Fatal(handled, code, stderr.String(), stage.Errors)
		}
	}
	for _, options := range []verifyOptions{{out: "stage.json"}, {outITF: "trace.json"}} {
		var stderr bytes.Buffer
		if handled, code := verifyFrontendOutputContext(context.Background(), options, stage, &stderr); !handled || code != 1 || stderr.Len() != 0 {
			t.Fatal(handled, code, stderr.String())
		}
	}
	for _, unrelated := range []verifyStage{{Status: "failure"}, {Frontend: stage.Frontend, BackendResult: true}, {Frontend: stage.Frontend, Status: "ok"}} {
		if handled, _ := verifyFrontendOutputContext(context.Background(), verifyOptions{}, unrelated, io.Discard); handled {
			t.Fatal("frontend reporter consumed a backend result", unrelated)
		}
	}
	failureStage := stage
	failureStage.Explanation = "parsing failed"
	failureStage.Errors = append(append([]quint.ErrorMessage{}, stage.Errors...), quint.ErrorMessage{Explanation: "another error"})
	for _, writer := range []*verifyFrontendFailWriter{{err: io.ErrClosedPipe}, {short: true}} {
		if handled, code := verifyFrontendOutputContext(context.Background(), verifyOptions{}, failureStage, writer); !handled || code != 1 || writer.calls != 1 {
			t.Fatal(handled, code, writer)
		}
	}
}

func TestVerifyFrontendSourceSnapshotAndLoadingErrors(t *testing.T) {
	t.Parallel()
	sources := map[string]string{"M.qnt": "module M { val bad=missing }"}
	metadata := verifyFrontendSources(sources)
	sources["M.qnt"] = "changed"
	if got := metadata.sourceCode.Get("M.qnt"); got != "module M { val bad=missing }" {
		t.Fatal("source snapshot aliases the resolver map", got)
	}
	missing := &quint.SourceLoadError{Path: "M.qnt", Cause: os.ErrNotExist}
	if got := verifyFrontendLoadingError("relative.qnt", missing).Error(); got != "file relative.qnt does not exist" {
		t.Fatal(got)
	}
	if got := verifyFrontendLoadingError("relative.qnt", errors.New("read failed")).Error(); got != "file relative.qnt could not be opened due to read failed" {
		t.Fatal(got)
	}
	for _, explanation := range []string{"context canceled", "load Apalache configuration: read failed"} {
		var stderr bytes.Buffer
		stage := verifyStage{Frontend: metadata, Explanation: explanation}
		if handled, code := verifyFrontendOutputContext(context.Background(), verifyOptions{}, stage, &stderr); !handled || code != 1 || stderr.String() != "error: "+explanation+"\n" {
			t.Fatal(handled, code, stderr.String())
		}
	}
	// Source excerpts stay plain even when the terminal can display color.
	color := 1
	ctx := WithTerminalOptions(context.Background(), TerminalOptions{ColorLevel: &color})
	end := quint.Position{Line: 0, Col: 25}
	stage := verifyStage{Frontend: metadata, Explanation: "parsing failed", Errors: []quint.ErrorMessage{{Explanation: "[QNT404] Name 'missing' not found", Locations: []quint.Location{{Source: "M.qnt", Start: quint.Position{Line: 0, Col: 19}, End: &end}}}}}
	var plain, colored bytes.Buffer
	verifyFrontendOutputContext(context.Background(), verifyOptions{}, stage, &plain)
	verifyFrontendOutputContext(ctx, verifyOptions{}, stage, &colored)
	if colored.String() != plain.String() || !strings.Contains(plain.String(), "^^^^^^^") {
		t.Fatal("primary source excerpt acquired terminal styling", plain.String(), colored.String())
	}
}

func TestVerifyFrontendFailuresBeforeParsingAtVerbosityZero(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	missingConfig := filepath.Join(t.TempDir(), "missing.json")
	backend := verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
		t.Fatal("preparation failure reached the checker")
		return apalache.VerificationResult{}, nil
	})
	for _, test := range []struct {
		ctx    context.Context
		extra  []string
		prefix string
	}{
		{canceled, nil, "error: context canceled\n"},
		{context.Background(), []string{"--apalache-config=" + missingConfig}, "error: load Apalache configuration: "},
	} {
		var stdout, stderr bytes.Buffer
		args := append([]string{"unused.qnt", "--verbosity=0"}, test.extra...)
		if code := verifyWithBackend(test.ctx, args, &stdout, &stderr, backend); code != 1 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), test.prefix) {
			t.Fatal(code, stdout.String(), stderr.String())
		}
	}
}

type verifyFrontendFailWriter struct {
	err   error
	short bool
	calls int
}

func (writer *verifyFrontendFailWriter) Write(data []byte) (int, error) {
	writer.calls++
	if writer.short {
		return len(data) - 1, nil
	}
	return 0, writer.err
}
