package quint

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"slices"
	"sort"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/glycerine/insdict"
)

// NormalizeValue constructs the deterministic structural form of a concrete
// value. It resets IDs to zero, sorts records, deduplicates sets, keeps
// the last map value for equal keys, and sorts sets/map pairs by their printed
// expressions using en-US collation. Lists and tuples retain element order.
// This does not evaluate operators or expand symbolic runtime collections.
// Runtime display can reorder collation ties through object-hash history;
// DecodeNormalizedITFTrace and SerializeRuntimeValueContext preserve that path.
func NormalizeValue(input Expr) (Expr, error) {
	var comparison *runtimeCollator
	defer func() { runtimeCollators.release(comparison) }()
	var normalize func(Expr) (Expr, error)
	sortValues := func(values []Expr) {
		if len(values) < 2 {
			return
		}
		if comparison == nil {
			comparison = runtimeCollators.acquire()
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
		out := Expr{Kind: "app", Opcode: e.Opcode, Args: make([]Expr, len(e.Args))}
		for i, child := range e.Args {
			value, err := normalize(child)
			if err != nil {
				return Expr{}, err
			}
			out.Args[i] = value
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
			names := make([]string, 0, fields.Len())
			for name := range fields.All() {
				names = append(names, name)
			}
			sort.Slice(names, func(i, j int) bool { return utf16Less(names[i], names[j]) })
			out.Args = make([]Expr, len(names)*2)
			for i, name := range names {
				out.Args[2*i] = Expr{Kind: "str", Str: name}
				out.Args[2*i+1] = fields.Get(name)
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
			out.Args = make([]Expr, 0, pairs.Len())
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
	return string(appendNormalizedValueKey(nil, value))
}

func appendNormalizedValueKey(dst []byte, value Expr) []byte {
	var localChildren [8][]byte
	children := localChildren[:]
	if len(value.Args) > len(localChildren) {
		children = make([][]byte, len(value.Args))
	} else {
		children = children[:len(value.Args)]
	}
	capacity := len(value.Kind) + len(value.Opcode) + 16
	literal := ""
	switch value.Kind {
	case "int":
		literal = value.Int.String()
	case "str":
		literal = value.Str
	case "bool":
		literal = "false"
		if value.Bool {
			literal = "true"
		}
	}
	capacity += len(literal)
	for i, child := range value.Args {
		children[i] = appendNormalizedValueKey(nil, child)
		// Child keys are already valid JSON with HTML/separators escaped.
		// Only their quotes and backslashes need another escaping level.
		capacity += len(children[i]) + 3 + bytes.Count(children[i], []byte{'"'}) + bytes.Count(children[i], []byte{'\\'})
	}
	if cap(dst)-len(dst) < capacity {
		grown := make([]byte, len(dst), len(dst)+capacity)
		copy(grown, dst)
		dst = grown
	}
	dst = append(dst, '[')
	dst = appendValueKeyString(dst, value.Kind)
	dst = append(dst, ',')
	dst = appendValueKeyString(dst, value.Opcode)
	switch value.Kind {
	case "int":
		dst = append(dst, ',')
		dst = appendValueKeyString(dst, literal)
	case "str":
		dst = append(dst, ',')
		dst = appendValueKeyString(dst, literal)
	case "bool":
		dst = append(dst, ',')
		dst = appendValueKeyString(dst, literal)
	}
	if value.Opcode == "Set" || value.Opcode == "Map" {
		slices.SortFunc(children, bytes.Compare)
	}
	for _, child := range children {
		dst = append(dst, ',')
		dst, _ = jsontext.AppendQuote(dst, child)
	}
	return append(dst, ']')
}

// Preserve the original json.Marshal([]string) identity bytes, including HTML,
// JavaScript separator, and invalid UTF-8 escaping. Most keys need only the
// append-only JSON string encoder; the uncommon escaping cases use v1's exact
// string representation. Child keys stay byte slices until the final result.
func appendValueKeyString[T ~string | ~[]byte](dst []byte, value T) []byte {
	if bytes.IndexAny([]byte(value), "<>&\u2028\u2029") >= 0 || !utf8.Valid([]byte(value)) {
		encoded, _ := json.Marshal(string(value))
		return append(dst, encoded...)
	}
	encoded, _ := jsontext.AppendQuote(dst, value)
	return encoded
}
