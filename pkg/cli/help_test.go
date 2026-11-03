package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

type cliTranscript struct {
	Name, Kind, Command, Stdout, Stderr string
	Args                                []string
	Status                              int
}

func cliTranscriptFixtures(t *testing.T) []cliTranscript {
	t.Helper()
	encoded, err := os.ReadFile("../testdata/parser/cli_transcripts.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct{ Cases []cliTranscript }
	if err := json.Unmarshal(encoded, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Cases) != 49 {
		t.Fatalf("incomplete upstream transcript inventory: %d", len(fixtures.Cases))
	}
	return fixtures.Cases
}

func TestUpstreamCLIHelpAndArgumentErrors(t *testing.T) {
	t.Parallel()
	for _, fixture := range cliTranscriptFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := Execute(context.Background(), fixture.Args, &stdout, &stderr)
			switch fixture.Kind {
			case "help":
				if code != fixture.Status || stderr.Len() != 0 {
					t.Fatal(code, stdout.String(), stderr.String())
				}
				assertCLIHelpCompatibility(t, fixture.Command, stdout.String(), fixture.Stdout)
			case "version":
				if code != fixture.Status || stdout.String() != strings.ReplaceAll(fixture.Stdout, "<version>", Version) || stderr.String() != fixture.Stderr {
					t.Fatal(code, stdout.String(), stderr.String())
				}
			default:
				expected := fixture.Stderr
				switch fixture.Name {
				case "unknown_short_h":
					// -h is a retained Go convenience alias; upstream only
					// declares --help. Its complete output is covered below.
					if code != 0 || stderr.Len() != 0 || stdout.String() != cliHelpTexts["run"] {
						t.Fatal(code, stdout.String(), stderr.String())
					}
					return
				case "invalid_run_backend", "invalid_test_backend", "invalid_repl_backend":
					expected = strings.ReplaceAll(expected, `"typescript", "rust"`, `"go"`)
				case "compile_invalid_endpoint", "verify_invalid_endpoint":
					// Remote Scala server selection is deliberately rejected:
					// both verification backends run inside this Go process.
					if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported") || !strings.Contains(stderr.String(), "Go Apalache") {
						t.Fatal(code, stdout.String(), stderr.String())
					}
					return
				}
				if code != fixture.Status || stdout.String() != fixture.Stdout || stderr.String() != expected {
					t.Fatalf("code %d want %d; stdout %q want %q; stderr %q want %q", code, fixture.Status, stdout.String(), fixture.Stdout, stderr.String(), expected)
				}
			}
		})
	}
}

var cliHelpOptionStart = regexp.MustCompile(`^\s+(?:-[a-zA-Z], )?--([a-zA-Z-]+)\s+`)

func cliHelpParts(text string) (string, map[string]string) {
	header, options, _ := strings.Cut(text, "\nOptions:\n")
	blocks := map[string]string{}
	name := ""
	for _, line := range strings.Split(options, "\n") {
		if match := cliHelpOptionStart.FindStringSubmatch(line); match != nil {
			name = match[1]
		}
		if name != "" {
			blocks[name] += " " + strings.TrimSpace(line)
		}
	}
	for name, block := range blocks {
		// Metadata may move to the preceding line when Go-specific options
		// change the builder's alignment width. Keep its content exact.
		blocks[name] = strings.ReplaceAll(strings.Join(strings.Fields(block), " "), ")[", ") [")
	}
	return strings.Join(strings.Fields(header), " "), blocks
}

func assertCLIHelpCompatibility(t *testing.T, command, got, upstream string) {
	t.Helper()
	if !strings.HasSuffix(got, "\n") {
		t.Fatal("help must end with a newline")
	}
	gotHeader, gotOptions := cliHelpParts(got)
	wantHeader, wantOptions := cliHelpParts(upstream)
	if command == "" || command == "verify" {
		wantHeader = strings.ReplaceAll(wantHeader, "via Apalache", "via Go Apalache or Go TLC")
	}
	if command == "compile" || command == "verify" {
		delete(wantOptions, "apalache-version")
		delete(wantOptions, "server-endpoint")
	}
	if command == "" || command == "repl" || command == "run" || command == "test" {
		wantOptions["backend"] = strings.ReplaceAll(wantOptions["backend"], `"typescript", "rust"`, `"go"`)
		wantOptions["backend"] = strings.ReplaceAll(wantOptions["backend"], `[default: "rust"]`, `[default: "go"]`)
	}
	if command == "run" {
		wantOptions["n-threads"] = "--n-threads number of simulator threads (the Go simulator supports 1) [number] [default: 1]"
	}
	if command == "verify" {
		wantOptions["random-transitions"] = strings.ReplaceAll(wantOptions["random-transitions"], "simulation)", "simulation); unsupported by the current Go checker")
		wantOptions["tlc-config"] = strings.ReplaceAll(wantOptions["tlc-config"], "a TLC configuration", "a native Go TLC configuration")
	}
	if gotHeader != wantHeader || !reflect.DeepEqual(gotOptions, wantOptions) {
		t.Fatalf("help differs beyond documented Go backend choices:\nheader %q want %q\noptions %#v\nwant %#v", gotHeader, wantHeader, gotOptions, wantOptions)
	}
	if command == "parse" || command == "typecheck" || command == "docs" {
		if got != upstream {
			t.Fatal("backend-independent help must match the complete upstream text")
		}
	}
}

func TestCLIArgumentErrorsAcrossCommands(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"parse", "typecheck", "compile", "run", "test", "verify", "docs", "repl"} {
		var stdout, stderr bytes.Buffer
		if code := Execute(context.Background(), []string{command, "M.qnt", "--unknown"}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.String() != "Unknown argument: unknown\n" {
			t.Fatal(command, code, stdout.String(), stderr.String())
		}
		stdout.Reset()
		stderr.Reset()
		if code := Execute(context.Background(), []string{command, "-h"}, &stdout, &stderr); code != 0 || stdout.String() != cliHelpTexts[command] || stderr.Len() != 0 {
			t.Fatal(command, code, stdout.String(), stderr.String())
		}
	}
}

type cliHelpFailWriter struct{ short bool }

func (writer cliHelpFailWriter) Write(value []byte) (int, error) {
	if writer.short {
		return len(value) - 1, nil
	}
	return 0, errors.New("help writer failed")
}

func TestCLIHelpChecksWriterFailures(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"parse", "typecheck", "compile", "run", "test", "verify", "docs", "repl"} {
		for _, short := range []bool{false, true} {
			var stderr bytes.Buffer
			want := "help writer failed"
			if short {
				want = io.ErrShortWrite.Error()
			}
			if code := Execute(context.Background(), []string{command, "--help"}, cliHelpFailWriter{short: short}, &stderr); code != 1 || !strings.Contains(stderr.String(), want) {
				t.Fatal(command, short, code, stderr.String())
			}
		}
	}
}
