package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func assertDefaultSeed(t *testing.T, seed quint.Integer) {
	t.Helper()
	if seed.Big().Sign() < 0 || seed.Big().Cmp(big.NewInt(1<<53-1)) >= 0 {
		t.Fatal("default seed outside TypeScript's initial-state range", seed)
	}
}

func TestRunDefaultSeedRangeAndReplay(t *testing.T) {
	t.Parallel()
	input := runTestSource(t, "module M { var x:int action init={nondet p=Set(3,7,11).oneOf(); x'=p} action step=x'=x+1 }")
	output := filepath.Join(t.TempDir(), "run.json")
	var stdout, stderr bytes.Buffer
	arguments := []string{input, "--max-samples=1", "--max-steps=1", "--out=" + output}
	if code := commandRun(context.Background(), arguments, &stdout, &stderr); code != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal(code, stdout.String(), stderr.String())
	}
	var initial, replay struct {
		Seed  quint.Integer
		Trace []quint.Expr
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &initial); err != nil {
		t.Fatal(err)
	}
	assertDefaultSeed(t, initial.Seed)
	if len(initial.Trace) != 2 {
		t.Fatal("missing generated trace", string(data))
	}
	arguments = append(arguments, "--seed="+initial.Seed.String())
	if code := commandRun(context.Background(), arguments, &stdout, &stderr); code != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal(code, stdout.String(), stderr.String())
	}
	data, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &replay); err != nil {
		t.Fatal(err)
	}
	if replay.Seed != initial.Seed || !reflect.DeepEqual(initial.Trace, replay.Trace) {
		t.Fatal("default run could not be replayed", initial, replay)
	}
}

func TestTestDefaultSeedRangeAndReplay(t *testing.T) {
	t.Parallel()
	input := testCommandFile(t, "module M { val failingTest=false }")
	code, stdout, stderr := testCommandCall(t, context.Background(), input, "--max-samples=1")
	if code != 1 || stderr != "error: Tests failed\n" {
		t.Fatal(code, stdout, stderr)
	}
	match := regexp.MustCompile(`Use --seed=(0x[0-9a-f]+) --match=\^failingTest\$`).FindStringSubmatch(stdout)
	if len(match) != 2 {
		t.Fatal("missing default replay seed", stdout)
	}
	seed, err := quint.ParseSeed(match[1])
	if err != nil {
		t.Fatal(err)
	}
	assertDefaultSeed(t, seed)
	code, replay, stderr := testCommandCall(t, context.Background(), input, "--seed="+match[1])
	if code != 1 || stderr != "error: Tests failed\n" || !strings.Contains(replay, match[0]) {
		t.Fatal("default failed test could not be replayed", code, replay, stderr)
	}
}

func TestREPLDefaultSeedRangeAndExplicitReplay(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := RunREPL(context.Background(), strings.NewReader(".seed\n"), &stdout, &stderr, REPLLoopOptions{Quiet: true}); code != 0 || stderr.Len() != 0 {
		t.Fatal(code, stdout.String(), stderr.String())
	}
	seedText := strings.TrimSuffix(strings.TrimPrefix(stdout.String(), ".seed="), "\n")
	seed, err := quint.ParseSeed(seedText)
	if err != nil {
		t.Fatal(err, stdout.String())
	}
	assertDefaultSeed(t, seed)
	stdout.Reset()
	if code := RunREPL(context.Background(), strings.NewReader(".seed\n.clear\n.seed\n"), &stdout, &stderr, REPLLoopOptions{Quiet: true, Seed: &seed}); code != 0 || stderr.Len() != 0 {
		t.Fatal(code, stdout.String(), stderr.String())
	}
	if want := ".seed=" + seed.String() + "\n\n.seed=" + seed.String() + "\n"; stdout.String() != want {
		t.Fatal("explicit seed or .clear changed the replay state", stdout.String(), want)
	}
}

func TestUpstreamREPLSeedInitialization(t *testing.T) {
	t.Parallel()
	var cases []struct{ Seed, Input, Stdout, Stderr string }
	data, err := os.ReadFile("../testdata/parser/repl_rng_initialization.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 5 {
		t.Fatal("missing upstream initial/reset seed histories")
	}
	for _, example := range cases {
		t.Run(example.Seed, func(t *testing.T) {
			seed, err := quint.ParseSeed(example.Seed)
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := RunREPL(context.Background(), strings.NewReader(example.Input), &stdout, &stderr, REPLLoopOptions{Quiet: true, Seed: &seed})
			if code != 0 || stdout.String() != example.Stdout || stderr.String() != example.Stderr {
				t.Fatalf("code %d\nGo stdout %q stderr %q\nTypeScript stdout %q stderr %q", code, stdout.String(), stderr.String(), example.Stdout, example.Stderr)
			}
		})
	}
}
