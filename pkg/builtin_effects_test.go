package quint

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestUpstreamBuiltinEffectSignatures(t *testing.T) {
	var fixtures []struct {
		Name   string
		Arity  int
		Scheme EffectScheme
	}
	data, err := os.ReadFile("testdata/parser/builtin_effects.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range fixtures {
		seen[f.Name] = true
		t.Run(fmt.Sprintf("%s/%d", f.Name, f.Arity), func(t *testing.T) {
			got, err := BuiltinEffectSignature(f.Name, f.Arity)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(f.Scheme)
			if string(a) != string(b) {
				t.Fatalf("Go %s\nTS %s", a, b)
			}
		})
	}
	for n := range fixedEffectSignatures {
		if !seen[n] {
			t.Fatalf("extra signature %s", n)
		}
	}
	for n := range fixedEffectPropagation {
		if !seen[n] {
			t.Fatalf("extra propagation %s", n)
		}
	}
}
func TestBuiltinEffectErrorsAndIsolation(t *testing.T) {
	if _, err := BuiltinEffectSignature("missing", 2); err == nil {
		t.Fatal("unknown accepted")
	}
	if _, err := BuiltinEffectSignature("List", -1); err == nil {
		t.Fatal("negative arity accepted")
	}
	for _, n := range []string{"iadd", "assign", "actionAll", "matchVariant"} {
		s, err := BuiltinEffectSignature(n, 3)
		if err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(s)
		s.Effect.Result.Components[0].Kind = "changed"
		next, err := BuiltinEffectSignature(n, 3)
		if err != nil {
			t.Fatal(err)
		}
		after, _ := json.Marshal(next)
		if string(before) != string(after) {
			t.Fatal("shared signature state")
		}
	}
}
