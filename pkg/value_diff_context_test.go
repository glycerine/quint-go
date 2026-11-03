package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeValueDiffContextConcreteOracle(t *testing.T) {
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
			t.Parallel()
			old, err := NewRuntimeValue(fixture.Old)
			if err != nil {
				t.Fatal(err)
			}
			new, err := NewRuntimeValue(fixture.New)
			if err != nil {
				t.Fatal(err)
			}
			got, err := FormatRuntimeValueDiffContext(context.Background(), old, new, fixture.Width, ValueDiffOptions{CollapseThreshold: fixture.CollapseThreshold, FirstColumn: fixture.FirstColumn})
			if err != nil || got != fixture.Output {
				t.Fatalf("got %q (%v), want %q", got, err, fixture.Output)
			}
		})
	}
}

func valueDiffContextValue(t *testing.T, source string) RuntimeValue {
	t.Helper()
	expression, err := ParseExpression(source)
	if err != nil {
		t.Fatal(err)
	}
	value, err := EvaluatePureExpression(context.Background(), expression, nil)
	if err != nil {
		t.Fatal(source, err)
	}
	return value
}

func TestRuntimeValueDiffContextDomains(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		name, old, new, want string
		collapse             int
	}{
		{"interval_root", "1.to(3)", "1.to(3)", "Set(1, 2, 3)", 2},
		{"interval_changed", "1.to(3)", "2.to(4)", "Set(1, 2, 3, 4)", 2},
		{"interval_empty", "1.to(0)", "2.to(0)", "Set()", 2},
		{"interval_explicit", "1.to(3)", "Set(3,2,1)", "Set(1, 2, 3)", 0},
		{"interval_explicit_reverse", "Set(3,2,1)", "1.to(3)", "Set(3, 2, 1)", 0},
		{"product", "tuples(Set(2,1),Set(true,false))", "tuples(Set(2,1),Set(true,false))", "Set([2, true], [1, true], [2, false], [1, false])", 0},
		{"powerset", "Set(2,1).powerset()", "Set(2,1).powerset()", "Set(Set(), Set(2), Set(1), Set(2, 1))", 0},
		{"mapset", "Set(2,1).setOfMaps(Set(false,true))", "Set(2,1).setOfMaps(Set(false,true))", "Set(\n  Map(2 -> false, 1 -> false),\n  Map(2 -> true, 1 -> false),\n  Map(2 -> false, 1 -> true),\n  Map(2 -> true, 1 -> true)\n)", 0},
		{"record_nested_domain", "{domain:1.to(3),x:1}", "{domain:Set(3,2,1),x:2}", "{ domain: ..., x: 1 => 2 }", 2},
		{"map_nested_domain", "Map((1,1.to(3)),(2,3))", "Map((1,Set(3,2,1)),(3,4))", "Map(1 -> ..., 2 -> 3, 3 -> 4)", 2},
		{"list_nested_domain", "List(1.to(3),1)", "List(Set(3,2,1),2)", "[..., 1 => 2]", 2},
		{"variant_nested_domain", "variant(\"D\",1.to(3))", "variant(\"D\",Set(3,2,1))", "D(Set(1, 2, 3))", 2},
		{"mixed_set_fallback", "1.to(2)", "List(2,3)", "Set(1, 2, 3)", 2},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			old := valueDiffContextValue(t, fixture.old)
			new := valueDiffContextValue(t, fixture.new)
			oldView, newView := old.Expression(), new.Expression()
			got, err := FormatRuntimeValueDiffContext(context.Background(), old, new, 80, ValueDiffOptions{CollapseThreshold: fixture.collapse})
			if err != nil || got != fixture.want {
				t.Fatalf("got %q (%v), want %q", got, err, fixture.want)
			}
			if !reflect.DeepEqual(old.Expression(), oldView) || !reflect.DeepEqual(new.Expression(), newView) {
				t.Fatal("diff modified retained values")
			}
		})
	}
}

