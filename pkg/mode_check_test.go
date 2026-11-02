package quint

import (
	"testing"
)

func TestModeDefinitionDiagnostics(t *testing.T) {
	id := NewIDGenerator().Next()
	for _, tc := range []struct {
		text        string
		q           Qualifier
		replacement string
	}{
		{"Read['x']", PureVal, "val"},
		{"Update['x']", Val, "action"},
		{"Temporal['x']", Action, "temporal"},
		{"(a) => a", PureVal, "pure def"},
	} {
		c := NewModeChecker()
		e, err := ParseEffect(tc.text)
		if err != nil {
			t.Fatal(err)
		}
		if err = c.CheckDefinition(id, "f", tc.q, EffectScheme{Effect: e}); err != nil {
			t.Fatal(err)
		}
		d, ok := c.Errors.Get2(id.String())
		if !ok || d.Code != "QNT200" || *d.Reference != id {
			t.Fatal(d)
		}
		data := d.Data
		if data.Fix == nil || data.Fix.Replacement != tc.replacement || data.Fix.Original != QualifierString(tc.q) {
			t.Fatal(data)
		}
	}
}
func TestModeSuggestionsAndAgreement(t *testing.T) {
	c := NewModeChecker()
	id := NewIDGenerator().Next()
	e, _ := ParseEffect("Pure")
	if err := c.CheckDefinition(id, "f", Def, EffectScheme{Effect: e}); err != nil {
		t.Fatal(err)
	}
	if c.Errors.Len() != 0 || c.Suggestions.Get(id.String()) != PureVal {
		t.Fatal(c)
	}
	e, _ = ParseEffect("Read['x']")
	c = NewModeChecker()
	if err := c.CheckDefinition(id, "f", Val, EffectScheme{Effect: e}); err != nil || c.Errors.Len() != 0 || c.Suggestions.Len() != 0 {
		t.Fatal(c, err)
	}
}
