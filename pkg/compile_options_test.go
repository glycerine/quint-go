package quint

import (
	"reflect"
	"testing"
)

func TestCompilationMainSelection(t *testing.T) {
	modules := []Module{{Name: "Explicit"}, {Name: "Default"}, {Name: "file"}}
	for _, tc := range []struct {
		explicit, source, input string
		want                    int
	}{
		{"Explicit", "Default", "file.qnt", 0},
		{"", "Default", "file.qnt", 1},
		{"", "", "/tmp/file.qnt", 2},
	} {
		got, err := SelectMainModule(modules, tc.explicit, tc.source, tc.input)
		if err != nil || got != tc.want {
			t.Fatal(got, err)
		}
	}
	if _, err := SelectMainModule(modules, "missing", "", ""); err == nil || err.Error() != "module missing does not exist" {
		t.Fatal(err)
	}
	if GuessMainModule("", "", "file.QNT") != "file.QNT" {
		t.Fatal("suffix comparison changed")
	}
}
func TestCompilationInvariantFlags(t *testing.T) {
	text, list := CompilationInvariants("legacy", []string{"a", "true", "b"})
	if text != "legacy and a and true and b" || !reflect.DeepEqual(list, []string{"legacy", "a", "true", "b"}) {
		t.Fatal(text, list)
	}
	text, list = CompilationInvariants("true", nil)
	if text != "true" || len(list) != 0 {
		t.Fatal(text, list)
	}
}
