package quint

import (
	"encoding/json"
	"sync"
	"testing"
)

func BenchmarkBuiltinEffectSignature(b *testing.B) {
	for _, name := range []string{"assign", "ite", "setBy"} {
		b.Run(name, func(b *testing.B) {
			if _, err := BuiltinEffectSignature(name, 3); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := BuiltinEffectSignature(name, 3); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestFixedBuiltinEffectCacheOwnership(t *testing.T) {
	t.Parallel()
	for name := range fixedEffectSignatures {
		t.Run(name, func(t *testing.T) {
			first, err := BuiltinEffectSignature(name, 0)
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(first)
			if err != nil {
				t.Fatal(err)
			}
			// Reparse only in this finite ownership test, not on every call.
			parsed, err := parseEffectScheme(fixedEffectSignatures[name])
			if err != nil {
				t.Fatal(err)
			}
			oracle, _ := json.Marshal(parsed)
			if string(before) != string(oracle) {
				t.Fatal("cached signature differs from parser", string(before), string(oracle))
			}
			assertBuiltinEffectTemplateHasNoOccurrences(t, first.Effect)
			mutateBuiltinEffectScheme(&first)
			second, err := BuiltinEffectSignature(name, 999)
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(second)
			if err != nil || string(before) != string(after) {
				t.Fatal("caller mutated cached signature", string(before), string(after), err)
			}
		})
	}
}

func assertBuiltinEffectTemplateHasNoOccurrences(t *testing.T, effect Effect) {
	t.Helper()
	var entity func(Entity)
	entity = func(value Entity) {
		if len(value.StateVariables) != 0 || value.Reference != nil {
			t.Fatal("fixed effect template acquired occurrence-sensitive state; cloning policy must be revisited", value)
		}
		for _, child := range value.Entities {
			entity(child)
		}
	}
	for _, component := range effect.Components {
		entity(component.Entity)
	}
	for _, parameter := range effect.Params {
		assertBuiltinEffectTemplateHasNoOccurrences(t, parameter)
	}
	if effect.Result != nil {
		assertBuiltinEffectTemplateHasNoOccurrences(t, *effect.Result)
	}
}

func mutateBuiltinEffectScheme(scheme *EffectScheme) {
	var entity func(*Entity)
	var effect func(*Effect)
	entity = func(value *Entity) {
		for i := range value.Entities {
			entity(&value.Entities[i])
		}
		if value.Reference != nil {
			*value.Reference = ID{}
		}
		for i := range value.StateVariables {
			value.StateVariables[i].Name = "changed"
			value.StateVariables[i].Reference = ID{}
		}
		value.Name, value.Kind = "changed", "changed"
	}
	effect = func(value *Effect) {
		for i := range value.Params {
			effect(&value.Params[i])
		}
		if value.Result != nil {
			effect(value.Result)
		}
		for i := range value.Components {
			entity(&value.Components[i].Entity)
			value.Components[i].Kind = "changed"
		}
		value.Name, value.Kind = "changed", "changed"
	}
	effect(&scheme.Effect)
	for i := range scheme.EffectVariables {
		scheme.EffectVariables[i] = "changed"
	}
	for i := range scheme.EntityVariables {
		scheme.EntityVariables[i] = "changed"
	}
}

func TestBuiltinEffectCacheConcurrentOwnership(t *testing.T) {
	t.Parallel()
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for name := range fixedEffectSignatures {
				scheme, err := BuiltinEffectSignature(name, 3)
				if err != nil {
					t.Error(err)
					return
				}
				mutateBuiltinEffectScheme(&scheme)
			}
		}()
	}
	workers.Wait()
}

func TestBuiltinEffectCacheBoundsAndErrors(t *testing.T) {
	t.Parallel()
	if len(fixedBuiltinEffectCache) != len(fixedEffectSignatures) {
		t.Fatal("cache differs from fixed inventory")
	}
	for _, name := range []string{"List", "Set", "Map", "actionAll", "actionAny", "matchVariant"} {
		if _, cached := fixedBuiltinEffectCache[name]; cached {
			t.Fatal("variable-arity signature cached", name)
		}
		for _, arity := range []int{0, 2, 5} {
			if _, err := BuiltinEffectSignature(name, arity); err != nil {
				t.Fatal(name, arity, err)
			}
		}
	}
	for _, test := range []struct {
		name  string
		arity int
		want  string
	}{
		{"assign", -1, "negative effect signature arity: -1"},
		{"missing", 2, `unknown builtin effect signature "missing"`},
	} {
		if _, err := BuiltinEffectSignature(test.name, test.arity); err == nil || err.Error() != test.want {
			t.Fatal(test, err)
		}
	}
	if len(fixedBuiltinEffectCache) != len(fixedEffectSignatures) {
		t.Fatal("cache grew")
	}
}
