package quint

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Preserve the previous textual construction as an independent parser oracle.
func parsedDynamicBuiltinOracle(name string, arity int) (TypeScheme, error) {
	if arity < 0 {
		return TypeScheme{}, fmt.Errorf("negative builtin arity")
	}
	arg, result, text := "", "", ""
	switch name {
	case "List":
		arg, result = "a", "List[a]"
	case "Set":
		arg, result = "a", "Set[a]"
	case "Map":
		arg, result = "(a, b)", "a -> b"
	case "and", "or", "actionAll", "actionAny":
		arg, result = "bool", "bool"
	case "matchVariant":
		if arity < 1 || arity%2 == 0 {
			return TypeScheme{}, fmt.Errorf("matchVariant requires a value and pattern/function pairs")
		}
		args := []string{"a"}
		for i := 1; i < arity; i += 2 {
			args = append(args, "str", "(a) => b")
		}
		text = "(" + strings.Join(args, ", ") + ") => b"
	case "tuples":
		names, args := []string{}, []string{}
		for i := 0; i < arity; i++ {
			n := fmt.Sprintf("t%d", i)
			names = append(names, n)
			args = append(args, "Set["+n+"]")
		}
		text = "(" + strings.Join(args, ", ") + ") => Set[(" + strings.Join(names, ", ") + ")]"
	default:
		return TypeScheme{}, fmt.Errorf("no ordinary builtin signature for %q", name)
	}
	if arg != "" {
		args := make([]string, arity)
		for i := range args {
			args[i] = arg
		}
		text = "(" + strings.Join(args, ", ") + ") => " + result
	}
	return parseBuiltinTypeScheme(text)
}

func TestUpstreamDynamicBuiltinTypeTrees(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name   string
		Arity  int
		Scheme TypeScheme
	}
	data, err := os.ReadFile("testdata/parser/builtin_types_dynamic.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fmt.Sprintf("%s/%d", fixture.Name, fixture.Arity), func(t *testing.T) {
			got, err := BuiltinTypeSignature(fixture.Name, fixture.Arity)
			// JSON omitempty removes empty tree slices in both representations.
			actual, e1 := json.Marshal(got)
			expected, e2 := json.Marshal(fixture.Scheme)
			if err != nil || e1 != nil || e2 != nil || string(actual) != string(expected) {
				t.Fatalf("got %s, want %s: %v", actual, expected, err)
			}
			if err = got.Type.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDynamicBuiltinTypeTreeOwnership(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"List", "Set", "Map", "and", "or", "actionAll", "actionAny", "matchVariant", "tuples"} {
		original, err := BuiltinTypeSignature(name, 5)
		if err != nil {
			t.Fatal(err)
		}
		independent, err := BuiltinTypeSignature(name, 5)
		if err != nil {
			t.Fatal(err)
		}
		mutateBuiltinTypeScheme(&independent)
		check, err := BuiltinTypeSignature(name, 5)
		if err != nil || !reflect.DeepEqual(original, check) {
			t.Fatal(name, "calls share mutable trees", err)
		}
		// Mutating any argument must not alter sibling arguments or the result.
		for argument := range original.Type.Args {
			fresh, err := BuiltinTypeSignature(name, 5)
			if err != nil {
				t.Fatal(err)
			}
			before, err := cloneBuiltinTypeScheme(fresh)
			if err != nil {
				t.Fatal(err)
			}
			part := TypeScheme{Type: fresh.Type.Args[argument]}
			mutateBuiltinTypeScheme(&part)
			fresh.Type.Args[argument] = part.Type
			for i := range fresh.Type.Args {
				if i != argument && !reflect.DeepEqual(fresh.Type.Args[i], before.Type.Args[i]) {
					t.Fatal(name, "argument aliases sibling", argument, i)
				}
			}
			if !reflect.DeepEqual(fresh.Type.Res, before.Type.Res) {
				t.Fatal(name, "argument aliases result", argument)
			}
		}

	}
}

func TestDynamicBuiltinTypeSignatureErrors(t *testing.T) {
	for _, input := range []struct {
		name  string
		arity int
	}{{"List", -1}, {"matchVariant", 0}, {"matchVariant", 2}, {"unknown", 1}, {"unknown", int(^uint(0) >> 1)}, {"Rec", 2}} {
		_, want := parsedDynamicBuiltinOracle(input.name, input.arity)
		_, got := dynamicBuiltinTypeScheme(input.name, input.arity)
		if got == nil || got.Error() != want.Error() {
			t.Fatal(input, got, want)
		}
	}
}

func BenchmarkDynamicBuiltinTypeSignature(b *testing.B) {
	for _, name := range []string{"List", "Map", "matchVariant", "tuples"} {
		for _, implementation := range []string{"parsed", "direct"} {
			b.Run(name+"/"+implementation, func(b *testing.B) {
				build := parsedDynamicBuiltinOracle
				if implementation == "direct" {
					build = BuiltinTypeSignature
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := build(name, 9); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
