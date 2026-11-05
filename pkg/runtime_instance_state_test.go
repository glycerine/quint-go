package quint

import "testing"

// TestInstanceStateUpdateChecks pins how two instances of the same stateful
// module are typechecked. Both TypeScript and Go group updates by the bare
// variable name, so one action that updates A::x and B::x is rejected with
// QNT202 even though the two variables are separate. Independent per-instance
// action sequences typecheck cleanly.
func TestInstanceStateUpdateChecks(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		wantQNT202 bool
	}{
		{
			name:       "co-update-same-name",
			src:        "module P { var x: int action init = x' = 0 } module M { import P as A import P as B action both = all { A::init, B::x' = B::x } }",
			wantQNT202: true,
		},
		{
			name:       "separate-instance-runs",
			src:        "module P { var x: int action init = x' = 0 action inc = x' = x + 1 } module M { import P as A import P as B run aTest = A::init.then(A::inc).expect(A::x == 1) run bTest = B::init.then(B::inc).expect(B::x == 1) }",
			wantQNT202: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseModulesWithLocations("fixture.qnt", tc.src)
			if err != nil {
				t.Fatal(err)
			}
			linked, err := ResolveLinkedModules(parsed.Modules)
			if err != nil || len(linked.Errors) != 0 {
				t.Fatal(err, linked.Errors)
			}
			analysis, err := AnalyzeModules(parsed.Modules, linked.Table)
			if err != nil {
				t.Fatal(err)
			}
			got := false
			for _, d := range analysis.Diagnostics {
				if d.Code == QNT202 && d.Message == "Multiple updates of variable x" {
					got = true
				}
			}
			if got != tc.wantQNT202 {
				t.Fatalf("QNT202 = %v, want %v; diagnostics %+v", got, tc.wantQNT202, analysis.Diagnostics)
			}
		})
	}
}
