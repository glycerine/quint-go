package quint

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
)

// These formulas preserve the implementation before canonical decimal inputs
// could bypass big.Int. They serve as independent public-behavior oracles.
func originalParseInteger(text string) (Integer, error) {
	n, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return Integer{}, fmt.Errorf("invalid integer %q", text)
	}
	if n.Sign() == 0 {
		return Integer{}, nil
	}
	return Integer{decimal: n.String()}, nil
}

func originalParseID(text string) (ID, error) {
	n, err := originalParseInteger(text)
	if err != nil {
		return ID{}, err
	}
	if n.Big().Sign() < 0 {
		return ID{}, fmt.Errorf("negative IR ID %s", text)
	}
	if n.String() == "0" {
		return ID{}, nil
	}
	return ID{value: n}, nil
}

func originalDecimalJSON(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if data[0] == '-' {
		data = data[1:]
	}
	if len(data) == 0 || (len(data) > 1 && data[0] == '0') {
		return false
	}
	for _, digit := range data {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

type originalIntegerJSON struct{ value Integer }

func (n *originalIntegerJSON) UnmarshalJSON(data []byte) error {
	if !originalDecimalJSON(data) {
		return fmt.Errorf("expected integer JSON number, got %q", data)
	}
	value, err := originalParseInteger(string(data))
	if err != nil {
		return err
	}
	n.value = value
	return nil
}

type originalIDJSON struct{ value ID }

func (id *originalIDJSON) UnmarshalJSON(data []byte) error {
	var n originalIntegerJSON
	if err := n.UnmarshalJSON(data); err != nil {
		return err
	}
	value, err := originalParseID(n.value.String())
	if err != nil {
		return err
	}
	id.value = value
	return nil
}

func integerErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestCanonicalIntegerParsingMatchesOriginal(t *testing.T) {
	large := strings.Repeat("1234567890", 30)
	inputs := []string{
		"0", "-0", "+0", "00", "000", "-000", "+000", "01", "-01", "+01",
		"123", "-123", "+123", "000123", "-000123", "+000123",
		"9223372036854775808", "-9223372036854775809", large, "-" + large,
		"+" + large, "000" + large, "-000" + large,
		"", " ", " 1", "1 ", "\n1", "1\n", "--1", "+-1", "-+1", "-", "+",
		"1_000", "0x10", "0b10", "1.2", "1e2", "1E2", "NaN", "Inf",
		"١٢", "１２", "−1", "1\x00", "\xff", "1\xff",
	}
	for _, input := range inputs {
		got, gotErr := ParseInteger(input)
		want, wantErr := originalParseInteger(input)
		if got != want || integerErrorText(gotErr) != integerErrorText(wantErr) {
			t.Fatalf("ParseInteger(%q): got %q/%v, want %q/%v", input, got, gotErr, want, wantErr)
		}
		if gotErr == nil {
			encoded, err := json.Marshal(got)
			if err != nil || string(encoded) != want.String() {
				t.Fatalf("marshal ParseInteger(%q): %q/%v", input, encoded, err)
			}
		}
		gotID, gotErr := ParseID(input)
		wantID, wantErr := originalParseID(input)
		if gotID != wantID || integerErrorText(gotErr) != integerErrorText(wantErr) {
			t.Fatalf("ParseID(%q): got %q/%v, want %q/%v", input, gotID, gotErr, wantID, wantErr)
		}
		seed, _ := ParseID("73")
		textID := seed
		textErr := textID.UnmarshalText([]byte(input))
		if integerErrorText(textErr) != integerErrorText(wantErr) {
			t.Fatalf("ID.UnmarshalText(%q): got %v, want %v", input, textErr, wantErr)
		}
		if textErr != nil {
			if textID != seed {
				t.Fatalf("failed text decode %q changed ID receiver", input)
			}
		} else if textID != wantID {
			t.Fatalf("ID.UnmarshalText(%q): got %q, want %q", input, textID, wantID)
		}
	}
}

func TestCanonicalIntegerJSONMatchesOriginal(t *testing.T) {
	large := strings.Repeat("9876543210", 30)
	inputs := []string{
		"0", "-0", "1", "-1", large, "-" + large, "  42\n", "\t-0\r\n",
		"", " ", "+0", "+1", "00", "01", "-00", "-01", "-", "+", "--1",
		"null", "true", "false", `"12"`, `"-0"`, "[]", "{}", "[1]",
		"0.0", "-0.0", "1.5", "1e3", "1E-3", "0x10", "1_000", "NaN", "Inf",
		"1 2", "0true", "2\n{}", "1,", "1\x00", "\xff", "1\xff", "١٢", "１２",
	}
	seed, _ := ParseInteger("73")
	idSeed, _ := ParseID("73")
	for _, input := range inputs {
		for _, outerJSON := range []bool{false, true} {
			got, want := seed, originalIntegerJSON{value: seed}
			gotID, wantID := idSeed, originalIDJSON{value: idSeed}
			var gotErr, wantErr, gotIDErr, wantIDErr error
			if outerJSON {
				gotErr, wantErr = json.Unmarshal([]byte(input), &got), json.Unmarshal([]byte(input), &want)
				gotIDErr, wantIDErr = json.Unmarshal([]byte(input), &gotID), json.Unmarshal([]byte(input), &wantID)
			} else {
				gotErr, wantErr = got.UnmarshalJSON([]byte(input)), want.UnmarshalJSON([]byte(input))
				gotIDErr, wantIDErr = gotID.UnmarshalJSON([]byte(input)), wantID.UnmarshalJSON([]byte(input))
			}
			if got != want.value || integerErrorText(gotErr) != integerErrorText(wantErr) {
				t.Fatalf("integer JSON %q (outer %v): got %q/%v, want %q/%v", input, outerJSON, got, gotErr, want.value, wantErr)
			}
			if gotID != wantID.value || integerErrorText(gotIDErr) != integerErrorText(wantIDErr) {
				t.Fatalf("ID JSON %q (outer %v): got %q/%v, want %q/%v", input, outerJSON, gotID, gotIDErr, wantID.value, wantIDErr)
			}
			if gotErr != nil && got != seed {
				t.Fatalf("failed integer JSON %q changed receiver", input)
			}
			if gotIDErr != nil && gotID != idSeed {
				t.Fatalf("failed ID JSON %q changed receiver", input)
			}
			if gotErr == nil {
				encoded, err := json.Marshal(got)
				if err != nil || string(encoded) != want.value.String() {
					t.Fatalf("integer JSON %q marshal: %q/%v", input, encoded, err)
				}
			}
			if gotIDErr == nil {
				encoded, err := json.Marshal(gotID)
				if err != nil || string(encoded) != wantID.value.String() {
					t.Fatalf("ID JSON %q marshal: %q/%v", input, encoded, err)
				}
			}
		}
	}
}

func TestCanonicalIntegerOwnsDecodedBytes(t *testing.T) {
	for _, input := range []string{"0", "-0", "42", "-42", strings.Repeat("1234567890", 30)} {
		source := []byte(input)
		var n Integer
		if err := n.UnmarshalJSON(source); err != nil {
			t.Fatal(err)
		}
		want, _ := originalParseInteger(input)
		for i := range source {
			source[i] = '9'
		}
		encoded, _ := n.MarshalJSON()
		for i := range encoded {
			encoded[i] = '8'
		}
		n.Big().SetInt64(7)
		if n != want {
			t.Fatalf("integer %q retained mutable decoder or marshal bytes", input)
		}
		if want.Big().Sign() < 0 {
			continue
		}
		source = []byte(input)
		var id ID
		if err := id.UnmarshalJSON(source); err != nil {
			t.Fatal(err)
		}
		for i := range source {
			source[i] = '9'
		}
		text, _ := id.MarshalText()
		for i := range text {
			text[i] = '8'
		}
		if id.value != want {
			t.Fatalf("ID %q retained mutable decoder or marshal bytes", input)
		}
	}
}

func BenchmarkCanonicalIntegerDecode(b *testing.B) {
	for _, input := range []struct{ name, text string }{
		{"small", "42"}, {"large", strings.Repeat("1234567890", 10)},
	} {
		for _, implementation := range []struct {
			name  string
			parse func(string) (Integer, error)
		}{{"original", originalParseInteger}, {"canonical", ParseInteger}} {
			b.Run(input.name+"/"+implementation.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_, _ = implementation.parse(input.text)
				}
			})
		}
		data := []byte(input.text)
		b.Run(input.name+"/ID-original", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var id originalIDJSON
				_ = id.UnmarshalJSON(data)
			}
		})
		b.Run(input.name+"/ID-canonical", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var id ID
				_ = id.UnmarshalJSON(data)
			}
		})
	}
}
