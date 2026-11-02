package quint

import (
	"os"
	"reflect"
	"regexp"
	"testing"
)

func TestFixedBuiltinTypeSignatures(t *testing.T) {
	source, err := os.ReadFile("../quint/src/types/builtinSignatures.ts")
	if err != nil {
		t.Fatal(err)
	}
	entries := regexp.MustCompile(`\{ name: '([^']+)', type: '([^']+)' \}`).FindAllStringSubmatch(string(source), -1)
	if len(entries) != len(fixedBuiltinTypes) {
		t.Fatal("signature inventory drift")
	}
	for _, entry := range entries {
		if fixedBuiltinTypes[entry[1]] != entry[2] {
			t.Fatal("changed signature", entry[1])
		}
		scheme, err := BuiltinTypeSignature(entry[1], 0)
		if err != nil {
			t.Fatalf("%s: %v", entry[1], err)
		}
		if err = scheme.Type.Validate(); err != nil {
			t.Fatalf("%s: %v", entry[1], err)
		}
		tv, rv, err := TypeNames(&scheme.Type)
		if err != nil || !reflect.DeepEqual(tv, scheme.TypeVariables) || !reflect.DeepEqual(rv, scheme.RowVariables) {
			t.Fatal("quantification mismatch")
		}
	}
}
func TestVariableBuiltinTypeSignatures(t *testing.T) {
	for _, name := range []string{"List", "Set", "Map", "and", "or", "actionAll", "actionAny", "tuples"} {
		for _, arity := range []int{0, 1, 3} {
			s, err := BuiltinTypeSignature(name, arity)
			if err != nil {
				t.Fatalf("%s/%d: %v", name, arity, err)
			}
			if s.Type.Kind != "oper" || len(s.Type.Args) != arity {
				t.Fatalf("wrong arity %s/%d", name, arity)
			}
		}
	}
	s, err := BuiltinTypeSignature("matchVariant", 5)
	if err != nil || len(s.Type.Args) != 5 || s.Type.Args[2].Kind != "oper" {
		t.Fatalf("match signature: %+v %v", s, err)
	}
	for _, c := range []struct {
		name  string
		arity int
	}{{"unknown", 1}, {"Rec", 2}, {"List", -1}, {"matchVariant", 2}} {
		if _, err := BuiltinTypeSignature(c.name, c.arity); err == nil {
			t.Errorf("accepted %+v", c)
		}
	}
}
