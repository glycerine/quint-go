package quint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/glycerine/insdict"
)

func analyzedProjectFixture(t testing.TB, source string, extra []string) (AnalysisResult, []Declaration, *IDGenerator) {
	t.Helper()
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs(source, ids)
	if err != nil {
		t.Fatal(err)
	}
	modules, diagnostics := SortModules(modules)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	linked, err := ResolveLinkedModules(modules)
	if err != nil || len(linked.Errors) != 0 {
		t.Fatal(err, linked.Errors)
	}
	modules, diagnostics = SortModuleDeclarations(modules, linked.Table)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	previous, err := AnalyzeModules(modules, linked.Table)
	if err != nil || len(previous.Diagnostics) != 0 {
		t.Fatal(err, previous.Diagnostics)
	}
	additions := []Declaration{}
	for _, text := range extra {
		parsed, err := ParseExpressionOrDeclaration(text, ids)
		if err != nil || parsed.Kind != "declaration" {
			t.Fatal(text, err, parsed.Kind)
		}
		additions = append(additions, parsed.Declarations...)
	}
	return previous, additions, ids
}

func analyzedProjectModules(previous AnalysisResult, additions []Declaration, main string) []Module {
	working := append([]Module{}, previous.Modules...)
	for i := range working {
		if working[i].Name == main {
			working[i].Declarations = append(append([]Declaration{}, working[i].Declarations...), additions...)
		}
	}
	return working
}

// Retain the checked constructor's complete source-analysis formula so the
// incremental path also compares raw flattened lookup/type/effect IDs.
func analyzedProjectFullOracle(ctx context.Context, modules []Module, ids *IDGenerator) (FlatteningResult, error) {
	working, diagnostics := SortModules(modules)
	if len(diagnostics) != 0 {
		return FlatteningResult{}, diagnostics[0]
	}
	linked, err := ResolveLinkedModules(working)
	if err != nil {
		return FlatteningResult{}, err
	}
	if len(linked.Errors) != 0 {
		return FlatteningResult{}, linked.Errors[0]
	}
	working, diagnostics = SortModuleDeclarations(working, linked.Table)
	if len(diagnostics) != 0 {
		return FlatteningResult{}, diagnostics[0]
	}
	analysis, err := AnalyzeModules(working, linked.Table)
	if err != nil {
		return FlatteningResult{}, err
	}
	if len(analysis.Diagnostics) != 0 {
		return FlatteningResult{}, analysis.Diagnostics[0]
	}
	privateIDs := NewIDGenerator()
	privateIDs.next.Set(&ids.next)
	return FlattenModules(analysis.Modules, linked.Table, privateIDs, nil, AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes})
}

