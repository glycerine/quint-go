package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

func signatureTestRequest(t *testing.T, snapshot *Snapshot, position Position) (signatureHelp, *ResponseError) {
	t.Helper()
	params, err := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": snapshot.URI}, "position": position})
	if err != nil {
		t.Fatal(err)
	}
	result, problem := signatureHelpRequest(params, snapshot)
	if problem != nil {
		return signatureHelp{}, problem
	}
	return result.(signatureHelp), nil
}

func TestSignatureHelpUsesSourceDocsAndEnclosingApplications(t *testing.T) {
	t.Parallel()
	text := `module M {
  /// Add **two** numbers.
  pure def plus(x: int, y: int): int = x + y
  pure val plain = 3
  /// Count docs.
  type Count = int
  pure def identity(n: Count): Count = n
  pure val call = plus(1, 2)
  pure val nested = plus(plus(4, 5), 6)
  pure val built = Set(1, 2).size()
  pure val plainUse = plain
}`
	files := insdict.NewDict[string, string]()
	files.Set("/signature/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/signature/main.qnt", files)
	for _, item := range []struct {
		needle string
		index  int
		shift  int
		name   string
		label  string
		doc    string
	}{
		{"plus(1", 0, 1, "plus", "pure def plus: (int, int) => int", "Add **two** numbers."},
		{"plus(1", 0, 4, "plus", "pure def plus: (int, int) => int", "Add **two** numbers."},
		{"plus(1", 0, 5, "plus", "pure def plus: (int, int) => int", "Add **two** numbers."},
		{"plus(1, 2)", 0, 8, "plus", "pure def plus: (int, int) => int", "Add **two** numbers."},
		{"plus(4", 0, 5, "plus", "pure def plus: (int, int) => int", "Add **two** numbers."},
		{"plain\n", 0, 0, "plain", "pure val plain", ""},
		{"Count):", 0, 1, "Count", "type Count = int", "Count docs."},
		{"size()", 0, 0, "size", "pure def size: (Set[a]) => int", "builtin"},
	} {
		t.Run(item.needle+string(rune(item.shift+'0')), func(t *testing.T) {
			position := semanticTestPosition(t, text, item.needle, item.index)
			position.Character += item.shift
			result, problem := signatureTestRequest(t, snapshot, position)
			if problem != nil || len(result.Signatures) != 1 || result.ActiveSignature != 0 || result.ActiveParameter != 0 {
				t.Fatal(result, problem)
			}
			info := result.Signatures[0]
			if info.Name != item.name || info.Label != item.label {
				t.Fatalf("got %+v, want %s: %s", info, item.name, item.label)
			}
			if item.doc == "" {
				if info.Documentation != nil {
					t.Fatal("undocumented definition has markup", info)
				}
			} else if info.Documentation == nil || info.Documentation.Kind != "markdown" || item.doc != "builtin" && info.Documentation.Value != item.doc {
				t.Fatal(info)
			}
		})
	}
	for _, needle := range []string{"def plus", "module M", "x + y", "Set(1"} {
		result, problem := signatureTestRequest(t, snapshot, semanticTestPosition(t, text, needle, 0))
		if problem != nil || len(result.Signatures) != 0 {
			t.Fatal(needle, result, problem)
		}
	}
}

func TestSignatureHelpUTF16AndMultiline(t *testing.T) {
	t.Parallel()
	text := `module M {
  /// Astral prefix must not move the call cursor.
  pure def echo(value: str): str = value
  pure val a = ("😀", echo("ok"))
  pure val b = echo(
    "line"
  )
}`
	files := insdict.NewDict[string, string]()
	files.Set("/signature/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/signature/main.qnt", files)
	for _, needle := range []string{"echo(\"ok\")", "\"line\"", "  )"} {
		position := semanticTestPosition(t, text, needle, 0)
		if needle == "  )" {
			position.Character += 2
		}
		result, problem := signatureTestRequest(t, snapshot, position)
		if problem != nil || len(result.Signatures) != 1 || result.Signatures[0].Name != "echo" {
			t.Fatal(needle, position, result, problem)
		}
	}
	invalid := semanticTestPosition(t, text, "😀", 0)
	invalid.Character++
	if _, problem := signatureTestRequest(t, snapshot, invalid); problem == nil || problem.Code != -32602 {
		t.Fatal("UTF-16 surrogate interior accepted", problem)
	}
}

