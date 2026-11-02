package quint

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/glycerine/insdict"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// Preserve the preallocation-free formula as a differential equivalence oracle.
func normalizeValueAppendOracle(input Expr) (Expr, error) {
	var comparison *collate.Collator
	var normalize func(Expr) (Expr, error)
	sortValues := func(values []Expr) {
		if len(values) < 2 {
			return
		}
		if comparison == nil {
			comparison = collate.New(language.AmericanEnglish)
		}
		type entry struct {
			value Expr
			text  string
		}
		entries := make([]entry, len(values))
		for i, value := range values {
			entries[i] = entry{value, ExpressionString(value)}
		}
		sort.SliceStable(entries, func(i, j int) bool { return comparison.CompareString(entries[i].text, entries[j].text) < 0 })
		for i, entry := range entries {
			values[i] = entry.value
		}
	}
	normalize = func(e Expr) (Expr, error) {
		switch e.Kind {
		case "int":
			return Expr{Kind: "int", Int: e.Int}, nil
		case "str":
			return Expr{Kind: "str", Str: e.Str}, nil
		case "bool":
			return Expr{Kind: "bool", Bool: e.Bool}, nil
		case "app":
		default:
			return Expr{}, fmt.Errorf("Cannot convert %s to a runtime value", ExpressionString(e))
		}
		switch e.Opcode {
		case "List", "Tup", "Set", "Map", "Rec", "variant":
		default:
			return Expr{}, fmt.Errorf("Cannot convert %s to a runtime value", ExpressionString(e))
		}
		out := Expr{Kind: "app", Opcode: e.Opcode, Args: []Expr{}}
		for _, child := range e.Args {
			value, err := normalize(child)
			if err != nil {
				return Expr{}, err
			}
			out.Args = append(out.Args, value)
		}
		switch e.Opcode {
		case "Rec":
			if len(out.Args)%2 != 0 {
				return Expr{}, fmt.Errorf("record requires field/value pairs")
			}
			fields := insdict.NewDict[string, Expr]()
			for i := 0; i < len(out.Args); i += 2 {
				if out.Args[i].Kind != "str" {
					return Expr{}, fmt.Errorf("record field must be a string")
				}
				fields.Set(out.Args[i].Str, out.Args[i+1])
			}
			names := []string{}
			for name := range fields.All() {
				names = append(names, name)
			}
			sort.Slice(names, func(i, j int) bool { return utf16Less(names[i], names[j]) })
			out.Args = []Expr{}
			for _, name := range names {
				out.Args = append(out.Args, Expr{Kind: "str", Str: name}, fields.Get(name))
			}
		case "Set":
			seen := map[string]bool{}
			values := []Expr{}
			for _, value := range out.Args {
				key := normalizedValueKey(value)
				if !seen[key] {
					seen[key] = true
					values = append(values, value)
				}
			}
			sortValues(values)
			out.Args = values
		case "Map":
			pairs := insdict.NewDict[string, Expr]()
			for _, pair := range out.Args {
				if pair.Kind != "app" || pair.Opcode != "Tup" || len(pair.Args) != 2 {
					return Expr{}, fmt.Errorf("map requires two-element tuple entries")
				}
				pairs.Set(normalizedValueKey(pair.Args[0]), pair)
			}
			out.Args = []Expr{}
			for _, pair := range pairs.All() {
				out.Args = append(out.Args, pair)
			}
			sortValues(out.Args)
		case "variant":
			if len(out.Args) != 2 || out.Args[0].Kind != "str" {
				return Expr{}, fmt.Errorf("variant requires a string label and value")
			}
		}
		return out, nil
	}
	return normalize(input)
}

func normalizePreallocApp(op string, values ...Expr) Expr {
	return Expr{Kind: "app", Opcode: op, Args: values}
}

func normalizePreallocInt(value int) Expr {
	number, _ := ParseInteger(fmt.Sprint(value))
	return Expr{Kind: "int", Int: number}
}