func analyzedProjectSnapshot(t testing.TB, previous AnalysisResult, additions []Declaration) []byte {
	t.Helper()
	data, err := json.Marshal(struct {
		Modules   []Module
		Analysis  AnalysisOutput
		Additions []Declaration
	}{previous.Modules, AnalysisOutput{Types: previous.Types, Effects: previous.Effects, Modes: previous.Modes}, additions})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func analyzedProjectEffectOccurrences(effects *insdict.Dict[string, EffectScheme]) []*int {
	occurrences := []*int{}
	var entity func(Entity)
	entity = func(value Entity) {
		for _, variable := range value.StateVariables {
			occurrences = append(occurrences, variable.identity)
		}
		for _, child := range value.Entities {
			entity(child)
		}
	}
	var effect func(Effect)
	effect = func(value Effect) {
		for _, component := range value.Components {
			entity(component.Entity)
		}
		for _, parameter := range value.Params {
			effect(parameter)
		}
		if value.Result != nil {
			effect(*value.Result)
		}
	}
	for _, scheme := range effects.All() {
		effect(scheme.Effect)
	}
	return occurrences
}

func analyzedProjectEffectGraph(effects *insdict.Dict[string, EffectScheme]) []int {
	labels := map[*int]int{nil: 0}
	graph := []int{}
	for _, occurrence := range analyzedProjectEffectOccurrences(effects) {
		label, found := labels[occurrence]
		if !found {
			label = len(labels)
			labels[occurrence] = label
		}
		graph = append(graph, label)
	}
	return graph
}

func TestProjectEvaluatorFromAnalysisRawOwnership(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		extra        []string
		answer       string
	}{
		{"zero_additions", "module M { val answer=42 }", nil, "42"},
		{"new_annotation_alias", "module M { type Box[a]={item:a} pure def identity(x)=x val base=1 }", []string{"type NewBox[a]=Box[a]", "val annotated:NewBox[int]={item:identity(base)}", "val answer=annotated.item"}, "1"},
		{"forward_additions", "module M { val base=3 }", []string{"val answer=twice(base)", "pure def twice(x)=x+x"}, "6"},
		{"forward_annotation", "module M { val base=3 }", []string{"val answer:NewInt=base", "type NewInt=int"}, "3"},
		{"polymorphic_addition", "module M { pure def identity(x)=x val base=3 }", []string{"pure def first(x,y)=x", "val answer=first(identity(base),true)"}, "3"},
		{"polymorphic_fresh_names", "module M {pure def identity(x)=x val z=identity(1) val w=identity(true)}", []string{"pure def answer(x)=identity(x)"}, "9"},
		{"imported_instances", "module A {const N:int pure def identity(x)=x val result=identity(N)} module M {import A(N=1) as I import A(N=7) as J}", []string{"val answer=I::result+J::result"}, "8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			previous, additions, ids := analyzedProjectFixture(t, tc.source, tc.extra)
			before, next := analyzedProjectSnapshot(t, previous, additions), ids.next.String()
			oldFlat, err := analyzedProjectFullOracle(context.Background(), analyzedProjectModules(previous, additions, "M"), ids)
			if err != nil {
				t.Fatal(err)
			}
			flat, err := prepareAnalyzedEvaluationProject(context.Background(), previous, additions, ids, "M")
			if err != nil {
				t.Fatal(err)
			}
			for _, pair := range [][2]any{{flat.Modules, oldFlat.Modules}, {flat.Table, oldFlat.Table}, {flat.Analysis, oldFlat.Analysis}} {
				a, e1 := json.Marshal(pair[0])
				b, e2 := json.Marshal(pair[1])
				if e1 != nil || e2 != nil || !bytes.Equal(a, b) {
					t.Fatalf("incremental preparation changed raw modules/table/type/effect IDs: %v %v\ngot %s\nwant %s", e1, e2, a, b)
				}
			}
			old, err := NewProjectEvaluator(context.Background(), analyzedProjectModules(previous, additions, "M"), ids, "M", nil, false)
			if err != nil {
				t.Fatal(err)
			}
			got, err := NewProjectEvaluatorFromAnalysis(context.Background(), previous, additions, ids, "M", nil, false)
			if err != nil {
				t.Fatal(err)
			}
			var args []RuntimeValue
			if tc.name == "polymorphic_fresh_names" {
				nine, _ := ParseExpression("9")
				value, _ := NewRuntimeValue(nine)
				args = []RuntimeValue{value}
			}
			a, e1 := got.Evaluate(context.Background(), "answer", args)
			b, e2 := old.Evaluate(context.Background(), "answer", args)
			if e1 != nil || e2 != nil || a.String() != tc.answer || !a.Equal(b) || got.ObjectHashState().Counter() != old.ObjectHashState().Counter() {
				t.Fatal("source context or typed execution changed", a.String(), b.String(), e1, e2)
			}
			if ids.next.String() != next || !bytes.Equal(before, analyzedProjectSnapshot(t, previous, additions)) {
				t.Fatal("preparation or execution mutated source modules/analysis/additions/IDs")
			}
		})
	}
}

