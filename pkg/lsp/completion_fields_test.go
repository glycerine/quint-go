package lsp

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

func TestUpstreamCompletionFieldsAndTypeFamilies(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Type     quint.Type
		Builtins []string
		Fields   []semanticCompletionItem
	}
	data, err := os.ReadFile("testdata/completion_fields.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		got := suggestedMemberBuiltins(fixture.Type)
		if len(got) != len(fixture.Builtins) || len(got) > 0 && !reflect.DeepEqual(got, fixture.Builtins) {
			t.Fatal(fixture.Type.Kind, got, fixture.Builtins)
		}
		if got := completionFieldsForType(fixture.Type); !reflect.DeepEqual(got, fixture.Fields) {
			t.Fatal(fixture.Type.Kind, got, fixture.Fields)
		}
	}
}

func fieldCompletionTest(t *testing.T, text string, files *insdict.Dict[string, string]) (*Snapshot, semanticCompletionList) {
	t.Helper()
	marker := strings.Index(text, "|")
	if marker < 0 {
		t.Fatal("missing cursor")
	}
	text = text[:marker] + text[marker+1:]
	if files == nil {
		files = insdict.NewDict[string, string]()
	}
	files.Set("/fields/main.qnt", text)
	snapshot, err := analyzeDocument(context.Background(), Document{URI: "file:///fields/main.qnt", Version: 8, Text: text}, files)
	if err != nil {
		t.Fatal(err)
	}
	file, _ := quint.NewSourceFile("main", text)
	cursor, err := file.PositionAt(marker)
	if err != nil {
		t.Fatal(err)
	}
	response, problem := semanticTestRequest(t, snapshot, "textDocument/completion", Position{Line: cursor.Line, Character: cursor.Col}, "")
	if problem != nil {
		t.Fatal(problem)
	}
	return snapshot, response.(semanticCompletionList)
}
func TestSemanticRecordTupleAndMemberCompletions(t *testing.T) {
	t.Parallel()
	for _, example := range []struct {
		text      string
		labels    []string
		fieldType string
	}{
		{`module M { pure val r = { z: 1, alpha: true } pure val use = r.| }`, []string{"fieldNames", "with", "z", "alpha"}, ""},
		{`module M { pure val r = { z: 1, alpha: true } pure val use = r.al|pha }`, []string{"alpha"}, "bool"},
		{`module M { pure val r = (1,true) pure val use = r.| }`, []string{"_1", "_2"}, ""},
		{`module M { pure val r = (1,true) pure val use = r._2| }`, []string{"_2"}, "bool"},
		{`module M { pure val r = List(1,2) pure val use = r.he|ad() }`, []string{"head"}, ""},
		{`module M { pure val r = true pure val use = r.| }`, []string{"iff", "implies"}, ""},
		{`module M { pure val r = 1 pure val use = r.| }`, []string{"to"}, ""},
		{`module M { pure val r = "text" pure val use = r.| }`, []string{}, ""},
		{`module M { pure val r = { with:1 } pure val use = r.| }`, []string{"fieldNames", "with", "with"}, ""},
	} {
		_, got := fieldCompletionTest(t, example.text, nil)
		labels := []string{}
		for _, item := range got.Items {
			labels = append(labels, item.Label)
		}
		if !reflect.DeepEqual(labels, example.labels) {
			t.Fatal(example.text, labels, example.labels)
		}
		if example.fieldType != "" && (got.Items[0].Detail != example.fieldType || got.Items[0].Kind != 5) {
			t.Fatal(got)
		}
		if strings.Contains(example.text, "r.he|") && (got.Items[0].Documentation == nil || !strings.Contains(got.Items[0].Detail, "head")) {
			t.Fatal("builtin docs absent", got)
		}
	}
}
func TestSemanticCompletionUsesCurrentScopesAliasesAndImportBuffers(t *testing.T) {
	t.Parallel()
	_, got := fieldCompletionTest(t, `module M { pure val r = { outer:1 } pure def f(r: { inner: bool }): bool = r.in|ner }`, nil)
	if len(got.Items) != 1 || got.Items[0].Label != "inner" || got.Items[0].Detail != "bool" {
		t.Fatal(got)
	}
	_, got = fieldCompletionTest(t, `module M { type Config = { enabled: bool } pure def f(r: Config): Config = r.| }`, nil)
	if len(got.Items) != 3 || got.Items[2].Label != "enabled" {
		t.Fatal(got)
	}
	files := insdict.NewDict[string, string]()
	files.Set("/fields/lib.qnt", `module Lib { pure val r = { imported:1 } }`)
	snapshot, got := fieldCompletionTest(t, `module M { import Lib from "lib" pure val emoji = "😀" pure val use = Lib::r.im| }`, files)
	if len(snapshot.Diagnostics) == 0 {
		t.Fatal("incomplete source unexpectedly parsed")
	}
	if len(got.Items) != 1 || got.Items[0].Label != "imported" || got.Items[0].Detail != "int" {
		t.Fatal(got)
	}
	if snapshot.Version != 8 || !strings.Contains(snapshot.Text, "Lib::r.im") || snapshot.Sources.Get("/fields/main.qnt") != snapshot.Text {
		t.Fatal("completion mutated its original snapshot", snapshot)
	}
	snapshot, got = fieldCompletionTest(t, `module M { import Lib from "lib" pure val emoji = "😀" pure val use = Lib::r.| }`, files)
	if snapshot.Frontend.Table != nil || snapshot.Sources.Get("/fields/lib.qnt") != files.Get("/fields/lib.qnt") {
		t.Fatal("incomplete main did not retain current imported buffer")
	}
	if len(got.Items) != 3 || got.Items[2].Label != "imported" || got.Items[2].Detail != "int" {
		t.Fatal("incomplete main completion lost its import overlay", got)
	}
}
func TestSemanticFieldCompletionLexicalAndUnknownReceivers(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		`module M { pure val use = missing.| }`,
		`module M { pure def f(r) = r.| }`,
	} {
		_, got := fieldCompletionTest(t, text, nil)
		if len(got.Items) != 0 {
			t.Fatal(text, got)
		}
	}
	for _, text := range []string{`module M { pure val r = "r.|" }`, `module M { pure val r=1 // r.|` + "\n}", `module M { pure val r=1 /* r.| */ }`} {
		marker := strings.Index(text, "|")
		clean := strings.Replace(text, "|", "", 1)
		file, _ := quint.NewSourceFile("main", clean)
		position, _ := file.PositionAt(marker)
		if _, handled := completionFieldContext(&Snapshot{URI: "untitled:lexical", Text: clean}, Position{Line: position.Line, Character: position.Col}); handled {
			t.Fatal("field completion inside non-code", text)
		}
	}
	if _, handled := completionFieldContext(&Snapshot{URI: "untitled:bad", Text: "r."}, Position{Character: 999}); handled {
		t.Fatal("invalid cursor accepted")
	}
}
