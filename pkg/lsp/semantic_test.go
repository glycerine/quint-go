package lsp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

func semanticTestSnapshot(t *testing.T, main string, files *insdict.Dict[string, string]) *Snapshot {
	t.Helper()
	text, ok := files.Get2(main)
	if !ok {
		t.Fatal("missing test main")
	}
	snapshot, err := analyzeDocument(context.Background(), Document{URI: uriForSource(main), Version: 1, Text: text}, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Diagnostics) != 0 {
		t.Fatal(snapshot.Diagnostics)
	}
	return snapshot
}
func semanticTestPosition(t *testing.T, text, needle string, occurrence int) Position {
	t.Helper()
	offset := 0
	for i := 0; i <= occurrence; i++ {
		found := strings.Index(text[offset:], needle)
		if found < 0 {
			t.Fatal("missing position", needle, occurrence)
		}
		offset += found
		if i < occurrence {
			offset += len(needle)
		}
	}
	file, _ := quint.NewSourceFile("test", text)
	position, err := file.PositionAt(offset)
	if err != nil {
		t.Fatal(err)
	}
	return Position{Line: position.Line, Character: position.Col}
}
func semanticTestRequest(t *testing.T, snapshot *Snapshot, method string, position Position, extra string) (any, *ResponseError) {
	t.Helper()
	uri, _ := json.Marshal(snapshot.URI)
	pos, _ := json.Marshal(position)
	params := json.RawMessage(`{"textDocument":{"uri":` + string(uri) + `},"position":` + string(pos) + extra + `}`)
	return semanticRequest(method, params, snapshot)
}
func semanticTestRangeText(t *testing.T, snapshot *Snapshot, location Location) string {
	t.Helper()
	source, _ := sourceForURI(location.URI)
	text, ok := semanticSource(snapshot, source)
	if !ok {
		t.Fatal("missing range source", location.URI)
	}
	file, _ := quint.NewSourceFile(source, text)
	a, err := file.ByteOffset(location.Range.Start.Line, location.Range.Start.Character)
	if err != nil {
		t.Fatal(err)
	}
	b, err := file.ByteOffset(location.Range.End.Line, location.Range.End.Character)
	if err != nil {
		t.Fatal(err)
	}
	return text[a:b]
}

