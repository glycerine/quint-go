package quint

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestModuleSourceMapCoverage(t *testing.T) {
	text := `module M {
 const N: int
 var x: int
 assume _ = N > 0
 type Option[a] = Some(a) | None
 val (a,b) = pair
 import Other(N=3) as O
 val f = ((a,b)) => match a { Some(x) => x | _ => b }
} module Empty {}`
	ms, sources, err := ParseModulesWithSource("m.qnt", text)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := ParseModules(text)
	if err != nil || !reflect.DeepEqual(ms, plain) {
		t.Fatal("source mapping changed IR")
	}
	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err = json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	seen := map[ID]bool{}
	var check func(any)
	check = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if number, ok := x["id"].(float64); ok {
				raw, _ := json.Marshal(number)
				id, err := ParseID(string(raw))
				if err != nil {
					t.Fatal(err)
				}
				loc, ok := sources[id]
				if !ok || loc.Source != "m.qnt" || loc.End == nil {
					t.Fatalf("unmapped ID %s", id)
				}
				seen[id] = true
			}
			for _, child := range x {
				check(child)
			}
		case []any:
			for _, child := range x {
				check(child)
			}
		}
	}
	check(value)
	if len(sources) != len(seen) {
		t.Fatalf("source entries %d vs IR IDs %d", len(sources), len(seen))
	}
}
func TestModuleExactSourceSpans(t *testing.T) {
	text := "module M {\n val x = 1\n type T = A | B\n import X(N=2) as Y\n}"
	ms, sources, err := ParseModulesWithSource("m", text)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := NewSourceFile("m", text)
	excerpt := func(id ID) string {
		t.Helper()
		loc := sources[id]
		a, err := f.ByteOffset(loc.Start.Line, loc.Start.Col)
		if err != nil {
			t.Fatal(err)
		}
		b, err := f.ByteOffset(loc.End.Line, loc.End.Col)
		if err != nil {
			t.Fatal(err)
		}
		return text[a:b]
	}
	ds := ms[0].Declarations
	for _, tc := range []struct {
		id   ID
		want string
	}{{ms[0].ID, text}, {ds[0].ID, "val x = 1"}, {ds[0].Expr.ID, "1"}, {ds[1].ID, "type T = A | B"}, {ds[2].ID, "A"}, {ds[3].ID, "B"}, {ds[4].Overrides[0].Parameter.ID, "N"}} {
		if got := excerpt(tc.id); got != tc.want {
			t.Fatalf("span %q want %q", got, tc.want)
		}
	}
	for _, source := range []string{"module M {", string([]byte{0xff})} {
		if _, m, err := ParseModulesWithSource("bad", source); err == nil || m != nil {
			t.Fatal("invalid source accepted")
		}
	}
}
