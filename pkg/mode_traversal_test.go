package quint

import (
	"github.com/glycerine/insdict"
	"testing"
)

func TestModeTraversal(t *testing.T) {
	ms, err := ParseModules("module M { var x:int pure val bad=val local=x; local val good=1 }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	inf := NewEffectInferrer(linked.Table)
	inf.InferEffects(ms[0].Declarations)
	c := NewModeChecker()
	if err = c.CheckModes(ms[0].Declarations, inf.Effects); err != nil {
		t.Fatal(err)
	}
	d := ms[0].Declarations[1]
	if _, ok := c.Errors.Get2(d.ID.String()); !ok {
		t.Fatal("missing outer violation")
	}
	if c.Suggestions.Len() == 0 {
		t.Fatal("missing pure suggestion")
	}
	// Missing inference results are skipped.
	if err = NewModeChecker().CheckModes(ms[0].Declarations, insdict.NewDict[string, EffectScheme]()); err != nil {
		t.Fatal(err)
	}
}
func TestModeInstancePurity(t *testing.T) {
	ids := NewIDGenerator()
	impure, pure := ids.Next(), ids.Next()
	effects := insdict.NewDict[string, EffectScheme]()
	read, _ := ParseEffect("Read['x']")
	effects.Set(impure.String(), EffectScheme{Effect: read})
	effects.Set(pure.String(), EffectScheme{Effect: Effect{Kind: "concrete"}})
	d := Declaration{Kind: "instance", Overrides: []Override{
		{Parameter: Parameter{Name: "p"}, Expr: Expr{Kind: "name", ID: impure}},
		{Parameter: Parameter{Name: "q"}, Expr: Expr{Kind: "bool", ID: pure}},
	}}
	c := NewModeChecker()
	if err := c.CheckModes([]Declaration{d}, effects); err != nil {
		t.Fatal(err)
	}
	if c.Errors.Len() != 1 {
		t.Fatal(c.Errors)
	}
	got := c.Errors.Get(impure.String())
	if got.Code != "QNT201" || got.Message != "Instance overrides must be pure, but the value for p reads variables 'x'" || *got.Reference != impure {
		t.Fatal(got)
	}
}

func TestActionAsTemporalDiagnostic(t *testing.T) {
	ms, err := ParseModules("module M { var x:int action step=x'=x+1 action bad=step and step temporal good=step and step }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	inf := NewEffectInferrer(linked.Table)
	inf.InferEffects(ms[0].Declarations)
	c := NewModeChecker()
	if err = c.CheckModes(ms[0].Declarations, inf.Effects); err != nil {
		t.Fatal(err)
	}
	app := ms[0].Declarations[2].Expr
	d, ok := c.Errors.Get2(app.ID.String())
	want := "Using an action as an argument of `and` is only allowed in temporal definitions, but it is used in action `bad`. To combine actions, use `all { ... }` instead."
	if !ok || d.Message != want || c.Errors.Len() != 1 {
		t.Fatal(d, c.Errors)
	}
}
