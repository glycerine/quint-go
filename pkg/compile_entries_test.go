package quint

import "testing"

func TestCompilationEntries(t *testing.T) {
	ids := NewIDGenerator()
	ds, err := SynthesizeCompilationEntries(CompilationEntries{Target: "json"}, ids)
	if err != nil || len(ds) != 0 {
		t.Fatal(ds, err)
	}
	ds, err = SynthesizeCompilationEntries(CompilationEntries{Target: "json", Flatten: true, Invariants: []string{"safe", "x>0"}, InductiveInvariant: "ind", Temporal: "live"}, ids)
	if err != nil || len(ds) != 5 {
		t.Fatal(ds, err)
	}
	for i, want := range []string{"q::init", "q::step", "q::inv", "q::inductiveInv", "q::temporalProps"} {
		if ds[i].Name != want {
			t.Fatal(ds[i])
		}
	}
	if ds[0].Expr.Name != "init" || ds[1].Expr.Name != "step" || ds[2].Expr.Opcode != "and" || len(ds[2].Expr.Args) != 1 {
		t.Fatal(ds)
	}
	custom := "start"
	next, err := SynthesizeCompilationEntries(CompilationEntries{Target: "json", Init: &custom}, ids)
	if err != nil || len(next) != 1 || next[0].Expr.Name != "start" || next[0].ID == ds[0].ID {
		t.Fatal(next, err)
	}
	bad := "("
	if _, err = SynthesizeCompilationEntries(CompilationEntries{Target: "json", Init: &bad}, ids); err == nil {
		t.Fatal("malformed expression accepted")
	}
}

func TestCompilationInvariantReportEntries(t *testing.T) {
	labels := []string{"x>0 and y>0", "x<2", "x<2"}
	declarations, err := SynthesizeCompilationEntries(CompilationEntries{Target: "json", Invariants: labels, InvariantReports: true}, NewIDGenerator())
	if err != nil || len(declarations) != 4 {
		t.Fatal(declarations, err)
	}
	for i := range labels {
		if declarations[i+1].Name != InvariantReportName(i) || declarations[i+1].Qualifier != Val {
			t.Fatal(declarations[i+1])
		}
	}
	if declarations[1].Expr.Opcode != "and" || declarations[2].Expr.Opcode != "ilt" || labels[0] != "x>0 and y>0" {
		t.Fatal("predicate boundaries or input changed", declarations)
	}
	empty, err := SynthesizeCompilationEntries(CompilationEntries{Target: "json", InvariantReports: true}, NewIDGenerator())
	if err != nil || len(empty) != 0 {
		t.Fatal(empty, err)
	}
}
