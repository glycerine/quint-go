package quint

import (
	"encoding/json"
	"sort"
	"testing"
)

// This is the original identity formula, kept as an independent byte-level
// compatibility oracle and benchmark baseline, not used by production code.
func originalNormalizedValueKey(value Expr) string {
	parts := []string{value.Kind, value.Opcode}
	switch value.Kind {
	case "int":
		parts = append(parts, value.Int.String())
	case "str":
		parts = append(parts, value.Str)
	case "bool":
		if value.Bool {
			parts = append(parts, "true")
		} else {
			parts = append(parts, "false")
		}
	}
	children := []string{}
	for _, child := range value.Args {
		children = append(children, originalNormalizedValueKey(child))
	}
	if value.Opcode == "Set" || value.Opcode == "Map" {
		sort.Strings(children)
	}
	parts = append(parts, children...)
	encoded, _ := json.Marshal(parts)
	return string(encoded)
}

func TestNormalizedValueKeyPreservesExactBytes(t *testing.T) {
	text := func(value string) Expr { return Expr{Kind: "str", Str: value} }
	app := func(op string, children ...Expr) Expr {
		return Expr{Kind: "app", Opcode: op, Args: children}
	}
	integer, _ := ParseInteger("-900719925474099312345678901234567890")
	values := []Expr{
		{Kind: "int"}, {Kind: "int", Int: integer},
		{Kind: "bool"}, {Kind: "bool", Bool: true},
		text(""), text("plain"), text("quote\"slash\\newline\n\t\r\b\f\x00\x01"),
		text("<script>&</script>"), text("é"), text("e\u0301"), text("\u2028\u2029"),
		text("\xff\xfe"), text("\ufffd"), text("🎲𐀀"),
		{Kind: "lambda"}, {Kind: "name", Name: "ignored by value identity"},
	}
	for _, op := range []string{"List", "Tup", "Set", "Map", "Rec", "variant"} {
		values = append(values, app(op))
	}
	largeSet, largeMap := app("Set"), app("Map")
	for i := 0; i < 40; i++ {
		number, _ := ParseInteger(string(rune('0' + i%10)))
		key := Expr{Kind: "int", Int: number}
		largeSet.Args = append(largeSet.Args, key)
		largeMap.Args = append(largeMap.Args, app("Tup", key, text("value")))
	}
	values = append(values, largeSet, largeMap)
	for depth := 0; depth < 3; depth++ {
		start := len(values)
		for i := 0; i < 8; i++ {
			left, right := values[(i*3+depth)%start], values[(i*7+5)%start]
			values = append(values,
				app("List", left, right), app("Tup", right, left),
				app("Set", left, right, left),
				app("Map", app("Tup", left, right), app("Tup", right, left)),
				app("Rec", text("z"), left, text("é"), right),
				app("variant", text("V"), app("List", left, right)),
			)
		}
	}
	deep := text("<>&\u2028\xff")
	for i := 0; i < 10; i++ {
		deep = app("List", deep)
	}
	values = append(values, deep)
	for i, value := range values {
		got, want := normalizedValueKey(value), originalNormalizedValueKey(value)
		if got != want {
			t.Fatalf("value %d identity changed\ngot  %s\nwant %s", i, got, want)
		}
	}
}

func BenchmarkNormalizedValueKey(b *testing.B) {
	integer := func(n string) Expr {
		value, _ := ParseInteger(n)
		return Expr{Kind: "int", Int: value}
	}
	pair := func(a, value Expr) Expr {
		return Expr{Kind: "app", Opcode: "Tup", Args: []Expr{a, value}}
	}
	nested := Expr{Kind: "app", Opcode: "Map", Args: []Expr{
		pair(integer("2"), integer("20")), pair(integer("1"), integer("10")),
	}}
	for range 3 {
		nested = Expr{Kind: "app", Opcode: "List", Args: []Expr{nested}}
	}
	for _, input := range []struct {
		name  string
		value Expr
	}{{"scalar", integer("42")}, {"nested", nested}} {
		for _, implementation := range []struct {
			name string
			key  func(Expr) string
		}{{"original", originalNormalizedValueKey}, {"append", normalizedValueKey}} {
			b.Run(input.name+"/"+implementation.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_ = implementation.key(input.value)
				}
			})
		}
	}
}
