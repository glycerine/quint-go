package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamReferenceOrder(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name   string
		Input  []Expr
		Output []Expr
	}
	data, err := os.ReadFile("testdata/parser/runtime_reference_order.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			values := make([]RuntimeValue, len(fixture.Input))
			for i, expression := range fixture.Input {
				var err error
				values[i], err = NewRuntimeValue(expression)
				if err != nil {
					t.Fatal(err)
				}
			}
			before := append([]RuntimeValue(nil), values...)
			ordered, err := runtimeReferenceOrder(values)
			if err != nil {
				t.Fatal(err)
			}
			if len(ordered) != len(fixture.Output) {
				t.Fatal(len(ordered), len(fixture.Output))
			}
			for i, expression := range fixture.Output {
				want, err := NewRuntimeValue(expression)
				if err != nil || !ordered[i].Equal(want) {
					t.Fatal(i, ordered[i].String(), want.String(), err)
				}
			}
			for i, value := range values {
				if !value.Equal(before[i]) {
					t.Fatal("input order changed")
				}
			}
		})
	}
}

func TestReferenceOrderValidationAndOwnership(t *testing.T) {
	if _, err := runtimeReferenceOrder([]RuntimeValue{{}}); err == nil {
		t.Fatal("invalid value accepted")
	}
	closure := newRuntimeClosure(0, nil)
	if _, err := runtimeReferenceOrder([]RuntimeValue{closure}); err == nil {
		t.Fatal("closure accepted")
	}
	value, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	values := []RuntimeValue{value}
	ordered, err := runtimeReferenceOrder(values)
	if err != nil {
		t.Fatal(err)
	}
	ordered[0] = RuntimeValue{}
	if !values[0].Equal(value) {
		t.Fatal("slice aliases caller")
	}
}
