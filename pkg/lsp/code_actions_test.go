package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

func codeActionTestJSON(t *testing.T, value any) any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func codeActionTestParams(t *testing.T, uri string, span Range, diagnostics any) json.RawMessage {
	t.Helper()
	params, err := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri}, "range": span, "context": map[string]any{"diagnostics": diagnostics}})
	if err != nil {
		t.Fatal(err)
	}
	return params
}

func codeActionTestEdits(t *testing.T, action codeAction, uri string) []semanticTextEdit {
	t.Helper()
	var changes map[string][]semanticTextEdit
	if err := json.Unmarshal(action.Edit.Changes, &changes); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || len(changes[uri]) != 1 {
		t.Fatal(changes)
	}
	return changes[uri]
}

func TestUpstreamCodeActions(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Cases []struct {
			Name, URI, Text string
			Params, Actions json.RawMessage
		}
	}
	encoded, err := os.ReadFile("testdata/code_actions.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(encoded, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			snapshot := &Snapshot{URI: fixture.URI, Text: fixture.Text}
			response, problem := codeActionRequest(fixture.Params, snapshot)
			want := codeActionTestJSON(t, fixture.Actions)
			switch fixture.Name {
			case "request_match_not_diagnostic_line":
				// Upstream borrows another line's whitespace width and edits
				// outside the requested line. Ignore this unrelated diagnostic.
				want = []any{}
			case "first_match_width_diagnostic_start":
				// The earlier match has a different width. Replace the complete
				// original at the actual target instead of leaving its last byte.
				action := want.([]any)[0].(map[string]any)
				changes := action["edit"].(map[string]any)["changes"].(map[string]any)
				edit := changes[fixture.URI].([]any)[0].(map[string]any)
				edit["range"].(map[string]any)["end"].(map[string]any)["character"] = float64(23)
			}
			if problem != nil || !reflect.DeepEqual(codeActionTestJSON(t, response), want) {
				t.Fatalf("problem=%v\ngot=%s\nwant=%s", problem, mustMarshalCodeActionTest(t, response), fixture.Actions)
			}
			// Responses retain owned diagnostic JSON after the caller reuses
			// its request buffer, and never modify the document snapshot.
			for i := range fixture.Params {
				fixture.Params[i] = ' '
			}
			if !reflect.DeepEqual(codeActionTestJSON(t, response), want) || snapshot.Text != fixture.Text {
				t.Fatal("response retained request storage or changed the open buffer")
			}
		})
	}
}

func TestCodeActionsCorrectNaturalUpstreamQualifierRanges(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Cases []struct {
			Name, URI, Text, EditedText string
			Params, Actions, After      json.RawMessage
		}
	}
	encoded, err := os.ReadFile("testdata/code_actions_natural.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(encoded, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			if !strings.Contains(fixture.EditedText, "pure val val") || !bytes.Contains(fixture.After, []byte(`"QNT000"`)) {
				t.Fatal("missing actual upstream corruption evidence", fixture.EditedText)
			}
			snapshot := &Snapshot{URI: fixture.URI, Text: fixture.Text}
			response, problem := codeActionRequest(fixture.Params, snapshot)
			if problem != nil || len(response.([]codeAction)) != 1 {
				t.Fatal(response, problem)
			}
			edit := codeActionTestEdits(t, response.([]codeAction)[0], fixture.URI)[0]
			if edit.Range.Start != semanticTestPosition(t, fixture.Text, "pure val", 0) || edit.Range.End.Character-edit.Range.Start.Character != 8 {
				t.Fatal("fix did not target the verified qualifier", edit)
			}
			changed, err := applyChanges(fixture.Text, []contentChange{{Range: &edit.Range, Text: edit.NewText}})
			if err != nil || changed != strings.Replace(fixture.Text, "pure val", "val", 1) {
				t.Fatal(changed, err)
			}
			path, _ := sourceForURI(fixture.URI)
			files := insdict.NewDict[string, string]()
			files.Set(path, changed)
			reanalyzed, err := analyzeDocument(context.Background(), Document{URI: fixture.URI, Version: 2, Text: changed}, files)
			if err != nil || len(reanalyzed.Diagnostics) != 0 {
				t.Fatal(err, reanalyzed.Diagnostics)
			}
		})
	}
}

