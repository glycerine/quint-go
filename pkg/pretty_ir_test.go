package quint

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUpstreamPublicPrettyIR(t *testing.T) {
	t.Parallel()
	fixtures := readAPIPrintingFixtures(t)
	check := func(name, got, expected, referenceError string, err error) {
		t.Helper()
		if referenceError != "" {
			if err == nil || !strings.Contains(err.Error(), "type applications") {
				t.Fatalf("%s: source %s should be an explicit unsupported error, got %q %v", name, referenceError, got, err)
			}
			return
		}
		if err != nil || got != expected {
			t.Fatalf("%s: got %q want %q: %v", name, got, expected, err)
		}
	}
	for _, fixture := range fixtures.PrettyTypes {
		got, err := FormatType(fixture.Input, fixture.Width, fixture.FirstColumn)
		check(fixture.Name, got, fixture.Output, fixture.Error, err)
	}
	for _, fixture := range fixtures.PrettySchemes {
		got, err := FormatTypeScheme(fixture.Input, fixture.Width, fixture.FirstColumn)
		check(fixture.Name, got, fixture.Output, fixture.Error, err)
	}
	for _, fixture := range fixtures.PrettyDeclarations {
		got, err := FormatDeclaration(fixture.Input, fixture.Width, fixture.FirstColumn, fixture.Options)
		check(fixture.Name, got, fixture.Output, fixture.Error, err)
	}
}

func TestPrettyIRValidationAndOwnership(t *testing.T) {
	t.Parallel()
	declaration := Declaration{Kind: "def", Name: "value", Qualifier: Val, Expr: &Expr{Kind: "bool"}}
	collision := TypeScheme{Type: Type{Kind: "bool"}, TypeVariables: []string{"same"}, RowVariables: []string{"same"}}
	for _, call := range []func() error{
		func() error { _, err := FormatType(Type{Kind: "int"}, -1, 0); return err },
		func() error { _, err := FormatTypeScheme(TypeScheme{Type: Type{Kind: "int"}}, 80, -1); return err },
		func() error { _, err := FormatDeclaration(declaration, -1, 0); return err },
		func() error { _, err := FormatType(Type{Kind: "set"}, 80, 0); return err },
		func() error { _, err := FormatTypeScheme(TypeScheme{Type: Type{Kind: "bad"}}, 80, 0); return err },
		func() error { _, err := FormatDeclaration(Declaration{Kind: "var", Name: "bad"}, 80, 0); return err },
		func() error {
			_, err := FormatDeclaration(declaration, 80, 0, DeclarationPrintOptions{}, DeclarationPrintOptions{})
			return err
		},
		func() error {
			_, err := FormatDeclaration(declaration, 80, 0, DeclarationPrintOptions{Type: &TypeScheme{Type: Type{Kind: "bad"}}})
			return err
		},
		func() error { _, err := FormatTypeScheme(collision, 80, 0); return err },
		func() error {
			bad := declaration
			bad.Expr = &Expr{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "str", Str: "missing"}}}
			_, err := FormatDeclaration(bad, 80, 0)
			return err
		},
	} {
		if err := call(); err == nil {
			t.Fatal("accepted malformed pretty-print input")
		}
	}
	fixtures := readAPIPrintingFixtures(t)
	input := struct {
		Module Module
		Scheme TypeScheme
	}{fixtures.ModuleCases[1].Input, fixtures.PrettySchemes[10].Input}
	before, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = FormatTypeScheme(input.Scheme, 10, 0)
	for _, declaration := range input.Module.Declarations {
		_, _ = FormatDeclaration(declaration, 10, 0, DeclarationPrintOptions{Type: &input.Scheme})
		for _, typ := range []*Type{declaration.TypeAnnotation, declaration.Type} {
			if typ != nil {
				_, _ = FormatType(*typ, 10, 0)
			}
		}
	}
	after, err := json.Marshal(input)
	if err != nil || string(before) != string(after) {
		t.Fatal("pretty formatter changed borrowed input", err)
	}
}
