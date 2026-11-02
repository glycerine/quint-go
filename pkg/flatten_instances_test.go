package quint

import (
	"github.com/glycerine/insdict"
	"testing"
)

func TestFlattenInstances(t *testing.T) {
	ids := NewIDGenerator()
	ms, err := parseModulesWithIDs("module A { const c:int val a=c def f(x)=x+c } module B { import A(c=1) as I val a=I::a val b=I::f(2) }", ids)
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	modules := insdict.NewDict[string, Module]()
	for _, m := range ms {
		modules.Set(m.Name, m)
	}
	result, err := FlattenInstances(ms[1], modules, linked.Table, ids, nil, AnalysisOutput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result[0].Name != "B::I" || result[1].Declarations[0].Kind != "import" {
		t.Fatal(result)
	}
	owner := result[1]
	if owner.Declarations[1].Expr.Name != "B::I::a" || owner.Declarations[2].Expr.Opcode != "B::I::f" {
		t.Fatal(owner)
	}
	if ms[1].Declarations[1].Expr.Name != "I::a" {
		t.Fatal("input mutated")
	}
	relinked, err := ResolveLinkedModules(result)
	if err != nil || len(relinked.Errors) > 0 {
		t.Fatal(err, relinked.Errors)
	}
}
