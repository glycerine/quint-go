package quint

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestBuiltinInventoryMatchesUpstream(t *testing.T) {
	source, err := os.ReadFile("../quint/src/ir/quintIr.ts")
	if err != nil {
		t.Fatal(err)
	}
	_, rest, ok := strings.Cut(string(source), "export const builtinOpCodes = [")
	if !ok {
		t.Fatal("missing upstream opcode array")
	}
	body, _, ok := strings.Cut(rest, "] as const")
	if !ok {
		t.Fatal("missing upstream opcode array terminator")
	}
	matches := regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(body, -1)
	want := make([]string, len(matches))
	for i, m := range matches {
		want[i] = m[1]
	}
	got := BuiltinOpcodes()
	if len(got) == 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("inventory mismatch: %v != %v", got, want)
	}
	seen := map[string]bool{}
	for _, op := range got {
		if seen[op] {
			t.Fatalf("duplicate opcode %s", op)
		}
		seen[op] = true
		if !IsBuiltinOpcode(op) || !IsBuiltinApp(Expr{Kind: "app", Opcode: op}) {
			t.Fatalf("missing builtin %s", op)
		}
		if IsBuiltinApp(Expr{Kind: "name", Opcode: op}) {
			t.Fatalf("name classified as application: %s", op)
		}
	}
	for _, op := range []string{"", "unknown", "IADD", "set ", "User::iadd"} {
		if IsBuiltinOpcode(op) || IsBuiltinApp(Expr{Kind: "app", Opcode: op}) {
			t.Fatalf("accepted nonbuiltin %q", op)
		}
	}
	got[0] = "changed"
	if BuiltinOpcodes()[0] != want[0] {
		t.Fatal("caller changed shared inventory")
	}
}
