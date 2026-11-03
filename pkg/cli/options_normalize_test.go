package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type cliOptionsFixture struct {
	Name, Command, Input, Error, Pending string
	Args, Commands                       []string
	Options                              map[string]any
	Help, Version                        bool
}

func cliOptionsFixtures(t *testing.T) []cliOptionsFixture {
	t.Helper()
	data, err := os.ReadFile("../testdata/parser/cli_options.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []cliOptionsFixture
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func cliFixtureArgs(fixture cliOptionsFixture) []string {
	args := fixture.Args
	if len(args) > 0 && args[0] == fixture.Command {
		args = args[1:]
	}
	return args
}

func cliFixtureString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func TestUpstreamCommonCLIFlagNormalization(t *testing.T) {
	t.Parallel()
	for _, fixture := range cliOptionsFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			args := cliFixtureArgs(fixture)
			help, version, _ := coreInformationalOptions(args)
			if fixture.Name == "run_short_h_contract" {
				// -h is an established Go CLI extension. Upstream advertises
				// only --help and reports the short spelling as unknown.
				if !help {
					t.Fatal("Go short help extension was lost")
				}
				return
			}
			if help != fixture.Help || version != fixture.Version {
				t.Fatal("informational option precedence", args, help, version, fixture.Help, fixture.Version)
			}
			if fixture.Help || fixture.Version {
				var out, stderr bytes.Buffer
				if code := Execute(context.Background(), fixture.Args, &out, &stderr); code != 0 || out.Len() == 0 || stderr.Len() > 0 {
					t.Fatal(code, out.String(), stderr.String())
				}
				return
			}
			normalized, err := cliNormalizeOptions(fixture.Command, args)
			if err != nil {
				if err.Error() != fixture.Error {
					t.Fatalf("Go %q, upstream %q", err, fixture.Error)
				}
				return
			}
			if fixture.Error != "" {
				// These validations belong to the command, rather than token
				// normalization. The Go port additionally permits literal
				// filenames after -- and selects its own backend names.
				switch fixture.Name {
				case "compile_invalid_target", "compile_invalid_endpoint", "run_invalid_backend", "run_trace_limit_validation", "run_seed_trace_limit_validation", "verify_invalid_backend", "parse_positionals_after_terminator":
				default:
					t.Fatal("upstream option error was silently accepted", fixture.Error, normalized)
				}
			}
			arrays := map[string][]string{}
			var positionals []string
			for i, token := range normalized {
				if token == "--" {
					positionals = normalized[i+1:]
					break
				}
				name, value, inline := strings.Cut(strings.TrimPrefix(token, "--"), "=")
				if !inline {
					t.Fatal("normalizer emitted a flag with ambiguous arity", normalized)
				}
				expected, exists := fixture.Options[name]
				if fixture.Command == "test" && name == "out-itf" && fixture.Options["output"] != nil {
					expected, exists = fixture.Options["output"], true
				}
				if !exists {
					t.Fatal("normalized flag absent from actual upstream argv", name, value, fixture.Options)
				}
				switch expected := expected.(type) {
				case bool:
					if value != strconv.FormatBool(expected) {
						t.Fatal(name, value, expected)
					}
				case float64:
					if got := coreNumberOption(value); got != expected {
						t.Fatal(name, value, expected)
					}
				case []any:
					arrays[name] = append(arrays[name], value)
				case map[string]any:
					endpoint := expected["hostname"].(string) + ":" + cliFixtureString(expected["port"])
					if value != endpoint {
						t.Fatal(name, value, endpoint)
					}
				case string:
					numericNaN := expected == "NaN" && math.IsNaN(coreNumberOption(value))
					if value != expected && !numericNaN {
						t.Fatal(name, value, expected)
					}
				default:
					t.Fatal(name, expected)
				}
			}
			for name, got := range arrays {
				var want []string
				for _, item := range fixture.Options[name].([]any) {
					want = append(want, cliFixtureString(item))
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatal(name, got, want)
				}
			}
			if !strings.Contains(strings.Join(args, "\x00"), "--\x00") && fixture.Error == "" {
				want := fixture.Commands
				if fixture.Command != "repl" {
					want = []string{fixture.Input}
				}
				if len(positionals) != len(want) || len(want) > 0 && !reflect.DeepEqual(positionals, want) {
					t.Fatal("positional ownership changed", positionals, want)
				}
			}
		})
	}
}

