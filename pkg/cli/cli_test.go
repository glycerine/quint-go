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

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

func coreTestFile(t *testing.T, name, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func coreInvoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestCoreFrontendCommands(t *testing.T) {
	t.Parallel()
	path := coreTestFile(t, "different.qnt", "module M { pure val answer = 999999999999999999999999999999999999 }\n")
	for _, command := range []string{"parse", "typecheck"} {
		code, stdout, stderr := coreInvoke(t, command, path)
		if code != 0 || stdout != "" || stderr != "" {
			t.Fatal(command, code, stdout, stderr)
		}
		out := filepath.Join(t.TempDir(), "stage.json")
		code, stdout, stderr = coreInvoke(t, command, "--out", out, path)
		if code != 0 || stdout != "" || stderr != "" {
			t.Fatal(command, code, stdout, stderr)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		wantStage := "parsing"
		if command == "typecheck" {
			wantStage = "typechecking"
			if len(fields["types"]) == 0 || len(fields["effects"]) == 0 || fields["modes"] != nil {
				t.Fatal(string(data))
			}
		}
		if string(fields["stage"]) != strconvQuote(wantStage) || string(fields["errors"]) != "[]" || string(fields["warnings"]) != "[]" || len(fields["table"]) == 0 || !bytes.Contains(data, []byte("999999999999999999999999999999999999")) {
			t.Fatal(string(data))
		}
	}
	code, stdout, stderr := coreInvoke(t, "compile", path, "--flatten=false")
	if code != 0 || stderr != "" {
		t.Fatal(code, stdout, stderr)
	}
	var compiled struct {
		Stage   string
		Main    string
		Modules []quint.Module
	}
	if err := json.Unmarshal([]byte(stdout), &compiled); err != nil || compiled.Stage != "compiling" || compiled.Main != "M" || len(compiled.Modules) != 1 || len(compiled.Modules[0].Declarations) != 1 {
		t.Fatal(stdout, err)
	}
}

func strconvQuote(text string) string { data, _ := json.Marshal(text); return string(data) }

func TestCoreCompileFlattenImportsAndTLA(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Lib.qnt"), []byte("module Lib { pure val n = 1 }"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "file.qnt")
	source := "module Main {\nimport Lib.* from \"Lib\"\nvar x: int\naction init = x' = n\naction step = x' = x + n\n}"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := coreInvoke(t, "compile", path)
	if code != 0 || stderr != "" {
		t.Fatal(code, stdout, stderr)
	}
	var compiled struct {
		Main    string
		Modules []quint.Module
	}
	if err := json.Unmarshal([]byte(stdout), &compiled); err != nil || compiled.Main != "Main" || len(compiled.Modules) != 1 {
		t.Fatal(stdout, err)
	}
	names := []string{}
	for _, declaration := range compiled.Modules[0].Declarations {
		names = append(names, declaration.Name)
	}
	if !strings.Contains(strings.Join(names, ","), "q::init") || !strings.Contains(strings.Join(names, ","), "q::step") {
		t.Fatal(names)
	}
	code, stdout, stderr = coreInvoke(t, "compile", path, "--target=tlaplus", "--flatten=false")
	if code != 0 || !strings.Contains(stdout, "---- MODULE Main ----") || !strings.Contains(stdout, "VARIABLE") || !strings.Contains(stderr, "flattening is required") {
		t.Fatal(code, stdout, stderr)
	}
}

func TestCoreFrontendFailuresAndOutput(t *testing.T) {
	t.Parallel()
	valid := coreTestFile(t, "file.qnt", "module M { pure val n = 1 }")
	undefined := coreTestFile(t, "file.qnt", "module M { pure val n = missing }")
	invalidType := coreTestFile(t, "file.qnt", "module M { pure val n: bool = 1 }")
	syntax := coreTestFile(t, "file.qnt", "module M { val n = }")
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"parse", filepath.Join(t.TempDir(), "absent.qnt")}, "does not exist"},
		{[]string{"parse", undefined}, "QNT404"},
		{[]string{"parse", syntax}, "parsing failed"},
		{[]string{"typecheck", invalidType}, "typechecking failed"},
		{[]string{"compile", valid}, "init"},
		{[]string{"compile", valid, "--flatten=false", "--main=Absent"}, "module Absent does not exist"},
		{[]string{"parse", valid, "--out", filepath.Join(t.TempDir(), "missing", "stage.json")}, "no such file"},
		{[]string{"parse", valid, "--source-map", filepath.Join(t.TempDir(), "missing", "source.json")}, "no such file"},
	} {
		code, stdout, stderr := coreInvoke(t, test.args...)
		if code != 1 || stdout != "" || !strings.Contains(stderr, test.want) {
			t.Fatal(test.args, code, stdout, stderr)
		}
	}
	out := filepath.Join(t.TempDir(), "failure.json")
	code, stdout, stderr := coreInvoke(t, "typecheck", invalidType, "--out", out)
	if code != 1 || stdout != "" || stderr != "" {
		t.Fatal(code, stdout, stderr)
	}
	data, err := os.ReadFile(out)
	if err != nil || !bytes.Contains(data, []byte(`"stage":"typechecking"`)) || !bytes.Contains(data, []byte(`"explanation"`)) {
		t.Fatal(string(data), err)
	}
	code, stdout, stderr = coreInvoke(t, "compile", valid, "--flatten=false", "--out", out)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatal(code, stdout, stderr)
	}
}