func mustMarshalCodeActionTest(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestCodeActionsRepairAnalyzedSourceGraph(t *testing.T) {
	t.Parallel()
	for _, example := range []struct{ name, text, code string }{
		{"qualifier", "module M { import Lib.* from \"lib\"\n pure val emoji=\"😀\" pure   val bad=observed\n}", "QNT200"},
		{"type name", "module M { type lower = int }", quint.QNT007},
		{"map type", "module M { const m: Map[int, str] }", quint.QNT015},
	} {
		t.Run(example.name, func(t *testing.T) {
			path := "/quick fix/main.qnt"
			document := Document{URI: uriForSource(path), Version: 1, Text: example.text}
			files := insdict.NewDict[string, string]()
			files.Set(path, document.Text)
			dependency := "module Lib { var x: int val observed=x }"
			files.Set("/quick fix/lib.qnt", dependency)
			snapshot, err := analyzeDocument(context.Background(), document, files)
			if err != nil || len(snapshot.Diagnostics) != 1 || snapshot.Diagnostics[0].Code != example.code || len(snapshot.Diagnostics[0].Data) == 0 {
				t.Fatal(err, snapshot.Diagnostics)
			}
			diagnostic := snapshot.Diagnostics[0]
			response, problem := codeActionRequest(codeActionTestParams(t, document.URI, diagnostic.Range, []Diagnostic{diagnostic}), snapshot)
			if problem != nil || len(response.([]codeAction)) != 1 {
				t.Fatal(problem, response)
			}
			edit := codeActionTestEdits(t, response.([]codeAction)[0], document.URI)[0]
			if example.code == "QNT200" {
				if edit.Range.Start != semanticTestPosition(t, document.Text, "pure   val", 0) || edit.Range.End.Character-edit.Range.Start.Character != 10 || edit.NewText != "val" {
					t.Fatal("qualifier fix lost UTF-16 or whitespace width", edit)
				}
			}
			changed, err := applyChanges(document.Text, []contentChange{{Range: &edit.Range, Text: edit.NewText}})
			if err != nil || changed == document.Text || files.Get(path) != document.Text || files.Get("/quick fix/lib.qnt") != dependency {
				t.Fatal("edit changed the wrong source", changed, err)
			}
			document.Text, document.Version = changed, 2
			files.Set(path, changed)
			reanalyzed, err := analyzeDocument(context.Background(), document, files)
			if err != nil || len(reanalyzed.Diagnostics) != 0 || reanalyzed.Version != 2 {
				t.Fatal("edited source did not reanalyze cleanly", err, reanalyzed.Diagnostics)
			}
		})
	}
}

func TestCodeActionsIgnoreMalformedAndIrrelevantFixes(t *testing.T) {
	t.Parallel()
	uri := "untitled:quick-fixes"
	snapshot := &Snapshot{URI: uri, Text: "pure val a=1\nsecond"}
	span := Range{Start: Position{}, End: Position{Character: 8}}
	for _, data := range []string{
		`null`, `false`, `1`, `[]`, `{}`, `{"fix":null}`, `{"fix":"replace"}`,
		`{"fix":{"kind":"insert","original":"pure val","replacement":"val"}}`,
		`{"fix":{"kind":"replace","replacement":"val"}}`,
		`{"fix":{"kind":"replace","original":null,"replacement":"val"}}`,
		`{"fix":{"kind":"replace","original":1,"replacement":"val"}}`,
		`{"fix":{"kind":"replace","original":"","replacement":"val"}}`,
		`{"fix":{"kind":"replace","original":" \t","replacement":"val"}}`,
		`{"fix":{"kind":"replace","original":"pure val"}}`,
		`{"fix":{"kind":"replace","original":"pure val","replacement":false}}`,
		`{"fix":{"kind":"replace","original":"second","replacement":"val"}}`,
	} {
		diagnostic := json.RawMessage(`{"range":` + string(mustMarshalCodeActionTest(t, span)) + `,"data":` + data + `}`)
		response, problem := codeActionRequest(codeActionTestParams(t, uri, span, []json.RawMessage{diagnostic}), snapshot)
		if problem != nil || len(response.([]codeAction)) != 0 {
			t.Fatal(data, problem, response)
		}
	}
	data := json.RawMessage(`{"fix":{"kind":"replace","original":"pure val","replacement":"val"}}`)
	for _, invalid := range []Range{{Start: Position{Character: -1}}, {End: Position{Character: 999}}, {Start: Position{Character: 4}, End: Position{Character: 2}}} {
		diagnostic := Diagnostic{Range: invalid, Data: data}
		response, problem := codeActionRequest(codeActionTestParams(t, uri, span, []Diagnostic{diagnostic}), snapshot)
		if problem != nil || len(response.([]codeAction)) != 0 {
			t.Fatal(invalid, problem, response)
		}
	}
	for _, malformed := range []json.RawMessage{nil, json.RawMessage(`{`), json.RawMessage(`{}`), json.RawMessage(`{"textDocument":{"uri":"untitled:quick-fixes"},"range":{}}`), codeActionTestParams(t, uri, Range{Start: Position{Character: 999}}, nil)} {
		if _, problem := codeActionRequest(malformed, snapshot); problem == nil || problem.Code != -32602 {
			t.Fatal(string(malformed), problem)
		}
	}
	for _, missing := range []*Snapshot{nil, {URI: "untitled:other", Text: snapshot.Text}} {
		response, problem := codeActionRequest(codeActionTestParams(t, uri, span, nil), missing)
		if problem != nil || len(response.([]codeAction)) != 0 {
			t.Fatal(problem, response)
		}
	}
	snapshot.Text = "😀pure val x=1"
	valid := Range{Start: Position{Character: 2}, End: Position{Character: 10}}
	splitSurrogate := Range{Start: Position{Character: 1}, End: Position{Character: 10}}
	diagnostic := Diagnostic{Range: splitSurrogate, Data: data}
	response, problem := codeActionRequest(codeActionTestParams(t, uri, valid, []Diagnostic{diagnostic}), snapshot)
	if problem != nil || len(response.([]codeAction)) != 0 {
		t.Fatal("diagnostic edit split a UTF-16 surrogate pair", response, problem)
	}
	if _, problem = codeActionRequest(codeActionTestParams(t, uri, splitSurrogate, nil), snapshot); problem == nil || problem.Code != -32602 {
		t.Fatal("request split a UTF-16 surrogate pair", problem)
	}
	// A whitespace regex may consume CRLF, but its resulting same-line edit
	// must still end at a real UTF-16 position instead of crossing the line.
	snapshot.Text = "pure\r\nnext"
	span.End.Character = 4
	diagnostic = Diagnostic{Range: span, Data: json.RawMessage(`{"fix":{"kind":"replace","original":"pure ","replacement":"val"}}`)}
	response, problem = codeActionRequest(codeActionTestParams(t, uri, span, []Diagnostic{diagnostic}), snapshot)
	if problem != nil || len(response.([]codeAction)) != 0 {
		t.Fatal(problem, response)
	}
}

func TestCodeActionDiagnosticDataOwnershipAndSource(t *testing.T) {
	t.Parallel()
	path := "/quickfix/main.qnt"
	id, _ := quint.ParseID("1")
	end := quint.Position{Col: 8}
	sources := quint.SourceMap{id: {Source: path, Start: quint.Position{}, End: &end}}
	data := &quint.DiagnosticData{Fix: &quint.DiagnosticFix{Kind: "replace", Original: "pure val", Replacement: "val"}}
	diagnostic := quint.Diagnostic{Reference: &id, Data: data}
	local := convertDiagnostic(path, sources, diagnostic)
	data.Fix.Replacement = "action"
	if !bytes.Contains(local.Data, []byte(`"replacement":"val"`)) {
		t.Fatal("published fix aliases inference data", string(local.Data))
	}
	foreign := convertDiagnostic("/quickfix/other.qnt", sources, diagnostic)
	if len(foreign.Data) != 0 || len(foreign.RelatedInformation) != 1 {
		t.Fatal("foreign fix leaked into requesting document", foreign)
	}
	trace := quint.ErrorLeaf("source", "details")
	data.Trace = &trace
	foreign = convertDiagnostic("/quickfix/other.qnt", sources, diagnostic)
	if !bytes.Contains(foreign.Data, []byte(`"trace"`)) || bytes.Contains(foreign.Data, []byte(`"fix"`)) || data.Fix == nil {
		t.Fatal("foreign fix filtering lost trace or changed original data", string(foreign.Data))
	}
	files := insdict.NewDict[string, string]()
	files.Set(path, "module M { import Lib.* from \"lib\" pure val n=1 }")
	files.Set("/quickfix/lib.qnt", "module Lib { var x:int pure val bad=x }")
	snapshot, err := analyzeDocument(context.Background(), Document{URI: uriForSource(path), Text: files.Get(path)}, files)
	if err != nil || len(snapshot.Diagnostics) == 0 {
		t.Fatal(err, snapshot.Diagnostics)
	}
	for _, diagnostic := range snapshot.Diagnostics {
		if len(diagnostic.Data) != 0 || len(diagnostic.RelatedInformation) == 0 || !strings.HasSuffix(diagnostic.RelatedInformation[0].Location.URI, "/lib.qnt") {
			t.Fatal("imported fix exposed against the main source", diagnostic)
		}
	}
}

func TestCodeActionsDoNotBorrowAnotherDeclarationsQualifier(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"module M { var x:int pure val good=1 action bad=x }",
		"module M { var x:int pure val good=1 pure /*keep*/ val bad=x }",
	} {
		uri := "untitled:stale-qualifier"
		start := semanticTestPosition(t, text, "bad", 0)
		end := start
		end.Character += 3
		span := Range{Start: start, End: end}
		diagnostic := Diagnostic{Range: span, Code: "QNT200", Data: json.RawMessage(`{"fix":{"kind":"replace","original":"pure val","replacement":"val"}}`)}
		response, problem := codeActionRequest(codeActionTestParams(t, uri, span, []Diagnostic{diagnostic}), &Snapshot{URI: uri, Text: text})
		if problem != nil || len(response.([]codeAction)) != 0 {
			t.Fatal("stale qualifier or embedded comment produced a destructive fix", text, response, problem)
		}
	}
}
