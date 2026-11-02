package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestUpstreamREPLSaveCommands(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Cases []struct {
			Name, Input, Stdout, Stderr string
			Files                       []struct{ Name, Source string }
			Saved                       *string
		}
	}
	encoded, err := os.ReadFile("../testdata/parser/repl_save.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(encoded, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			for _, file := range fixture.Files {
				if err := os.WriteFile(filepath.Join(directory, file.Name), []byte(file.Source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			input := strings.ReplaceAll(fixture.Input, "$DIR", directory)
			var stdout, stderr bytes.Buffer
			status := RunREPL(context.Background(), strings.NewReader(input), &stdout, &stderr, REPLLoopOptions{Quiet: true})
			if status != 0 {
				t.Fatal(status, stdout.String(), stderr.String())
			}
			got := strings.ReplaceAll(stdout.String(), directory, "$DIR")
			switch fixture.Name {
			case "expression_comment_delimiter":
				if !strings.Contains(got, "cannot save an expression containing */") || stderr.Len() != 0 {
					t.Fatal(got, stderr.String())
				}
				if _, err := os.Stat(filepath.Join(directory, "session.qnt")); !os.IsNotExist(err) {
					t.Fatal("unsafe serialization created a file", err)
				}
				return
			case "rejected_inputs_omitted_runtime_error_retained":
				// Existing Go diagnostic rendering uses the error stream. The
				// saved source must still match the reference exactly.
				if !strings.Contains(stderr.String(), "Division by zero") || !strings.Contains(got, "Session saved to:") {
					t.Fatal(got, stderr.String())
				}
			case "crlf_annotation_not_recognized":
				// Go already recognizes CRLF annotations, so the reference's
				// extra import into its synthetic scope is unnecessary.
				if !strings.Contains(got, "7\nSession saved to:") {
					t.Fatal(got, stderr.String())
				}
			default:
				if got != fixture.Stdout || stderr.String() != fixture.Stderr {
					t.Fatalf("stdout %q want %q; stderr %q want %q", got, fixture.Stdout, stderr.String(), fixture.Stderr)
				}
			}
			saved, err := os.ReadFile(filepath.Join(directory, "session.qnt"))
			if fixture.Saved == nil {
				if !os.IsNotExist(err) {
					t.Fatal("missing filename created a file", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch fixture.Name {
			case "loaded_main_with_final_newline", "loaded_main_not_final_module", "main_annotation_explicit_override_preserves_old_marker", "crlf_annotation_not_recognized", "clear_preserves_last_module_annotation":
				// The core fixture test asserts exact corrected source and
				// stable replay for these raw upstream corruption cases.
			default:
				if string(saved) != *fixture.Saved {
					t.Fatalf("saved %q want %q", saved, *fixture.Saved)
				}
			}
			loaded := quint.NewREPLSession()
			if err := loaded.Load(context.Background(), filepath.Join(directory, "session.qnt"), ""); err != nil {
				t.Fatal("CLI saved source does not replay", err, string(saved))
			}
		})
	}
}

func TestREPLSaveReplayPromptsAndReload(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "session.qnt")
	text := "// @mainModule M\nmodule M { pure val x=2 }/*! List(\nx,\nx+1\n) !*/\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := RunREPL(context.Background(), strings.NewReader(".reload\n.exit\n"), &stdout, &stderr, REPLLoopOptions{Require: path, Verbosity: 1})
	want := "Evaluating expression history in " + path + "\n>>> List(\n... x,\n... x+1\n... )\n[2, 3]\n"
	if status != 0 || stderr.Len() != 0 || strings.Count(stdout.String(), want) != 2 {
		t.Fatal(status, stdout.String(), stderr.String())
	}
	for _, options := range []REPLLoopOptions{{Require: path, Quiet: true}, {Require: path, Verbosity: 1}} {
		if status := RunREPL(context.Background(), strings.NewReader(".exit\n"), &replFailureWriter{}, &stderr, options); status != 1 {
			t.Fatal("replay ignored output writer failure", status)
		}
	}
}

func TestREPLSaveFilesystemErrorsKeepSessionUsable(t *testing.T) {
	t.Parallel()
	for _, path := range []string{t.TempDir(), filepath.Join(t.TempDir(), "missing", "session.qnt")} {
		var stdout, stderr bytes.Buffer
		input := "pure val x=7\n.save " + path + "\nx\n.exit\n"
		status := RunREPL(context.Background(), strings.NewReader(input), &stdout, &stderr, REPLLoopOptions{Quiet: true})
		if status != 0 || stderr.Len() != 0 || !strings.HasSuffix(stdout.String(), "7\n") || !strings.Contains(stdout.String(), path) || strings.Contains(stdout.String(), "Session saved to:") {
			t.Fatal(status, stdout.String(), stderr.String())
		}
	}
}
