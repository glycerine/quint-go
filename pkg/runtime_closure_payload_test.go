package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamClosurePayloads(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Source string
		Input  Expr
		Output *Expr
		Error  *Diagnostic
	}
	data, err := os.ReadFile("testdata/parser/runtime_closure_payload.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Source, func(t *testing.T) {
			got, err := EvaluatePureExpression(context.Background(), fixture.Input, nil)
			if fixture.Error != nil {
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) || diagnostic.Code != fixture.Error.Code || diagnostic.Message != fixture.Error.Message {
					t.Fatal(err, fixture.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Expression(), *fixture.Output) {
				actual, _ := json.Marshal(got.Expression())
				expected, _ := json.Marshal(*fixture.Output)
				t.Fatalf("got %s\nwant %s", actual, expected)
			}
		})
	}
}

func TestClosurePayloadObjectIdentityAndHashing(t *testing.T) {
	ctx := context.Background()
	n, _ := ParseInteger("7")
	scalar, _ := NewRuntimeValue(Expr{Kind: "int", Int: n})
	label, _ := NewRuntimeValue(Expr{Kind: "str", Str: "F"})
	closure := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) { return args[0], nil })
	fresh := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) { return args[0], nil })
	composite := func(op string, args ...RuntimeValue) RuntimeValue {
		t.Helper()
		value, err := newRuntimeComposite(op, args)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	if closure.Equal(closure) || closure.Equal(fresh) || !runtimeCollectionEqual(closure, closure) || runtimeCollectionEqual(closure, fresh) {
		t.Fatal("closure comparison uses mock display")
	}
	for _, op := range []string{"List", "Tup", "Rec", "variant", "Set", "Map"} {
		args := []RuntimeValue{closure}
		switch op {
		case "Rec", "variant":
			args = []RuntimeValue{label, closure}
		case "Map":
			args = []RuntimeValue{composite("Tup", scalar, closure)}
		}
		value := composite(op, args...)
		if !value.Valid() || !value.containsClosure {
			t.Fatal(op, "lost executable payload")
		}
		normalized, err := runtimeNormalForm(value)
		if err != nil || !normalized.containsClosure {
			t.Fatal(op, "normal form", err)
		}
		if op == "variant" {
			if value.Equal(normalized) {
				t.Fatal("variant payload.equals must not use object-identity shortcut")
			}
		} else if !value.Equal(normalized) || value.Hash() != normalized.Hash() {
			t.Fatal(op, "normalization changed collection equality/hash")
		}
		if _, err = runtimeReferenceHash(value); err != nil {
			t.Fatal(op, err)
		}
		var child RuntimeValue
		switch op {
		case "List", "Tup":
			children, _ := runtimeChildren(value)
			child = children[0]
		case "Rec", "variant":
			children, _ := runtimeChildren(value)
			child = children[1]
		case "Set", "Map":
			entries, _ := runtimeEntries(value)
			child = entries[0].Value
		}
		if !sameRuntimeObject(child, closure) {
			t.Fatal(op, "rebuilt executable child")
		}
		applied, err := child.Apply(ctx, []RuntimeValue{scalar})
		if err != nil || !applied.Equal(scalar) {
			t.Fatal(op, "callable child", err)
		}
		exported := value.Expression()
		mutateRuntimeClosureDisplay(&exported)
		applied, err = child.Apply(ctx, []RuntimeValue{scalar})
		if err != nil || !applied.Equal(scalar) {
			t.Fatal(op, "exported display mutation changed closure", err)
		}
	}
	if hash, err := runtimeReferenceHash(closure); err != nil || hash != 0 {
		t.Fatal("lambda reference hash", hash, err)
	}
	ordered, err := runtimeReferenceOrder([]RuntimeValue{closure, fresh, closure})
	if err != nil || len(ordered) != 2 || !sameRuntimeObject(ordered[0], closure) || !sameRuntimeObject(ordered[1], fresh) {
		t.Fatal("closure constructor order/dedup", err)
	}
	record := composite("Rec", label, closure, label, scalar)
	if record.containsClosure {
		t.Fatal("overwritten closure remained in record identity")
	}
	if _, err := NewRuntimeValue(closure.Expression()); err == nil {
		t.Fatal("mock lambda accepted as executable concrete input")
	}
}

func mutateRuntimeClosureDisplay(value *Expr) {
	if value.Kind == "lambda" {
		value.Params[0].Name = "modified"
		value.Body.Str = "modified"
	}
	for i := range value.Args {
		mutateRuntimeClosureDisplay(&value.Args[i])
	}
}

func TestClosureReferenceTreesPreserveKeyIdentity(t *testing.T) {
	value, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	closures := make([]RuntimeValue, 12)
	var entries []runtimeReferenceEntry
	for i := range closures {
		closures[i] = newRuntimeClosure(1, nil)
		entries = append(entries, runtimeReferenceEntry{closures[i], value})
	}
	persistent := runtimeReferenceMap{}
	for _, entry := range entries {
		var err error
		persistent, err = persistent.Set(entry.Key, entry.Value)
		if err != nil {
			t.Fatal(err)
		}
	}
	transient, err := runtimeReferenceMapFromEntries(entries)
	if err != nil {
		t.Fatal(err)
	}
	for _, tree := range []runtimeReferenceMap{persistent, transient} {
		for _, key := range closures {
			found, ok, err := tree.Lookup(key)
			if err != nil || !ok || !sameRuntimeObject(found, value) {
				t.Fatal("closure key lookup", err)
			}
		}
		if tree.Len() != len(closures) {
			t.Fatal("distinct lambdas deduplicated by mock expression")
		}
		noChange, err := tree.Set(closures[0], value)
		if err != nil || noChange.root != tree.root {
			t.Fatal("same-object update", err)
		}
		removed, err := tree.Delete(closures[0])
		if err != nil || removed.Len() != len(closures)-1 || tree.Len() != len(closures) {
			t.Fatal("persistent deletion", err)
		}
		if _, ok, err := removed.Lookup(closures[0]); err != nil || ok {
			t.Fatal("deleted closure key found", err)
		}
	}
}
