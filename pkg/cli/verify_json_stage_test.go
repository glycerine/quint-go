package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/backend/apalache"
	gotlc "github.com/glycerine/quint-go/pkg/backend/tlc"
)

// The oracle executes upstream CLI stages and checker result reporters. Only
// temporary paths are normalized; complete IR, analysis, IDs and trace payloads
// are compared, including fields absent before a checker returns.
func TestVerifyUpstreamJSONStages(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Backend, Result string
		Source                *string
		Directory, Called     bool
		Status                int
		Options               map[string]string
		Trace, Stage          json.RawMessage
		CheckerTraces         []json.RawMessage
	}
	data, err := os.ReadFile("testdata/verify_json_stage.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 21 {
		t.Fatalf("incomplete actual upstream fixture inventory: %d", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			input := filepath.Join(directory, "M.qnt")
			var err error
			if fixture.Directory {
				err = os.Mkdir(input, 0700)
			} else if fixture.Source != nil {
				err = os.WriteFile(input, []byte(*fixture.Source), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			output, itf := filepath.Join(directory, "stage.json"), filepath.Join(directory, "trace.itf.json")
			backend := fixture.Backend
			if backend == "" {
				backend = "apalache"
			}
			args := []string{input, "--backend=" + backend, "--verbosity=0", "--out", output}
			for key, value := range fixture.Options {
				args = append(args, "--"+key+"="+value)
			}
			// --out-itf is independent of shared stage JSON; preserve raw bytes.
			if len(fixture.Trace) > 0 {
				args = append(args, "--out-itf", itf)
			}
			called := false
			check := verifyTestBackend(func(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
				called = true
				switch fixture.Result {
				case "safe":
					return apalache.VerificationResult{CheckResult: apalache.CheckResult{Outcome: apalache.CheckSafe}}, nil
				case "violation", "deadlock":
					outcome := apalache.CheckViolation
					if fixture.Result == "deadlock" {
						outcome = apalache.CheckDeadlock
					}
					return apalache.VerificationResult{CheckResult: apalache.CheckResult{Outcome: outcome, Counterexamples: fixture.CheckerTraces}}, nil
				case "pass-failure":
					data := `{"pass_name":"BoundedChecker","error_data":{"checking_result":"Error","counterexamples":[` + string(fixture.Trace) + `]}}`
					return apalache.VerificationResult{}, &apalache.CommandFailure{Type: "PASS_FAILURE", Data: data}
				default:
					return apalache.VerificationResult{}, errors.New("injected checker result")
				}
			})
			tlc := func(ctx context.Context, compiled quint.CompilationResult, opts gotlc.Options) (gotlc.Result, error) {
				called = true
				if fixture.Result == "preparation" {
					return gotlc.Check(ctx, compiled, opts)
				}
				if fixture.Result == "error" {
					return gotlc.Result{}, errors.New("injected checker result")
				}
				outcome := gotlc.Safe
				if fixture.Result == "violation" {
					outcome = gotlc.Violation
				}
				return gotlc.Result{Outcome: outcome, Complete: true, ITF: fixture.Trace}, nil
			}
			var stdout, stderr bytes.Buffer
			code := verifyWithCheckers(context.Background(), args, &stdout, &stderr, check, tlc)
			got, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if code != fixture.Status || called != fixture.Called || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("code %d called %v stdout %q stderr %q stage %s", code, called, stdout.String(), stderr.String(), got)
			}
			if !reflect.DeepEqual(coreParityJSON(t, got, directory), coreParityJSON(t, fixture.Stage, directory)) {
				t.Fatalf("shared verification stage differs:\ngot %s\nwant %s", got, fixture.Stage)
			}
			if len(fixture.Trace) > 0 {
				raw, err := os.ReadFile(itf)
				input := fixture.Trace
				if backend == "apalache" && fixture.Result != "pass-failure" {
					input = fixture.CheckerTraces[0]
				}
				if err != nil || !bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(input)) {
					t.Fatalf("raw ITF changed: %s (%v)", raw, err)
				}
			}
		})
	}
}
