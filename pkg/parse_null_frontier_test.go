package quint

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type nullFrontierFixture struct {
	Name, Source    string
	LastID          ID
	Modules         json.RawMessage
	Diagnostics     []Diagnostic
	UpstreamSources UpstreamSourceMap
	NextID          ID
	UndefinedPaths  []string
	NullPaths       []string
	Phase2          struct {
		Status         string
		Modules        json.RawMessage
		UndefinedPaths []string
	}
	Phase3, Analysis, Parse struct{ Status, Name, Message string }
}

func nullFrontierFixtures(t *testing.T) []nullFrontierFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/parser_null_frontier.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []nullFrontierFixture
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 10 {
		t.Fatal("raw null frontier inventory", len(fixtures))
	}
	return fixtures
}

func nullFrontierJSON(t *testing.T, data []byte) any {
	t.Helper()
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestUpstreamRawNullRecovery(t *testing.T) {
	t.Parallel()
	for _, fixture := range nullFrontierFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			result, err := parseModulesRecoveringWithIDs(fixture.Name+".qnt", fixture.Source, NewIDGeneratorAfter(fixture.LastID))
			if len(fixture.NullPaths) != 0 {
				t.Fatal("reference operands became actual JS null instead of undefined")
			}
			if fixture.Phase2.Status != "returned" || !reflect.DeepEqual(nullFrontierJSON(t, fixture.Phase2.Modules), nullFrontierJSON(t, fixture.Modules)) || !reflect.DeepEqual(fixture.Phase2.UndefinedPaths, fixture.UndefinedPaths) {
				t.Fatal("reference phase2 failed to retain raw malformed modules")
			}
			if len(fixture.UndefinedPaths) == 0 {
				if err != nil {
					t.Fatal(err)
				}
				wire, e := json.Marshal(result.Modules)
				if e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(nullFrontierJSON(t, wire), nullFrontierJSON(t, fixture.Modules)) {
					t.Fatal("representable control changed")
				}
				return
			}
			for _, outcome := range []struct{ Status, Name, Message string }{fixture.Phase3, fixture.Analysis, fixture.Parse} {
				if outcome.Status != "threw" || outcome.Name != "TypeError" || outcome.Message != "Cannot read properties of undefined (reading 'kind')" {
					t.Fatal("reference no longer has this typed-IR frontier", outcome)
				}
			}
			var recovery *SyntaxRecoveryError
			if !errors.As(err, &recovery) || recovery.Raw == nil {
				t.Fatal("missing quarantined raw recovery", err)
			}
			if recovery.Error() != "unsupported syntax recovery: cannot convert malformed *quintparser.RecElemContext" || recovery.Unwrap() == nil {
				t.Fatal("typed recovery error contract changed", recovery)
			}
			raw := recovery.Raw
			if !reflect.DeepEqual(nullFrontierJSON(t, raw.Modules), nullFrontierJSON(t, fixture.Modules)) {
				t.Errorf("raw modules\n got%s\nwant%s", raw.Modules, fixture.Modules)
			}
			if !reflect.DeepEqual(raw.Diagnostics, fixture.Diagnostics) || !reflect.DeepEqual(raw.UpstreamSources, fixture.UpstreamSources) || raw.NextID != fixture.NextID {
				t.Fatal("raw diagnostic/source/continuation provenance differs")
			}
			file, e := NewSourceFile(fixture.Name+".qnt", fixture.Source)
			if e != nil {
				t.Fatal(e)
			}
			for id, upstream := range fixture.UpstreamSources {
				public, ok := raw.Sources[id]
				if !ok || public.Source != upstream.Source || public.Start != file.positions[upstream.Start.Index] || upstream.End == nil || public.End == nil || *public.End != file.positions[upstream.End.Index+1] {
					t.Fatal("raw recovery public source mismatch", id, public, upstream)
				}
			}
			var modules []json.RawMessage
			if e = json.Unmarshal(fixture.Modules, &modules); e != nil {
				t.Fatal(e)
			}
			badIndex, e := strconv.Atoi(strings.Split(fixture.UndefinedPaths[0], "/")[2])
			if e != nil {
				t.Fatal(e)
			}
			var prefix []Module
			prefixWire, e := json.Marshal(modules[:badIndex])
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(prefixWire, &prefix); e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(result.Modules, prefix) {
				t.Fatal("typed completed-module prefix changed")
			}
			original := Module{Name: "keep", Declarations: []Declaration{}}
			before := original
			if e = json.Unmarshal(modules[badIndex], &original); e == nil || !reflect.DeepEqual(original, before) {
				t.Fatal("malformed raw module entered strict typed IR", e)
			}
			if next := result.IDs.Next(); next == raw.NextID {
				t.Fatal("raw replay advanced the caller's typed generator")
			}
		})
	}
}

