package quint

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/glycerine/insdict"
)

// These fixtures retain the actual TypeScript flattenModules result, including
// its binding graph and retained analysis. Comparing public JSON deliberately
// excludes private runtime provenance, which is tested by source execution.
type flattenAliasFixture struct {
	Name                    string
	Source                  string
	InputBeforeAnalysis     []Module
	Input, Output           []Module
	LastID, NextID          string
	InputTable, Table       []flattenAliasBinding
	InputAnalysis, Analysis flattenAliasAnalysis
	InputSources, Sources   []flattenAliasSource
}

type flattenAliasBinding struct {
	ID           string       `json:"id"`
	Target       string       `json:"target"`
	Name         string       `json:"name"`
	Kind         string       `json:"kind"`
	Depth        int          `json:"depth"`
	Hidden       bool         `json:"hidden"`
	Namespaces   []string     `json:"namespaces"`
	Shadowing    bool         `json:"shadowing"`
	ImportedFrom *Declaration `json:"importedFrom"`
}

type flattenAliasType struct {
	ID     string     `json:"id"`
	Scheme TypeScheme `json:"scheme"`
}

type flattenAliasEffect struct {
	ID     string       `json:"id"`
	Scheme EffectScheme `json:"scheme"`
}

type flattenAliasMode struct {
	ID        string    `json:"id"`
	Qualifier Qualifier `json:"qualifier"`
}

type flattenAliasAnalysis struct {
	Types   []flattenAliasType   `json:"types"`
	Effects []flattenAliasEffect `json:"effects"`
	Modes   []flattenAliasMode   `json:"modes"`
}

type flattenAliasSource struct {
	ID       string   `json:"id"`
	Location Location `json:"location"`
}

func TestUpstreamFullFlattenAliasGraphs(t *testing.T) {
	data, err := os.ReadFile("testdata/parser/full_flatten_aliases.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []flattenAliasFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 18 {
		t.Fatalf("expected 18 saved alias graphs, got %d cases", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			linked, err := ResolveLinkedModules(fixture.InputBeforeAnalysis)
			if err != nil || len(linked.Errors) != 0 {
				t.Fatalf("resolve fixture: %v; %v", err, linked.Errors)
			}
			// Analysis expands application annotations but retains the original
			// constructor reference entries. Re-resolving only the expanded IR
			// would drop those keys before the flattening pass is even reached.
			if err := NewTypeApplicationResolver(linked.Table).ResolveLookupTable(); err != nil {
				t.Fatal(err)
			}
			flattenAliasEqual(t, "input bindings", flattenAliasBindings(linked.Table), fixture.InputTable)
			beforeModules, err := json.Marshal(fixture.Input)
			if err != nil {
				t.Fatal(err)
			}
			beforeBindings, err := json.Marshal(flattenAliasBindings(linked.Table))
			if err != nil {
				t.Fatal(err)
			}
			analysis := fixture.InputAnalysis.output()
			beforeTypes, err := json.Marshal(flattenAliasRetained(analysis).Types)
			if err != nil {
				t.Fatal(err)
			}
			sources := fixture.sourceMap(t, fixture.InputSources)
			ids := NewIDGenerator()
			if _, ok := ids.next.SetString(fixture.LastID, 10); !ok {
				t.Fatalf("invalid saved generator state %q", fixture.LastID)
			}
			ids.next.Add(&ids.next, big.NewInt(1))
			result, err := FlattenModules(fixture.Input, linked.Table, ids, sources, analysis)
			if err != nil {
				t.Fatal(err)
			}
			flattenAliasEqual(t, "modules", result.Modules, fixture.Output)
			flattenAliasEqual(t, "bindings", flattenAliasBindings(result.Table), fixture.Table)
			flattenAliasEqual(t, "analysis", flattenAliasRetained(result.Analysis), fixture.Analysis)
			flattenAliasEqual(t, "sources", sources, fixture.sourceMap(t, fixture.Sources))
			if got := ids.Next().String(); got != fixture.NextID {
				t.Fatalf("next ID: Go %s; TypeScript %s", got, fixture.NextID)
			}
			flattenAliasEqual(t, "input modules remain owned", fixture.Input, json.RawMessage(beforeModules))
			flattenAliasEqual(t, "input bindings remain owned", flattenAliasBindings(linked.Table), json.RawMessage(beforeBindings))
			flattenAliasEqual(t, "input type table remains owned", flattenAliasRetained(analysis).Types, json.RawMessage(beforeTypes))
			relinked, err := ResolveLinkedModules(result.Modules)
			if err != nil || len(relinked.Errors) != 0 {
				t.Fatalf("resolve flattened modules: %v; %v", err, relinked.Errors)
			}
			flattenAliasEqual(t, "flattened bindings survive resolution", flattenAliasBindings(relinked.Table), fixture.Table)
		})
	}
}

