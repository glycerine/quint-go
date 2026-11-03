package quint

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/glycerine/insdict"
)

type upstreamTypeHelperSubstitution struct {
	Kind, Name string
	Value      json.RawMessage
}

type upstreamTypeHelperScheme struct {
	ID     string
	Scheme TypeScheme
}

type upstreamTypeHelperLookup struct {
	ID               string
	Annotation, Type *Type
}

type upstreamTypeHelperProject struct {
	Modules []Module
	Types   []upstreamTypeHelperScheme
	Table   []upstreamTypeHelperLookup
}

func decodeUpstreamTypeHelperSubstitutions(t *testing.T, input []upstreamTypeHelperSubstitution) []Substitution {
	t.Helper()
	output := make([]Substitution, len(input))
	for index, replacement := range input {
		output[index] = Substitution{Kind: replacement.Kind, Name: replacement.Name}
		var err error
		switch replacement.Kind {
		case "type":
			err = json.Unmarshal(replacement.Value, &output[index].Type)
		case "row":
			err = json.Unmarshal(replacement.Value, &output[index].Row)
		default:
			t.Fatal("unknown upstream substitution kind", replacement.Kind)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return output
}

func upstreamTypeHelperLookupTypes(table *insdict.Dict[ID, LookupDefinition]) []upstreamTypeHelperLookup {
	output := []upstreamTypeHelperLookup{}
	for id, definition := range table.All() {
		entry := upstreamTypeHelperLookup{ID: id.String()}
		if definition.Declaration != nil {
			entry.Annotation = definition.Declaration.TypeAnnotation
			if definition.Declaration.Kind == "typedef" {
				entry.Type = definition.Declaration.Type
			}
		} else if definition.Parameter != nil {
			entry.Annotation = definition.Parameter.TypeAnnotation
		}
		output = append(output, entry)
	}
	// Upstream built its lookup before dependency-ordering the saved modules.
	// Reconstructing it from that IR can change insertion order, so compare
	// lookup payloads by ID while preserving inferred scheme order below.
	sort.Slice(output, func(i, j int) bool { return output[i].ID < output[j].ID })
	return output
}

func assertUpstreamTypeHelperJSON(t *testing.T, label string, got, want any) {
	t.Helper()
	a, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Errorf("%s Go %s\nTypeScript %s", label, a, b)
	}
}

// Replay actual upstream tests without Node, keeping ordered replacements,
// quantifiers, constraints, source IDs and rewritten module/type payloads.
func TestUpstreamTypeHelperSuites(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Metadata struct {
			Tests        []struct{ Family, Name string }
			SourceSHA256 map[string]string
			Operations   map[string]int
		}
		Cases []struct {
			Family, Name, Operation      string
			First, Second, Substitutions []upstreamTypeHelperSubstitution
			Input, Output                json.RawMessage
		}
	}
	data, err := os.ReadFile("testdata/parser/type_upstream_helpers.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Metadata.Tests) != 16 || len(fixture.Cases) != 16 || len(fixture.Metadata.SourceSHA256) != 3 {
		t.Fatal("missing actual upstream helper tests or provenance")
	}
	for _, hash := range fixture.Metadata.SourceSHA256 {
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != 32 {
			t.Fatal("invalid upstream source hash", hash, err)
		}
	}
	counts := map[string]int{}
	for _, test := range fixture.Cases {
		counts[test.Operation]++
	}
	if !reflect.DeepEqual(counts, fixture.Metadata.Operations) || counts["compose"] != 2 || counts["applySubstitution"] != 6 || counts["applySubstitutionToConstraint"] != 3 || counts["simplify"] != 2 || counts["inlineTypeAliases"] != 3 {
		t.Fatal("incomplete upstream operation coverage", counts)
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			decode := func(input json.RawMessage, output any) {
				t.Helper()
				if err := json.Unmarshal(input, output); err != nil {
					t.Fatal(err)
				}
			}
			switch test.Operation {
			case "compose":
				got, failure := (TypeUnifier{}).Compose(decodeUpstreamTypeHelperSubstitutions(t, test.First), decodeUpstreamTypeHelperSubstitutions(t, test.Second))
				var expected []upstreamTypeHelperSubstitution
				decode(test.Output, &expected)
				if failure != nil || !reflect.DeepEqual(got, decodeUpstreamTypeHelperSubstitutions(t, expected)) {
					t.Fatal("upstream composition differs", got, failure, string(test.Output))
				}
			case "applySubstitution":
				var input, want Type
				decode(test.Input, &input)
				decode(test.Output, &want)
				got, err := ApplySubstitution(decodeUpstreamTypeHelperSubstitutions(t, test.Substitutions), input)
				if err != nil {
					t.Fatal(err)
				}
				assertUpstreamTypeHelperJSON(t, "substituted type", got, want)
			case "applySubstitutionToConstraint":
				var input, want Constraint
				decode(test.Input, &input)
				decode(test.Output, &want)
				got, err := ApplyConstraintSubstitution(decodeUpstreamTypeHelperSubstitutions(t, test.Substitutions), input)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("substituted constraint differs", got, err, string(test.Output))
				}
			case "simplify":
				var input, want TypeScheme
				decode(test.Input, &input)
				decode(test.Output, &want)
				assertUpstreamTypeHelperJSON(t, "simplified scheme", SimplifyScheme(input), want)
			case "inlineTypeAliases":
				var input, want upstreamTypeHelperProject
				decode(test.Input, &input)
				decode(test.Output, &want)
				linked, err := ResolveLinkedModules(input.Modules)
				if err != nil || len(linked.Errors) != 0 {
					t.Fatal(err, linked.Errors)
				}
				sort.Slice(input.Table, func(i, j int) bool { return input.Table[i].ID < input.Table[j].ID })
				sort.Slice(want.Table, func(i, j int) bool { return want.Table[i].ID < want.Table[j].ID })
				assertUpstreamTypeHelperJSON(t, "input lookup types", upstreamTypeHelperLookupTypes(linked.Table), input.Table)
				types := insdict.NewDict[string, TypeScheme]()
				for _, entry := range input.Types {
					types.Set(entry.ID, entry.Scheme)
				}
				got, err := InlineTypeAliases(input.Modules, linked.Table, AnalysisOutput{Types: types})
				if err != nil {
					t.Fatal(err)
				}
				assertUpstreamTypeHelperJSON(t, "rewritten modules", got.Modules, want.Modules)
				assertUpstreamTypeHelperJSON(t, "rewritten lookup types", upstreamTypeHelperLookupTypes(got.Table), want.Table)
				schemes := []upstreamTypeHelperScheme{}
				for id, scheme := range got.Analysis.Types.All() {
					schemes = append(schemes, upstreamTypeHelperScheme{ID: id, Scheme: scheme})
				}
				assertUpstreamTypeHelperJSON(t, "rewritten inferred schemes", schemes, want.Types)
			default:
				t.Fatal("unknown upstream operation", test.Operation)
			}
		})
	}
}
