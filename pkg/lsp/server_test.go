package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

type capturedMessages struct {
	mu       sync.Mutex
	messages []json.RawMessage
}

func (c *capturedMessages) send(message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.messages = append(c.messages, body)
	c.mu.Unlock()
	return nil
}
func (c *capturedMessages) take() []json.RawMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	messages := c.messages
	c.messages = nil
	return messages
}
func handleTest(t *testing.T, server *Server, capture *capturedMessages, method string, id any, params any) {
	t.Helper()
	message := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id != nil {
		message["id"] = id
	}
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	if err = server.Handle(context.Background(), body, capture.send); err != nil {
		t.Fatal(err)
	}
}
func initializeTest(t *testing.T, server *Server, capture *capturedMessages) {
	t.Helper()
	handleTest(t, server, capture, "initialize", 1, map[string]any{})
	capture.take()
}
func openParams(uri, text string, version int) any {
	return map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "quint", "version": version, "text": text}}
}
func changeParams(uri, text string, version int) any {
	return map[string]any{"textDocument": map[string]any{"uri": uri, "version": version}, "contentChanges": []any{map[string]any{"text": text}}}
}
func waitTest(t *testing.T, server *Server) {
	t.Helper()
	if err := server.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func diagnosticsTest(t *testing.T, messages []json.RawMessage) []struct {
	URI         string
	Version     *int
	Diagnostics []Diagnostic
} {
	t.Helper()
	out := []struct {
		URI         string
		Version     *int
		Diagnostics []Diagnostic
	}{}
	for _, body := range messages {
		var message struct {
			Method string
			Params struct {
				URI         string
				Version     *int
				Diagnostics []Diagnostic
			}
		}
		if err := json.Unmarshal(body, &message); err != nil {
			t.Fatal(err)
		}
		if message.Method == "textDocument/publishDiagnostics" {
			out = append(out, message.Params)
		}
	}
	return out
}
func TestServerLifecycleAndErrors(t *testing.T) {
	t.Parallel()
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	handleTest(t, server, capture, "textDocument/hover", 7, map[string]any{})
	if got := capture.take(); len(got) != 1 || !bytes.Contains(got[0], []byte(`"code":-32002`)) {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "initialize", "startup", map[string]any{})
	got := capture.take()
	if len(got) != 1 || !bytes.Contains(got[0], []byte(`"positionEncoding":"utf-16"`)) || !bytes.Contains(got[0], []byte(`"change":2`)) {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "initialize", 2, map[string]any{})
	if got := capture.take(); len(got) != 1 || !bytes.Contains(got[0], []byte(`"code":-32600`)) {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "initialized", nil, map[string]any{})
	handleTest(t, server, capture, "unknown/notification", nil, map[string]any{})
	if got := capture.take(); len(got) != 0 {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "unknown/request", 3, map[string]any{})
	if got := capture.take(); len(got) != 1 || !bytes.Contains(got[0], []byte(`"code":-32601`)) {
		t.Fatal(got)
	}
	for _, body := range []string{"{", `[]`, `{"jsonrpc":"1.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":false,"method":"initialize"}`} {
		if err := server.Handle(context.Background(), []byte(body), capture.send); err != nil {
			t.Fatal(err)
		}
	}
	if got := capture.take(); len(got) != 4 {
		t.Fatal(got)
	}
	largeID := strings.Repeat("9", 400)
	if err := server.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","id":`+largeID+`,"method":"unknown/request"}`), capture.send); err != nil {
		t.Fatal(err)
	}
	if got := capture.take(); len(got) != 1 || !bytes.Contains(got[0], []byte(`"id":`+largeID)) {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "shutdown", 4, nil)
	if got := capture.take(); len(got) != 1 || !bytes.Contains(got[0], []byte(`"result":null`)) || bytes.Contains(got[0], []byte(`"error"`)) {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "textDocument/hover", 5, map[string]any{})
	if got := capture.take(); len(got) != 1 || !bytes.Contains(got[0], []byte(`"code":-32600`)) {
		t.Fatal(got)
	}
	if err := server.Handle(context.Background(), []byte(` {"jsonrpc":"2.0","method":"exit"} `), capture.send); !errors.Is(err, errExit) {
		t.Fatal(err)
	}
	if err := server.Handle(context.Background(), nil, nil); err == nil {
		t.Fatal("missing send accepted")
	}
}
func TestServerOpenChangeClose(t *testing.T) {
	t.Parallel()
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	uri := "file:///tmp/live.qnt"
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, `module M { pure val x = missing }`, 1))
	waitTest(t, server)
	got := diagnosticsTest(t, capture.take())
	if len(got) != 1 || got[0].URI != uri || *got[0].Version != 1 || len(got[0].Diagnostics) != 1 || got[0].Diagnostics[0].Code != "QNT404" {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(uri, `module M { pure val x = 1 }`, 2))
	waitTest(t, server)
	got = diagnosticsTest(t, capture.take())
	if len(got) != 1 || *got[0].Version != 2 || len(got[0].Diagnostics) != 0 {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "unsupported/feature", 9, map[string]any{"textDocument": map[string]any{"uri": uri}})
	if messages := capture.take(); len(messages) != 1 || !bytes.Contains(messages[0], []byte(`"code":-32601`)) {
		t.Fatal(messages)
	}
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(uri, "stale", 1))
	waitTest(t, server)
	if got := capture.take(); len(got) != 0 {
		t.Fatal("old version was analyzed", got)
	}
	handleTest(t, server, capture, "textDocument/documentSymbol", 10, map[string]any{"textDocument": map[string]any{"uri": uri}})
	waitTest(t, server)
	if got := capture.take(); len(got) != 1 || !bytes.Contains(got[0], []byte(`"name":"x"`)) {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "textDocument/didClose", nil, map[string]any{"textDocument": map[string]any{"uri": uri}})
	waitTest(t, server)
	got = diagnosticsTest(t, capture.take())
	if len(got) != 1 || got[0].Version != nil || len(got[0].Diagnostics) != 0 {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "textDocument/hover", 11, map[string]any{"textDocument": map[string]any{"uri": uri}, "position": Position{}})
	if got := capture.take(); len(got) != 1 || !bytes.Contains(got[0], []byte(`"result":null`)) {
		t.Fatal(got)
	}
}
func TestServerRejectsStaleAndClosedAnalysis(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	release := make(chan struct{})
	latest := make(chan struct{})
	server := NewServer(Options{Analyze: func(ctx context.Context, doc Document) (*Snapshot, error) {
		if doc.Version == 1 {
			close(started)
			<-release
		} else {
			close(latest)
		}
		return &Snapshot{URI: doc.URI, Version: doc.Version, Text: doc.Text, Diagnostics: []Diagnostic{{Severity: 1, Source: "quint", Message: doc.Text}}}, nil
	}})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	uri := "untitled:stale"
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, "old", 1))
	<-started
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(uri, "new", 2))
	<-latest
	close(release)
	waitTest(t, server)
	got := diagnosticsTest(t, capture.take())
	if len(got) != 1 || *got[0].Version != 2 || got[0].Diagnostics[0].Message != "new" {
		t.Fatal(got)
	}
	started = make(chan struct{})
	release = make(chan struct{})
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, "reopened", 1))
	<-started
	handleTest(t, server, capture, "textDocument/didClose", nil, map[string]any{"textDocument": map[string]any{"uri": uri}})
	close(release)
	waitTest(t, server)
	got = diagnosticsTest(t, capture.take())
	if len(got) != 1 || got[0].Version != nil || len(got[0].Diagnostics) != 0 {
		t.Fatal(got)
	}
}
func TestServerImportedOpenBufferChanges(t *testing.T) {
	t.Parallel()
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams("file:///tmp/editor-import/a.qnt", `module A { pure val value = true }`, 1))
	waitTest(t, server)
	capture.take()
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams("file:///tmp/editor-import/main.qnt", `module M { import A.* from "a" pure val x = value + 1 }`, 1))
	waitTest(t, server)
	got := diagnosticsTest(t, capture.take())
	found := false
	for _, item := range got {
		if item.URI == "file:///tmp/editor-import/main.qnt" && len(item.Diagnostics) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal(got)
	}
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams("file:///tmp/editor-import/a.qnt", `module A { pure val value = 1 }`, 2))
	waitTest(t, server)
	got = diagnosticsTest(t, capture.take())
	if len(got) != 2 {
		t.Fatal(got)
	}
	for _, item := range got {
		if len(item.Diagnostics) != 0 {
			t.Fatal(got)
		}
	}
}
func TestServerInvalidEditsAreAtomic(t *testing.T) {
	t.Parallel()
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	uri := "untitled:edit"
	text := `module M { pure val x = "😀" }`
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, text, 1))
	waitTest(t, server)
	capture.take()
	params := map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []any{map[string]any{"text": "new"}, map[string]any{"range": Range{Start: Position{Character: 10}, End: Position{Character: 10}}, "text": "invalid"}}}
	handleTest(t, server, capture, "textDocument/didChange", nil, params)
	waitTest(t, server)
	server.mu.Lock()
	state := server.documents.Get(uri)
	gotText, version := state.Text, state.Version
	server.mu.Unlock()
	if gotText != text || version != 1 {
		t.Fatal(gotText, version)
	}
	if got := capture.take(); len(got) != 1 || !bytes.Contains(got[0], []byte(`"method":"window/logMessage"`)) {
		t.Fatal(got)
	}
}
func TestServerFramedSessionAndCancellation(t *testing.T) {
	t.Parallel()
	var input bytes.Buffer
	for _, message := range []any{map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}}, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown"}, map[string]any{"jsonrpc": "2.0", "method": "exit"}} {
		if err := WriteMessage(&input, message); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	if err := NewServer(Options{}).Serve(context.Background(), &input, &output); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(&output)
	for range 2 {
		if _, err := ReadMessage(reader); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReadMessage(reader); err != io.EOF {
		t.Fatal(err)
	}
	var exit bytes.Buffer
	_ = WriteMessage(&exit, map[string]any{"jsonrpc": "2.0", "method": "exit"})
	if err := NewServer(Options{}).Serve(context.Background(), &exit, io.Discard); !errors.Is(err, ErrExitBeforeShutdown) {
		t.Fatal(err)
	}
	if err := NewServer(Options{}).Serve(context.Background(), strings.NewReader("bad\n"), io.Discard); err == nil {
		t.Fatal("bad frame accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewServer(Options{}).Serve(ctx, strings.NewReader(""), io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	var initialize bytes.Buffer
	_ = WriteMessage(&initialize, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"})
	if err := NewServer(Options{}).Serve(context.Background(), &initialize, failWriter{}); !errors.Is(err, errWrite) {
		t.Fatal(err)
	}
}

func TestServerCancelledRequestAndWorkerShutdown(t *testing.T) {
	t.Parallel()
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	uri := "untitled:cancel-request"
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, `module M { pure val x = 1 }`, 1))
	waitTest(t, server)
	capture.take()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body := []byte(`{"jsonrpc":"2.0","id":99,"method":"textDocument/documentSymbol","params":{"textDocument":{"uri":"untitled:cancel-request"}}}`)
	if err := server.Handle(ctx, body, capture.send); err != nil {
		t.Fatal(err)
	}
	waitTest(t, server)
	got := capture.take()
	if len(got) != 1 || !bytes.Contains(got[0], []byte(`"code":-32800`)) {
		t.Fatal(got)
	}
	requestContext, requestCancel := context.WithCancel(context.Background())
	defer requestCancel()
	server.mu.Lock()
	key, _ := requestKey(json.RawMessage(`"active"`))
	server.requests[key] = &requestState{cancel: requestCancel}
	server.mu.Unlock()
	if err := server.Handle(context.Background(), []byte(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":"act\u0069ve"}}`), capture.send); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(requestContext.Err(), context.Canceled) {
		t.Fatal("cancelRequest did not cancel active request")
	}
	started := make(chan struct{})
	stopped := make(chan struct{})
	server = NewServer(Options{Analyze: func(ctx context.Context, doc Document) (*Snapshot, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	}})
	defer server.Close()
	initializeTest(t, server, capture)
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, "pending", 1))
	<-started
	handleTest(t, server, capture, "shutdown", 100, nil)
	<-stopped
	waitTest(t, server)
	got = capture.take()
	if len(got) != 1 || !bytes.Contains(got[0], []byte(`"id":100`)) {
		t.Fatal(got)
	}
}
