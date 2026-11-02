package quint

import (
	"encoding/json"
	"testing"
)

func TestFlattenedDeclarationMetadataSurvivesNameResolution(t *testing.T) {
	modules, err := ParseModules("module M { pure val first:int=1 pure val second=first }")
	if err != nil {
		t.Fatal(err)
	}
	declaration := &modules[0].Declarations[0]
	hidden := true
	declaration.Hidden = &hidden
	declaration.Namespaces = []string{"Outer", "Inner"}
	declaration.Shadowing = json.RawMessage("true")
	star := "*"
	imported := Declaration{ID: NewIDGenerator().Next(), Kind: "import", ProtoName: "Original", DefName: &star}
	declaration.ImportedFrom, err = json.Marshal(imported)
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(modules)
	if err != nil {
		t.Fatal(err)
	}
	checkCollected := func(t *testing.T, definition LookupDefinition) {
		t.Helper()
		if !definition.Hidden || !definition.Shadowing || definition.ImportedFrom != nil || definition.Depth != 0 {
			t.Fatalf("collected metadata: %+v", definition)
		}
		flattenAliasEqual(t, "collected namespaces", definition.Namespaces, []string{"Outer", "Inner"})
		if definition.Declaration == nil || definition.Declaration.TypeAnnotation != nil || len(definition.Declaration.ImportedFrom) != 0 {
			t.Fatalf("collector should omit operator annotation and previous import: %+v", definition.Declaration)
		}
	}
	local := CollectLocalNames(modules)
	if len(local.Errors) != 0 {
		t.Fatal(local.Errors)
	}
	collected, ok := local.Modules.Get("M").Lookup("first")
	if !ok {
		t.Fatal("local declaration missing")
	}
	checkCollected(t, collected)
	linked, err := ResolveLinkedModules(modules)
	if err != nil || len(linked.Errors) != 0 {
		t.Fatal(err, linked.Errors)
	}
	collected, ok = linked.Modules.Get("M").Lookup("first")
	if !ok {
		t.Fatal("linked declaration missing")
	}
	checkCollected(t, collected)
	resolved, err := ResolveLocalModule(modules[0])
	if err != nil || len(resolved.Errors) != 0 {
		t.Fatal(err, resolved.Errors)
	}
	for _, table := range []*ExpressionNameResolver{resolved, {Table: linked.Table}} {
		// The definition's own table entry retains its declaration metadata;
		// a reference uses the new locally collected binding instead.
		own := table.Table.Get(declaration.ID)
		if !own.Hidden || !own.Shadowing || own.ImportedFrom == nil || own.ImportedFrom.ProtoName != "Original" || own.Declaration.TypeAnnotation == nil {
			t.Fatalf("own declaration metadata: %+v", own)
		}
		flattenAliasEqual(t, "own namespaces", own.Namespaces, declaration.Namespaces)
		checkCollected(t, table.Table.Get(modules[0].Declarations[1].Expr.ID))
		own.Namespaces[0] = "changed"
		if declaration.Namespaces[0] != "Outer" {
			t.Fatal("binding namespace aliases caller declaration")
		}
	}
	flattenAliasEqual(t, "resolution leaves caller IR unchanged", modules, json.RawMessage(before))
}

func TestFlattenedHiddenDeclarationsStayHiddenFromImporters(t *testing.T) {
	modules, err := ParseModules("module A { pure val hidden=1 pure val visible=2 } module B { import A.* pure val result=hidden+visible }")
	if err != nil {
		t.Fatal(err)
	}
	hidden := true
	modules[0].Declarations[0].Hidden = &hidden
	result, err := ResolveLinkedModules(modules)
	if err != nil || len(result.Errors) != 1 || result.Errors[0].Code != QNT404 || result.Errors[0].Message != "Name 'hidden' not found" {
		t.Fatalf("hidden flattened definition leaked through import: %v; %+v", err, result.Errors)
	}
	if _, ok := result.Modules.Get("B").Lookup("visible"); !ok {
		t.Fatal("visible definition was lost")
	}
}

func TestDeclarationLookupOptionalMetadata(t *testing.T) {
	modules, err := ParseModules("module M { pure val value=1 }")
	if err != nil {
		t.Fatal(err)
	}
	declaration := modules[0].Declarations[0]
	depth := 3
	declaration.Depth = &depth
	declaration.Shadowing = json.RawMessage("false")
	// Null and unknown import metadata do not create a provenance binding.
	for _, raw := range []json.RawMessage{nil, json.RawMessage("null"), json.RawMessage(`{"kind":"unknown"}`)} {
		declaration.ImportedFrom = raw
		definition := declarationLookup(declaration)
		if definition.Hidden || definition.Shadowing || definition.ImportedFrom != nil || definition.Depth != 3 {
			t.Fatalf("optional metadata: %+v", definition)
		}
	}
}
