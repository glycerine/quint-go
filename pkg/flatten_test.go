package quint

import (
	"strings"
	"testing"

	"github.com/glycerine/insdict"
)

func TestFlattenModuleMissingExport(t *testing.T) {
	modules, err := ParseModules("module M { export Missing.* }")
	if err != nil {
		t.Fatal(err)
	}
	_, err = FlattenModule(modules[0], insdict.NewDict[string, Module](), NewExpressionNameResolver(nil).Table)
	if err == nil || !strings.Contains(err.Error(), "export module Missing not found") {
		t.Fatalf("unexpected export error: %v", err)
	}
}
