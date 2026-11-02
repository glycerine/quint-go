package quint

import (
	"reflect"
	"testing"
)

func TestNondeterminismChecks(t *testing.T) {
	for _, tc := range []struct {
		source string
		codes  []string
	}{
		{"module M { val x=Set(1).oneOf() }", []string{QNT203}},
		{"module M { val x=apalache::generate(1) }", []string{QNT203}},
		{"module M { nondet x=Set(1).oneOf() }", []string{QNT206}},
		{"module M { action step=nondet x=Set(1).oneOf(); x>0 }", []string{}},
		{"module M { action step=nondet x=1; x }", []string{QNT204, QNT205}},
		{"module M { action step=nondet x=Set(1).oneOf(); x }", []string{QNT205}},
	} {
		ms, err := ParseModules(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		linked, _ := ResolveLinkedModules(ms)
		analysis, err := AnalyzeModuleTypes(ms, linked.Table)
		if err != nil {
			t.Fatal(err)
		}
		checker := NewNondetChecker(linked.Table)
		diagnostics, err := checker.Check(analysis.Types, ms[0].Declarations)
		if err != nil {
			t.Fatal(err)
		}
		codes := []string{}
		for _, d := range diagnostics {
			codes = append(codes, d.Code)
			if d.Reference == nil || d.Data == nil {
				t.Fatal(d)
			}
		}
		if !reflect.DeepEqual(codes, tc.codes) {
			t.Fatalf("%s: %+v", tc.source, diagnostics)
		}
	}
}
func TestNondetMissingTypesAndAccumulation(t *testing.T) {
	ms, err := ParseModules("module M { action step=nondet x=1; x val y=Set(1).oneOf() }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	checker := NewNondetChecker(linked.Table)
	ds, err := checker.Check(nil, ms[0].Declarations[:1])
	if err != nil || len(ds) != 1 || ds[0].Code != QNT204 {
		t.Fatal(ds, err)
	}
	ds, err = checker.Check(nil, ms[0].Declarations[1:])
	if err != nil || len(ds) != 2 || ds[1].Code != QNT203 {
		t.Fatal(ds, err)
	}
}
