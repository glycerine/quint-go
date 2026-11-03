package quint

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamITFUnicode(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Node, ICU, Locale string
		Cases             []struct {
			Name, Input, NormalizationError string
			UnsupportedUnicode              bool
			Decoded, Normalized             json.RawMessage
			StringUnits                     [][]uint16
		}
	}
	data, err := os.ReadFile("testdata/parser/itf_unicode.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 19 || fixture.Node == "" || fixture.ICU == "" || fixture.Locale == "" {
		t.Fatal("missing source oracle evidence")
	}
	for _, f := range fixture.Cases {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			input := []byte(f.Input)
			trace, err := DecodeITFTrace(input)
			if f.UnsupportedUnicode {
				if !itfUnicodeHasLoneUnit(f.StringUnits) {
					t.Fatal("source accepted case does not demonstrate a lone UTF-16 code unit")
				}
				if err == nil || !strings.Contains(err.Error(), "not valid Unicode") {
					t.Fatalf("unsupported JavaScript UTF-16 string was not rejected: %v", err)
				}
				if _, err := DecodeNormalizedITFTrace(input); err == nil {
					t.Fatal("unsupported Unicode normalized with replacement characters")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			itfUnicodeEqualExpressions(t, trace.States, f.Decoded)
			if !bytes.Equal(trace.Raw, input) {
				t.Fatal("raw trace changed")
			}
			if f.Name == "unicode-headers" && (!reflect.DeepEqual(trace.Vars, []string{"😀", "�", `\ud800`}) || !reflect.DeepEqual(trace.Params, []string{"𐀀"})) {
				t.Fatal("valid Unicode header names changed", trace.Vars, trace.Params)
			}
			normalized, err := DecodeNormalizedITFTrace(input)
			if f.NormalizationError != "" {
				if err == nil {
					t.Fatal("source symbolic normalization failure became a concrete value")
				}
				if _, err := EncodeITFTrace(trace.Vars, trace.States, false); err == nil {
					t.Fatal("symbolic name encoded as a concrete ITF value")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			itfUnicodeEqualExpressions(t, normalized.States, f.Normalized)
			encoded, err := EncodeITFTrace(trace.Vars, trace.States, false)
			if err != nil {
				t.Fatal(err)
			}
			roundTrip, err := DecodeITFTrace(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(roundTrip.States, trace.States) {
				t.Fatalf("Unicode changed on round trip: %s", encoded)
			}
			if f.Name == "metadata-is-retained-without-interpreting-text" {
				if string(trace.Metadata) != `{"text":"\ud800","description":"😀"}` || len(trace.StateMetadata) != 1 || string(trace.StateMetadata[0]) != `{"index":0,"note":"\udc00"}` {
					t.Fatal("original metadata string escapes were lost")
				}
				if trace.Loop == nil || *trace.Loop != 0 || !reflect.DeepEqual(trace.Params, []string{"N"}) || !reflect.DeepEqual(trace.Vars, []string{"x"}) {
					t.Fatal("trace header changed")
				}
				input[0] = 'x'
				if trace.Raw[0] != '{' || normalized.Raw[0] != '{' || trace.Metadata[0] != '{' || normalized.Metadata[0] != '{' {
					t.Fatal("retained trace aliases input bytes")
				}
			}
		})
	}
}

func itfUnicodeEqualExpressions(t *testing.T, expressions []Expr, expected json.RawMessage) {
	t.Helper()
	var want []Expr
	if err := json.Unmarshal(expected, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expressions, want) {
		got, _ := json.Marshal(expressions)
		t.Fatalf("Go %s\nTypeScript %s", got, expected)
	}
}

func itfUnicodeHasLoneUnit(strings [][]uint16) bool {
	for _, units := range strings {
		for i := 0; i < len(units); i++ {
			u := units[i]
			if u >= 0xd800 && u <= 0xdbff {
				if i+1 == len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
					return true
				}
				i++
			} else if u >= 0xdc00 && u <= 0xdfff {
				return true
			}
		}
	}
	return false
}

func TestITFRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()
	for name, invalid := range map[string]string{
		"invalid-byte":       string([]byte{0xff}),
		"encoded-surrogate":  string([]byte{0xed, 0xa0, 0x80}),
		"overlong-sequence":  string([]byte{0xc0, 0xaf}),
		"truncated-sequence": string([]byte{0xf0, 0x9f, 0x98}),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			value := Expr{Kind: "str", Str: invalid}
			for _, expression := range []Expr{
				value,
				{Kind: "app", Opcode: "List", Args: []Expr{value}},
				{Kind: "app", Opcode: "Rec", Args: []Expr{value, {Kind: "bool", Bool: true}}},
				{Kind: "app", Opcode: "variant", Args: []Expr{value, {Kind: "bool", Bool: true}}},
			} {
				if _, err := EncodeITFValue(expression); err == nil || !strings.Contains(err.Error(), "not valid UTF-8") {
					t.Fatalf("invalid UTF-8 encoded as replacement text: %v", err)
				}
			}
			for _, raw := range []string{
				`"` + invalid + `"`,
				`{"` + invalid + `":true}`,
				`{"#unserializable":"` + invalid + `"}`,
			} {
				if _, err := DecodeITFValue([]byte(raw), nil); err == nil || !strings.Contains(err.Error(), "not valid Unicode") {
					t.Fatalf("invalid UTF-8 decoded with replacement: %v", err)
				}
			}
		})
	}
}