func TestSignatureHelpBindingFallbackPrecedence(t *testing.T) {
	t.Parallel()
	text := `module M {
  pure def first(n: int): int = n
  pure def second(n: int): int = n
  pure val use = first(1)
}`
	files := insdict.NewDict[string, string]()
	files.Set("/signature/main.qnt", text)
	snapshot := semanticTestSnapshot(t, "/signature/main.qnt", files)
	position := semanticTestPosition(t, text, "first(1)", 0)
	ids := insdict.NewDict[string, quint.ID]()
	for _, declaration := range snapshot.Frontend.Modules[0].Declarations {
		ids.Set(declaration.Name, declaration.ID)
	}
	ref := snapshot.Frontend.IDs.Next()
	positional := snapshot.Frontend.IDs.Next()
	snapshot.Frontend.Sources[positional] = quint.Location{Source: "/signature/main.qnt", Start: quint.Position{Line: position.Line, Col: position.Character}}
	for _, item := range []struct {
		name        string
		declaration bool
		positional  bool
		want        string
	}{
		{name: "first matching binding", want: "first"},
		{name: "declaration beats first match", declaration: true, want: "second"},
		{name: "position beats declaration", declaration: true, positional: true, want: "first"},
	} {
		t.Run(item.name, func(t *testing.T) {
			working := *snapshot
			working.Frontend.Table = quint.NewExpressionNameResolver(quint.NewNameScope("M")).Table
			working.Frontend.Table.Set(ref, quint.LookupDefinition{ID: ids.Get("first"), Name: "first"})
			if item.declaration {
				working.Frontend.Table.Set(ids.Get("second"), quint.LookupDefinition{ID: ids.Get("second"), Name: "first"})
			}
			if item.positional {
				working.Frontend.Table.Set(positional, quint.LookupDefinition{ID: ids.Get("first"), Name: "first"})
			}
			result, problem := signatureTestRequest(t, &working, position)
			if problem != nil || len(result.Signatures) != 1 || result.Signatures[0].Name != item.want {
				t.Fatal(result, problem)
			}
		})
	}
}

func TestSignatureHelpRejectsInvalidParametersAndHandlesMissingState(t *testing.T) {
	t.Parallel()
	for _, params := range []string{`null`, `{}`, `{"textDocument":{"uri":"file:///a"}}`, `{"textDocument":{"uri":"file:///a"},"position":false}`, `[]`} {
		if _, problem := signatureHelpRequest(json.RawMessage(params), nil); problem == nil || problem.Code != -32602 {
			t.Fatal(params, problem)
		}
	}
	valid := json.RawMessage(`{"textDocument":{"uri":"file:///a"},"position":{"line":0,"character":0}}`)
	for _, snapshot := range []*Snapshot{nil, {URI: "file:///different", Text: ""}, {URI: "file:///a", Text: ""}} {
		result, problem := signatureHelpRequest(valid, snapshot)
		if problem != nil || !reflect.DeepEqual(result, emptySignatureHelp()) {
			t.Fatal(result, problem)
		}
	}
	for _, snapshot := range []*Snapshot{{URI: "file:///a", Text: string([]byte{255})}} {
		if _, problem := signatureHelpRequest(valid, snapshot); problem == nil {
			t.Fatal("invalid source accepted")
		}
	}
	for _, position := range []Position{{Line: -1}, {Character: -1}, {Line: 1}} {
		if _, problem := signatureTestRequest(t, &Snapshot{URI: "file:///a"}, position); problem == nil {
			t.Fatal(position, problem)
		}
	}
}

