package quint

import "testing"

func TestConcreteEffectModes(t *testing.T) {
	for _, tc := range []struct {
		text        string
		quantified  []string
		mode        Qualifier
		explanation string
	}{
		{"Pure", nil, PureVal, "doesn't read or update any state variable"},
		{"Read[r]", nil, PureVal, "doesn't read or update any state variable"},
		{"Read[r]", []string{"r"}, Val, "reads variables r"},
		{"Read['x']", nil, Val, "reads variables 'x'"},
		{"Read['x'] & Update['y']", nil, Action, "updates variables 'y'"},
		{"Update['y'] & Temporal['x']", nil, Temporal, "performs temporal operations over variables 'x'"},
		{"Temporal[t] & Update['x']", nil, Action, "updates variables 'x'"},
		{"Read[r, 'x']", nil, Val, "reads variables r, 'x'"},
		{"Read[] & Update[] & Temporal[]", nil, PureVal, "doesn't read or update any state variable"},
	} {
		e, err := ParseEffect(tc.text)
		if err != nil {
			t.Fatal(err)
		}
		mode, explanation := ConcreteEffectMode(EffectScheme{Effect: e, EntityVariables: tc.quantified})
		if mode != tc.mode || explanation != tc.explanation {
			t.Fatalf("%s: %s %s", tc.text, mode, explanation)
		}
	}
	mode, _ := ConcreteEffectMode(EffectScheme{Effect: Effect{Kind: "variable", Name: "e"}})
	if mode != PureVal {
		t.Fatal(mode)
	}
}
func TestModeGeneralizationOrder(t *testing.T) {
	order := []Qualifier{PureVal, Val, PureDef, Def, Nondet, Action, Temporal, Run}
	for a, x := range order {
		for b, y := range order {
			if ModeMoreGeneral(x, y) != (a > b) {
				t.Fatal(x, y)
			}
		}
	}
}

func TestArrowEffectModes(t *testing.T) {
	for _, tc := range []struct {
		text             string
		vars             []string
		annotation, want Qualifier
		explanation      string
	}{
		{"(Read[r]) => Read[r]", []string{"r"}, Def, PureDef, "has 1 parameter"},
		{"(Read[r]) => Read[r]", []string{"r"}, Val, Def, "has 1 parameter"},
		{"(Read[r]) => Read[r, 'x']", []string{"r"}, Def, Def, "reads variables 'x'"},
		{"(Pure) => Update['x']", nil, PureDef, Action, "updates variables 'x'"},
		{"(Update[u]) => Temporal[u]", []string{"u"}, Def, PureDef, "has 1 parameter"},
		{"((Read[r]) => Read[s]) => Read[s]", []string{"r", "s"}, Def, PureDef, "has 1 parameter"},
		{"(a,b) => a", nil, Def, PureDef, "has 2 parameters"},
		{"() => Pure", nil, Def, PureDef, "has 0 parameter"},
	} {
		e, err := ParseEffect(tc.text)
		if err != nil {
			t.Fatal(err)
		}
		m, x, err := ModeForEffect(EffectScheme{Effect: e, EntityVariables: tc.vars}, tc.annotation)
		if err != nil || m != tc.want || x != tc.explanation {
			t.Fatalf("%s: %s %s %v", tc.text, m, x, err)
		}
	}
	e, _ := ParseEffect("(a) => (b) => b")
	if _, _, err := ModeForEffect(EffectScheme{Effect: e}, Def); err == nil {
		t.Fatal("operator result accepted")
	}
}
