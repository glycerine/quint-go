package quint

import (
	"encoding/json"
	"sync"
	"testing"
)

func BenchmarkBuiltinTypeSignature(b *testing.B) {
	for _, name := range []string{"iadd", "foldl", "setToMap"} {
		b.Run(name, func(b *testing.B) {
			if _, err := BuiltinTypeSignature(name, 2); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := BuiltinTypeSignature(name, 2); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Every mutable part of every fixed signature must belong to its caller.
func TestFixedBuiltinTypeCacheOwnership(t *testing.T) {
	t.Parallel()
	for name := range fixedBuiltinTypes {
		t.Run(name, func(t *testing.T) {
			first, err := BuiltinTypeSignature(name, 0)
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(first)
			if err != nil {
				t.Fatal(err)
			}
			mutateBuiltinTypeScheme(&first)
			second, err := BuiltinTypeSignature(name, 17)
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(second)
			if err != nil || string(before) != string(after) {
				t.Fatal("cached signature mutated", string(before), string(after), err)
			}
		})
	}
}

func TestBuiltinTypeSchemeCloneRowsAndQuantifiers(t *testing.T) {
	t.Parallel()
	original, err := parseBuiltinTypeScheme("(Set[List[a]], {x:b|r}, (a -> b)) => (a, {y:b|s})")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(original)
	clone, err := cloneBuiltinTypeScheme(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(clone.TypeVariables) != 2 || len(clone.RowVariables) != 2 {
		t.Fatal(clone)
	}
	mutateBuiltinTypeScheme(&clone)
	after, err := json.Marshal(original)
	if err != nil || string(before) != string(after) {
		t.Fatal("clone aliases input", err)
	}
}

func mutateBuiltinTypeScheme(s *TypeScheme) {
	var mutateType func(*Type)
	var mutateRow func(*Row)
	mutateType = func(typ *Type) {
		if typ == nil {
			return
		}
		mutateType(typ.Elem)
		mutateType(typ.Arg)
		mutateType(typ.Res)
		mutateType(typ.Ctor)
		for i := range typ.Args {
			mutateType(&typ.Args[i])
		}
		mutateRow(typ.Fields)
		typ.Kind = "changed"
		typ.Name = "changed"
	}
	mutateRow = func(row *Row) {
		if row == nil {
			return
		}
		for i := range row.Fields {
			mutateType(&row.Fields[i].Type)
			row.Fields[i].Name = "changed"
		}
		mutateRow(row.Other)
		row.Kind = "changed"
		row.Name = "changed"
	}
	mutateType(&s.Type)
	for i := range s.TypeVariables {
		s.TypeVariables[i] = "changed"
	}
	for i := range s.RowVariables {
		s.RowVariables[i] = "changed"
	}
}

func TestBuiltinTypeCacheConcurrentOwnership(t *testing.T) {
	t.Parallel()
	var workers sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for name := range fixedBuiltinTypes {
				s, err := BuiltinTypeSignature(name, 3)
				if err != nil {
					t.Error(err)
					return
				}
				if err = s.Type.Validate(); err != nil {
					t.Error(err)
					return
				}
				mutateBuiltinTypeScheme(&s)
			}
		}()
	}
	workers.Wait()
}

func TestBuiltinTypeCacheBoundsAndArityErrors(t *testing.T) {
	t.Parallel()
	if len(fixedBuiltinTypeCache) != len(fixedBuiltinTypes) {
		t.Fatal("cache key set differs from fixed inventory")
	}
	for _, name := range []string{"List", "Set", "Map", "tuples", "and", "matchVariant"} {
		for _, arity := range []int{1, 3, 5} {
			s, err := BuiltinTypeSignature(name, arity)
			if err != nil || len(s.Type.Args) != arity {
				t.Fatal(name, arity, s, err)
			}
		}
		if _, found := fixedBuiltinTypeCache[name]; found {
			t.Fatal("variable arity cached", name)
		}
	}
	if len(fixedBuiltinTypeCache) != len(fixedBuiltinTypes) {
		t.Fatal("cache grew")
	}
	for _, input := range []struct {
		name    string
		arity   int
		message string
	}{
		{"iadd", -1, "negative builtin arity"},
		{"List", -1, "negative builtin arity"},
		{"matchVariant", 0, "matchVariant requires a value and pattern/function pairs"},
		{"matchVariant", 2, "matchVariant requires a value and pattern/function pairs"},
		{"missing", 1, "no ordinary builtin signature for \"missing\""},
	} {
		if _, err := BuiltinTypeSignature(input.name, input.arity); err == nil || err.Error() != input.message {
			t.Fatal(input, err)
		}
	}
}
