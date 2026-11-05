package quint

import "testing"

// TestInstanceOverrideReadsOtherInstance pins a parity result: an instance
// override whose value reads another instance's constant (`import P(n = A::k + 1)`)
// is rejected with QNT099 (cyclic declarations) by TypeScript and Go, while the
// same override reading an importer definition is accepted.
func TestInstanceOverrideReadsOtherInstance(t *testing.T) {
	cases := []struct {
		name    string
		main    string
		wantErr bool
	}{
		{
			name:    "override-reads-other-instance",
			main:    "module M { import P(n = 1) as A from \"p\" import P(n = A::k + 1) as B from \"p\" run aTest = all { true }.expect(A::f(0) == 1) run bTest = all { true }.expect(B::f(0) == 6) }",
			wantErr: true,
		},
		{
			name:    "override-reads-importer-def",
			main:    "module M { pure val j = 5 import P(n = 1) as A from \"p\" import P(n = j + 1) as B from \"p\" run aTest = all { true }.expect(A::f(0) == 1) run bTest = all { true }.expect(B::f(0) == 6) }",
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &fixtureSourceResolver{sources: map[string]string{
				"main.qnt": tc.main,
				"p.qnt":    "module P { const n: int pure val k = 5 pure def f(x) = x + n }",
			}}
			result, err := ParseProject(r, r.LookupPath(".", "main.qnt"))
			if err != nil {
				t.Fatal(err)
			}
			got := false
			for _, d := range result.Diagnostics {
				if d.Code == QNT099 {
					got = true
				}
			}
			if got != tc.wantErr {
				t.Fatalf("QNT099 = %v, want %v; diagnostics %+v", got, tc.wantErr, result.Diagnostics)
			}
		})
	}
}