// Unlike source ofItf, the Go API exposes vars and params as decoded strings.
// Its checked header boundary must not expose replacement text for invalid names.
func TestITFCheckedUnicodeHeaders(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"vars", "params"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			for _, malformed := range []string{
				`["\ud800"]`, `["\udc00"]`,
				`["valid","\ud800"]`, `["` + string([]byte{0xff}) + `"]`,
				`[null]`, `["valid",null]`, `[true]`, `[1]`, `[{}]`, `[[]]`,
				`1`, `true`, `"name"`, `{}`,
			} {
				input := []byte(`{"` + field + `":` + malformed + `,"states":[]}`)
				if _, err := DecodeITFTrace(input); err == nil {
					t.Fatalf("lossy or non-string %s header accepted: %q", field, malformed)
				}
				if _, err := DecodeNormalizedITFTrace(input); err == nil {
					t.Fatalf("lossy or non-string %s header normalized: %q", field, malformed)
				}
			}
			for _, valid := range []struct {
				JSON string
				Want []string
			}{
				{`null`, nil}, {`[]`, []string{}},
				{`["\ud83d\ude00","\ufffd","\\ud800"]`, []string{"😀", "�", `\ud800`}},
			} {
				trace, err := DecodeITFTrace([]byte(`{"` + field + `":` + valid.JSON + `,"states":[]}`))
				if err != nil {
					t.Fatal(err)
				}
				got := trace.Vars
				if field == "params" {
					got = trace.Params
				}
				if !reflect.DeepEqual(got, valid.Want) {
					t.Fatalf("%s: got %#v, want %#v", valid.JSON, got, valid.Want)
				}
			}
		})
	}
	trace, err := DecodeITFTrace([]byte(`{"states":[]}`))
	if err != nil || trace.Vars != nil || trace.Params != nil {
		t.Fatal("absent optional headers changed", trace, err)
	}
	for _, vars := range [][]string{
		{string([]byte{0xff})}, {"valid", string([]byte{0xed, 0xa0, 0x80})},
	} {
		if _, err := EncodeITFTrace(vars, nil, false); err == nil {
			t.Fatal("invalid UTF-8 header encoded with replacement text")
		}
	}
	for _, vars := range [][]string{nil, {}, {"😀", "�", `\ud800`}} {
		encoded, err := EncodeITFTrace(vars, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		roundTrip, err := DecodeITFTrace(encoded)
		if err != nil || !reflect.DeepEqual(roundTrip.Vars, append([]string{}, vars...)) {
			t.Fatal("valid encoded header changed", string(encoded), err)
		}
	}
}
