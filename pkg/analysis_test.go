package quint

import "testing"

func TestCombinedAnalysis(t *testing.T) {
	for _, tc := range []struct {
		source string
		code   string
	}{
		{"module M { var x:int action step=x'=x+1 }", ""},
		{"module M { var x:int pure val bad=x }", "QNT200"},
		{"module M { val bad=Set(1).oneOf() }", "QNT203"},
	} {
		ms, err := ParseModules(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		linked, _ := ResolveLinkedModules(ms)
		result, err := AnalyzeModules(ms, linked.Table)
		if err != nil {
			t.Fatal(err)
		}
		if result.Types.Len() == 0 || result.Effects.Len() == 0 {
			t.Fatal("missing analysis tables")
		}
		if tc.code == "" {
			if len(result.Diagnostics) != 0 {
				t.Fatal(result.Diagnostics)
			}
		} else {
			found := false
			for _, d := range result.Diagnostics {
				if d.Code == tc.code {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s: %+v", tc.code, result.Diagnostics)
			}
		}
	}
}

func TestAnalysisErrorTreesAndModuleOrder(t *testing.T) {
	ms, err := ParseModules("module A { val bad=1+true } module B { val later=2 }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	result, err := AnalyzeModules(ms, linked.Table)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Modules) != 2 || len(result.Diagnostics) != 2 {
		t.Fatalf("%+v", result.Diagnostics)
	}
	for _, d := range result.Diagnostics {
		if d.Code != "QNT000" || d.Data == nil || d.Data.Trace == nil || d.Message != d.Data.Trace.String() {
			t.Fatal(d)
		}
	}
	if *result.Diagnostics[0].Reference != *result.Diagnostics[1].Reference {
		t.Fatal("retained upstream error repetition changed")
	}
}

func TestIncrementalAnalysis(t *testing.T) {
	ms, err := ParseModules("module M { def identity(x)=x val a=identity(1) val b=identity(true) }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	first, err := AnalyzeModules([]Module{{Declarations: ms[0].Declarations[:1]}}, linked.Table)
	if err != nil {
		t.Fatal(err)
	}
	oldEffects := first.Effects.Len()
	previous := AnalysisOutput{Types: first.Types, Effects: first.Effects, Modes: first.Modes}
	next, err := AnalyzeIncremental(previous, linked.Table, ms[0].Declarations[1:])
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Diagnostics) != 0 {
		t.Fatal(next.Diagnostics)
	}
	if next.Types != first.Types || next.Modes != first.Modes || next.Effects == first.Effects {
		t.Fatal("retention semantics")
	}
	if first.Effects.Len() != oldEffects || next.Effects.Len() <= oldEffects {
		t.Fatal("effects dictionary mutated")
	}
	for _, d := range ms[0].Declarations[1:] {
		if _, ok := next.Types.Get2(d.ID.String()); !ok {
			t.Fatal("missing type")
		}
		if _, ok := next.Effects.Get2(d.ID.String()); !ok {
			t.Fatal("missing effect")
		}
	}
}
