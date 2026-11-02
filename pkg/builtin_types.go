package quint

import (
	"fmt"
	"sync"
)

// Fixed builtin signatures mirror the reference TypeScript table.
var fixedBuiltinTypes = map[string]string{
	"Nat":                "Set[int]",
	"Int":                "Set[int]",
	"Bool":               "Set[bool]",
	"eq":                 "(a, a) => bool",
	"neq":                "(a, a) => bool",
	"iff":                "(bool, bool) => bool",
	"implies":            "(bool, bool) => bool",
	"not":                "(bool) => bool",
	"exists":             "(Set[a], (a) => bool) => bool",
	"forall":             "(Set[a], (a) => bool) => bool",
	"in":                 "(a, Set[a]) => bool",
	"contains":           "(Set[a], a) => bool",
	"union":              "(Set[a], Set[a]) => Set[a]",
	"intersect":          "(Set[a], Set[a]) => Set[a]",
	"exclude":            "(Set[a], Set[a]) => Set[a]",
	"subseteq":           "(Set[a], Set[a]) => bool",
	"filter":             "(Set[a], (a) => bool) => Set[a]",
	"map":                "(Set[a], (a) => b) => Set[b]",
	"fold":               "(Set[a], b, (b, a) => b) => b",
	"powerset":           "(Set[a]) => Set[Set[a]]",
	"flatten":            "(Set[Set[a]]) => Set[a]",
	"allLists":           "(Set[a]) => Set[List[a]]",
	"allListsUpTo":       "(Set[a], int) => Set[List[a]]",
	"getOnlyElement":     "(Set[a]) => a",
	"chooseSome":         "(Set[a]) => a",
	"oneOf":              "(Set[a]) => a",
	"isFinite":           "(Set[a]) => bool",
	"size":               "(Set[a]) => int",
	"get":                "(a -> b, a) => b",
	"keys":               "(a -> b) => Set[a]",
	"mapBy":              "(Set[a], (a) => b) => a -> b",
	"setToMap":           "(Set[(a, b)]) => (a -> b)",
	"setOfMaps":          "(Set[a], Set[b]) => Set[a -> b]",
	"set":                "(a -> b, a, b) => a -> b",
	"setBy":              "(a -> b, a, (b) => b) => a -> b",
	"put":                "(a -> b, a, b) => a -> b",
	"append":             "(List[a], a) => List[a]",
	"concat":             "(List[a], List[a]) => List[a]",
	"head":               "(List[a]) => a",
	"tail":               "(List[a]) => List[a]",
	"length":             "(List[a]) => int",
	"nth":                "(List[a], int) => a",
	"indices":            "(List[a]) => Set[int]",
	"replaceAt":          "(List[a], int, a) => List[a]",
	"slice":              "(List[a], int, int) => List[a]",
	"range":              "(int, int) => List[int]",
	"select":             "(List[a], (a) => bool) => List[a]",
	"foldl":              "(List[a], b, (b, a) => b) => b",
	"foldr":              "(List[a], b, (a, b) => b) => b",
	"iadd":               "(int, int) => int",
	"isub":               "(int, int) => int",
	"imul":               "(int, int) => int",
	"idiv":               "(int, int) => int",
	"imod":               "(int, int) => int",
	"ipow":               "(int, int) => int",
	"ilt":                "(int, int) => bool",
	"igt":                "(int, int) => bool",
	"ilte":               "(int, int) => bool",
	"igte":               "(int, int) => bool",
	"to":                 "(int, int) => Set[int]",
	"iuminus":            "(int) => int",
	"always":             "(bool) => bool",
	"eventually":         "(bool) => bool",
	"next":               "(a) => a",
	"orKeep":             "(bool, a) => bool",
	"mustChange":         "(bool, a) => bool",
	"enabled":            "(bool) => bool",
	"weakFair":           "(bool, a) => bool",
	"strongFair":         "(bool, a) => bool",
	"leadsTo":            "(bool, bool) => bool",
	"assign":             "(a, a) => bool",
	"ite":                "(bool, a, a) => a",
	"then":               "(bool, bool) => bool",
	"expect":             "(bool, bool) => bool",
	"reps":               "(int, int => bool) => bool",
	"fail":               "(bool) => bool",
	"assert":             "(bool) => bool",
	"q::debug":           "(str, a) => a",
	"q::lastTrace":       "List[a]",
	"q::test":            "(int, int, int, bool, bool, bool) => bool",
	"q::testOnce":        "(int, int, bool, bool, bool) => bool",
	"apalache::generate": "(int) => a",
}

// Parsed fixed signatures are immutable cache entries. The finite key set comes
// only from the builtin table; caller-selected variable arities are never cached.
// Each result is copied before it leaves BuiltinTypeSignature.
type builtinTypeCacheEntry struct {
	once   sync.Once
	scheme TypeScheme
	err    error
}

var fixedBuiltinTypeCache = func() map[string]*builtinTypeCacheEntry {
	cache := make(map[string]*builtinTypeCacheEntry, len(fixedBuiltinTypes))
	for name := range fixedBuiltinTypes {
		cache[name] = &builtinTypeCacheEntry{}
	}
	return cache
}()

func cloneBuiltinTypeScheme(scheme TypeScheme) (TypeScheme, error) {
	typ, err := ApplySubstitution(nil, scheme.Type)
	if err != nil {
		return TypeScheme{}, err
	}
	scheme.Type = typ
	scheme.TypeVariables = append([]string{}, scheme.TypeVariables...)
	scheme.RowVariables = append([]string{}, scheme.RowVariables...)
	return scheme, nil
}

// BuiltinTypeSignature returns the quantified signature for a builtin. Product
// and variant operations with special constraints are intentionally absent.
func BuiltinTypeSignature(name string, arity int) (TypeScheme, error) {
	if arity < 0 {
		return TypeScheme{}, fmt.Errorf("negative builtin arity")
	}
	if cached, found := fixedBuiltinTypeCache[name]; found {
		cached.once.Do(func() { cached.scheme, cached.err = parseBuiltinTypeScheme(fixedBuiltinTypes[name]) })
		if cached.err != nil {
			return TypeScheme{}, cached.err
		}
		return cloneBuiltinTypeScheme(cached.scheme)
	}
	return dynamicBuiltinTypeScheme(name, arity)
}

func parseBuiltinTypeScheme(text string) (TypeScheme, error) {
	typ, err := ParseType(text)
	if err != nil {
		return TypeScheme{}, err
	}
	types, rows, err := TypeNames(&typ)
	if err != nil {
		return TypeScheme{}, err
	}
	return TypeScheme{Type: typ, TypeVariables: types, RowVariables: rows}, nil
}
