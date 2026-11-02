package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamRuntimeValueEquality(t *testing.T) {
	var fixture struct {
		Expressions []Expr
		Equal       [][]bool
	}
	data, err := os.ReadFile("testdata/parser/runtime_value.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	values := make([]RuntimeValue, len(fixture.Expressions))
	for i, e := range fixture.Expressions {
		values[i], err = NewRuntimeValue(e)
		if err != nil {
			t.Fatal(i, err)
		}
	}
	for i, a := range values {
		for j, b := range values {
			// Upstream has an untyped set-like fallback that equates unlike
			// constructors. Only equal-type comparisons are language-valid.
			left, right := fixture.Expressions[i], fixture.Expressions[j]
			want := fixture.Equal[i][j]
			if left.Kind != right.Kind || left.Opcode != right.Opcode {
				want = false
			}
			if got := a.Equal(b); got != want {
				t.Fatalf("%d %s == %d %s: %v, want %v", i, a.String(), j, b.String(), got, fixture.Equal[i][j])
			}
			if a.Equal(b) && a.Hash() != b.Hash() {
				t.Fatal("equal values have different hashes", i, j)
			}
		}
	}
}

func TestRuntimeValueOwnershipAndIdentity(t *testing.T) {
	input, err := ParseExpression(`Rec("x", List(Set(2,1)))`)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(input)
	if err != nil {
		t.Fatal(err)
	}
	before, hash := value.String(), value.Hash()
	input.Args[1].Args[0].Args = nil
	output := value.Expression()
	output.Args[1].Args[0].Args = nil
	if value.String() != before || value.Hash() != hash {
		t.Fatal("mutable runtime value")
	}
	copy, err := NewRuntimeValue(value.Expression())
	if err != nil || !copy.Equal(value) {
		t.Fatal("roundtrip", err)
	}
	// Distinct Unicode strings may sort equally but remain distinct language values.
	a, _ := ParseExpression(`Set("é","é")`)
	b, _ := ParseExpression(`Set("é","é")`)
	va, _ := NewRuntimeValue(a)
	vb, _ := NewRuntimeValue(b)
	if !va.Equal(vb) || va.Hash() != vb.Hash() {
		t.Fatal("collation ties affect identity")
	}
	var invalid RuntimeValue
	if invalid.Valid() || invalid.Equal(invalid) || invalid.Equal(value) || value.Equal(invalid) || invalid.String() != "<invalid>" {
		t.Fatal("invalid value accepted")
	}
	if _, err := NewRuntimeValue(Expr{Kind: "name", Name: "x"}); err == nil {
		t.Fatal("symbolic value accepted")
	}
}
