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
