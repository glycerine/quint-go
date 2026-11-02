package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func codeActionServerResponse(t *testing.T, messages []json.RawMessage, id int) []codeAction {
	t.Helper()
	for _, encoded := range messages {
		var response struct {
			ID     int
			Result []codeAction
			Error  *ResponseError
		}
		if err := json.Unmarshal(encoded, &response); err != nil {
			t.Fatal(err)
		}
		if response.ID == id {
			if response.Error != nil || response.Result == nil {
				t.Fatal(string(encoded))
			}
			return response.Result
		}
	}
	t.Fatal("missing code-action response", id)
	return nil
}

func TestServerCodeActionEditsOpenGraph(t *testing.T) {
	t.Parallel()
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	handleTest(t, server, capture, "initialize", 1, map[string]any{})
	if messages := capture.take(); len(messages) != 1 || !bytes.Contains(messages[0], []byte(`"codeActionProvider":true`)) {
		t.Fatal(messages)
	}
	dependencyURI := "file:///server%20quickfix/lib.qnt"
	uri := "file:///server%20quickfix/main.qnt"
	dependency := "module Lib { var x:int val observed=x }"
	text := "module M { import Lib.* from \"lib\"\n pure val bad=observed\n}"
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(dependencyURI, dependency, 1))
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, text, 1))
	waitTest(t, server)
	var diagnostic Diagnostic
	for _, publication := range diagnosticsTest(t, capture.take()) {
		if publication.URI == uri && len(publication.Diagnostics) == 1 {
			diagnostic = publication.Diagnostics[0]
		}
	}
	if diagnostic.Code != "QNT200" || len(diagnostic.Data) == 0 {
		t.Fatal(diagnostic)
	}
	params := codeActionTestParams(t, uri, diagnostic.Range, []Diagnostic{diagnostic})
	handleTest(t, server, capture, "textDocument/codeAction", 2, params)
	waitTest(t, server)
	actions := codeActionServerResponse(t, capture.take(), 2)
	if len(actions) != 1 {
		t.Fatal(actions)
	}
	edit := codeActionTestEdits(t, actions[0], uri)[0]
	handleTest(t, server, capture, "textDocument/didChange", nil, map[string]any{
		"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []contentChange{{Range: &edit.Range, Text: edit.NewText}},
	})
	waitTest(t, server)
	clean := false
	for _, publication := range diagnosticsTest(t, capture.take()) {
		if publication.URI == uri && publication.Version != nil && *publication.Version == 2 && len(publication.Diagnostics) == 0 {
			clean = true
		}
	}
	server.mu.Lock()
	main := server.documents.Get(uri)
	other := server.documents.Get(dependencyURI)
	correctSources := main != nil && main.Text == strings.Replace(text, "pure val", "val", 1) && other != nil && other.Text == dependency
	server.mu.Unlock()
	if !clean || !correctSources {
		t.Fatal("edited graph was not reanalyzed or changed another source")
	}
	handleTest(t, server, capture, "textDocument/didClose", nil, map[string]any{"textDocument": map[string]any{"uri": uri}})
	waitTest(t, server)
	capture.take()
	handleTest(t, server, capture, "textDocument/codeAction", 3, params)
	waitTest(t, server)
	if actions := codeActionServerResponse(t, capture.take(), 3); len(actions) != 0 {
		t.Fatal("closed document retained actions", actions)
	}
}

func TestServerCodeActionsUseCurrentBufferDuringAnalysis(t *testing.T) {
	t.Parallel()
	server := NewServer(Options{Analyze: func(ctx context.Context, document Document) (*Snapshot, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	uri := "untitled:pending-quick-fix"
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, "pure val x=1", 1))
	text := "  pure   val x=1"
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(uri, text, 2))
	span := Range{Start: Position{Character: 2}, End: Position{Character: 12}}
	diagnostic := Diagnostic{Range: span, Data: json.RawMessage(`{"fix":{"kind":"replace","original":"pure val","replacement":"val"}}`)}
	params := codeActionTestParams(t, uri, span, []Diagnostic{diagnostic})
	message := mustMarshalCodeActionTest(t, map[string]any{"jsonrpc": "2.0", "id": 7, "method": "textDocument/codeAction", "params": params})
	responses := make(chan json.RawMessage, 1)
	if err := server.Handle(context.Background(), message, func(value any) error {
		encoded, err := json.Marshal(value)
		if err == nil {
			responses <- encoded
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case encoded := <-responses:
		actions := codeActionServerResponse(t, []json.RawMessage{encoded}, 7)
		if len(actions) != 1 {
			t.Fatal(actions)
		}
		edit := codeActionTestEdits(t, actions[0], uri)[0]
		if edit.Range != span || edit.NewText != "val" {
			t.Fatal("quick fix used stale analyzed text", edit)
		}
	case <-time.After(time.Second):
		t.Fatal("code action waited for pending analysis")
	}
	server.Close()
	waitTest(t, server)
}
