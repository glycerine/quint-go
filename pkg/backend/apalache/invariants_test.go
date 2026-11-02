package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func compileInvariantReports(t *testing.T, source string, invariants []string) quint.CompilationResult {
	t.Helper()
	parsed, err := quint.ParseModulesWithLocations("M.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := quint.ResolveLinkedModules(parsed.Modules)
	if err != nil || len(linked.Errors) != 0 {
		t.Fatal(err, linked.Errors)
	}
	sorted, diagnostics := quint.SortModuleDeclarations(parsed.Modules, linked.Table)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	analyzed, err := quint.AnalyzeModules(sorted, linked.Table)
	if err != nil || len(analyzed.Diagnostics) != 0 {
		t.Fatal(err, analyzed.Diagnostics)
	}
	compiled, err := quint.CompileModules(analyzed.Modules, quint.AnalysisOutput{Types: analyzed.Types, Effects: analyzed.Effects, Modes: analyzed.Modes}, parsed.IDs, parsed.Sources, quint.CompilationOptions{Main: "M", CompilationEntries: quint.CompilationEntries{Target: "json", Flatten: true, Invariants: invariants, InvariantReports: true}})
	if err != nil || len(compiled.Diagnostics) != 0 || len(compiled.AnalysisDiagnostics) != 0 {
		t.Fatal(err, compiled.Diagnostics, compiled.AnalysisDiagnostics)
	}
	return compiled
}

func invariantTrace(t *testing.T, state string) json.RawMessage {
	t.Helper()
	parsed, err := quint.ParseExpressionOrDeclaration(state, quint.NewIDGenerator())
	if err != nil || parsed.Expr == nil {
		t.Fatal(err, parsed)
	}
	value, err := quint.EvaluatePureExpression(context.Background(), *parsed.Expr, nil)
	if err != nil {
		t.Fatal(err)
	}
	trace, err := quint.EncodeITFTrace(nil, []quint.Expr{value.Expression()}, false)
	if err != nil {
		t.Fatal(err)
	}
	return trace
}

func TestUpstreamIndividualInvariantReports(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/verification_invariants.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name, Declarations, State string
		Invariants                []string
		Violated                  []int
		Error                     *struct{ Code, Message string }
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			compiled := compileInvariantReports(t, "module M { "+fixture.Declarations+" action init=true action step=true }", fixture.Invariants)
			before, _ := json.Marshal(compiled)
			trace := invariantTrace(t, fixture.State)
			traceBefore := string(trace)
			got, err := FindViolatedInvariants(context.Background(), compiled, fixture.Invariants, trace)
			if fixture.Error != nil {
				var diagnostic quint.Diagnostic
				if !errors.As(err, &diagnostic) || string(diagnostic.Code) != fixture.Error.Code || diagnostic.Message != fixture.Error.Message || got != nil {
					t.Fatalf("got %v, %v; expected %+v", got, err, fixture.Error)
				}
			} else if err != nil || !reflect.DeepEqual(got, fixture.Violated) {
				t.Fatalf("got %v, %v; want %v", got, err, fixture.Violated)
			}
			after, _ := json.Marshal(compiled)
			if string(before) != string(after) || traceBefore != string(trace) {
				t.Fatal("reporting mutated compilation or trace")
			}
		})
	}
}

func TestInvariantReportsFromActualChecker(t *testing.T) {
	compiled := compileInvariantReports(t, "module M {var x:int var y:int action init=all{x'=1,y'=0} action step=all{x'=x,y'=y}}", []string{"x>0", "y>0", "x<0"})
	checked, err := New().Check(context.Background(), compiled, nil, CheckOptions{MaxSteps: 0})
	if err != nil || checked.Outcome != CheckViolation || len(checked.Counterexamples) == 0 {
		t.Fatal(checked, err)
	}
	got, err := FindViolatedInvariants(context.Background(), compiled, []string{"x>0", "y>0", "x<0"}, checked.Counterexamples[0])
	if err != nil || !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatal(got, err)
	}
}

func TestInvariantReportsInstancesAndLastState(t *testing.T) {
	compiled := compileInvariantReports(t, "module A {const limit:int var x:int val safe=x<limit} module M {import A(limit=3) as I action init=I::x'=0 action step=I::x'=I::x+1}", []string{"I::safe", "I::x<5"})
	trace := json.RawMessage(`{"vars":["I::x"],"states":[{"I::x":{"#bigint":"0"}},{"I::x":{"#bigint":"3"}}],"loop":0}`)
	for _, declaration := range compiled.MainModule.Declarations {
		if declaration.Kind == "var" {
			trace = json.RawMessage(strings.ReplaceAll(string(trace), "I::x", declaration.Name))
		}
	}
	got, err := FindViolatedInvariants(context.Background(), compiled, []string{"I::safe", "I::x<5"}, trace)
	if err != nil || !reflect.DeepEqual(got, []int{0}) {
		t.Fatal(got, err)
	}
}

func TestInvariantReportFailures(t *testing.T) {
	compiled := compileInvariantReports(t, "module M {var x:int action init=x'=0 action step=x'=x}", []string{"x>0"})
	if got, err := FindViolatedInvariants(context.Background(), compiled, nil, nil); err != nil || len(got) != 0 || got == nil {
		t.Fatal(got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FindViolatedInvariants(ctx, compiled, []string{"x>0"}, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, raw := range []string{`{`, `{}`, `{"states":[]}`, `{"states":[1]}`, `{"states":[{"x":{"#unserializable":"Int"}}]}`} {
		if _, err := FindViolatedInvariants(context.Background(), compiled, []string{"x>0"}, json.RawMessage(raw)); err == nil {
			t.Fatal("accepted invalid trace", raw)
		}
	}
	trace := json.RawMessage(`{"states":[{"x":1}]}`)
	if _, err := FindViolatedInvariants(context.Background(), compiled, []string{"x>0", "missing"}, trace); err == nil || !strings.Contains(err.Error(), "prepare invariant") {
		t.Fatal(err)
	}
	for _, alter := range []func(*quint.CompilationResult){
		func(c *quint.CompilationResult) { c.MainModule.Declarations[0].Kind = "invalid" },
		func(c *quint.CompilationResult) { c.MainModule.Name = "" },
		func(c *quint.CompilationResult) {
			c.MainModule.Declarations = append(c.MainModule.Declarations, quint.Declaration{Kind: "import", ProtoName: "A"})
		},
	} {
		invalid := compiled
		invalid.MainModule.Declarations = append([]quint.Declaration{}, compiled.MainModule.Declarations...)
		alter(&invalid)
		if _, err := FindViolatedInvariants(context.Background(), invalid, []string{"x>0"}, trace); err == nil {
			t.Fatal("invalid reporting module accepted")
		}
	}
	for i, declaration := range compiled.MainModule.Declarations {
		if declaration.Name == quint.InvariantReportName(0) {
			compiled.MainModule.Declarations[i].Expr = &quint.Expr{Kind: "int"}
		}
	}
	if _, err := FindViolatedInvariants(context.Background(), compiled, []string{"x>0"}, trace); err == nil || !strings.Contains(err.Error(), "Boolean") {
		t.Fatal(err)
	}
}
