package quint

import (
	"context"
	"testing"
)

// Ordinary import aliases retain their source IDs after flattening. Both names
// must survive dependency sorting and refer to the same compiled lambda object.
func TestModuleAliasesShareCompiledDefinition(t *testing.T) {
	t.Parallel()
	modules, err := ParseModules("module M { def f(x)=x+1 val first=f val second=f }")
	if err != nil {
		t.Fatal(err)
	}
	module := modules[0]
	alias := module.Declarations[0]
	alias.Name = "g"
	module.Declarations = []Declaration{module.Declarations[0], alias, module.Declarations[1], module.Declarations[2]}
	module.Declarations[3].Expr.Name = "g"
	evaluator, err := NewModuleEvaluator(module, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first, err := evaluator.Evaluate(ctx, "first", nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := evaluator.Evaluate(ctx, "second", nil)
	if err != nil || !sameRuntimeObject(first, second) {
		t.Fatal("aliases rebuilt the compiled lambda", err)
	}
	integer, _ := ParseInteger("1")
	one, _ := NewRuntimeValue(Expr{Kind: "int", Int: integer})
	for _, name := range []string{"f", "g"} {
		result, err := evaluator.Evaluate(ctx, name, []RuntimeValue{one})
		if err != nil || result.String() != "2" {
			t.Fatal(name, result.String(), err)
		}
	}
}
