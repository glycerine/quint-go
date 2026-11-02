package quint

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

type rustComparisonResponse struct {
	Response string          `json:"response"`
	OK       json.RawMessage `json:"ok,omitempty"`
	Err      *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Trace   []ID   `json:"#trace"`
	} `json:"err,omitempty"`
	Diagnostics []struct {
		Label string          `json:"label"`
		Value json.RawMessage `json:"value"`
	} `json:"diagnostics"`
}

type rustComparisonCase struct {
	Name, Source, Seed, Policy, Reason string
	GoSourceStatus                     string
	Input                              Expr
	Typescript                         struct {
		Output *Expr
		Error  *Diagnostic
		Thrown string
		Seed   string
		Picks  []struct {
			Name  string
			Value *Expr
		}
	}
	Rust      rustComparisonResponse
	RustSeed  string
	RustInput []json.RawMessage
}

func readRustComparisonCases(t *testing.T) []rustComparisonCase {
	t.Helper()
	var fixture struct {
		Metadata struct {
			RustPackage, RustBinarySHA256, CargoLockSHA256 string
			RustSourcesSHA256                              map[string]string
		}
		Cases []rustComparisonCase
	}
	data, err := os.ReadFile("testdata/parser/rust_comparison_runtime.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{fixture.Metadata.RustBinarySHA256, fixture.Metadata.CargoLockSHA256} {
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != 32 {
			t.Fatal("missing reference provenance", hash, err)
		}
	}
	if fixture.Metadata.RustPackage != "quint_evaluator 0.2.0" || len(fixture.Metadata.RustSourcesSHA256) < 5 || len(fixture.Cases) == 0 {
		t.Fatal("missing reference metadata or cases")
	}
	return fixture.Cases
}

// Default tests use saved output from the actual Rust binary and TypeScript
// runtime. They require neither Cargo, Node, a Rust executable nor network access.
func TestRustComparisonRuntimeCorpus(t *testing.T) {
	t.Parallel()
	for _, f := range readRustComparisonCases(t) {
		t.Run(f.Name, func(t *testing.T) {
			if f.Reason == "" || f.Source == "" || len(f.RustInput) != 3 || f.Rust.Response != "EvaluationResult" {
				t.Fatal("missing policy explanation, input or actual Rust result")
			}
			seed, err := ParseInteger(f.Seed)
			if err != nil {
				t.Fatal(err)
			}
			rng, storage := NewRNG(seed), NewVariableStorageWithMetadata(true)
			ctx := WithRuntimeHooks(context.Background(), RuntimeHooks{DebugWriter: io.Discard})
			got, err := EvaluateStateExpressionWithRNG(ctx, f.Input, storage, rng)
			if f.GoSourceStatus != "covered" {
				t.Fatal("unknown source comparison status", f.GoSourceStatus)
			}
			wantError := f.Typescript.Error
			if f.Policy == "structured-error" {
				// The TS evaluator wraps its BigInt remainder exception in QNT501;
				// Go deliberately returns the same QNT503 as the Rust reference.
				wantError = &Diagnostic{Code: "QNT503", Message: "Division by zero"}
			}
			if wantError != nil {
				var d Diagnostic
				if !errors.As(err, &d) || d.Code != wantError.Code || d.Message != wantError.Message {
					t.Fatal("Go diagnostic", err, "want", wantError)
				}
			} else {
				if err != nil || f.Typescript.Output == nil || f.Typescript.Thrown != "" {
					t.Fatal("Go evaluation", err, f.Typescript.Thrown)
				}
				want, err := NewRuntimeValue(*f.Typescript.Output)
				if err != nil {
					t.Fatal(err)
				}
				equal, err := got.EqualContext(ctx, want)
				if err != nil || !equal {
					t.Fatal("Go value", got.String(), "TypeScript", want.String(), err)
				}
			}
			if rng.State().String() != f.Typescript.Seed {
				t.Fatal("Go counter", rng.State(), "TypeScript", f.Typescript.Seed)
			}
			for _, pick := range f.Typescript.Picks {
				value, ok := storage.NondetPicks.Get2(pick.Name)
				if !ok || pick.Value == nil {
					t.Fatal("missing nondeterministic pick", pick.Name)
				}
				want, err := NewRuntimeValue(*pick.Value)
				if err != nil || !value.Equal(want) {
					t.Fatal("Go pick", value.String(), "TypeScript", want.String(), err)
				}
			}
			switch f.Policy {
			case "shared", "unordered-value":
				want := decodeRustComparisonValue(t, f.Rust.OK)
				if !got.Equal(want) {
					t.Fatal("Go value", got.String(), "Rust", want.String())
				}
			case "shared-error", "structured-error":
				if f.Rust.Err == nil || f.Rust.Err.Code != wantError.Code || f.Rust.Err.Message != wantError.Message {
					t.Fatal("Rust diagnostic", f.Rust.Err, "Go", err)
				}
				if len(f.Rust.Err.Trace) != 1 || f.Typescript.Error.Reference == nil || f.Rust.Err.Trace[0] != *f.Typescript.Error.Reference {
					t.Fatal("Rust trace and TypeScript source reference differ")
				}
			case "typescript-domain":
				if f.Rust.Err == nil || f.Rust.Err.Code != "QNT601" || f.Typescript.Output == nil {
					t.Fatal("expected documented Rust domain limit")
				}
			case "typescript-error":
				if f.Rust.Err == nil || f.Rust.Err.Code == wantError.Code {
					t.Fatal("expected documented diagnostic policy difference")
				}
			case "typescript-order":
				if got.Equal(decodeRustComparisonValue(t, f.Rust.OK)) {
					t.Fatal("expected documented backend iteration difference")
				}
			case "typescript-seed":
				if f.Rust.Err != nil || len(f.Rust.Diagnostics) != len(f.Typescript.Picks) {
					t.Fatal("missing raw Rust pick diagnostics", f.Rust)
				}
				for _, diagnostic := range f.Rust.Diagnostics {
					_ = decodeRustComparisonValue(t, diagnostic.Value)
				}
			default:
				t.Fatal("unrecognized comparison policy", f.Policy)
			}
		})
	}
}

