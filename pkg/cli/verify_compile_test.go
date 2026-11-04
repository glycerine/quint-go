package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/backend/apalache"
)

func verifyCompileInput(t *testing.T, input string) (quint.FrontendResult, quint.AnalysisResult) {
	t.Helper()
	resolver := quint.NewFileSourceResolver(nil, nil)
	front, err := quint.ParseProjectRecovering(resolver, quint.SourceLookupPath{NormalizedPath: input, SourceName: input})
	if err != nil || len(front.Diagnostics) > 0 {
		t.Fatal(err, front.Diagnostics)
	}
	analysis, err := quint.AnalyzeModules(front.Modules, front.Table)
	if err != nil || len(analysis.Diagnostics) > 0 {
		t.Fatal(err, analysis.Diagnostics)
	}
	return front, analysis
}

func verifyAnalysisJSON(t *testing.T, analysis quint.AnalysisResult) string {
	t.Helper()
	fields := insdict.NewDict[string, any]()
	fields.Set("types", coreStringTable(analysis.Types))
	fields.Set("effects", coreStringTable(analysis.Effects))
	fields.Set("modes", coreStringTable(analysis.Modes))
	encoded, err := coreMarshalStage(fields)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func verifyCompilationJSON(t *testing.T, compiled quint.CompilationResult) string {
	t.Helper()
	fields := insdict.NewDict[string, any]()
	fields.Set("modules", compiled.Modules)
	fields.Set("unflattened", compiled.UnflattenedModules)
	fields.Set("mainModule", compiled.MainModule)
	fields.Set("main", compiled.Main)
	fields.Set("table", coreLookupTable(compiled.Table))
	fields.Set("types", coreStringTable(compiled.Analysis.Types))
	fields.Set("effects", coreStringTable(compiled.Analysis.Effects))
	fields.Set("modes", coreStringTable(compiled.Analysis.Modes))
	fields.Set("diagnostics", compiled.Diagnostics)
	fields.Set("analysisDiagnostics", compiled.AnalysisDiagnostics)
	fields.Set("reportNames", compiled.InvariantReportNames)
	encoded, err := coreMarshalStage(fields)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestVerifyReportingCompilationOwnsPrivateView(t *testing.T) {
	t.Parallel()
	input := filepath.Join(t.TempDir(), "M.qnt")
	source := "module M { var x:int action init=x'=1 action step=x'=x+1 val q::reportInvariant0=false val q::reportInvariant0_1=false }"
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	front, analysis := verifyCompileInput(t, input)
	controlFront, controlAnalysis := verifyCompileInput(t, input)
	options := quint.CompilationOptions{Main: "M", CompilationEntries: quint.CompilationEntries{Target: "json", Flatten: true, Invariants: []string{"x>0,x<3", "x<0"}, InductiveInvariant: "x>=0"}}
	published, private, err := verifyCompileModules(front, analysis, options, true)
	if err != nil || len(published.Diagnostics) > 0 || len(private.Diagnostics) > 0 || len(private.AnalysisDiagnostics) > 0 {
		t.Fatal(err, published.Diagnostics, private.Diagnostics, private.AnalysisDiagnostics)
	}
	control, err := quint.CompileModules(controlAnalysis.Modules, quint.AnalysisOutput{Types: controlAnalysis.Types, Effects: controlAnalysis.Effects, Modes: controlAnalysis.Modes}, controlFront.IDs, controlFront.Sources, options)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := verifyCompilationJSON(t, published), verifyCompilationJSON(t, control); got != want {
		t.Fatalf("private instrumentation changed the public compilation:\ngot %s\nwant %s", got, want)
	}
	if !reflect.DeepEqual(front.Sources, controlFront.Sources) {
		t.Fatal("private instrumentation changed the caller source map")
	}
	if got, want := verifyAnalysisJSON(t, analysis), verifyAnalysisJSON(t, controlAnalysis); got != want {
		t.Fatalf("private instrumentation changed caller analysis:\ngot %s\nwant %s", got, want)
	}
	if front.IDs.Next() != controlFront.IDs.Next() {
		t.Fatal("private instrumentation advanced the caller's ID sequence")
	}
	if !reflect.DeepEqual(private.InvariantReportNames, []string{"q::reportInvariant0_2", "q::reportInvariant1"}) {
		t.Fatal(private.InvariantReportNames)
	}
	violated, err := apalache.FindViolatedInvariants(context.Background(), private, options.Invariants, json.RawMessage(`{"vars":["x"],"states":[{"x":1}]}`))
	if err != nil || !reflect.DeepEqual(violated, []int{1}) {
		t.Fatal("comma predicates or private name selection evaluated incorrectly", violated, err)
	}
	private.Analysis.Types.Set("private", quint.TypeScheme{Type: quint.Type{Kind: "int"}})
	private.Analysis.Effects.Set("private", quint.EffectScheme{Effect: quint.Effect{Kind: "concrete"}})
	private.Analysis.Modes.Set("private", quint.Val)
	_, callerType := analysis.Types.Get2("private")
	_, callerEffect := analysis.Effects.Get2("private")
	_, callerMode := analysis.Modes.Get2("private")
	_, publicType := published.Analysis.Types.Get2("private")
	_, publicEffect := published.Analysis.Effects.Get2("private")
	_, publicMode := published.Analysis.Modes.Get2("private")
	if callerType || callerEffect || callerMode || publicType || publicEffect || publicMode {
		t.Fatal("private dictionaries alias the source compilation")
	}
}

func TestVerifyPrivateAnalysisClonesNestedValues(t *testing.T) {
	t.Parallel()
	id, _ := quint.ParseID("7")
	analysis := quint.AnalysisOutput{Types: insdict.NewDict[string, quint.TypeScheme](), Effects: insdict.NewDict[string, quint.EffectScheme](), Modes: insdict.NewDict[string, quint.Qualifier]()}
	analysis.Types.Set("7", quint.TypeScheme{Type: quint.Type{Kind: "list", ID: &id, Elem: &quint.Type{Kind: "var", Name: "a"}}, TypeVariables: []string{"a"}, RowVariables: []string{"r"}})
	analysis.Effects.Set("7", quint.EffectScheme{Effect: quint.Effect{Kind: "arrow", Params: []quint.Effect{{Kind: "variable", Name: "e"}}, Result: &quint.Effect{Kind: "concrete", Components: []quint.EffectComponent{{Kind: "read", Entity: quint.Entity{Kind: "union", Reference: &id, Entities: []quint.Entity{{Kind: "concrete", StateVariables: []quint.StateVariable{quint.NewStateVariable("x", id)}}}}}}}}, EffectVariables: []string{"e"}, EntityVariables: []string{"r"}})
	analysis.Modes.Set("7", quint.Val)
	before, _ := json.Marshal([]any{analysis.Types.Get("7"), analysis.Effects.Get("7")})
	copy, err := verifyCloneAnalysis(analysis)
	if err != nil {
		t.Fatal(err)
	}
	scheme := copy.Types.Get("7")
	scheme.Type.Elem.Name = "changed"
	*scheme.Type.ID, _ = quint.ParseID("99")
	scheme.TypeVariables[0], scheme.RowVariables[0] = "changed", "changed"
	effect := copy.Effects.Get("7")
	effect.Effect.Params[0].Name = "changed"
	effect.Effect.Result.Components[0].Entity.Entities[0].StateVariables[0].Name = "changed"
	*effect.Effect.Result.Components[0].Entity.Reference, _ = quint.ParseID("98")
	effect.EffectVariables[0], effect.EntityVariables[0] = "changed", "changed"
	copy.Modes.Set("7", quint.Action)
	after, _ := json.Marshal([]any{analysis.Types.Get("7"), analysis.Effects.Get("7")})
	if string(before) != string(after) || analysis.Modes.Get("7") != quint.Val {
		t.Fatal("private compilation shares mutable analysis values")
	}
	if copy, err := verifyCloneAnalysis(quint.AnalysisOutput{}); err != nil || copy.Types != nil || copy.Effects != nil || copy.Modes != nil {
		t.Fatal(copy, err)
	}
	analysis.Types.Set("broken", quint.TypeScheme{Type: quint.Type{Kind: "list"}})
	if _, err := verifyCloneAnalysis(analysis); err == nil || !strings.Contains(err.Error(), "missing child") {
		t.Fatal("invalid caller analysis must fail before private compilation", err)
	}
	ids := quint.NewIDGenerator()
	invalid := quint.AnalysisResult{Effects: analysis.Effects, Modes: analysis.Modes}
	invalid.Types = analysis.Types
	if _, _, err := verifyCompileModules(quint.FrontendResult{IDs: ids}, invalid, quint.CompilationOptions{}, true); err == nil || ids.Next().String() != "1" {
		t.Fatal("invalid private analysis advanced the caller's generator", err)
	}
}
