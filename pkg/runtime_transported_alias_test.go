package quint

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// TestTransportedAliasFirstSelectionIsFixedAtFlatten pins the JSON-transport
// boundary for an exported alias read through an instance. Source evaluation
// keeps first-compilation order (see TestUpstreamSavedExportAliasContextHistories).
// Transported flattened IR has no per-reference history, so the single flattened
// declaration must give the same registrations for either reference order.
func TestTransportedAliasFirstSelectionIsFixedAtFlatten(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Source string `json:"source"`
		Main   string `json:"main"`
	}
	data, err := os.ReadFile("testdata/parser/runtime_variable_context_pending.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs(fixture.Source, ids)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := ResolveLinkedModules(modules)
	if err != nil || len(linked.Errors) != 0 {
		t.Fatal(err, linked.Errors)
	}
	analysis, err := AnalyzeModules(modules, linked.Table)
	if err != nil || len(analysis.Diagnostics) != 0 {
		t.Fatal(err, analysis.Diagnostics)
	}
	flat, err := FlattenModules(analysis.Modules, linked.Table, ids, nil, AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(flat.Modules)
	if err != nil {
		t.Fatal(err)
	}
	var transported []Module
	if err := json.Unmarshal(wire, &transported); err != nil {
		t.Fatal(err)
	}
	var main Module
	for _, module := range transported {
		if module.Name == fixture.Main {
			main = module
		}
	}

	run := func(order []string) ([]runtimeVariableRegistration, []string) {
		evaluator, err := NewModuleEvaluator(main, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range order {
			if err := evaluator.PrepareDefinition(context.Background(), name); err != nil {
				t.Fatal(name, err)
			}
		}
		var registrations []runtimeVariableRegistration
		for key, register := range evaluator.Storage.current.All() {
			registrations = append(registrations, runtimeVariableRegistration{Key: key, Name: register.Name})
		}
		return registrations, evaluator.Storage.VarNames()
	}
	viaFirstRegs, viaFirstNames := run([]string{"via", "direct"})
	directFirstRegs, directFirstNames := run([]string{"direct", "via"})
	wantNames := []string{"C::Outer::Alias::x"}
	if !reflect.DeepEqual(viaFirstNames, wantNames) {
		t.Fatalf("via-first names = %v, want %v", viaFirstNames, wantNames)
	}
	if !reflect.DeepEqual(directFirstNames, wantNames) {
		t.Fatalf("direct-first names = %v, want %v", directFirstNames, wantNames)
	}
	if !reflect.DeepEqual(viaFirstRegs, directFirstRegs) {
		t.Fatalf("transported flattened IR depends on reference order: via-first %v, direct-first %v", viaFirstRegs, directFirstRegs)
	}
}
