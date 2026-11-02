package quint

import (
	"os"
	"reflect"
	"regexp"
	"testing"
)

func TestDiagnosticCodeInventory(t *testing.T) {
	source, err := os.ReadFile("../quint/src/quintError.ts")
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`\| '(QNT[0-9]+)'`).FindAllStringSubmatch(string(source), -1)
	want := make([]string, len(matches))
	for i, m := range matches {
		want[i] = m[1]
	}
	codes := DiagnosticCodes()
	if len(codes) == 0 || !reflect.DeepEqual(codes, want) {
		t.Fatalf("code inventory differs: got %v want %v", codes, want)
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if seen[code] {
			t.Fatalf("duplicate code %s", code)
		}
		seen[code] = true
		if !IsDiagnosticCode(code) {
			t.Fatalf("unrecognized declared code %s", code)
		}
	}
	for _, code := range []string{"", "QNT", "QNT999", "qnt000", "QNT0000", " QNT000"} {
		if IsDiagnosticCode(code) {
			t.Fatalf("accepted unknown code %q", code)
		}
	}
	codes[0] = "changed"
	if DiagnosticCodes()[0] != QNT000 {
		t.Fatal("caller mutated shared inventory")
	}
}
