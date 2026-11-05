package quint

import (
	"context"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func replEvaluate(t *testing.T, session *REPLSession, text, want string) REPLResult {
	t.Helper()
	got, err := session.Evaluate(context.Background(), text)
	if err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	if got.Kind != "value" || got.Value.String() != want {
		t.Fatalf("%s: %s %s, want %s", text, got.Kind, got.Value.String(), want)
	}
	return got
}

func TestREPLSessionIncrementalAnalysis(t *testing.T) {
	t.Parallel()
	session := NewREPLSession()
	ctx := context.Background()
	for _, text := range []string{"pure val x = 2", "pure def f(y) = x + y", "type Box[a] = { value: a }", "pure def box(a): (a) => Box[a] = { value: a }"} {
		got, err := session.Evaluate(ctx, text)
		if err != nil || got.Kind != "declaration" {
			t.Fatal(text, got.Kind, err)
		}
	}
	got := replEvaluate(t, session, "f(3)", "5")
	if got.Type == nil || got.Type.Type.Kind != "int" {
		t.Fatal("inferred expression type", got.Type)
	}
	replEvaluate(t, session, "box(7).value", "7")
	replEvaluate(t, session, "List(1,2,3).foldl(0,(a,b) => a+b)", "6")
	got, err := session.Evaluate(ctx, " // comment only\n")
	if err != nil || got.Kind != "none" {
		t.Fatal(got, err)
	}
	if len(session.History()) != 7 {
		t.Fatal("accepted history", session.History())
	}
}

func TestREPLSessionRejectsInvalidInputsWithoutLeakingDefinitions(t *testing.T) {
	t.Parallel()
	session := NewREPLSession()
	ctx := context.Background()
	for _, text := range []string{"pure val bad = unknown", "pure val bad = 1 + true", "var bad: bool = true", "1 +", "unknown + 1", "pure val bad: bool = 1"} {
		if _, err := session.Evaluate(ctx, text); err == nil {
			t.Fatal("invalid input accepted", text)
		}
	}
	if len(session.History()) != 0 {
		t.Fatal("rejected inputs entered history", session.History())
	}
	if _, err := session.Evaluate(ctx, "pure val bad = 9"); err != nil {
		t.Fatal("rejected binding leaked", err)
	}
	if _, err := session.Evaluate(ctx, "pure val bad = 10"); err == nil {
		t.Fatal("duplicate accepted")
	}
	replEvaluate(t, session, "bad", "9")
	_, err := session.Evaluate(ctx, "1/0")
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code != "QNT503" {
		t.Fatal("runtime error", err)
	}
	if len(session.History()) != 3 {
		t.Fatal("runtime failures should enter expression history", session.History())
	}
	history := session.History()
	history[0] = "changed"
	if session.History()[0] != "pure val bad = 9" {
		t.Fatal("history alias")
	}
}

func TestREPLSessionStateCommitsAndDefinitionEdits(t *testing.T) {
	t.Parallel()
	session := NewREPLSession()
	ctx := context.Background()
	for _, text := range []string{"var x: int", "var y: int", "action init = all { x' = 1, y' = 2 }", "action step = all { x' = x+1, y' = y+2 }"} {
		if _, err := session.Evaluate(ctx, text); err != nil {
			t.Fatal(text, err)
		}
	}
	got := replEvaluate(t, session, "init", "true")
	if !got.Shifted || len(got.Missing) != 0 {
		t.Fatal("init commit", got)
	}
	replEvaluate(t, session, "x + y", "3")
	if _, err := session.Evaluate(ctx, "val doubled = x*2"); err != nil {
		t.Fatal(err)
	}
	replEvaluate(t, session, "doubled", "2")
	closure := replEvaluate(t, session, "(n) => x+n", "((_a0) => \"lambda_1_params\")").Value
	replEvaluate(t, session, "step", "true")
	replEvaluate(t, session, "doubled", "4")
	one, _ := NewRuntimeValue(Expr{Kind: "int", Int: IntegerFromBig(big.NewInt(1))})
	value, err := closure.Apply(ctx, []RuntimeValue{one})
	if err != nil || value.String() != "3" {
		t.Fatal("returned closure lost state owner", value.String(), err)
	}

	state, err := session.State()
	if err != nil || state.String() != "Rec(\"x\", 2, \"y\", 4)" {
		t.Fatal(state.String(), err)
	}
	got = replEvaluate(t, session, "true", "true")
	if got.Shifted || len(got.Missing) != 0 {
		t.Fatal("read-only Boolean shifted", got)
	}
	got = replEvaluate(t, session, "x' = 5", "true")
	if !got.Shifted || strings.Join(got.Missing, ",") != "y" {
		t.Fatal("partial action warnings", got)
	}
	if _, err := session.Evaluate(ctx, "y"); err == nil {
		t.Fatal("missing variable carried over")
	}
	replEvaluate(t, session, "q::lastTrace.length()", "3")
}

// A top-level q::testOnce names action definitions whose bodies read state
// variables that init has already bound. It must not evaluate those actions
// before the simulation binds the variables (QNT502 "Variable n not set").
func TestREPLSessionTestOnceBindsActionStateVariables(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "plain.qnt")
	source := "module plain {\n  var n: int\n  action init = n' = 0\n  action step = n' = n + 1\n  val inv = n < 3\n}\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	session := NewREPLSession()
	if err := session.Load(ctx, path, "plain"); err != nil {
		t.Fatal(err)
	}
	replEvaluate(t, session, "q::testOnce(10, 1, init, step, inv)", `"violation"`)
}