func (fixture flattenAliasFixture) sourceMap(t *testing.T, entries []flattenAliasSource) SourceMap {
	t.Helper()
	result := SourceMap{}
	for _, entry := range entries {
		id, err := ParseID(entry.ID)
		if err != nil {
			t.Fatal(err)
		}
		// This pass only copies opaque locations. The fixture intentionally
		// retains upstream inclusive coordinates; frontend UTF-16 conversion
		// is covered by source-map and language-server tests.
		result[id] = entry.Location
	}
	return result
}

func (analysis flattenAliasAnalysis) output() AnalysisOutput {
	result := AnalysisOutput{
		Types:   insdict.NewDict[string, TypeScheme](),
		Effects: insdict.NewDict[string, EffectScheme](),
		Modes:   insdict.NewDict[string, Qualifier](),
	}
	for _, entry := range analysis.Types {
		result.Types.Set(entry.ID, entry.Scheme)
	}
	for _, entry := range analysis.Effects {
		result.Effects.Set(entry.ID, entry.Scheme)
	}
	for _, entry := range analysis.Modes {
		result.Modes.Set(entry.ID, entry.Qualifier)
	}
	return result
}

func flattenAliasRetained(analysis AnalysisOutput) flattenAliasAnalysis {
	result := flattenAliasAnalysis{Types: []flattenAliasType{}, Effects: []flattenAliasEffect{}, Modes: []flattenAliasMode{}}
	if analysis.Types != nil {
		for id, scheme := range analysis.Types.All() {
			result.Types = append(result.Types, flattenAliasType{ID: id, Scheme: scheme})
		}
	}
	if analysis.Effects != nil {
		for id, scheme := range analysis.Effects.All() {
			result.Effects = append(result.Effects, flattenAliasEffect{ID: id, Scheme: scheme})
		}
	}
	if analysis.Modes != nil {
		for id, qualifier := range analysis.Modes.All() {
			result.Modes = append(result.Modes, flattenAliasMode{ID: id, Qualifier: qualifier})
		}
	}
	return result
}

func flattenAliasBindings(table *insdict.Dict[ID, LookupDefinition]) []flattenAliasBinding {
	result := []flattenAliasBinding{}
	for id, binding := range table.All() {
		kind := "param"
		if binding.Declaration != nil {
			kind = binding.Declaration.Kind
		}
		result = append(result, flattenAliasBinding{
			ID: id.String(), Target: binding.ID.String(), Name: binding.Name,
			Kind: kind, Depth: binding.Depth, Hidden: binding.Hidden,
			Namespaces: append([]string{}, binding.Namespaces...), Shadowing: binding.Shadowing,
			ImportedFrom: binding.ImportedFrom,
		})
	}
	return result
}

func flattenAliasEqual(t *testing.T, label string, got, want any) {
	t.Helper()
	encode := func(value any) []byte {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	gotJSON, wantJSON := encode(got), encode(want)
	if bytes.Equal(gotJSON, wantJSON) {
		return
	}
	decode := func(data []byte) any {
		t.Helper()
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		return decoded
	}
	gotValue, wantValue := decode(gotJSON), decode(wantJSON)
	if !reflect.DeepEqual(gotValue, wantValue) {
		path, gotPart, wantPart := flattenAliasDifference(gotValue, wantValue, "$")
		t.Fatalf("%s differ at %s:\nGo %s\nTypeScript %s", label, path, encode(gotPart), encode(wantPart))
	}
}

func flattenAliasDifference(got, want any, path string) (string, any, any) {
	if gotMap, ok := got.(map[string]any); ok {
		if wantMap, ok := want.(map[string]any); ok {
			keys := make([]string, 0, len(gotMap)+len(wantMap))
			for key := range gotMap {
				keys = append(keys, key)
			}
			for key := range wantMap {
				if _, ok := gotMap[key]; !ok {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				gotPart, gotOK := gotMap[key]
				wantPart, wantOK := wantMap[key]
				if gotOK != wantOK || !reflect.DeepEqual(gotPart, wantPart) {
					return flattenAliasDifference(gotPart, wantPart, path+"."+key)
				}
			}
		}
	}
	if gotSlice, ok := got.([]any); ok {
		if wantSlice, ok := want.([]any); ok {
			if len(gotSlice) != len(wantSlice) {
				return path + ".length", len(gotSlice), len(wantSlice)
			}
			for i := range gotSlice {
				if !reflect.DeepEqual(gotSlice[i], wantSlice[i]) {
					return flattenAliasDifference(gotSlice[i], wantSlice[i], path+"["+strconv.Itoa(i)+"]")
				}
			}
		}
	}
	return path, got, want
}
