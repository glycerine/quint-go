package quint

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestUpstreamRuntimeValueDiff(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name              string
		Old, New          Expr
		Width             int
		FirstColumn       int
		CollapseThreshold int
		Output            string
	}
	data, err := os.ReadFile("testdata/parser/value_diff.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			old, err := NewRuntimeValue(fixture.Old)
			if err != nil {
				t.Fatal(err)
			}
			new, err := NewRuntimeValue(fixture.New)
			if err != nil {
				t.Fatal(err)
			}
			beforeOld, beforeNew := old.Expression(), new.Expression()
			got, err := FormatRuntimeValueDiff(old, new, fixture.Width, ValueDiffOptions{CollapseThreshold: fixture.CollapseThreshold, FirstColumn: fixture.FirstColumn})
			if err != nil || got != fixture.Output {
				t.Fatalf("got %q (%v)\nwant %q", got, err, fixture.Output)
			}
			afterOld, _ := json.Marshal(old.Expression())
			expectedOld, _ := json.Marshal(beforeOld)
			afterNew, _ := json.Marshal(new.Expression())
			expectedNew, _ := json.Marshal(beforeNew)
			if string(afterOld) != string(expectedOld) || string(afterNew) != string(expectedNew) {
				t.Fatal("formatting mutated values")
			}
		})
	}
}

func TestValueDiffDecorationsAndWidth(t *testing.T) {
	t.Parallel()
	value := func(source string) RuntimeValue {
		t.Helper()
		expression, err := ParseExpression(source)
		if err != nil {
			t.Fatal(err)
		}
		v, err := NewRuntimeValue(expression)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	old := value("{ x: List(1,2), removed: true }")
	new := value("{ x: List(1,3), added: false }")
	var roles []ValueDiffRole
	decorate := func(role ValueDiffRole, text string) string {
		roles = append(roles, role)
		code := "90"
		if role == ValueDiffRemoved {
			code = "31"
		}
		if role == ValueDiffAdded {
			code = "32"
		}
		return "\x1b[" + code + "m" + text + "\x1b[39m"
	}
	strip := regexp.MustCompile("\x1b\\[[0-9;]*m")
	for _, width := range []int{80, 24, 10, 0} {
		plain, err := FormatRuntimeValueDiff(old, new, width)
		if err != nil {
			t.Fatal(err)
		}
		styled, err := FormatRuntimeValueDiff(old, new, width, ValueDiffOptions{Decorate: decorate})
		if err != nil {
			t.Fatal(err)
		}
		if got := strip.ReplaceAllString(styled, ""); got != plain {
			t.Fatalf("style affected width: got %q want %q", got, plain)
		}
		if !strings.Contains(styled, "\x1b[31mremoved\x1b[39m") || !strings.Contains(styled, "\x1b[32madded\x1b[39m") || !strings.Contains(styled, "\x1b[90mx\x1b[39m") {
			t.Fatal("missing semantic roles", styled)
		}
	}
	if len(roles) == 0 {
		t.Fatal("decorator not invoked")
	}
	roles = nil
	if _, err := FormatRuntimeValueDiff(old, old, 80, ValueDiffOptions{Decorate: decorate}); err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		if role != ValueDiffUnchanged {
			t.Fatal("equal root was marked changed", role)
		}
	}
}

func TestValueDiffValidationAndClosureBoundary(t *testing.T) {
	t.Parallel()
	truth, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	for _, test := range []struct {
		old, new RuntimeValue
		width    int
		options  ValueDiffOptions
	}{
		{RuntimeValue{}, truth, 80, ValueDiffOptions{}},
		{truth, RuntimeValue{}, 80, ValueDiffOptions{}},
		{truth, truth, -1, ValueDiffOptions{}},
		{truth, truth, 80, ValueDiffOptions{FirstColumn: -1}},
	} {
		if _, err := FormatRuntimeValueDiff(test.old, test.new, test.width, test.options); err == nil {
			t.Fatal("invalid diff accepted")
		}
	}
	closure := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return truth, nil })
	if _, err := FormatRuntimeValueDiff(closure, truth, 80); err == nil {
		t.Fatal("closure representation silently printed")
	}
	if _, err := FormatRuntimeValueDiff(truth, closure, 80); err == nil {
		t.Fatal("closure representation silently printed")
	}
	list, err := newRuntimeComposite("List", []RuntimeValue{closure})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FormatRuntimeValueDiff(list, list, 80); err == nil {
		t.Fatal("nested closure representation silently printed")
	}
	// Closures can occur in every retained runtime collection. Each comparison
	// path must return the same explicit boundary, including added/removed
	// children and values that compare equal through collection object identity.
	composite := func(op string, children ...RuntimeValue) RuntimeValue {
		t.Helper()
		value, err := newRuntimeComposite(op, children)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	label, _ := NewRuntimeValue(Expr{Kind: "str", Str: "x"})
	otherLabel, _ := NewRuntimeValue(Expr{Kind: "str", Str: "y"})
	closureRecord := composite("Rec", label, closure)
	closureMap := composite("Map", composite("Tup", truth, closure))
	closureKeyMap := composite("Map", composite("Tup", closure, truth))
	closureSet := composite("Set", closure)
	for _, test := range []struct {
		name     string
		old, new RuntimeValue
	}{
		{"list_changed", list, composite("List", truth)},
		{"list_removed", list, composite("List")},
		{"list_added", composite("List"), list},
		{"record_equal", closureRecord, closureRecord},
		{"record_changed", closureRecord, composite("Rec", label, truth)},
		{"record_removed", closureRecord, composite("Rec")},
		{"record_added", composite("Rec"), closureRecord},
		{"map_equal", closureMap, closureMap},
		{"map_value_changed", closureMap, composite("Map", composite("Tup", truth, truth))},
		{"map_removed", closureMap, composite("Map")},
		{"map_added", composite("Map"), closureMap},
		{"map_key_equal", closureKeyMap, closureKeyMap},
		{"map_key_removed", closureKeyMap, composite("Map")},
		{"map_key_added", composite("Map"), closureKeyMap},
		{"variant_changed", composite("variant", label, closure), composite("variant", label, truth)},
		{"variant_label_old", composite("variant", label, closure), composite("variant", otherLabel, truth)},
		{"variant_label_new", composite("variant", label, truth), composite("variant", otherLabel, closure)},
		{"set_equal", closureSet, closureSet},
		{"set_removed", closureSet, composite("Set")},
		{"set_added", composite("Set"), closureSet},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := FormatRuntimeValueDiff(test.old, test.new, 80); err == nil || !strings.Contains(err.Error(), "lexical closures") {
				t.Fatalf("got %v, expected explicit closure error", err)
			}
		})
	}
}
