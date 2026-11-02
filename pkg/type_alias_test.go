package quint

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTypeAliasExpansion(t *testing.T) {
	ms, err := ParseModules("module M { type Opaque type Base=int type Chain=Base type Box={x:Chain, y:List[Chain]} const value: (Box, Opaque) }")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := ResolveLinkedModules(ms)
	if len(r.Errors) != 0 {
		t.Fatal(r.Errors)
	}
	input := *ms[0].Declarations[4].TypeAnnotation
	before, _ := json.Marshal(ms)
	outer, err := ResolveTypeAlias(r.Table, input)
	if err != nil {
		t.Fatal(err)
	}
	if outer.Fields.Fields[0].Type.Kind != "const" {
		t.Fatal("outer resolver traversed children")
	}
	output, err := InlineTypeAliasesInType(r.Table, input)
	if err != nil {
		t.Fatal(err)
	}
	fields := output.Fields.Fields[0].Type.Fields.Fields
	if fields[0].Type.Kind != "int" || fields[1].Type.Elem.Kind != "int" {
		t.Fatal("nested aliases")
	}
	if output.Fields.Fields[1].Type.Kind != "const" || output.Fields.Fields[1].Type.Name != "Opaque" {
		t.Fatal("abstract type changed")
	}
	fields[0].Type.Kind = "str"
	after, _ := json.Marshal(ms)
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
}

func TestTypeAliasCyclesAndSharing(t *testing.T) {
	for _, source := range []string{
		"module M { type A=A const x:A }",
		"module M { type A=B type B=A const x:A }",
		"module M { type A=List[A] const x:A }",
	} {
		ms, err := ParseModules(source)
		if err != nil {
			t.Fatal(err)
		}
		r, _ := ResolveLinkedModules(ms)
		_, err = InlineTypeAliasesInType(r.Table, *ms[0].Declarations[len(ms[0].Declarations)-1].TypeAnnotation)
		if err == nil || !strings.Contains(err.Error(), "cyclic type alias") {
			t.Fatal(err)
		}
	}
	ms, _ := ParseModules("module M { type A=int const x:(A,A) }")
	r, _ := ResolveLinkedModules(ms)
	out, err := InlineTypeAliasesInType(r.Table, *ms[0].Declarations[1].TypeAnnotation)
	if err != nil {
		t.Fatal(err)
	}
	out.Fields.Fields[0].Type.Kind = "str"
	if out.Fields.Fields[1].Type.Kind != "int" {
		t.Fatal("shared alias expansion")
	}
}

func TestTypeAliasApplicationConstructor(t *testing.T) {
	ms, err := ParseModules("module M { type T=int type Box[a]=List[a] const x:Box[T] }")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := ResolveLinkedModules(ms)
	out, err := InlineTypeAliasesInType(r.Table, *ms[0].Declarations[2].TypeAnnotation)
	if err != nil {
		t.Fatal(err)
	}
	if out.Ctor.Kind != "const" || out.Ctor.Name != "Box" || out.Args[0].Kind != "int" {
		t.Fatal("constructor/argument handling")
	}
	unresolved, _ := ParseType("Missing")
	out, err = InlineTypeAliasesInType(r.Table, unresolved)
	if err != nil || out.Name != "Missing" {
		t.Fatal("unresolved type changed")
	}
}

func TestImportedAliasesAcrossTypeShapes(t *testing.T) {
	ms, err := ParseModules("module A { type T=int } module B { import A.* const f:(T,Set[T])=>T const g:T->T const r:{x:T|tail} }")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := ResolveLinkedModules(ms)
	if len(r.Errors) != 0 {
		t.Fatal(r.Errors)
	}
	for _, d := range ms[1].Declarations[1:] {
		out, err := InlineTypeAliasesInType(r.Table, *d.TypeAnnotation)
		if err != nil {
			t.Fatal(err)
		}
		err = WalkType(&out, func(typ *Type) (bool, error) {
			if typ.Kind == "const" {
				t.Fatalf("unexpanded %s", typ.Name)
			}
			return true, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	row, err := InlineTypeAliasesInType(r.Table, *ms[1].Declarations[3].TypeAnnotation)
	if err != nil || row.Fields.Other.Kind != "var" || row.Fields.Other.Name != "tail" {
		t.Fatal("row tail changed")
	}
}