func TestServerSignatureHelpCurrentBuffersAndMissingAnalysis(t *testing.T) {
	t.Parallel()
	started, release := make(chan struct{}), make(chan struct{})
	server := NewServer(Options{Analyze: func(ctx context.Context, doc Document) (*Snapshot, error) {
		if doc.Version == 2 {
			close(started)
			<-release
		}
		files := insdict.NewDict[string, string]()
		path, _ := sourceForURI(doc.URI)
		files.Set(path, doc.Text)
		return analyzeDocument(ctx, doc, files)
	}})
	defer server.Close()
	capture := &capturedMessages{}
	handleTest(t, server, capture, "initialize", 1, map[string]any{})
	if messages := capture.take(); len(messages) != 1 || !bytes.Contains(messages[0], []byte(`"signatureHelpProvider":{"triggerCharacters":["("]}`)) {
		t.Fatal(messages)
	}
	uri := "untitled:signature"
	text := "module M {\n /// Old docs.\n pure def f(x: int): int = x\n pure val use = f(1)\n}"
	position := semanticTestPosition(t, text, "f(1)", 0)
	params := map[string]any{"textDocument": map[string]any{"uri": uri}, "position": position}
	handleTest(t, server, capture, "textDocument/signatureHelp", 2, params)
	if messages := capture.take(); len(messages) != 1 || !bytes.Contains(messages[0], []byte(`"signatures":[]`)) {
		t.Fatal(messages)
	}
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, text, 1))
	waitTest(t, server)
	capture.take()
	handleTest(t, server, capture, "textDocument/signatureHelp", 3, params)
	waitTest(t, server)
	if messages := capture.take(); len(messages) != 1 || !bytes.Contains(messages[0], []byte("Old docs.")) {
		t.Fatal(messages)
	}
	text = strings.Replace(text, "Old docs.", "New docs.", 1)
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(uri, text, 2))
	<-started
	handleTest(t, server, capture, "textDocument/signatureHelp", 4, params)
	if messages := capture.take(); len(messages) != 1 || !bytes.Contains(messages[0], []byte(`"signatures":[]`)) {
		t.Fatal("stale signatures returned", messages)
	}
	close(release)
	waitTest(t, server)
	capture.take()
	handleTest(t, server, capture, "textDocument/signatureHelp", 5, params)
	waitTest(t, server)
	if messages := capture.take(); len(messages) != 1 || !bytes.Contains(messages[0], []byte("New docs.")) || bytes.Contains(messages[0], []byte("Old docs.")) {
		t.Fatal(messages)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 6, "method": "textDocument/signatureHelp", "params": params})
	if err := server.Handle(ctx, body, capture.send); err != nil {
		t.Fatal(err)
	}
	waitTest(t, server)
	if messages := capture.take(); len(messages) != 1 || !bytes.Contains(messages[0], []byte(`"code":-32800`)) {
		t.Fatal(messages)
	}
}

func TestSignatureHelpUpstreamFixtures(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/signature_help.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name     string
		Files    []struct{ Path, Text string }
		Main     string
		Position Position
		Result   json.RawMessage
		Parsed   *bool
		Docs     *bool
	}
	if err := json.Unmarshal(body, &cases); err != nil {
		t.Fatal(err)
	}
	for _, item := range cases {
		t.Run(item.Name, func(t *testing.T) {
			files := insdict.NewDict[string, string]()
			for _, file := range item.Files {
				files.Set(filepath.Join("/signature-fixtures", file.Path), file.Text)
			}
			main := filepath.Join("/signature-fixtures", item.Main)
			var snapshot *Snapshot
			if item.Parsed == nil || *item.Parsed {
				snapshot, err = analyzeDocument(context.Background(), Document{URI: uriForSource(main), Version: 1, Text: files.Get(main)}, files)
				if err != nil {
					t.Fatal(err)
				}
				if item.Docs != nil && !*item.Docs {
					snapshot.Documentation = nil
				}
			}
			params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uriForSource(main)}, "position": item.Position})
			result, problem := signatureHelpRequest(params, snapshot)
			if problem != nil {
				t.Fatal(problem)
			}
			actual, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			_ = json.Unmarshal(actual, &got)
			_ = json.Unmarshal(item.Result, &want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %s\nwant %s", actual, item.Result)
			}
		})
	}
}

func TestServerSignatureHelpUnsavedImportOverlay(t *testing.T) {
	t.Parallel()
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	libraryURI := "file:///signature-overlay/lib.qnt"
	mainURI := "file:///signature-overlay/main.qnt"
	library := "module Lib {\n /// Original docs.\n pure def increment(n: int): int = n + 1\n}"
	main := "module Main {\n import Lib.* from \"./lib\"\n pure val use = increment(1)\n}"
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(libraryURI, library, 1))
	waitTest(t, server)
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(mainURI, main, 1))
	waitTest(t, server)
	capture.take()
	params := map[string]any{"textDocument": map[string]any{"uri": mainURI}, "position": semanticTestPosition(t, main, "increment(1)", 0)}
	handleTest(t, server, capture, "textDocument/signatureHelp", 10, params)
	waitTest(t, server)
	if messages := capture.take(); len(messages) != 1 || !bytes.Contains(messages[0], []byte("Original docs.")) {
		t.Fatal(messages)
	}
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(libraryURI, strings.Replace(library, "Original docs.", "Unsaved docs.", 1), 2))
	waitTest(t, server)
	capture.take()
	handleTest(t, server, capture, "textDocument/signatureHelp", 11, params)
	waitTest(t, server)
	if messages := capture.take(); len(messages) != 1 || !bytes.Contains(messages[0], []byte("Unsaved docs.")) || bytes.Contains(messages[0], []byte("Original docs.")) {
		t.Fatal(messages)
	}
}
