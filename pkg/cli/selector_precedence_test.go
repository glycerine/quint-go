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

	quint "github.com/glycerine/quint-go/pkg"
)

func TestUpstreamCLISelectorPrecedence(t *testing.T) {
	t.Parallel()
	type stage struct {
		Stage                   string
		Errors                  []quint.ErrorMessage
		Passed, Failed, Ignored []string
	}
	var fixtures struct {
		Cases []struct {
			Name, Command, Input string
			Files                []struct{ Name, Source string }
			Args                 []string
			Outcome              struct {
				Kind, Summary string
				Status        int
				Stage, Before stage
				Exception     struct{ Name, Message string }
			}
			Terminal *struct {
				Stdout, Stderr string
				Status         int
			}
		}
	}
	data, err := os.ReadFile("../testdata/parser/cli_selector_precedence.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
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
			output := filepath.Join(directory, "stage.json")
			args := []string{fixture.Command, filepath.Join(directory, fixture.Input), "--seed=0", "--out=" + output}
			args = append(args, fixture.Args...)
			if fixture.Command == "run" {
				args = append(args, "--max-steps=0")
			}
			var stdout, stderr bytes.Buffer
			status := Execute(context.Background(), args, &stdout, &stderr)
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("--out emitted text: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var got stage
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if fixture.Outcome.Kind == "throw" {
				// Upstream throws during discovery. The Go CLI converts that
				// boundary into a reported failure, preserving its last stage.
				if fixture.Outcome.Exception.Name != "SyntaxError" || fixture.Outcome.Exception.Message == "" {
					t.Fatal("fixture did not retain the actual upstream exception")
				}
				if status != 1 || got.Stage != fixture.Outcome.Before.Stage || len(got.Errors) != 1 || !strings.Contains(got.Errors[0].Explanation, "--match") {
					t.Fatalf("Go handling of upstream discovery exception: status=%d stage=%+v before=%+v", status, got, fixture.Outcome.Before)
				}
				return
			}
			want := fixture.Outcome.Stage
			if status != fixture.Outcome.Status || got.Stage != want.Stage || len(got.Errors) != len(want.Errors) {
				t.Fatalf("status=%d stage=%+v; want status=%d stage=%+v", status, got, fixture.Outcome.Status, want)
			}
			if want.Stage == "running" {
				// Selector errors have no source location; compare the entire
				// ordered payload, including duplicate entries and [] versus
				// absent locations/stacks, after decoding the actual wire data.
				if !reflect.DeepEqual(got.Errors, want.Errors) {
					t.Fatalf("selector diagnostics=%#v; want %#v", got.Errors, want.Errors)
				}
			} else {
				for i, problem := range want.Errors {
					// Type inference's free-variable identifiers are internal;
					// the diagnostic header identifies which validation won.
					gotHeader := strings.SplitN(got.Errors[i].Explanation, "\n", 2)[0]
					wantHeader := strings.SplitN(problem.Explanation, "\n", 2)[0]
					if gotHeader != wantHeader {
						t.Fatalf("error %d header=%q; want %q", i, gotHeader, wantHeader)
					}
				}
			}
			if !reflect.DeepEqual(got.Passed, want.Passed) || !reflect.DeepEqual(got.Failed, want.Failed) || !reflect.DeepEqual(got.Ignored, want.Ignored) {
				t.Fatalf("selection result=%+v; want %+v", got, want)
			}
			if fixture.Terminal != nil {
				args = []string{fixture.Command, filepath.Join(directory, fixture.Input), "--seed=0", "--verbosity=0", "--max-steps=0"}
				args = append(args, fixture.Args...)
				stdout.Reset()
				stderr.Reset()
				status = Execute(context.Background(), args, &stdout, &stderr)
				if status != fixture.Terminal.Status || stdout.String() != fixture.Terminal.Stdout || stderr.String() != fixture.Terminal.Stderr {
					t.Fatalf("terminal status=%d stdout=%q stderr=%q; want %+v", status, stdout.String(), stderr.String(), fixture.Terminal)
				}
			}
		})
	}
}