func TestSemanticDefinitionsReferencesHoverAndUTF16(t *testing.T) {
	t.Parallel()
	text := `module M {
  /// The original documentation.
  pure val title = "😀" pure val original = 1
  pure def f(original: int): int = original + 1
  pure val use = original + f(original)
}`
	files := insdict.NewDict[string, string]()
	files.Set("/semantic/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
	position := semanticTestPosition(t, text, "original", 4)
	response, problem := semanticTestRequest(t, snapshot, "textDocument/definition", position, "")
	if problem != nil {
		t.Fatal(problem)
	}
	locations := response.([]Location)
	if len(locations) != 1 || semanticTestRangeText(t, snapshot, locations[0]) != "original" || locations[0].Range.Start.Line != 2 {
		t.Fatal(locations)
	}
	declarationPosition := semanticTestPosition(t, text, "original", 1)
	if locations[0].Range.Start != declarationPosition {
		t.Fatal("UTF-16 declaration position", locations[0].Range.Start, declarationPosition)
	}
	for _, include := range []bool{false, true} {
		extra := `,"context":{"includeDeclaration":false}`
		want := 2
		if include {
			extra = `,"context":{"includeDeclaration":true}`
			want++
		}
		response, problem = semanticTestRequest(t, snapshot, "textDocument/references", position, extra)
		if problem != nil || len(response.([]Location)) != want {
			t.Fatal(response, problem)
		}
		for _, location := range response.([]Location) {
			if semanticTestRangeText(t, snapshot, location) != "original" {
				t.Fatal(location)
			}
		}
	}
	// A parameter with identical spelling is a separate resolved binding.
	parameterPosition := semanticTestPosition(t, text, "original", 3)
	response, problem = semanticTestRequest(t, snapshot, "textDocument/references", parameterPosition, `,"context":{"includeDeclaration":true}`)
	if problem != nil || len(response.([]Location)) != 2 {
		t.Fatal(response, problem)
	}
	response, problem = semanticTestRequest(t, snapshot, "textDocument/hover", position, "")
	if problem != nil || !strings.Contains(response.(semanticHover).Contents.Value, "pure val original: int") || response.(semanticHover).Contents.Kind != "markdown" {
		t.Fatal(response, problem)
	}
	// The end of a selected token resolves to the same symbol.
	position.Character += len("original")
	response, problem = semanticTestRequest(t, snapshot, "textDocument/definition", position, "")
	if problem != nil || len(response.([]Location)) != 1 {
		t.Fatal(response, problem)
	}
}

func TestSemanticCrossFileTypesQualifiedRenameAndSymbols(t *testing.T) {
	t.Parallel()
	files := insdict.NewDict[string, string]()
	files.Set("/semantic/lib.qnt", `module Lib {
  /// Constant docs.
  pure val limit = 1
  type Count = int
}`)
	text := `module Main {
  import Lib from "lib"
  pure val b = Lib::limit
  pure def a(x: Lib::Count): Lib::Count = x + Lib::limit
}`
	files.Set("/semantic/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
	position := semanticTestPosition(t, text, "Lib::limit", 0)
	response, problem := semanticTestRequest(t, snapshot, "textDocument/definition", position, "")
	if problem != nil || len(response.([]Location)) != 1 || response.([]Location)[0].URI != uriForSource("/semantic/lib.qnt") || semanticTestRangeText(t, snapshot, response.([]Location)[0]) != "limit" {
		t.Fatal(response, problem)
	}
	response, problem = semanticTestRequest(t, snapshot, "textDocument/hover", position, "")
	if problem != nil || !strings.Contains(response.(semanticHover).Contents.Value, "Constant docs.") {
		t.Fatal(response, problem)
	}
	response, problem = semanticTestRequest(t, snapshot, "textDocument/prepareRename", position, "")
	if problem != nil || semanticTestRangeText(t, snapshot, Location{URI: snapshot.URI, Range: response.(Range)}) != "limit" {
		t.Fatal(response, problem)
	}
	response, problem = semanticTestRequest(t, snapshot, "textDocument/rename", position, `,"newName":"ceiling"`)
	if problem != nil {
		t.Fatal(problem)
	}
	var changes map[string][]semanticTextEdit
	if err := json.Unmarshal(response.(semanticWorkspaceEdit).Changes, &changes); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || len(changes[snapshot.URI]) != 2 || len(changes[uriForSource("/semantic/lib.qnt")]) != 1 {
		t.Fatal(changes)
	}
	for uri, edits := range changes {
		for _, edit := range edits {
			if edit.NewText != "ceiling" || semanticTestRangeText(t, snapshot, Location{URI: uri, Range: edit.Range}) != "limit" {
				t.Fatal(uri, edit)
			}
		}
	}
	typePosition := semanticTestPosition(t, text, "Lib::Count", 0)
	response, problem = semanticTestRequest(t, snapshot, "textDocument/references", typePosition, `,"context":{"includeDeclaration":true}`)
	if problem != nil || len(response.([]Location)) != 3 {
		t.Fatal(response, problem)
	}
	response, problem = semanticTestRequest(t, snapshot, "textDocument/documentSymbol", Position{}, "")
	if problem != nil {
		t.Fatal(problem)
	}
	symbols := response.([]semanticSymbol)
	// The upstream outline retains dependency order and qualifier-specific kinds.
	if len(symbols) != 1 || symbols[0].Name != "Main" || len(symbols[0].Children) != 2 || symbols[0].Children[0].Name != "a" || symbols[0].Children[0].Kind != 12 || symbols[0].Children[1].Name != "b" || symbols[0].Children[1].Kind != 8 {
		t.Fatal(symbols)
	}
	for _, symbol := range symbols[0].Children {
		if semanticTestRangeText(t, snapshot, Location{URI: snapshot.URI, Range: symbol.SelectionRange}) != symbol.Name {
			t.Fatal(symbol)
		}
	}
}

func TestSemanticCompletionScopesAndPrefix(t *testing.T) {
	t.Parallel()
	text := `module M {
  pure val global = 1
  pure def f(parameter: int): int = { val local = parameter; local + global }
  pure val other = global
}`
	files := insdict.NewDict[string, string]()
	files.Set("/semantic/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
	position := semanticTestPosition(t, text, "local +", 0)
	response, problem := semanticTestRequest(t, snapshot, "textDocument/completion", position, "")
	if problem != nil {
		t.Fatal(problem)
	}
	labels := insdict.NewDict[string, bool]()
	for _, item := range response.(semanticCompletionList).Items {
		labels.Set(item.Label, true)
	}
	for _, label := range []string{"local", "parameter", "global", "f", "Bool", "map", "getOnlyElement"} {
		if !labels.Get(label) {
			t.Fatal("completion missing", label, response)
		}
	}
	position = semanticTestPosition(t, text, "global\n", 0)
	position.Character += 2
	response, problem = semanticTestRequest(t, snapshot, "textDocument/completion", position, "")
	if problem != nil {
		t.Fatal(problem)
	}
	items := response.(semanticCompletionList).Items
	if len(items) != 1 || items[0].Label != "global" || items[0].Detail != "int" {
		t.Fatal(items)
	}
	position.Character -= 2
	response, _ = semanticTestRequest(t, snapshot, "textDocument/completion", position, "")
	for _, item := range response.(semanticCompletionList).Items {
		if item.Label == "local" || item.Label == "parameter" {
			t.Fatal("out-of-scope completion", item)
		}
	}
}

func TestSemanticRenameValidationAndMalformedRequests(t *testing.T) {
	t.Parallel()
	text := `module M { pure val first = 1 pure val second = first }`
	files := insdict.NewDict[string, string]()
	files.Set("/semantic/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
	position := semanticTestPosition(t, text, "first", 0)
	for _, name := range []string{"", "_", "a::b", "x y", "if", "true", "Set", "Bool", "second"} {
		encoded, _ := json.Marshal(name)
		if _, problem := semanticTestRequest(t, snapshot, "textDocument/rename", position, `,"newName":`+string(encoded)); problem == nil || problem.Code != -32602 {
			t.Fatal("rename accepted", name, problem)
		}
	}
	for _, params := range []string{`{`, `{}`, `{"textDocument":{"uri":"file:///other.qnt"}}`, `{"textDocument":{"uri":"file:///semantic/main.qnt"}}`, `{"textDocument":{"uri":"file:///semantic/main.qnt"},"position":{"line":0,"character":999}}`} {
		if _, problem := semanticRequest("textDocument/hover", json.RawMessage(params), snapshot); problem == nil {
			t.Fatal("malformed accepted", params)
		}
	}
	response, problem := semanticTestRequest(t, snapshot, "textDocument/references", Position{}, "")
	if problem != nil || len(response.([]Location)) != 0 {
		t.Fatal(response, problem)
	}
	response, problem = semanticTestRequest(t, snapshot, "textDocument/hover", Position{}, "")
	if problem != nil || response != nil {
		t.Fatal(response, problem)
	}
	if _, problem := semanticTestRequest(t, snapshot, "textDocument/rename", Position{}, `,"newName":"newName"`); problem == nil {
		t.Fatal("rename outside symbol accepted")
	}
}

func TestSemanticTokenizerSkipsCommentsStringsAndSurrogates(t *testing.T) {
	t.Parallel()
	tokens := semanticNameTokens(`"wrong" /* wrong */ // wrong
right::name + "\\\"wrong"`, 0, len(`"wrong" /* wrong */ // wrong
right::name + "\\\"wrong"`))
	if len(tokens) != 1 || tokens[0].text != "right::name" {
		t.Fatal(tokens)
	}
	text := `module M { pure val s = "😀" }`
	files := insdict.NewDict[string, string]()
	files.Set("/semantic/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
	position := semanticTestPosition(t, text, "😀", 0)
	position.Character++
	if _, problem := semanticTestRequest(t, snapshot, "textDocument/hover", position, ""); problem == nil {
		t.Fatal("split surrogate accepted")
	}
}

func TestSemanticRepeatedOperatorCallsHaveDistinctRanges(t *testing.T) {
	t.Parallel()
	text := `module M { pure def f(x: int): int = x pure val result = f(1).f() }`
	files := insdict.NewDict[string, string]()
	files.Set("/semantic/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
	for occurrence := 1; occurrence <= 2; occurrence++ {
		position := semanticTestPosition(t, text, "f(", occurrence)
		response, problem := semanticTestRequest(t, snapshot, "textDocument/references", position, `,"context":{"includeDeclaration":true}`)
		if problem != nil || len(response.([]Location)) != 3 {
			t.Fatal(response, problem)
		}
		for _, location := range response.([]Location) {
			if semanticTestRangeText(t, snapshot, location) != "f" {
				t.Fatal(location)
			}
		}
	}
}

func TestSemanticSelectiveImportRenameAndBuiltinHover(t *testing.T) {
	t.Parallel()
	text := `module Lib { pure val count = 1 }
module Main { import Lib.count pure val result = count + Set(2).getOnlyElement() }`
	files := insdict.NewDict[string, string]()
	files.Set("/semantic/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
	position := semanticTestPosition(t, text, "count +", 0)
	response, problem := semanticTestRequest(t, snapshot, "textDocument/rename", position, `,"newName":"renamed"`)
	if problem != nil {
		t.Fatal(problem)
	}
	var changes map[string][]semanticTextEdit
	if err := json.Unmarshal(response.(semanticWorkspaceEdit).Changes, &changes); err != nil || len(changes[snapshot.URI]) != 3 {
		t.Fatal(changes, err)
	}
	position = semanticTestPosition(t, text, "getOnlyElement", 0)
	response, problem = semanticTestRequest(t, snapshot, "textDocument/hover", position, "")
	if problem != nil || response == nil || !strings.Contains(response.(semanticHover).Contents.Value, "int") {
		t.Fatal(response, problem)
	}
	response, problem = semanticTestRequest(t, snapshot, "textDocument/prepareRename", position, "")
	if problem != nil || response != nil {
		t.Fatal(response, problem)
	}
	if _, problem := semanticTestRequest(t, snapshot, "textDocument/rename", position, `,"newName":"renamed"`); problem == nil {
		t.Fatal("builtin rename accepted")
	}
}

func semanticTestApplyRename(t *testing.T, snapshot *Snapshot, files *insdict.Dict[string, string], position Position, newName string) *insdict.Dict[string, string] {
	t.Helper()
	nameJSON, _ := json.Marshal(newName)
	response, problem := semanticTestRequest(t, snapshot, "textDocument/rename", position, `,"newName":`+string(nameJSON))
	if problem != nil {
		t.Fatal(problem)
	}
	var editsByURI map[string][]semanticTextEdit
	if err := json.Unmarshal(response.(semanticWorkspaceEdit).Changes, &editsByURI); err != nil {
		t.Fatal(err)
	}
	updated := insdict.NewDict[string, string]()
	for path, text := range files.All() {
		edits := editsByURI[uriForSource(path)]
		sort.Slice(edits, func(i, j int) bool { return semanticPositionLess(edits[j].Range.Start, edits[i].Range.Start) })
		changes := make([]contentChange, len(edits))
		for i, edit := range edits {
			r := edit.Range
			changes[i] = contentChange{Range: &r, Text: edit.NewText}
		}
		result, err := applyChanges(text, changes)
		if err != nil {
			t.Fatal(path, err)
		}
		updated.Set(path, result)
	}
	return updated
}

func TestSemanticSelectiveRenameRechecksImportsExportsAndAliases(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, bridge, main, trigger string
	}{
		{"direct", "", `module Main { import Lib.limit from "lib" pure val use = limit }`, "limit from"},
		{"wildcard", "", `module Main { import Lib.* from "lib" pure val use = limit }`, "limit }"},
		{"reexport", `module Bridge { import Lib from "lib" export Lib.limit }`, `module Main { import Bridge.limit from "bridge" pure val use = limit }`, "limit from"},
		{"module_alias", `module Bridge { import Lib as Alias from "lib" export Alias.limit }`, `module Main { import Bridge.limit from "bridge" pure val use = limit }`, "limit }"},
		{"qualified_member", `module Bridge { import Lib from "lib" export Lib as Public }`, `module Main { import Bridge.Public::limit from "bridge" pure val use = Public::limit }`, "Public::limit from"},
		{"type_alias", "", `module Main { import Lib.Count from "lib" pure def use(x: Count): Count = x }`, "Count from"},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := insdict.NewDict[string, string]()
			files.Set("/semantic/lib.qnt", `module Lib { pure val limit = 1 type Count = int }`)
			if test.bridge != "" {
				files.Set("/semantic/bridge.qnt", test.bridge)
			}
			files.Set("/semantic/main.qnt", test.main)
			snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
			position := semanticTestPosition(t, test.main, test.trigger, 0)
			response, problem := semanticTestRequest(t, snapshot, "textDocument/definition", position, "")
			if problem != nil || len(response.([]Location)) != 1 || response.([]Location)[0].URI != uriForSource("/semantic/lib.qnt") {
				t.Fatal(response, problem)
			}
			newName := "renamed"
			if test.name == "type_alias" {
				if _, problem := semanticTestRequest(t, snapshot, "textDocument/rename", position, `,"newName":"lowercase"`); problem == nil {
					t.Fatal("lowercase type rename accepted")
				}
				newName = "Renamed"
			}
			updated := semanticTestApplyRename(t, snapshot, files, position, newName)
			renamed := semanticTestSnapshot(t, "/semantic/main.qnt", updated)
			renamedPosition := semanticTestPosition(t, renamed.Text, newName, 0)
			response, problem = semanticTestRequest(t, renamed, "textDocument/definition", renamedPosition, "")
			if problem != nil || len(response.([]Location)) != 1 || semanticTestRangeText(t, renamed, response.([]Location)[0]) != newName {
				t.Fatal(response, problem)
			}
			if test.name == "qualified_member" && !strings.Contains(renamed.Text, "Bridge.Public::renamed") {
				t.Fatal("qualified prefix changed", renamed.Text)
			}
		})
	}
}

func TestSemanticSelectiveRenameDistinguishesPrototypeAndUnrelatedBindings(t *testing.T) {
	t.Parallel()
	text := `module Count { pure val Count = 1 }
module Other { pure val Count = 2 }
module Main {
  import Count.Count
  import Other as Elsewhere
  pure val use = Count + Elsewhere::Count
}`
	files := insdict.NewDict[string, string]()
	files.Set("/semantic/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
	position := semanticTestPosition(t, text, "Count.Count", 0)
	position.Character += len("Count.")
	updated := semanticTestApplyRename(t, snapshot, files, position, "renamed")
	renamed := semanticTestSnapshot(t, "/semantic/main.qnt", updated)
	if !strings.Contains(renamed.Text, "module Count { pure val renamed") || !strings.Contains(renamed.Text, "import Count.renamed") || !strings.Contains(renamed.Text, "renamed + Elsewhere::Count") || !strings.Contains(renamed.Text, "module Other { pure val Count = 2 }") {
		t.Fatal(renamed.Text)
	}
}

func TestSemanticRenameUpdatesInstanceConstantOverride(t *testing.T) {
	t.Parallel()
	text := `module Lib { const limit: int pure val value = limit }
module Main { import Lib(limit=1) as Inst pure val use = Inst::limit }`
	files := insdict.NewDict[string, string]()
	files.Set("/semantic/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
	position := semanticTestPosition(t, text, "limit:", 0)
	updated := semanticTestApplyRename(t, snapshot, files, position, "renamed")
	renamed := semanticTestSnapshot(t, "/semantic/main.qnt", updated)
	if !strings.Contains(renamed.Text, "Lib(renamed=1)") || !strings.Contains(renamed.Text, "Inst::renamed") {
		t.Fatal(renamed.Text)
	}
}

func TestSemanticSelectedReexportUsesOwnerBindingAfterAliasReuse(t *testing.T) {
	t.Parallel()
	text := `module A { pure val selected = 1 }
module B { pure val selected = 2 }
module BridgeA { import A as Alias export Alias.selected }
module BridgeB { import B as Alias export Alias.selected }
module Main { import BridgeA.selected import BridgeB as Other pure val use = selected + Other::selected }`
	files := insdict.NewDict[string, string]()
	files.Set("/semantic/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/semantic/main.qnt", files)
	position := semanticTestPosition(t, text, "BridgeA.selected", 0)
	position.Character += len("BridgeA.")
	updated := semanticTestApplyRename(t, snapshot, files, position, "renamed")
	renamed := semanticTestSnapshot(t, "/semantic/main.qnt", updated)
	if !strings.Contains(renamed.Text, "module A { pure val renamed") || !strings.Contains(renamed.Text, "import A as Alias export Alias.renamed") || !strings.Contains(renamed.Text, "import B as Alias export Alias.selected") || !strings.Contains(renamed.Text, "renamed + Other::selected") {
		t.Fatal(renamed.Text)
	}
}
