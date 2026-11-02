package quint

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// StandardEffectPropagation promotes argument updates into temporal references.
func StandardEffectPropagation(arity int) EffectScheme {
	return propagateEffectComponents([]string{"read", "temporal"}, arity, true, false)
}
func propagateEffectComponents(kinds []string, arity int, updates, lambda bool) EffectScheme {
	param := func(index int, updates bool) Effect {
		ks := append([]string{}, kinds...)
		if updates {
			ks = append(ks, "update")
		}
		e := Effect{Kind: "concrete", Components: []EffectComponent{}}
		for _, k := range ks {
			e.Components = append(e.Components, EffectComponent{Kind: k, Entity: Entity{Kind: "variable", Name: string(k[0]) + strconv.Itoa(index)}})
		}
		return e
	}
	result := Effect{Kind: "concrete", Components: []EffectComponent{}}
	for _, k := range kinds {
		entities := []Entity{}
		for i := 1; i <= arity; i++ {
			entities = append(entities, Entity{Kind: "variable", Name: string(k[0]) + strconv.Itoa(i)})
		}
		if k == "temporal" && updates {
			for i := 1; i <= arity; i++ {
				entities = append(entities, Entity{Kind: "variable", Name: "u" + strconv.Itoa(i)})
			}
		}
		result.Components = append(result.Components, EffectComponent{Kind: k, Entity: Entity{Kind: "union", Entities: entities}})
	}
	e := Effect{Kind: "arrow", Params: []Effect{}, Result: &result}
	n := arity
	if lambda {
		n--
	}
	for i := 1; i <= n; i++ {
		e.Params = append(e.Params, param(i, updates))
	}
	if lambda {
		body := param(arity, updates)
		l := Effect{Kind: "arrow", Params: []Effect{}, Result: &body}
		for i := 1; i <= n; i++ {
			l.Params = append(l.Params, param(i, false))
		}
		e.Params = append(e.Params, l)
	}
	es, vs := EffectNames(e)
	return EffectScheme{Effect: e, EffectVariables: es, EntityVariables: vs}
}
func parseEffectScheme(text string) (EffectScheme, error) {
	e, err := ParseEffect(text)
	if err != nil {
		return EffectScheme{}, err
	}
	es, vs := EffectNames(e)
	return EffectScheme{Effect: e, EffectVariables: es, EntityVariables: vs}, nil
}

type builtinEffectCacheEntry struct {
	once   sync.Once
	scheme EffectScheme
	err    error
}

// The fixed inventory bounds the cache. Signatures with caller-selected arity
// continue to be constructed on demand and never add cache entries.
var fixedBuiltinEffectCache = func() map[string]*builtinEffectCacheEntry {
	entries := make(map[string]*builtinEffectCacheEntry, len(fixedEffectSignatures))
	for name := range fixedEffectSignatures {
		entries[name] = &builtinEffectCacheEntry{}
	}
	return entries
}()

func cloneBuiltinEffectScheme(scheme EffectScheme) EffectScheme {
	// The fixed strings contain only entity variables and unions, with no
	// concrete state-variable occurrences. Cloning therefore preserves each
	// parse's occurrence semantics while owning every mutable slice and pointer.
	scheme.Effect = cloneEffectSubstitutionValue(scheme.Effect)
	scheme.EffectVariables = append([]string{}, scheme.EffectVariables...)
	scheme.EntityVariables = append([]string{}, scheme.EntityVariables...)
	return scheme
}

// BuiltinEffectSignature returns the upstream signature at the requested arity.
// Fixed signatures ignore arity; type checking diagnoses invalid calls.
func BuiltinEffectSignature(name string, arity int) (EffectScheme, error) {
	if arity < 0 {
		return EffectScheme{}, fmt.Errorf("negative effect signature arity: %d", arity)
	}
	if p, ok := fixedEffectPropagation[name]; ok {
		return propagateEffectComponents(p.kinds, p.arity, p.updates, p.lambda), nil
	}
	if cached, ok := fixedBuiltinEffectCache[name]; ok {
		cached.once.Do(func() { cached.scheme, cached.err = parseEffectScheme(fixedEffectSignatures[name]) })
		if cached.err != nil {
			return EffectScheme{}, cached.err
		}
		return cloneBuiltinEffectScheme(cached.scheme), nil
	}
	switch name {
	case "List", "Set", "Map", "Rec", "Tup", "tuples", "and", "or":
		return StandardEffectPropagation(arity), nil
	case "actionAll":
		// The second actionAll entry in upstream's Map overrides the first.
		return propagateEffectComponents([]string{"read", "temporal", "update"}, arity, false, false), nil
	case "matchVariant":
		args := []string{"a"}
		reads := []string{}
		count := arity / 2
		if arity == 0 {
			// Upstream lodash range(-0.5) yields [0]; type checking rejects
			// the call, but signature lookup itself preserves this behavior.
			count = 1
		}
		for i := 0; i < count; i++ {
			r := "r" + strconv.Itoa(i)
			reads = append(reads, r)
			args = append(args, "Pure", "(a) => Read["+r+"] & Update[u]")
		}
		return parseEffectScheme("(" + strings.Join(args, ", ") + ") => Read[" + strings.Join(reads, ",") + "] & Update[u]")
	case "actionAny":
		args, reads, temporals := []string{}, []string{}, []string{}
		for i := 0; i < arity; i++ {
			r, t := "r"+strconv.Itoa(i), "t"+strconv.Itoa(i)
			reads = append(reads, r)
			temporals = append(temporals, t)
			args = append(args, "Read["+r+"] & Temporal["+t+"] & Update[u]")
		}
		return parseEffectScheme("(" + strings.Join(args, ", ") + ") => Read[" + strings.Join(reads, ",") + "] & Temporal[" + strings.Join(temporals, ",") + "] & Update[u]")
	}
	return EffectScheme{}, fmt.Errorf("unknown builtin effect signature %q", name)
}

type effectPropagationSpec struct {
	kinds           []string
	arity           int
	updates, lambda bool
}
