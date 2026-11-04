package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/backend/apalache"
)

type verifyCompileFixture struct {
	Name, Source, Stdout, Stderr string
	Options                      map[string]json.RawMessage
	Called                       bool
	Status                       int
	Stage                        json.RawMessage
}

func verifyCompileFixtures(t *testing.T) []verifyCompileFixture {
	t.Helper()
	encoded, err := os.ReadFile("../testdata/parser/verify_compile_failures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct{ Cases []verifyCompileFixture }
	if err = json.Unmarshal(encoded, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Cases) != 37 {
		t.Fatal("incomplete upstream selector fixture inventory", len(fixtures.Cases))
	}
	return fixtures.Cases
}

func verifyCompileFixtureArgs(t *testing.T, options map[string]json.RawMessage) []string {
	t.Helper()
	keys := make([]string, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	args := []string{}
	for _, key := range keys {
		var scalar string
		if err := json.Unmarshal(options[key], &scalar); err == nil {
			args = append(args, "--"+key+"="+scalar)
			continue
		}
		var values []string
		if err := json.Unmarshal(options[key], &values); err != nil {
			t.Fatal(err)
		}
		for _, value := range values {
			args = append(args, "--"+key+"="+value)
		}
	}
	return args
}

func TestVerifyUpstreamCompileSelectorFailures(t *testing.T) {
	t.Parallel()
	for _, fixture := range verifyCompileFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			input := filepath.Join(directory, "M.qnt")
			if err := os.WriteFile(input, []byte(fixture.Source), 0600); err != nil {
				t.Fatal(err)
			}
			args := append([]string{input, "--verbosity=0"}, verifyCompileFixtureArgs(t, fixture.Options)...)
			called := false
			backend := verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
				called = true
				return apalache.VerificationResult{}, errors.New("injected checker result")
			})
			var stdout, stderr bytes.Buffer
			code := verifyWithBackend(context.Background(), args, &stdout, &stderr, backend)
			if code != fixture.Status || called != fixture.Called {
				t.Fatal(code, called, stdout.String(), stderr.String())
			}
			wantOut := fixture.Stdout
			if fixture.Name == "step-syntax" {
				// The upstream recovering parser leaks an internal DEBUG
				// line for this hole. Keep that raw oracle, while Go reports
				// the user-facing argument error without implementation logs.
				if wantOut != "[DEBUG] generating undefined expr to fill hole in: x=\n" {
					t.Fatal("unexpected upstream parser output", wantOut)
				}
				wantOut = ""
			}
			if stdout.String() != wantOut || strings.ReplaceAll(stderr.String(), directory, "<fixture>") != fixture.Stderr {
				t.Fatalf("stdout %q want %q\nstderr %q want %q", stdout.String(), wantOut, stderr.String(), fixture.Stderr)
			}
			called = false
			stdout.Reset()
			stderr.Reset()
			output := filepath.Join(directory, "stage.json")
			code = verifyWithBackend(context.Background(), append(args, "--out="+output), &stdout, &stderr, backend)
			if code != fixture.Status || called != fixture.Called || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatal(code, called, stdout.String(), stderr.String())
			}
			encoded, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(coreParityJSON(t, encoded, directory), coreParityJSON(t, fixture.Stage, directory)) {
				t.Fatalf("selector stage differs:\ngot %s\nwant %s", encoded, fixture.Stage)
			}
		})
	}
}

func TestVerifyMalformedPropertySelectorsReturnFailure(t *testing.T) {
	t.Parallel()
	encoded, err := os.ReadFile("../testdata/parser/verify_compile_failures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Cases  []verifyCompileFixture
		Thrown []struct {
			Name, Class, Message string
			Options              map[string]json.RawMessage
		}
	}
	if err = json.Unmarshal(encoded, &fixtures); err != nil || len(fixtures.Thrown) != 4 || len(fixtures.Cases) == 0 {
		t.Fatal(err, len(fixtures.Thrown))
	}
	for _, fixture := range fixtures.Thrown {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			// Upstream throws from parseDefOrThrow before a result stage
			// exists. Preserve that oracle and return a checked Go failure.
			if fixture.Class != "Error" || !strings.Contains(fixture.Message, "parsing") {
				t.Fatal("unexpected upstream exception", fixture.Class, fixture.Message)
			}
			directory := t.TempDir()
			input, output := filepath.Join(directory, "M.qnt"), filepath.Join(directory, "stage.json")
			if err := os.WriteFile(input, []byte(fixtures.Cases[0].Source), 0600); err != nil {
				t.Fatal(err)
			}
			backend := verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
				t.Fatal("malformed selector reached the checker")
				return apalache.VerificationResult{}, nil
			})
			args := append([]string{input, "--verbosity=0"}, verifyCompileFixtureArgs(t, fixture.Options)...)
			var stdout, stderr bytes.Buffer
			if code := verifyWithBackend(context.Background(), args, &stdout, &stderr, backend); code != 1 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "error: ") {
				t.Fatal(code, stdout.String(), stderr.String())
			}
			stdout.Reset()
			stderr.Reset()
			if code := verifyWithBackend(context.Background(), append(args, "--out="+output), &stdout, &stderr, backend); code != 1 || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatal(code, stdout.String(), stderr.String())
			}
			encoded, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var stage struct {
				Stage  string
				Errors []json.RawMessage
			}
			if err := json.Unmarshal(encoded, &stage); err != nil || stage.Stage != "typechecking" || len(stage.Errors) != 0 {
				t.Fatal(err, string(encoded))
			}
		})
	}
}
