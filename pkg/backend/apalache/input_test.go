package apalache

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	checker "github.com/glycerine/apalache-go"
	quint "github.com/glycerine/quint-go/pkg"
)

func compileSource(t *testing.T, source string, inductive ...string) quint.CompilationResult {
	t.Helper()
	parsed, err := quint.ParseModulesWithLocations("M.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := quint.ResolveLinkedModules(parsed.Modules)
	if err != nil || len(linked.Errors) > 0 {
		t.Fatal(err, linked.Errors)
	}
	analyzed, err := quint.AnalyzeModules(parsed.Modules, linked.Table)
	if err != nil || len(analyzed.Diagnostics) > 0 {
		t.Fatal(err, analyzed.Diagnostics)
	}
	inductiveExpression := ""
	if len(inductive) > 0 {
		inductiveExpression = inductive[0]
	}
	compiled, err := quint.CompileModules(analyzed.Modules, quint.AnalysisOutput{Types: analyzed.Types, Effects: analyzed.Effects, Modes: analyzed.Modes}, parsed.IDs, parsed.Sources, quint.CompilationOptions{Main: "M", CompilationEntries: quint.CompilationEntries{Target: "json", Flatten: true, Invariants: []string{"safe"}, InductiveInvariant: inductiveExpression}})
	if err != nil || len(compiled.Diagnostics) > 0 || len(compiled.AnalysisDiagnostics) > 0 {
		t.Fatal(err, compiled.Diagnostics, compiled.AnalysisDiagnostics)
	}
	return compiled
}

func TestSourceToGoApalache(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		safe         bool
	}{
		{"safe", "module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x>=0 }", true},
		{"violation", "module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x<1 }", false},
		{"instance", "module A { const bound:int var x:int action init=x'=0 action step=x'=x+1 val safe=x<=bound } module M { import A(bound=1) as I action init=I::init action step=I::step val safe=I::safe }", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compiled := compileSource(t, tc.source)
			before, _ := json.Marshal(compiled.MainModule)
			data, err := MarshalInput(compiled)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(compiled.MainModule)
			if string(before) != string(after) {
				t.Fatal("compiled module mutated")
			}
			if _, err := checker.ParseQuintJSON(data); err != nil {
				t.Fatal(err)
			}
			result, err := New().Check(context.Background(), compiled, nil, CheckOptions{MaxSteps: 1})
			if err != nil {
				t.Fatal(err)
			}
			if tc.safe {
				if result.Outcome != CheckSafe {
					t.Fatal(result)
				}
			} else {
				if result.Outcome != CheckViolation || len(result.Counterexamples) == 0 {
					t.Fatal(result)
				}
				if !json.Valid(result.Counterexamples[0]) {
					t.Fatal("invalid ITF trace")
				}
			}
			if tc.name == "safe" {
				tla, err := New().TLA(context.Background(), compiled)
				if err != nil || !strings.Contains(tla, "MODULE M") {
					t.Fatalf("TLA: %s %v", tla, err)
				}
				if !strings.Contains(tla, "x = 0") || strings.Contains(tla, "x' = 0") {
					t.Fatalf("init not converted: %s", tla)
				}
			}
		})
	}
}

func TestInputRejections(t *testing.T) {
	compiled := compileSource(t, "module M { action init=true action step=true val safe=true }")
	if _, err := MarshalInput(quint.CompilationResult{}); err == nil {
		t.Fatal("empty compilation accepted")
	}
	broken := compiled
	broken.Diagnostics = []quint.Diagnostic{{Code: quint.QNT501, Message: "bad argument"}}
	if _, err := MarshalInput(broken); err == nil {
		t.Fatal("diagnostics accepted")
	}
	broken = compiled
	broken.MainModule.Declarations = append([]quint.Declaration{{Kind: "import", ProtoName: "Other"}}, compiled.MainModule.Declarations...)
	if _, err := MarshalInput(broken); err == nil || !strings.Contains(err.Error(), "flattened definitions") {
		t.Fatal(err)
	}
}

func TestInputProjectionAndOrderedTypes(t *testing.T) {
	compiled := compileSource(t, "/// docs\nmodule M { type T=int def f(x:T):T=val y=x; y action init=true action step=true val safe=9007199254740993>0 }")
	// Lookup-only metadata must not cross the strict importer boundary, even on
	// nested local definitions and parameters. Its removal must leave source IR intact.
	hidden := true
	for i := range compiled.MainModule.Declarations {
		d := &compiled.MainModule.Declarations[i]
		d.Hidden = &hidden
		d.Namespaces = []string{"N"}
		d.ImportedFrom = json.RawMessage(`{"kind":"import"}`)
		d.Shadowing = json.RawMessage(`true`)
		if d.Name == "f" {
			d.Expr.Params[0].Shadowing = json.RawMessage(`true`)
			d.Expr.Body.Opdef.Hidden = &hidden
			d.Expr.Body.Opdef.Shadowing = json.RawMessage(`true`)
			d.Expr.Body.Opdef.ImportedFrom = json.RawMessage(`{"kind":"import"}`)
		}
	}
	before, _ := json.Marshal(compiled.MainModule)
	data, err := MarshalInput(compiled)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"hidden"`) || strings.Contains(string(data), `"shadowing"`) || strings.Contains(string(data), `"importedFrom"`) || strings.Contains(string(data), `"typedef"`) {
		t.Fatal("frontend-only fields retained")
	}
	if !strings.Contains(string(data), "9007199254740993") {
		t.Fatal("large integer lost")
	}
	var envelope struct {
		Types   json.RawMessage
		Modules []json.RawMessage
	}
	if err = json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	previous := -1
	for id := range compiled.Analysis.Types.All() {
		key, _ := json.Marshal(id)
		index := strings.Index(string(envelope.Types), string(key)+":")
		if index <= previous {
			t.Fatal("type insertion order lost", id)
		}
		previous = index
	}
	after, _ := json.Marshal(compiled.MainModule)
	if string(before) != string(after) {
		t.Fatal("projection mutated source")
	}
	// A transport entry with an unsupported type constructor fails explicitly.
	compiled.Analysis.Types.Set("unresolved", quint.TypeScheme{Type: quint.Type{Kind: "app", Ctor: &quint.Type{Kind: "const", Name: "T"}, Args: []quint.Type{{Kind: "int"}}}})
	if _, err = MarshalInput(compiled); err == nil || !strings.Contains(err.Error(), "unresolved type application") {
		t.Fatal(err)
	}
}