func TestProjectEvaluatorFromAnalysisRetainsSourceConstraints(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		extra        []string
		wantError    bool
	}{
		{"generic_const", "module M {const c:a}", []string{"val answer=c"}, false},
		{"generic_var", "module M {var c:a}", []string{"val answer=c"}, false},
		{"generic_const_conflict", "module M {const c:a}", []string{"val first=c+1", "val second=c and true"}, true},
		{"generic_var_conflict", "module M {var c:a}", []string{"val first=c+1", "val second=c and true"}, true},
		{"generic_row_conflict", "module M {const c:{item:int | r}}", []string{"val first=c.extra+1", "val second=c.extra and true"}, true},
		{"imported_generic_conflict", "module A {const c:a} module M {const d:a import A(c=d) as I}", []string{"val first=I::c+1", "val second=I::c and true"}, true},
		{"pending_assumption", "module M {assume bad=1}", []string{"val answer=1"}, true},
		{"pending_assumption_no_additions", "module M {assume bad=1}", nil, false},
		{"valid_assumption", "module M {assume good=true}", []string{"val answer=1"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			previous, additions, ids := analyzedProjectFixture(t, tc.source, tc.extra)
			before, next := analyzedProjectSnapshot(t, previous, additions), ids.next.String()
			want, oldErr := analyzedProjectFullOracle(context.Background(), analyzedProjectModules(previous, additions, "M"), ids)
			got, newErr := prepareAnalyzedEvaluationProject(context.Background(), previous, additions, ids, "M")
			if (oldErr != nil) != tc.wantError || (newErr != nil) != tc.wantError {
				t.Fatal("source constraints lost", newErr, oldErr)
			}
			oldJSON, oldMarshal := json.Marshal(oldErr)
			newJSON, newMarshal := json.Marshal(newErr)
			if oldMarshal != nil || newMarshal != nil || !bytes.Equal(oldJSON, newJSON) {
				t.Fatal("source constraint diagnostic payload changed", string(newJSON), string(oldJSON), oldMarshal, newMarshal)
			}
			if oldErr == nil {
				oldJSON, oldMarshal = json.Marshal(want)
				newJSON, newMarshal = json.Marshal(got)
				if oldMarshal != nil || newMarshal != nil || !bytes.Equal(oldJSON, newJSON) {
					t.Fatal("generic/assumption source metadata changed", string(newJSON), string(oldJSON), oldMarshal, newMarshal)
				}
			}
			if ids.next.String() != next || !bytes.Equal(before, analyzedProjectSnapshot(t, previous, additions)) {
				t.Fatal("fallback changed source analysis or caller IDs")
			}
		})
	}
}

func TestProjectEvaluatorFromAnalysisStatefulContexts(t *testing.T) {
	t.Parallel()
	previous, _, ids := analyzedProjectFixture(t, `module Shared { pure def identity(x)=x }
module A { const N:int import Shared.* var x:int action init=x'=N action step=x'=identity(x+1) val safe=x<N+3 }
module M { import A(N=1) as I import A(N=5) as J action init=all { I::init, J::init } action step=all { I::step, J::step } val safe=I::safe and J::safe }`, nil)
	init, step := "init", "step"
	additions, err := SynthesizeCompilationEntries(CompilationEntries{Target: "json", Init: &init, Step: &step, Invariants: []string{"safe"}}, ids)
	if err != nil {
		t.Fatal(err)
	}
	for i := range additions {
		if additions[i].Name == "q::init" || additions[i].Name == "q::step" {
			additions[i].Qualifier = Run
		}
	}
	before, next := analyzedProjectSnapshot(t, previous, additions), ids.next.String()
	occurrences := analyzedProjectEffectOccurrences(previous.Effects)
	oldFlat, err := analyzedProjectFullOracle(context.Background(), analyzedProjectModules(previous, additions, "M"), ids)
	if err != nil {
		t.Fatal(err)
	}
	flat, err := prepareAnalyzedEvaluationProject(context.Background(), previous, additions, ids, "M")
	if err != nil {
		t.Fatal(err)
	}
	x, _ := json.Marshal(flat.Analysis)
	y, _ := json.Marshal(oldFlat.Analysis)
	if !bytes.Equal(x, y) || !slices.Equal(analyzedProjectEffectGraph(flat.Analysis.Effects), analyzedProjectEffectGraph(oldFlat.Analysis.Effects)) {
		t.Fatal("flattened analysis metadata or effect occurrence sharing changed", string(x), string(y))
	}
	old, err := NewProjectEvaluator(context.Background(), analyzedProjectModules(previous, additions, "M"), ids, "M", NewRNG(Integer{}), true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := NewProjectEvaluatorFromAnalysis(context.Background(), previous, additions, ids, "M", NewRNG(Integer{}), true)
	if err != nil {
		t.Fatal(err)
	}
	options := SimulationOptions{Init: "q::init", Step: "q::step", Invariant: "q::inv", MaxSamples: 1, MaxSteps: 2, NumberOfTraces: 1}
	aRecorder, bRecorder := NewTraceRecorder(3, got.RNG, 1), NewTraceRecorder(3, old.RNG, 1)
	a, e1 := got.Simulate(WithExecutionListener(context.Background(), aRecorder), options)
	b, e2 := old.Simulate(WithExecutionListener(context.Background(), bRecorder), options)
	x, _ = json.Marshal(a)
	y, _ = json.Marshal(b)
	if e1 != nil || e2 != nil || a.Status != "ok" || !bytes.Equal(x, y) || got.ObjectHashState().Counter() != old.ObjectHashState().Counter() {
		t.Fatal("independent instance/source/action/trace histories changed", string(x), string(y), e1, e2)
	}
	x, _ = json.Marshal(aRecorder.BestTraces())
	y, _ = json.Marshal(bRecorder.BestTraces())
	if !bytes.Equal(x, y) {
		t.Fatal("source listener frame IDs or action names changed", string(x), string(y))
	}
	if ids.next.String() != next || !bytes.Equal(before, analyzedProjectSnapshot(t, previous, additions)) || !slices.Equal(occurrences, analyzedProjectEffectOccurrences(previous.Effects)) {
		t.Fatal("stateful execution changed source analysis or caller IDs")
	}
}

func TestProjectEvaluatorFromAnalysisConcurrentOwnership(t *testing.T) {
	t.Parallel()
	previous, additions, ids := analyzedProjectFixture(t, "module A { const N:int pure def identity(x)=x val result=identity(N) } module M { import A(N=3) as I }", []string{"val answer=I::identity(I::result)+1"})
	before, next := analyzedProjectSnapshot(t, previous, additions), ids.next.String()
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 2 {
				evaluator, err := NewProjectEvaluatorFromAnalysis(context.Background(), previous, additions, ids, "M", nil, true)
				if err != nil {
					t.Error(err)
					return
				}
				answer, err := evaluator.Evaluate(context.Background(), "answer", nil)
				if err != nil || answer.String() != "4" {
					t.Error(answer.String(), err)
				}
			}
		})
	}
	workers.Wait()
	if ids.next.String() != next || !bytes.Equal(before, analyzedProjectSnapshot(t, previous, additions)) {
		t.Fatal("concurrent construction mutated shared source analysis or parser IDs")
	}
}