func TestRawNullRecoveryOwnershipAndProjectBoundary(t *testing.T) {
	t.Parallel()
	fixture := nullFrontierFixtures(t)[2]
	result, err := ParseModulesRecovering(fixture.Name+".qnt", fixture.Source)
	var recovery *SyntaxRecoveryError
	if !errors.As(err, &recovery) || recovery.Raw == nil {
		t.Fatal(err)
	}
	beforeModules, e := json.Marshal(result.Modules)
	if e != nil {
		t.Fatal(e)
	}
	beforeSources, e := json.Marshal(result.Sources)
	if e != nil {
		t.Fatal(e)
	}
	beforeRawSources, e := json.Marshal(result.UpstreamSources)
	if e != nil {
		t.Fatal(e)
	}
	beforeDiagnostics, e := json.Marshal(result.Diagnostics)
	if e != nil {
		t.Fatal(e)
	}
	for _, location := range recovery.Raw.Sources {
		if location.End != nil {
			location.End.Index = -1
		}
	}
	for _, location := range recovery.Raw.UpstreamSources {
		if location.End != nil {
			location.End.Index = -1
		}
	}
	for _, diagnostic := range recovery.Raw.Diagnostics {
		if diagnostic.Reference != nil {
			*diagnostic.Reference = ID{}
		}
	}
	clear(recovery.Raw.Sources)
	clear(recovery.Raw.UpstreamSources)
	for i := range recovery.Raw.Modules {
		recovery.Raw.Modules[i] = ' '
	}
	afterModules, _ := json.Marshal(result.Modules)
	afterSources, _ := json.Marshal(result.Sources)
	afterRawSources, _ := json.Marshal(result.UpstreamSources)
	afterDiagnostics, _ := json.Marshal(result.Diagnostics)
	if !bytes.Equal(beforeModules, afterModules) || !bytes.Equal(beforeSources, afterSources) || !bytes.Equal(beforeRawSources, afterRawSources) || !bytes.Equal(beforeDiagnostics, afterDiagnostics) {
		t.Fatal("raw recovery aliases typed output")
	}
	resolver := &nullFrontierResolver{name: fixture.Name + ".qnt", source: fixture.Source}
	project, err := ParseProjectRecovering(resolver, SourceLookupPath{NormalizedPath: resolver.name, SourceName: resolver.name})
	if !errors.As(err, &recovery) || recovery.Raw == nil || project.Table != nil || project.Scopes != nil || len(project.Modules) != 1 || project.Modules[0].Name != "Good" {
		t.Fatal("raw malformed graph escaped into name resolution", err)
	}
	if !reflect.DeepEqual(nullFrontierJSON(t, recovery.Raw.Modules), nullFrontierJSON(t, fixture.Modules)) {
		t.Fatal("project boundary discarded the raw snapshot")
	}
}

type nullFrontierResolver struct{ name, source string }

func (r *nullFrontierResolver) LookupPath(stem, path string) SourceLookupPath {
	return SourceLookupPath{NormalizedPath: path, SourceName: path}
}
func (r *nullFrontierResolver) StemPath(path SourceLookupPath) string { return "" }
func (r *nullFrontierResolver) Load(path SourceLookupPath) (string, error) {
	if path.NormalizedPath != r.name {
		return "", errors.New("unexpected test source lookup")
	}
	return r.source, nil
}
