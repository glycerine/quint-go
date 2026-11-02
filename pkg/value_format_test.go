package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamValueFormatting(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name               string
		Input              Expr
		Width, FirstColumn int
		Output             string
	}
	data, err := os.ReadFile("testdata/parser/value_format.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		got, err := FormatValueExpression(fixture.Input, fixture.Width, fixture.FirstColumn)
		if err != nil || got != fixture.Output {
			t.Fatalf("%s: got %q want %q: %v", fixture.Name, got, fixture.Output, err)
		}
	}
}

func TestValueFormattingValidationAndOwnership(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		value         Expr
		width, column int
	}{
		{Expr{Kind: "bool"}, -1, 0}, {Expr{Kind: "bool"}, 80, -1},
		{Expr{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "str", Str: "x"}}}, 80, 0},
		{Expr{Kind: "app", Opcode: "variant"}, 80, 0},
	} {
		if _, err := FormatValueExpression(test.value, test.width, test.column); err == nil {
			t.Fatal("invalid format input accepted", test)
		}
	}
	expression, err := ParseExpression(`Rec("x", List(1,2), "label", variant("Some", Tup()))`)
	if err != nil {
		t.Fatal(err)
	}
	before := ExpressionString(expression)
	if _, err = FormatValueExpression(expression, 12, 0); err != nil {
		t.Fatal(err)
	}
	if ExpressionString(expression) != before {
		t.Fatal("formatter changed its input")
	}
}