func TestProjectEvaluatorFromAnalysisDiagnosticsAndValidation(t *testing.T) {
	t.Parallel()
	for _, extra := range [][]string{{"val answer=missing"}, {"val answer=1+true"}, {"val answer:Box[int,bool]={item:1}"}, {"val a=b", "val b=a"}} {
		t.Run(fmt.Sprint(extra), func(t *testing.T) {
			t.Parallel()
			previous, additions, ids := analyzedProjectFixture(t, "module M { type Box[a]={item:a} val base=1 }", extra)
			before, next := analyzedProjectSnapshot(t, previous, additions), ids.next.String()
			_, got := NewProjectEvaluatorFromAnalysis(context.Background(), previous, additions, ids, "M", nil, false)
			_, want := NewProjectEvaluator(context.Background(), analyzedProjectModules(previous, additions, "M"), ids, "M", nil, false)
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(want)
			if got == nil || want == nil || fmt.Sprint(got) != fmt.Sprint(want) || !bytes.Equal(a, b) {
				t.Fatal("diagnostic payload/raw reference changed", got, want, string(a), string(b))
			}
			if ids.next.String() != next || !bytes.Equal(before, analyzedProjectSnapshot(t, previous, additions)) {
				t.Fatal("failing additions mutated completed analysis or caller IDs")
			}
		})
	}
	previous, _, ids := analyzedProjectFixture(t, "module M { val answer=1 }", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewProjectEvaluatorFromAnalysis(ctx, AnalysisResult{}, nil, nil, "M", nil, false); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation must precede input access", err)
	}
	if _, err := NewProjectEvaluatorFromAnalysis(context.Background(), previous, nil, nil, "M", nil, false); err == nil || err.Error() != "project evaluation requires the parser ID generator" {
		t.Fatal(err)
	}
	if _, err := NewProjectEvaluatorFromAnalysis(context.Background(), previous, nil, ids, "Missing", nil, false); err == nil {
		t.Fatal("missing main accepted")
	}
	incomplete := AnalysisResult{TypeAnalysisResult: TypeAnalysisResult{Modules: previous.Modules}}
	if _, err := NewProjectEvaluatorFromAnalysis(context.Background(), incomplete, nil, ids, "M", nil, false); err == nil || err.Error() != "project evaluation requires completed source analysis" {
		t.Fatal(err)
	}
	id, _ := ParseID("999")
	diagnostic := Diagnostic{Code: "QNT000", Message: "source analysis failed", Reference: &id}
	previous.Diagnostics = []Diagnostic{diagnostic}
	if _, err := NewProjectEvaluatorFromAnalysis(context.Background(), previous, nil, ids, "M", nil, false); fmt.Sprint(err) != diagnostic.Error() {
		t.Fatal("source diagnostic ignored", err)
	}
}
