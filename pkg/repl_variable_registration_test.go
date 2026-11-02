package quint

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestREPLInstanceVariableRegistrationRetainsCells(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "model.qnt")
	source := "module A { const c:int var x:int action init=x'=c action step=x'=x+1 } module M { import A(c=0) as I }"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	session := NewREPLSession()
	if err := session.Load(ctx, path, "M"); err != nil {
		t.Fatal(err)
	}
	if session.runtime.Storage.current.Len() != 0 {
		t.Fatal("load eagerly registered unused instance variables")
	}
	var firstKey string
	var firstCurrent, firstNext *RuntimeRegister
	for _, step := range []struct {
		input, result, current string
		shifted                bool
	}{
		{"I::init", "true", "0", true},
		{"I::x", "0", "0", false},
		{"I::step", "true", "1", true},
		{"I::x", "1", "1", false},
		{"I::step", "true", "2", true},
		{"I::x", "2", "2", false},
	} {
		result, err := session.Evaluate(ctx, step.input)
		if err != nil || result.Value.String() != step.result || result.Shifted != step.shifted {
			t.Fatalf("%s: result=%s shifted=%v err=%v", step.input, result.Value.String(), result.Shifted, err)
		}
		if len(result.Missing) != 0 {
			t.Fatalf("%s: unchanged instance variable produced missing warnings: %v", step.input, result.Missing)
		}
		storage := session.runtime.Storage
		if storage.current.Len() != 1 || storage.next.Len() != 1 {
			t.Fatalf("%s: recompiled instance added duplicate cells: current=%d next=%d", step.input, storage.current.Len(), storage.next.Len())
		}
		for key, current := range storage.current.All() {
			next := storage.next.Get(key)
			if firstCurrent == nil {
				firstKey, firstCurrent, firstNext = key, current, next
			}
			if key != firstKey || current != firstCurrent || next != firstNext {
				t.Fatalf("%s: recompiled instance replaced its retained cells", step.input)
			}
			if current.Name != "M::I::x" || current.Value.String() != step.current || next.Value.Valid() {
				t.Fatalf("%s: current=%s value=%s next=%s", step.input, current.Name, current.Value.String(), next.Value.String())
			}
		}
	}
}

func TestREPLImportedClosureRetainsOmittedVariableBinding(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "model.qnt")
	source := "module A { var x:int action init=x'=0 action step=x'=x+1 def f(n)=x+n } module M { import A as I }"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	session := NewREPLSession()
	if err := session.Load(ctx, path, "M"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Evaluate(ctx, "I::init"); err != nil {
		t.Fatal(err)
	}
	result, err := session.Evaluate(ctx, "I::f")
	if err != nil || result.Value.expression.Kind != "lambda" {
		t.Fatal("imported function", result.Value.String(), err)
	}
	closure := result.Value
	zero, err := NewRuntimeValue(Expr{Kind: "int"})
	if err != nil {
		t.Fatal(err)
	}
	var retained *RuntimeRegister
	for _, register := range session.runtime.Storage.current.All() {
		retained = register
	}
	for _, step := range []struct {
		input, want string
	}{
		{"1+1", "0"},
		{"I::step", "1"},
		{"2+2", "1"},
		{"I::step", "2"},
	} {
		result, err := session.Evaluate(ctx, step.input)
		if err != nil || len(result.Missing) != 0 {
			t.Fatal(step.input, result.Missing, err)
		}
		value, err := closure.Apply(ctx, []RuntimeValue{zero})
		if err != nil || value.String() != step.want {
			t.Fatalf("saved closure after %s: value=%s want=%s err=%v", step.input, value.String(), step.want, err)
		}
		storage := session.runtime.Storage
		if storage.current.Len() != 1 || storage.next.Len() != 1 {
			t.Fatalf("%s: imported alias added duplicate variable cells", step.input)
		}
		for _, register := range storage.current.All() {
			if register != retained {
				t.Fatalf("%s: imported variable lost its original cell", step.input)
			}
		}
	}
}
