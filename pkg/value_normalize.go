package quint

import (
	"encoding/json"
	"fmt"
	"sort"
	"unicode/utf16"

	"github.com/glycerine/insdict"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// NormalizeValue implements the concrete-value round trip used by upstream ITF
// normalization. It resets IDs to zero, sorts records, deduplicates sets, keeps
// the last map value for equal keys, and sorts sets/map pairs by their printed
// expressions using en-US collation. Lists and tuples retain element order.
// This does not evaluate operators or expand symbolic runtime collections.
func NormalizeValue(input Expr) (Expr, error) {
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

// DecodeNormalizedITFTrace provides upstream ofItfNormalized's concrete-state
// normalization while retaining the original trace bytes and metadata.
func DecodeNormalizedITFTrace(data []byte) (ITFTrace, error) {
	trace, err := DecodeITFTrace(data)
	if err != nil {
		return ITFTrace{}, err
	}
	for i, state := range trace.States {
		trace.States[i], err = NormalizeValue(state)
		if err != nil {
			return ITFTrace{}, fmt.Errorf("normalizing ITF state %d: %w", i, err)
		}
	}
	return trace, nil
}

func utf16Less(a, b string) bool {
	left, right := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] != right[i] {
			return left[i] < right[i]
		}
	}
	return len(left) < len(right)
}

// Identity is independent of locale-sort ties: distinct Unicode strings may
// collate equally, but sets/maps still compare by their exact values.
func normalizedValueKey(value Expr) string {
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
		children = append(children, normalizedValueKey(child))
	}
	if value.Opcode == "Set" || value.Opcode == "Map" {
		sort.Strings(children)
	}
	parts = append(parts, children...)
	encoded, _ := json.Marshal(parts)
	return string(encoded)
}
