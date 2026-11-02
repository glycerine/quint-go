package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamReferenceHashes(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name                    string
		Input                   Expr
		Hash                    int64
		ImmutableHash           int32
		NormalizedHash          int64
		NormalizedImmutableHash int32
	}
	data, err := os.ReadFile("testdata/parser/runtime_reference_hash.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			value, err := NewRuntimeValue(f.Input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := runtimeReferenceHash(value)
			if err != nil || got != f.ImmutableHash {
				t.Fatalf("Immutable.hash: got %d, want %d: %v", got, f.ImmutableHash, err)
			}
			code, err := runtimeReferenceHashCode(value.expression)
			if err != nil || code != f.Hash {
				t.Fatalf("hashCode: got %d, want %d: %v", code, f.Hash, err)
			}
			normalized, err := NewRuntimeValue(value.expression)
			if err != nil {
				t.Fatal(err)
			}
			got, err = runtimeReferenceHash(normalized)
			if err != nil || got != f.NormalizedImmutableHash {
				t.Fatalf("normalized Immutable.hash: got %d, want %d: %v", got, f.NormalizedImmutableHash, err)
			}
			code, err = runtimeReferenceHashCode(normalized.expression)
			if err != nil || code != f.NormalizedHash {
				t.Fatalf("normalized hashCode: got %d, want %d: %v", code, f.NormalizedHash, err)
			}
		})
	}
}

func TestReferenceHashRejectsUnsupportedValues(t *testing.T) {
	for _, value := range []RuntimeValue{{}, {identity: "closure", closure: &runtimeClosure{}}, {identity: "invalid", expression: Expr{Kind: "name", Name: "x"}}} {
		if _, err := runtimeReferenceHash(value); err == nil {
			t.Fatal("unsupported value accepted")
		}
	}
	for _, e := range []Expr{{Kind: "app", Opcode: "variant"}, {Kind: "app", Opcode: "unknown"}, {Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "str", Str: "x"}}}, {Kind: "app", Opcode: "Map", Args: []Expr{{Kind: "int"}}}} {
		if _, err := runtimeReferenceHashCode(e); err == nil {
			t.Fatal("unsupported expression accepted", e)
		}
	}
}
