package quint

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
)

type apiPrintCase struct {
	Name               string
	Input              Declaration
	Options            DeclarationPrintOptions
	Width, FirstColumn int
	Output, Error      string
}

type apiPrintingFixtures struct {
	IRCases     []apiPrintCase
	ModuleCases []struct {
		Input  Module
		Output string
	}
	PrettyTypes []struct {
		Name               string
		Input              Type
		Width, FirstColumn int
		Output, Error      string
	}
	PrettySchemes []struct {
		Name               string
		Input              TypeScheme
		Width, FirstColumn int
		Output, Error      string
	}
	PrettyDeclarations []apiPrintCase
}

var apiPrintingFixtureData = sync.OnceValues(func() (apiPrintingFixtures, error) {
	data, err := os.ReadFile("testdata/parser/api_printing.json")
	if err != nil {
		return apiPrintingFixtures{}, err
	}
	var fixtures apiPrintingFixtures
	err = json.Unmarshal(data, &fixtures)
	return fixtures, err
})

func readAPIPrintingFixtures(t *testing.T) apiPrintingFixtures {
	t.Helper()
	fixtures, err := apiPrintingFixtureData()
	if err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestUpstreamPublicIRPrinting(t *testing.T) {
	t.Parallel()
	fixtures := readAPIPrintingFixtures(t)
	for _, fixture := range fixtures.IRCases {
		got, err := DeclarationString(fixture.Input, fixture.Options)
		if err != nil || got != fixture.Output {
			t.Fatalf("%s: got %q want %q: %v", fixture.Name, got, fixture.Output, err)
		}
		if isDocumentationDefinition(fixture.Input) {
			definition, err := DefinitionString(fixture.Input, fixture.Options)
			if err != nil || definition != fixture.Output {
				t.Fatalf("definition %s: got %q want %q: %v", fixture.Name, definition, fixture.Output, err)
			}
		}
	}
	for _, fixture := range fixtures.ModuleCases {
		got, err := ModuleString(fixture.Input)
		if err != nil || got != fixture.Output {
			t.Fatalf("module %s: got %q want %q: %v", fixture.Input.Name, got, fixture.Output, err)
		}
	}
}

func TestIRPrintingValidationAndOwnership(t *testing.T) {
	t.Parallel()
	invalid := []Declaration{
		{}, {Kind: "def", Name: "missing", Qualifier: Def},
		{Kind: "typedef", Name: "bad", Type: &Type{Kind: "set"}},
		{Kind: "def", Name: "bad", Qualifier: Def, Expr: &Expr{Kind: "lambda", Qualifier: Def}},
		{Kind: "def", Name: "bad", Qualifier: Def, Expr: &Expr{Kind: "let"}},
		{Kind: "def", Name: "bad", Qualifier: Def, Expr: &Expr{Kind: "let", Body: &Expr{Kind: "bool"}, Opdef: &OperatorDef{Kind: "def", Name: "local", Qualifier: Def, Expr: &Expr{Kind: "bool"}, TypeAnnotation: &Type{Kind: "set"}}}},
		{Kind: "def", Name: "bad", Qualifier: Def, Expr: &Expr{Kind: "app", Opcode: "f", Args: []Expr{{Kind: "unknown"}}}},
		{Kind: "instance", ProtoName: "M", Overrides: []Override{{Expr: Expr{Kind: "bool"}}}},
		{Kind: "instance", ProtoName: "M", Overrides: []Override{{Parameter: Parameter{Name: "x"}, Expr: Expr{Kind: "unknown"}}}},
	}
	for _, input := range invalid {
		if _, err := DeclarationString(input); err == nil {
			t.Fatal("accepted malformed declaration", input)
		}
	}
	if _, err := ModuleString(Module{}); err == nil {
		t.Fatal("accepted unnamed module")
	}
	if _, err := ModuleString(Module{Name: "M", Declarations: invalid[:1]}); err == nil {
		t.Fatal("accepted malformed module declaration")
	}
	if _, err := DefinitionString(Declaration{Kind: "import", ProtoName: "M"}); err == nil {
		t.Fatal("accepted import as definition")
	}
	definition := Declaration{Kind: "def", Name: "value", Qualifier: Val, Expr: &Expr{Kind: "bool"}}
	collision := TypeScheme{Type: Type{Kind: "bool"}, TypeVariables: []string{"same"}, RowVariables: []string{"same"}}
	for _, call := range []func() error{
		func() error {
			_, err := DeclarationString(definition, DeclarationPrintOptions{}, DeclarationPrintOptions{})
			return err
		},
		func() error {
			_, err := DefinitionString(definition, DeclarationPrintOptions{}, DeclarationPrintOptions{})
			return err
		},
		func() error { _, err := DefinitionString(invalid[1]); return err },
		func() error {
			_, err := DeclarationString(definition, DeclarationPrintOptions{Type: &TypeScheme{Type: Type{Kind: "bad"}}})
			return err
		},
		func() error {
			_, err := DeclarationString(definition, DeclarationPrintOptions{Type: &collision})
			return err
		},
	} {
		if err := call(); err == nil {
			t.Fatal("accepted invalid printing option/input")
		}
	}
	fixtures := readAPIPrintingFixtures(t)
	module := fixtures.ModuleCases[1].Input
	before, err := json.Marshal(module)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ModuleString(module); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(module)
	if err != nil || string(before) != string(after) {
		t.Fatal("printer changed borrowed input", err)
	}
	annotation := Type{Kind: "int"}
	definition.TypeAnnotation = &annotation
	text, err := DeclarationString(definition, DeclarationPrintOptions{Type: &TypeScheme{Type: Type{Kind: "bad"}}})
	if err != nil || !strings.Contains(text, ": int") {
		t.Fatal("inferred type overrode source annotation", text, err)
	}
}
