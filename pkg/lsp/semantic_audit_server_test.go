package lsp

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

func semanticAuditRPC(t *testing.T, server *Server, capture *capturedMessages, method string, id int, uri string, position Position, extra map[string]any) json.RawMessage {
	t.Helper()
	params := map[string]any{"textDocument": map[string]any{"uri": uri}, "position": position}
	for key, value := range extra {
		params[key] = value
	}
	handleTest(t, server, capture, method, id, params)
	waitTest(t, server)
	for _, raw := range capture.take() {
		var response struct {
			ID     int
			Result json.RawMessage
			Error  *ResponseError
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			t.Fatal(err)
		}
		if response.ID != id {
			continue
		}
		if response.Error != nil {
			t.Fatal(response.Error)
		}
		return response.Result
	}
	t.Fatal("semantic request did not return a response", method)
	return nil
}

func TestServerExpressionHoverSymbolsAndOverrideRename(t *testing.T) {
	t.Parallel()
	const uri = "file:///rpc-semantic-audit/main.qnt"
	text := `module Proto { const transform:int=>int pure val answer=transform(1) }
module Main {
  type Count=int
  assume consistent=true
  pure val emoji="😀"
  pure val r={count:1}
  pure val result=r.count
  import Proto(transform=(parameter)=>parameter+1) as Configured
  pure val configured=Configured::answer
}`
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, text, 1))
	waitTest(t, server)
	for _, publication := range diagnosticsTest(t, capture.take()) {
		if len(publication.Diagnostics) != 0 {
			t.Fatal(publication.Diagnostics)
		}
	}
	field := semanticTestPosition(t, text, "count\n", 0)
	result := semanticAuditRPC(t, server, capture, "textDocument/hover", 101, uri, field, nil)
	var hover semanticHover
	if err := json.Unmarshal(result, &hover); err != nil || !strings.Contains(hover.Contents.Value, "(field) count: int") || !semanticContains(hover.Range, field) {
		t.Fatalf("field hover: %s; %v", result, err)
	}
	arrow := semanticTestPosition(t, text, "=>parameter", 0)
	result = semanticAuditRPC(t, server, capture, "textDocument/hover", 102, uri, arrow, nil)
	if err := json.Unmarshal(result, &hover); err != nil || !strings.Contains(hover.Contents.Value, "parameter+1") || !semanticContains(hover.Range, arrow) {
		t.Fatalf("lambda hover: %s; %v", result, err)
	}
	result = semanticAuditRPC(t, server, capture, "textDocument/documentSymbol", 103, uri, Position{}, nil)
	var symbols []semanticSymbol
	if err := json.Unmarshal(result, &symbols); err != nil || len(symbols) != 2 {
		t.Fatalf("document symbols: %s; %v", result, err)
	}
	foundMain := false
	for _, module := range symbols {
		if module.Name != "Main" {
			continue
		}
		foundMain = true
		if len(module.Children) != 4 {
			t.Fatal("outline retained types/assumptions/imports", module.Children)
		}
		for _, symbol := range module.Children {
			if symbol.Kind != 8 {
				t.Fatal("pure values must be field symbols", symbol)
			}
		}
	}
	if !foundMain {
		t.Fatal("missing local module outline")
	}
	parameter := semanticTestPosition(t, text, "parameter+1", 0)
	result = semanticAuditRPC(t, server, capture, "textDocument/definition", 104, uri, parameter, nil)
	var definitions []Location
	if err := json.Unmarshal(result, &definitions); err != nil || len(definitions) != 1 || definitions[0].URI != uri {
		t.Fatalf("override definition: %s; %v", result, err)
	}
	snapshot := &Snapshot{URI: uri, Text: text}
	if semanticTestRangeText(t, snapshot, definitions[0]) != "parameter" || definitions[0].Range.Start == parameter {
		t.Fatal("override reference did not navigate to its declaration", definitions)
	}
	result = semanticAuditRPC(t, server, capture, "textDocument/rename", 105, uri, parameter, map[string]any{"newName": "renamed"})
	var rename struct {
		Changes map[string][]semanticTextEdit
	}
	if err := json.Unmarshal(result, &rename); err != nil || len(rename.Changes) != 1 || len(rename.Changes[uri]) != 2 {
		t.Fatalf("override rename: %s; %v", result, err)
	}
	edits := rename.Changes[uri]
	sort.Slice(edits, func(i, j int) bool { return semanticPositionLess(edits[j].Range.Start, edits[i].Range.Start) })
	changes := make([]contentChange, len(edits))
	for i, edit := range edits {
		if semanticTestRangeText(t, snapshot, Location{URI: uri, Range: edit.Range}) != "parameter" {
			t.Fatal("rename edited outside the parameter token", edit)
		}
		rangeCopy := edit.Range
		changes[i] = contentChange{Range: &rangeCopy, Text: edit.NewText}
	}
	updated, err := applyChanges(text, changes)
	if err != nil {
		t.Fatal(err)
	}
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(uri, updated, 2))
	waitTest(t, server)
	publications := diagnosticsTest(t, capture.take())
	if len(publications) != 1 || publications[0].Version == nil || *publications[0].Version != 2 || len(publications[0].Diagnostics) != 0 {
		t.Fatal("renamed override did not reanalyze cleanly", publications)
	}
	position := semanticTestPosition(t, updated, "renamed+1", 0)
	result = semanticAuditRPC(t, server, capture, "textDocument/definition", 106, uri, position, nil)
	if err := json.Unmarshal(result, &definitions); err != nil || len(definitions) != 1 || semanticTestRangeText(t, &Snapshot{URI: uri, Text: updated}, definitions[0]) != "renamed" {
		t.Fatalf("renamed definition: %s; %v", result, err)
	}
}

func TestServerTypedIdentifierCompletion(t *testing.T) {
	t.Parallel()
	const uri = "file:///rpc-identifier-completion/main.qnt"
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	for version, example := range []struct {
		text   string
		labels []string
	}{
		{"module Main { var r:{count:int} val use=r }\n", []string{"fieldNames", "with", "count"}},
		{"module Main { var r:bool val use=r }\n", []string{"iff", "implies"}},
	} {
		if version == 0 {
			handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, example.text, 1))
		} else {
			handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(uri, example.text, version+1))
		}
		waitTest(t, server)
		for _, publication := range diagnosticsTest(t, capture.take()) {
			if len(publication.Diagnostics) != 0 {
				t.Fatal(publication.Diagnostics)
			}
		}
		position := semanticTestPosition(t, example.text, "r }", 0)
		position.Character++ // Ctrl+Space immediately after the complete identifier.
		result := semanticAuditRPC(t, server, capture, "textDocument/completion", 201+version, uri, position, map[string]any{"context": map[string]any{"triggerKind": 1}})
		var completion semanticCompletionList
		if err := json.Unmarshal(result, &completion); err != nil || len(completion.Items) != len(example.labels) {
			t.Fatalf("identifier completion: %s; %v", result, err)
		}
		for i, label := range example.labels {
			if completion.Items[i].Label != label {
				t.Fatal("completion did not use the current identifier type", completion.Items)
			}
		}
	}
}