func TestRuntimeValueDiffContextCancellationAndErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	truth := valueDiffContextValue(t, "true")
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := FormatRuntimeValueDiffContext(canceled, truth, truth, 80); got != "" || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	for _, fixture := range []struct {
		old, new RuntimeValue
		width    int
		options  ValueDiffOptions
	}{
		{RuntimeValue{}, truth, 80, ValueDiffOptions{}},
		{truth, RuntimeValue{}, 80, ValueDiffOptions{}},
		{truth, truth, -1, ValueDiffOptions{}},
		{truth, truth, 80, ValueDiffOptions{FirstColumn: -1}},
	} {
		if got, err := FormatRuntimeValueDiffContext(ctx, fixture.old, fixture.new, fixture.width, fixture.options); got != "" || err == nil {
			t.Fatal("invalid diff accepted", got, err)
		}
	}
	for _, fixture := range []struct{ source, message string }{
		{"Int", "Infinite set Int is non-enumerable"},
		{"tuples(Set(),Int)", "Infinite set Int is non-enumerable"},
		{"Int.powerset()", "Either state is Left"},
		{"Set().setOfMaps(Int)", "Infinite set Int is non-enumerable"},
	} {
		value := valueDiffContextValue(t, fixture.source)
		got, err := FormatRuntimeValueDiffContext(ctx, value, value, 80)
		var thrown RuntimeDomainException
		if got != "" || !errors.As(err, &thrown) || thrown.Name != "Error" || thrown.Message != fixture.message {
			t.Fatal("non-enumerable value lost its checked error", fixture.source, got, err)
		}
	}
	// Distinct compound values can fail during extensional comparison before
	// either side is rendered. That error must also remain visible unchanged.
	oldProduct := valueDiffContextValue(t, "tuples(Int)")
	newProduct := valueDiffContextValue(t, "tuples(Int)")
	if got, err := FormatRuntimeValueDiffContext(ctx, oldProduct, newProduct, 80); got != "" || err == nil || err.Error() != "Infinite set Int is non-enumerable" {
		t.Fatal("checked equality error was swallowed", got, err)
	}
	closure := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return truth, nil })
	if got, err := FormatRuntimeValueDiffContext(ctx, closure, truth, 80); got != "" || err == nil || !strings.Contains(err.Error(), "closures") {
		t.Fatal("closure silently rendered", got, err)
	}
	// Traversing a huge interval must obey the context without constructing the
	// whole value or returning a prefix of the requested diff.
	huge := valueDiffContextValue(t, "0.to(1000000000000000000000000000000)")
	limited := &valueDiffCancellation{Context: ctx, remaining: 20}
	if got, err := FormatRuntimeValueDiffContext(limited, huge, huge, 80); got != "" || !errors.Is(err, context.Canceled) {
		t.Fatal("domain iteration ignored cancellation", got, err)
	}
	// Cancellation can occur after the document is built, including inside a
	// caller's decorator. A successful-looking partial display must not escape.
	decorating, stop := context.WithCancel(ctx)
	if got, err := FormatRuntimeValueDiffContext(decorating, truth, truth, 80, ValueDiffOptions{Decorate: func(_ ValueDiffRole, text string) string { stop(); return text }}); got != "" || !errors.Is(err, context.Canceled) {
		t.Fatal("final cancellation ignored", got, err)
	}
}

func TestRuntimeValueDiffContextCollapseWithoutEnumeration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	huge := valueDiffContextValue(t, "0.to(1000000000000000000000000000000)")
	label := valueDiffContextValue(t, "\"domain\"")
	x := valueDiffContextValue(t, "\"x\"")
	one := valueDiffContextValue(t, "1")
	two := valueDiffContextValue(t, "2")
	old, err := NewRuntimeCompositeContext(ctx, "Rec", []RuntimeValue{label, huge, x, one})
	if err != nil {
		t.Fatal(err)
	}
	new, err := NewRuntimeCompositeContext(ctx, "Rec", []RuntimeValue{label, huge, x, two})
	if err != nil {
		t.Fatal(err)
	}
	limited := &valueDiffCancellation{Context: ctx, remaining: 40}
	got, err := FormatRuntimeValueDiffContext(limited, old, new, 80, ValueDiffOptions{CollapseThreshold: 2})
	if err != nil || got != "{ domain: ..., x: 1 => 2 }" || limited.remaining == 0 {
		t.Fatal("unchanged compact domain was enumerated", got, err)
	}
}

type valueDiffCancellation struct {
	context.Context
	remaining int
}

func (c *valueDiffCancellation) Err() error {
	if c.remaining > 0 {
		c.remaining--
	}
	if c.remaining == 0 {
		return context.Canceled
	}
	return nil
}