func TestCoreOptionValidation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		command string
		args    []string
		want    string
	}{
		{"parse", nil, "exactly one"},
		{"parse", []string{"a", "b"}, "exactly one"},
		{"parse", []string{"--wat", "a"}, "Unknown argument: wat"},
		{"parse", []string{"-x", "a"}, "Unknown argument: x"},
		{"parse", []string{"--out"}, "Not enough"},
		{"parse", []string{"--out=a", "--out=b", "file"}, "more than once"},
		{"typecheck", []string{"--source-map=a", "file"}, "Unknown argument"},
		{"compile", []string{"--target=wat", "file"}, "Invalid value"},
		{"compile", []string{"--flatten=wat", "file"}, "true or false"},
		{"compile", []string{"--no-flatten=false", "file"}, "Unknown argument"},
		{"compile", []string{"--no-flatten", "--flatten", "file"}, "more than once"},
		{"compile", []string{"--verbosity=-1", "file"}, "between 0 and 5"},
		{"compile", []string{"--verbosity=2.1", "file"}, "between 0 and 5"},
		{"compile", []string{"--apalache-version=x", "file"}, "in-process Go Apalache"},
		{"compile", []string{"--server-endpoint=host:8822", "file"}, "in-process Go Apalache"},
	} {
		_, err := coreParseOptions(test.command, test.args)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatal(test.command, test.args, err)
		}
	}
	o, err := coreParseOptions("compile", []string{"--flatten", "false", "file", "--main", "M", "--init=first", "--step", "second", "--invariant=inv", "--temporal=prop", "--verbosity=0", "--out=out"})
	if err != nil || o.input != "file" || o.flatten || o.main != "M" || *o.init != "first" || *o.step != "second" || o.invariant != "inv" || o.temporal != "prop" || o.verbosity != 0 || o.out != "out" {
		t.Fatal(o, err)
	}
	o, err = coreParseOptions("parse", []string{"--", "--file.qnt"})
	if err != nil || o.input != "--file.qnt" {
		t.Fatal(o, err)
	}
}

func TestCoreHelpVersionCancellationAndWriters(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--help"}, {"-h"}, {"parse", "--help"}, {"typecheck", "-h"}, {"compile", "--help"}} {
		code, stdout, stderr := coreInvoke(t, args...)
		if code != 0 || !strings.Contains(stdout, "quint") || stderr != "" {
			t.Fatal(args, code, stdout, stderr)
		}
	}
	code, stdout, stderr := coreInvoke(t, "--version")
	if code != 0 || stdout != Version+"\n" || stderr != "" {
		t.Fatal(code, stdout, stderr)
	}
	if code, _, _ = coreInvoke(t, "--version", "extra"); code != 1 {
		t.Fatal(code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if Execute(ctx, []string{"parse", "missing.qnt"}, nil, &output) != 1 || !strings.Contains(output.String(), "context canceled") {
		t.Fatal(output.String())
	}
	if Execute(context.Background(), []string{"--version"}, coreFailWriter{}, io.Discard) != 1 || Execute(context.Background(), []string{"--help"}, coreFailWriter{}, io.Discard) != 1 {
		t.Fatal("ignored writer failure")
	}
	if Execute(context.Background(), []string{"--help"}, nil, nil) != 0 {
		t.Fatal("nil streams")
	}
	file := coreTestFile(t, "file.qnt", "module M { pure val n = 1 }")
	if coreFrontendCommand(context.Background(), "compile", []string{file, "--flatten=false"}, coreFailWriter{}, io.Discard) != 1 {
		t.Fatal("ignored compilation writer failure")
	}
}

type coreFailWriter struct{}

func (coreFailWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestCoreStageLookupSerializationAndSourceMaps(t *testing.T) {
	t.Parallel()
	file := coreTestFile(t, "M.qnt", "module M { pure def f(x: int): int = x; pure val answer = f(9) }")
	out, locations := filepath.Join(t.TempDir(), "stage.json"), filepath.Join(t.TempDir(), "sources.json")
	code, stdout, stderr := coreInvoke(t, "parse", file, "--out", out, "--source-map", locations)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatal(code, stdout, stderr)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"Declaration"`)) || bytes.Contains(data, []byte(`"Parameter"`)) || !bytes.Contains(data, []byte(`"kind":"param"`)) || !bytes.Contains(data, []byte(`"depth":0`)) {
		t.Fatal(string(data))
	}
	data, err = os.ReadFile(locations)
	if err != nil {
		t.Fatal(err)
	}
	var compact struct {
		SourceIndex map[string]string
		Map         map[string][]json.RawMessage
	}
	if err = json.Unmarshal(data, &compact); err != nil || compact.SourceIndex["0"] != file || len(compact.Map) < 4 {
		t.Fatal(string(data), err)
	}
	for _, location := range compact.Map {
		if len(location) != 3 || string(location[0]) != "0" {
			t.Fatal(location)
		}
	}
	fields := insdict.NewDict[string, any]()
	fields.Set("first", 1)
	fields.Set("second", true)
	fields.Set("first", 2)
	encoded, err := coreMarshalStage(fields)
	if err != nil || string(encoded) != `{"first":2,"second":true}` {
		t.Fatal(string(encoded), err)
	}
	fields.Set("bad", func() {})
	if _, err = coreMarshalStage(fields); err == nil {
		t.Fatal("accepted unencodable stage")
	}
}