func normalizePreallocCases() []struct {
	name  string
	value Expr
} {
	str := func(value string) Expr { return Expr{Kind: "str", Str: value} }
	app, integer := normalizePreallocApp, normalizePreallocInt
	id, _ := ParseID("9007199254740993123456789")
	shared := app("List", app("Set", integer(2), integer(1)), str("shared"))
	cases := []struct {
		name  string
		value Expr
	}{
		{"zero", Expr{}},
		{"int", Expr{ID: id, Kind: "int", Int: integer(42).Int}},
		{"bool", Expr{ID: id, Kind: "bool", Bool: true}},
		{"str", Expr{ID: id, Kind: "str", Str: "é"}},
		{"scalar ignores extra fields", Expr{ID: id, Kind: "str", Str: "x", Name: "ignored", Args: []Expr{{Kind: "name", Name: "bad"}}}},
		{"nested shared trees", app("List", shared, shared, app("variant", str("Some"), shared))},
		{"tuple", app("Tup", app("List", integer(1), integer(2)), Expr{Kind: "bool"})},
		{"sets nested duplicate", app("Set", app("Set", integer(2), integer(1)), app("Set", integer(1), integer(2)))},
		{"set collation ties", app("Set", str("é"), str("e\u0301"), str("é"), str("Z"), str("a"))},
		{"records UTF16 order", app("Rec", str("\ue000"), integer(1), str("\U00010000"), integer(2), str("z"), integer(3))},
		{"record last wins", app("Rec", str("z"), shared, str("a"), integer(2), str("z"), integer(3))},
		{"map duplicate canonical keys", app("Map", app("Tup", app("Set", integer(2), integer(1)), str("first")), app("Tup", app("Set", integer(1), integer(2)), str("last")))},
		{"map ordering ties", app("Map", app("Tup", str("é"), integer(1)), app("Tup", str("e\u0301"), integer(2)), app("Tup", str("a"), integer(3)))},
		{"variant", app("variant", str("Some"), app("Rec", str("value"), shared))},
		{"name", Expr{Kind: "name", Name: "x"}},
		{"lambda", Expr{Kind: "lambda", Params: []Parameter{}, Body: &shared}},
		{"unsupported app", app("iadd", integer(1), integer(2))},
		{"record odd", app("Rec", str("field"))},
		{"record invalid label", app("Rec", Expr{Kind: "bool"}, integer(1))},
		{"record child error precedes label", app("Rec", Expr{Kind: "bool"}, integer(1), str("good"), Expr{Kind: "name", Name: "x"})},
		{"record child error precedes odd count", app("Rec", str("good"), integer(1), Expr{Kind: "name", Name: "x"})},
		{"map scalar pair", app("Map", integer(1))},
		{"map list pair", app("Map", app("List", integer(1), integer(2)))},
		{"map short tuple", app("Map", app("Tup", integer(1)))},
		{"map long tuple", app("Map", app("Tup", integer(1), integer(2), integer(3)))},
		{"map child error precedes pair", app("Map", app("Tup", Expr{Kind: "name", Name: "x"}))},
		{"variant short", app("variant", str("Some"))},
		{"variant long", app("variant", str("Some"), integer(1), integer(2))},
		{"variant invalid label", app("variant", integer(1), integer(2))},
		{"variant child error precedes label", app("variant", Expr{Kind: "bool"}, Expr{Kind: "name", Name: "x"})},
	}
	for _, op := range []string{"List", "Tup", "Rec", "Set", "Map", "variant"} {
		cases = append(cases,
			struct {
				name  string
				value Expr
			}{op + " nil", Expr{ID: id, Kind: "app", Opcode: op}},
			struct {
				name  string
				value Expr
			}{op + " empty", Expr{ID: id, Kind: "app", Opcode: op, Args: []Expr{}}})
	}
	for _, count := range []int{1, 2, 3, 8, 33} {
		values := make([]Expr, count)
		fields := make([]Expr, 0, count*2)
		pairs := make([]Expr, count)
		for i := range values {
			values[i] = app("List", integer(i%3), str(fmt.Sprint(count-i)))
			fields = append(fields, str(fmt.Sprint(i%3)), values[i])
			pairs[i] = app("Tup", integer(i%3), values[i])
		}
		for _, value := range []Expr{app("List", values...), app("Tup", values...), app("Rec", fields...), app("Set", values...), app("Map", pairs...)} {
			cases = append(cases, struct {
				name  string
				value Expr
			}{fmt.Sprintf("%s width %d", value.Opcode, count), value})
		}
	}
	return cases
}

func TestNormalizeValuePreallocationMatchesAppendFormula(t *testing.T) {
	t.Parallel()
	for _, tc := range normalizePreallocCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want, wantErr := normalizeValueAppendOracle(tc.value)
			got, gotErr := NormalizeValue(tc.value)
			if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) || !reflect.DeepEqual(got, want) {
				t.Fatalf("normalization differs:\n got %#v / %v\nwant %#v / %v", got, gotErr, want, wantErr)
			}
			if gotErr != nil {
				return
			}
			if normalizedValueKey(got) != normalizedValueKey(want) {
				t.Fatal("canonical key changed")
			}
			// Every normalized occurrence must own its tree, including two
			// occurrences of the same subtree in the input.
			var mutate func(*Expr)
			mutate = func(value *Expr) {
				for i := range value.Args {
					mutate(&value.Args[i])
				}
				value.ID, _ = ParseID("999")
				value.Str = "mutated"
				value.Args = append(value.Args, Expr{Kind: "name", Name: "mutated"})
			}
			if len(got.Args) > 1 {
				mutate(&got.Args[0])
				if !reflect.DeepEqual(got.Args[1:], want.Args[1:]) {
					t.Fatal("mutating one occurrence changed another occurrence")
				}
			}
			mutate(&got)
			after, err := NormalizeValue(tc.value)
			if err != nil || !reflect.DeepEqual(after, want) {
				t.Fatal("mutating output changed the input or another result", err)
			}
		})
	}
}