func TestCLIFlagAdaptersUseNormalizedValues(t *testing.T) {
	t.Parallel()
	run, err := runParseOptions([]string{"M.qnt", "--maxSamples=2.0", "--maxSteps=0x10", "--nTraces=2", "--nThreads=1", "--seed", "--verbosity=2e0", "--mbt", "false", "--invariants=first", "second", "--witnesses", "--hide"})
	if err != nil || run.maxSamples != 2 || run.maxSteps != 16 || run.traces != 2 || run.verbosity != 2 || run.metadata || !run.seedSet || run.seed != "0" || !reflect.DeepEqual(run.invariants, []string{"first", "second"}) || len(run.witnesses) != 0 || len(run.hide) != 0 {
		t.Fatal(run, err)
	}
	test, err := testParseOptions([]string{"--maxSamples=1e2", "M.qnt", "--seed=", "--outItf=first", "--output=second", "--verbosity=2.0"})
	if err != nil || test.maxSamples != 100 || test.seed != "0" || test.itf != "second" || test.verbosity != 2 {
		t.Fatal(test, err)
	}
	test, err = testParseOptions([]string{"M.qnt", "--main", "--out", "--seed", "--verbosity=0"})
	if err != nil || test.main != "" || test.out != "" || test.seed != "0" || test.maxSamples != 1 {
		t.Fatal(test, err)
	}
	repl, help, err := replParseOptions([]string{"1+2", "-q", "false", "val x=1", "--seed", "--no-quiet", "--verbosity=2.0", ".exit"})
	if err != nil || help != "" || repl.Quiet || repl.Verbosity != 2 || repl.Seed == nil || repl.Seed.String() != "0" || !reflect.DeepEqual(repl.Commands, []string{"1+2", "val x=1", ".exit"}) {
		t.Fatal(repl, help, err)
	}
	verify, wantsHelp, err := verifyParseOptions([]string{"M.qnt", "--maxSteps=0x10", "--randomTransitions=1", "--invariants=first", "second", "--inductiveInvariant=ind", "--outItf=trace.json", "--apalacheConfig=config.json"}, io.Discard, io.Discard)
	if err != nil || wantsHelp || verify.maxSteps != 16 || verify.randomTransitions || !reflect.DeepEqual([]string(verify.invariants), []string{"first", "second"}) || verify.inductive != "ind" || verify.outITF != "trace.json" || verify.config != "config.json" {
		t.Fatal(verify, wantsHelp, err)
	}
	verify, _, err = verifyParseOptions([]string{"M.qnt", "--backend=tlc", "--invariants", "--verbosity"}, io.Discard, io.Discard)
	if err != nil || verify.verbosity != 3 || len(verify.invariants) != 0 {
		t.Fatal(verify, err)
	}
	for _, command := range []string{"parse", "typecheck", "docs"} {
		args := []string{"M.qnt", "--out"}
		if command == "docs" {
			if options, _, err := docsParseOptions(args); err != nil || options.input != "M.qnt" || options.out != "" {
				t.Fatal(options, err)
			}
		} else if options, err := coreParseOptions(command, args); err != nil || options.input != "M.qnt" || options.out != "" {
			t.Fatal(options, err)
		}
	}
}

func TestCLIInterspersedREPLStartupAndInformationalFlags(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"repl", "1+2", "-q", ".exit", "--seed="},
		{"1+2", "--quiet", ".exit", "--seed"},
	} {
		var out, stderr bytes.Buffer
		if code := Execute(context.Background(), args, &out, &stderr); code != 0 || out.String() != "3\n" || stderr.Len() > 0 {
			t.Fatal(args, code, out.String(), stderr.String())
		}
	}
	var out, stderr bytes.Buffer
	if code := Execute(context.Background(), []string{"run", "M.qnt", "--help=false", "--version=false", "--unknown"}, &out, &stderr); code != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "Unknown argument: unknown") {
		t.Fatal(code, out.String(), stderr.String())
	}
}

func TestCLIFlagErrorsHelpAndExecutionBoundaries(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"parse", "typecheck", "compile", "run", "test", "verify", "repl", "docs"} {
		for _, informational := range []string{"--help", "--version"} {
			var stderr bytes.Buffer
			if status := Execute(context.Background(), []string{command, "--unknown", informational}, verifyBrokenWriter{}, &stderr); status != 1 || stderr.Len() == 0 {
				t.Fatal("help/version writer error lost", command, informational, status, stderr.String())
			}
		}
		_, err := cliNormalizeOptions(command, []string{"--out=a", "--out=b"})
		if command != "repl" && (err == nil || err.Error() != "--out can not be specified more than once") {
			t.Fatal(command, err)
		}
	}
	for _, test := range []struct {
		command string
		args    []string
	}{
		{"run", []string{"M.qnt", "--max-steps=1.5"}},
		{"test", []string{"M.qnt", "--max-samples=NaN"}},
		{"verify", []string{"M.qnt", "--max-steps=Infinity"}},
		{"repl", []string{"--verbosity=2.5"}},
	} {
		// These actual upstream parser values are pinned in pending oracle
		// cases. The Go execution APIs still require bounded integer inputs;
		// normalization must preserve the unsupported value without rounding.
		normalized, err := cliNormalizeOptions(test.command, test.args)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, token := range normalized {
			_, value, _ := strings.Cut(token, "=")
			if number := coreNumberOption(value); math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number {
				found = true
			}
		}
		if !found {
			t.Fatal("unsupported numeric value was silently rounded", test, normalized)
		}
	}
}
