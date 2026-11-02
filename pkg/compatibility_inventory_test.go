package quint

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The source opcode inventory already has an upstream drift test. This check
// ensures additions also receive a visible implementation/gap classification.
func TestCompatibilityInventory(t *testing.T) {
	data, err := os.ReadFile("COMPATIBILITY.md")
	if err != nil {
		t.Fatal(err)
	}
	_, matrix, ok := strings.Cut(string(data), "<!-- opcode-matrix:start -->")
	if !ok {
		t.Fatal("missing opcode matrix start")
	}
	matrix, _, ok = strings.Cut(matrix, "<!-- opcode-matrix:end -->")
	if !ok {
		t.Fatal("missing opcode matrix end")
	}
	want := make(map[string]bool)
	for _, name := range append(BuiltinOpcodes(), "getOnlyElement", "Bool", "Int", "Nat", "q::lastTrace") {
		want[name] = true
	}
	seen := make(map[string]bool)
	quotedName := regexp.MustCompile("`([^`]+)`")
	for _, line := range strings.Split(matrix, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) != 7 {
			t.Fatalf("matrix row must classify evaluation, type, effect and backend: %s", line)
		}
		for _, match := range quotedName.FindAllStringSubmatch(cells[1], -1) {
			name := match[1]
			if !want[name] {
				t.Errorf("unrecognized matrix entry %q", name)
			}
			if seen[name] {
				t.Errorf("duplicate matrix entry %q", name)
			}
			seen[name] = true
		}
		for _, status := range cells[2:6] {
			if strings.TrimSpace(status) == "" {
				t.Errorf("unclassified matrix row: %s", line)
			}
		}
	}
	for _, name := range append(BuiltinOpcodes(), "getOnlyElement", "Bool", "Int", "Nat", "q::lastTrace") {
		if !seen[name] {
			t.Errorf("missing compatibility classification for %s", name)
		}
	}
}
