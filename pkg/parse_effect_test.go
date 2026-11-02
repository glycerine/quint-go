package quint

import "testing"

func TestParseEffect(t *testing.T) {
	cases := [][2]string{{"Pure", "Pure"}, {"e", "e"}, {"() => Pure", "() => Pure"}, {"Read[]", "Read[]"}, {"Read['x', v, 'y', w]", "Read[v, w, 'x', 'y']"}, {"((Read['a']) => Update['b'], e) => Temporal['c']", "((Read['a']) => Update['b'], e) => Temporal['c']"}}
	for _, c := range cases {
		e, err := ParseEffect(c[0])
		if err != nil {
			t.Fatal(err)
		}
		if got := EffectString(e); got != c[1] {
			t.Fatalf("%q -> %q want %q", c[0], got, c[1])
		}
	}
	e, err := ParseEffect("(Read['x']) => Update['x', 'x']")
	if err != nil {
		t.Fatal(err)
	}
	first := EntityStateVariables(e.Params[0].Components[0].Entity)
	rest := EntityStateVariables(e.Result.Components[0].Entity)
	if first[0].Reference.String() != "1" || rest[0].Reference.String() != "2" || rest[1].Reference.String() != "3" {
		t.Fatal("state occurrence IDs out of order")
	}
	var checker MultipleUpdatesChecker
	errors := checker.Check([]EffectScheme{{Effect: e}})
	n := 0
	for range errors.All() {
		n++
	}
	if n != 2 {
		t.Fatalf("duplicate update diagnostics: %d", n)
	}
	for _, s := range []string{"", "Read[", "Read[@]", "Pure extra", "(Pure) =>", "Update['']"} {
		if _, err := ParseEffect(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
