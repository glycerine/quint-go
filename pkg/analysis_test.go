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
