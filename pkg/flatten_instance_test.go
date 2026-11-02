package quint

import "testing"

func TestInstantiateModule(t *testing.T) {
	ids := NewIDGenerator()
	ms, err := parseModulesWithIDs("module A { const c:int var x:int val a=x+c } module B { val outside=2 import A(c=outside) as I }", ids)
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	analysis, err := AnalyzeModules(ms, linked.Table)
	if err != nil {
		t.Fatal(err)
	}
	instance := ms[1].Declarations[1]
	out, imp, err := InstantiateModule("B", instance, ms[0], linked.Table, ids, nil, AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes})
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "B::I" || imp.Kind != "import" || imp.ProtoName != out.Name || *imp.DefName != "*" {
		t.Fatal(out, imp)
	}
	if len(out.Declarations) != 4 {
		t.Fatal(out)
	}
	if out.Declarations[0].Name != "outside" || out.Declarations[1].Name != "B::I::c" || out.Declarations[1].Qualifier != PureVal {
		t.Fatal(out)
	}
	if out.Declarations[2].ID == ms[0].Declarations[1].ID || out.Declarations[2].Name != "B::I::x" {
		t.Fatal("prototype not refreshed")
	}
	if ms[0].Declarations[1].Name != "x" {
		t.Fatal("prototype changed")
	}
}
