package quint

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/glycerine/insdict"
)

func TestRuntimeSourceCallDetection(t *testing.T) {
	t.Parallel()
	id, err := ParseID("7")
	if err != nil {
		t.Fatal(err)
	}
	table := insdict.NewDictFunc[ID, LookupDefinition](runtimeSourceIDHash)
	table.Set(id, LookupDefinition{ID: id, Name: "f"})
	call := Expr{ID: id, Kind: "app", Opcode: "f"}
	for _, test := range []struct {
		name       string
		expression Expr
		table      *insdict.Dict[ID, LookupDefinition]
		want       bool
	}{
		{"resolved", call, table, true},
		{"without table", call, nil, false},
		{"builtin", Expr{Kind: "app", Opcode: "iadd"}, table, false},
		{"name", Expr{ID: id, Kind: "name", Name: "f"}, table, false},
		{"argument", Expr{Kind: "app", Opcode: "List", Args: []Expr{call}}, table, true},
		{"lambda body", Expr{Kind: "lambda", Body: &call}, table, true},
		{"local definition", Expr{Kind: "let", Opdef: &OperatorDef{Expr: &call}}, table, true},
		{"empty definition", Expr{Kind: "let", Opdef: &OperatorDef{}}, table, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := runtimeHasSourceCall(test.expression, test.table); got != test.want {
				t.Fatal(got, test.want)
			}
		})
	}
}

func TestRuntimePureBuiltinProjectKeepsSourceFastPath(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"module M {val result=1+2}",
		"module M {val result=Set(1,2).map(x=>x+1)}",
		"module M {def unused(x)=x+1 val result=3}",
	} {
		ids := NewIDGenerator()
		modules, err := parseModulesWithIDs(source, ids)
		if err != nil {
			t.Fatal(err)
		}
		evaluator, err := NewProjectEvaluator(context.Background(), modules, ids, "M", nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if evaluator.module.runtimeSource != nil || evaluator.execution.source != nil {
			t.Fatal("builtin-only project retained an unnecessary source graph", source)
		}
		if _, err := evaluator.Evaluate(context.Background(), "result", nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRuntimeSourceCallDisplayOwnershipAndSavedContexts(t *testing.T) {
	t.Parallel()
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs("module A {def inc(x)=x+1} module M {import A as C def saved(x_18)=C::inc(x_18)}", ids)
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(modules)
	if err != nil {
		t.Fatal(err)
	}
	next := ids.next.String()
	evaluator, err := NewProjectEvaluator(context.Background(), modules, ids, "M", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(modules)
	if err != nil || string(before) != string(after) || ids.next.String() != next {
		t.Fatal("preparation mutated caller modules or IDs", err)
	}
	// The original names and bodies belong to the evaluator, even before its
	// first preparation and after the caller discards or changes parsed IR.
	for i := range modules {
		for j := range modules[i].Declarations {
			declaration := &modules[i].Declarations[j]
			if declaration.Expr != nil {
				declaration.Expr.Params[0].Name = "changed"
				*declaration.Expr.Body = Expr{Kind: "bool", Bool: false}
			}
		}
	}
	closure, err := evaluator.Evaluate(context.Background(), "saved", nil)
	if err != nil {
		t.Fatal(err)
	}
	one, err := ParseInteger("1")
	if err != nil {
		t.Fatal(err)
	}
	argument, err := NewRuntimeValue(Expr{Kind: "int", Int: one})
	if err != nil {
		t.Fatal(err)
	}
	first, second := NewTraceRecorder(3, evaluator.RNG), NewTraceRecorder(3, evaluator.RNG)
	for _, listener := range []*TraceRecorder{first, second, nil} {
		ctx := context.Background()
		if listener != nil {
			ctx = WithExecutionListener(ctx, listener)
		}
		result, err := closure.Apply(ctx, []RuntimeValue{argument})
		if err != nil || result.String() != "2" {
			t.Fatal(result.String(), err)
		}
	}
	for _, recorder := range []*TraceRecorder{first, second} {
		frame := recorder.CurrentFrame()
		if len(frame.Subframes) != 1 || ExpressionString(frame.Subframes[0].App) != "C::inc(x_18)" {
			t.Fatal("source callback used another invocation's listener or flattened names", frame)
		}
		// Mutating a returned recorder snapshot cannot corrupt a later display.
		frame.Subframes[0].App.Args[0].Name = "changed"
		if ExpressionString(recorder.CurrentFrame().Subframes[0].App) != "C::inc(x_18)" {
			t.Fatal("recorder leaked its retained source application")
		}
	}
	canceled, cancel := context.WithCancel(WithExecutionListener(context.Background(), second))
	cancel()
	if _, err := closure.Apply(canceled, []RuntimeValue{argument}); !errors.Is(err, context.Canceled) {
		t.Fatal("saved callback ignored invocation cancellation", err)
	}
	if len(second.CurrentFrame().Subframes) != 1 {
		t.Fatal("canceled callback changed the captured tree")
	}
}

func BenchmarkRuntimeSourceDisplayPreparation(b *testing.B) {
	for _, test := range []struct{ name, source string }{
		{"builtin", "module M {val result=Set(1,2).map(x=>x+1)}"},
		{"pure_callback", "module M {def inc(x)=x+1 def apply(f,x)=f(x) def result=apply(x=>inc(x),2)}"},
		{"imported_callback", "module A {def inc(x)=x+1 def apply(f,x)=f(x)} module M {import A as C def result=C::apply(x=>C::inc(x),2)}"},
	} {
		b.Run(test.name, func(b *testing.B) {
			ids := NewIDGenerator()
			modules, err := parseModulesWithIDs(test.source, ids)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := NewProjectEvaluator(context.Background(), modules, ids, "M", nil, false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