func decodeRustComparisonValue(t *testing.T, input json.RawMessage) RuntimeValue {
	t.Helper()
	value, err := DecodeITFValue(input, NewIDGenerator())
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewRuntimeValue(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// QUINT_RUST_EVALUATOR=/path/to/quint_evaluator go test ./pkg -run TestRustComparisonLiveReference
func TestRustComparisonLiveReference(t *testing.T) {
	binary := os.Getenv("QUINT_RUST_EVALUATOR")
	if binary == "" {
		t.Skip("set QUINT_RUST_EVALUATOR to replay the actual Rust reference")
	}
	t.Parallel()
	var requests bytes.Buffer
	want := []json.RawMessage{}
	for _, f := range readRustComparisonCases(t) {
		for _, request := range f.RustInput {
			var compact bytes.Buffer
			if err := json.Compact(&compact, request); err != nil {
				t.Fatal(err)
			}
			requests.Write(compact.Bytes())
			requests.WriteByte('\n')
		}
		initialized := json.RawMessage(`{"response":"Initialized","success":true}`)
		evaluated, err := json.Marshal(f.Rust)
		if err != nil {
			t.Fatal(err)
		}
		seed := json.RawMessage(`{"response":"SeedResult","seed":` + f.RustSeed + `}`)
		want = append(want, initialized, evaluated, seed)
	}
	file, err := os.CreateTemp(t.TempDir(), "requests-*.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err = file.Write(requests.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "repl-from-stdin")
	command.Stdin = file
	var output, stderr bytes.Buffer
	command.Stdout, command.Stderr = &output, &stderr
	if err := command.Run(); err != nil {
		t.Fatal(err, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != len(want) {
		t.Fatalf("responses got %d want %d: %s", len(lines), len(want), output.String())
	}
	decode := func(raw []byte) any {
		t.Helper()
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var result any
		if err := decoder.Decode(&result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	for i, line := range lines {
		if !reflect.DeepEqual(decode([]byte(line)), decode(want[i])) {
			t.Fatalf("response %d got %s\nwant %s", i, line, want[i])
		}
	}
}