func TestREPLSessionLoadImportsReloadAndClear(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.qnt")
	if err := os.WriteFile(filepath.Join(dir, "base.qnt"), []byte("module Base { pure val x = 40 }"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("module M { import Base.* from \"base\" pure val result = x+2 }"), 0600); err != nil {
		t.Fatal(err)
	}
	session := NewREPLSession()
	seed, _ := ParseInteger("77")
	session.SetSeed(seed)
	if err := session.Load(ctx, path, "M"); err != nil {
		t.Fatal(err)
	}
	replEvaluate(t, session, "result", "42")
	if _, err := session.Evaluate(ctx, "pure val extra = result+1"); err != nil {
		t.Fatal(err)
	}
	replEvaluate(t, session, "extra", "43")
	if err := session.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Evaluate(ctx, "extra"); err == nil {
		t.Fatal("reload retained interactive declarations")
	}
	if session.Seed().String() != "77" {
		t.Fatal("reload rewound seed", session.Seed())
	}
	// No selected module starts an empty scope, while preserving loaded modules
	// for an interactive import, as the TypeScript REPL does.
	if err := session.Load(ctx, path, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Evaluate(ctx, "result"); err == nil {
		t.Fatal("default load silently imported a module")
	}
	if _, err := session.Evaluate(ctx, "import M.*"); err != nil {
		t.Fatal(err)
	}
	replEvaluate(t, session, "result", "42")
	if err := os.WriteFile(path, []byte("// @mainModule M\nmodule M { pure val result=100 }"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := session.Load(ctx, path, ""); err != nil {
		t.Fatal(err)
	}
	replEvaluate(t, session, "result", "100")
	if err := session.Load(ctx, filepath.Join(dir, "missing.qnt"), "M"); err == nil {
		t.Fatal("missing file loaded")
	}
	if err := session.Reload(ctx); err != nil {
		t.Fatal("failed file read lost previous reload target", err)
	}
	replEvaluate(t, session, "result", "100")
	session.Reset()
	state, err := session.State()
	if err != nil || state.String() != "Rec()" || len(session.History()) != 0 || session.Seed().String() != "77" {
		t.Fatal("clear", state.String(), err, session.History(), session.Seed())
	}
	if err := session.Reload(ctx); err != nil {
		t.Fatal("clear lost reload path", err)
	}
	replEvaluate(t, session, "result", "100")
}

func TestREPLSessionCancellationLoadFailuresAndInputSources(t *testing.T) {
	t.Parallel()
	session := NewREPLSession()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := session.Evaluate(ctx, "1"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := session.Load(ctx, "missing.qnt", "M"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(session.History()) != 0 {
		t.Fatal("cancelled input entered history")
	}
	if err := session.Load(context.Background(), filepath.Join(t.TempDir(), "missing.qnt"), "M"); err == nil {
		t.Fatal("missing file loaded")
	}
	replEvaluate(t, session, "1", "1")
	for _, text := range []string{"pure val a=1", "a+2", "pure val b=4"} {
		if _, err := session.Evaluate(context.Background(), text); err != nil {
			t.Fatal(text, err)
		}
	}
	seen := map[string]bool{}
	for _, location := range session.sources {
		seen[location.Source] = true
	}
	if len(seen) != 4 {
		t.Fatal("input source identities", seen)
	}
}

func TestREPLSessionErrorRenderingAndRetainedAnalysisOwnership(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	session := NewREPLSession()
	if _, err := session.Evaluate(ctx, "pure val a = 7"); err != nil {
		t.Fatal(err)
	}
	before := session.analysis.Types.Len()
	_, first := session.Evaluate(ctx, "missing + 1")
	if first == nil {
		t.Fatal("missing name accepted")
	}
	_, second := session.Evaluate(ctx, "1 +")
	if second == nil {
		t.Fatal("syntax error accepted")
	}
	_, third := session.Evaluate(ctx, "pure val b: bool = a")
	if third == nil {
		t.Fatal("invalid annotation accepted")
	}
	if session.analysis.Types.Len() != before {
		t.Fatal("failed analysis mutated committed type table")
	}
	for _, pair := range []struct {
		err  error
		want string
	}{{first, "missing + 1"}, {second, "1 +"}, {third, "pure val b: bool = a"}} {
		message := session.RenderError(pair.err)
		if !strings.Contains(message, pair.want) || !strings.Contains(message, "<input-") || !strings.Contains(message, "^") {
			t.Fatal("source reporting", message)
		}
	}
	if session.RenderError(nil) != "" || session.RenderError(errors.New("other")) != "other" {
		t.Fatal("fallback error rendering")
	}
	got := replEvaluate(t, session, "a", "7")
	got.Type.Type.Kind = "bool"
	replEvaluate(t, session, "a+1", "8")
	if _, err := session.Evaluate(ctx, "pure val b = a+2"); err != nil {
		t.Fatal("failed declaration name leaked", err)
	}
	replEvaluate(t, session, "b", "9")
}

func TestREPLSessionReportsEveryUndefinedName(t *testing.T) {
	t.Parallel()
	session := NewREPLSession()
	ctx := context.Background()
	_, err := session.Evaluate(ctx, "aaa + bbb")
	var list *REPLDiagnostics
	if !errors.As(err, &list) || len(list.Diagnostics) != 2 {
		t.Fatalf("want two diagnostics, got %v", err)
	}
	rendered := session.RenderError(err)
	if strings.Count(rendered, "aaa") == 0 || strings.Count(rendered, "bbb") == 0 {
		t.Fatalf("rendered output misses a name:\n%s", rendered)
	}
	if len(session.History()) != 0 {
		t.Fatal("rejected input entered history", session.History())
	}
	_, err = session.Evaluate(ctx, "unknown + 1")
	var single Diagnostic
	if !errors.As(err, &single) || errors.As(err, &list) {
		t.Fatalf("single undefined name must stay a plain Diagnostic, got %v", err)
	}
}
