package quint

import "testing"

func TestEffectInferrerTraversal(t *testing.T) {
	for _, tc := range []struct {
		source, want string
		failed       bool
	}{
		{"module M { var x:int val a=x+1 }", "Read['x']", false},
		{"module M { var x:int action a=x'=x+1 }", "Read['x'] & Update['x']", false},
		{"module M { def identity(x)=x val a=identity(1) }", "Pure", false},
		{"module M { var x:int val a=def identity(y)=y; identity(x) }", "Read['x']", false},
		{"module M { var x:int temporal a=always(x>0) }", "Temporal['x']", false},
	} {
		ms, err := ParseModules(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		linked, err := ResolveLinkedModules(ms)
		if err != nil || len(linked.Errors) > 0 {
			t.Fatal(err, linked.Errors)
		}
		i := NewEffectInferrer(linked.Table)
		i.InferEffects(ms[0].Declarations)
		d := ms[0].Declarations[len(ms[0].Declarations)-1]
		s, e := i.Fetch(d.ID)
		if e != nil || EffectString(s.Effect) != tc.want {
			t.Fatalf("%s: %s %v", tc.source, EffectString(s.Effect), e)
		}
		if len(i.Substitutions) != 0 || len(i.Scopes.Frames) != 0 {
			t.Fatal("scope not cleared")
		}
	}
}
