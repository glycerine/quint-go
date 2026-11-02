package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/glycerine/insdict"
)

type Options struct {
	// Analyze overrides document analysis, primarily for embedding and tests.
	// Returned snapshots become immutable server-owned values.
	Analyze func(context.Context, Document) (*Snapshot, error)
}
type documentState struct {
	Document
	generation uint64
	cancel     context.CancelFunc
	snapshot   *Snapshot
}
type requestState struct{ cancel context.CancelFunc }
type Server struct {
	mu          sync.Mutex
	options     Options
	documents   *insdict.Dict[string, *documentState]
	requests    map[string]*requestState
	jobs        sync.WaitGroup
	initialized bool
	shutdown    bool
	exited      bool
}

func NewServer(options Options) *Server {
	return &Server{options: options, documents: insdict.NewDict[string, *documentState](), requests: map[string]*requestState{}}
}

var errExit = errors.New("LSP exit")
var ErrExitBeforeShutdown = errors.New("LSP exit received before shutdown")

// Serve handles Content-Length framed JSON-RPC until exit, EOF, or cancellation.
// It serializes output across analysis workers. On return it closes an input
// implementing io.Closer so a blocked read can finish, and drains worker jobs.
func (s *Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		if closer, ok := input.(io.Closer); ok {
			_ = closer.Close()
		}
		s.Close()
		_ = s.Wait(context.Background())
	}()
	type incoming struct {
		body []byte
		err  error
	}
	messages := make(chan incoming)
	go func() {
		reader := bufio.NewReader(input)
		for {
			body, err := ReadMessage(reader)
			select {
			case messages <- incoming{body, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	var writeMu sync.Mutex
	writeErrors := make(chan error, 1)
	send := func(message any) error {
		writeMu.Lock()
		err := WriteMessage(output, message)
		writeMu.Unlock()
		if err != nil {
			select {
			case writeErrors <- err:
			default:
			}
		}
		return err
	}
	for {
		select {
		case <-ctx.Done():
			if closer, ok := input.(io.Closer); ok {
				_ = closer.Close()
			}
			return ctx.Err()
		case err := <-writeErrors:
			return err
		case message := <-messages:
			if message.err == io.EOF {
				return nil
			}
			if message.err != nil {
				return message.err
			}
			if err := s.Handle(ctx, message.body, send); err != nil {
				if errors.Is(err, errExit) {
					s.mu.Lock()
					shutdown := s.shutdown
					s.mu.Unlock()
					if !shutdown {
						return ErrExitBeforeShutdown
					}
					return nil
				}
				return err
			}
		}
	}
}

func respond(send func(any) error, id json.RawMessage, result any, rpcError *ResponseError) error {
	if id == nil {
		id = json.RawMessage("null")
	}
	if rpcError != nil {
		return send(struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   *ResponseError  `json:"error"`
		}{"2.0", id, rpcError})
	}
	return send(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result"`
	}{"2.0", id, result})
}
func notify(send func(any) error, method string, params any) error {
	return send(struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{"2.0", method, params})
}

func requestKey(id json.RawMessage) (string, bool) {
	decoder := json.NewDecoder(bytes.NewReader(id))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return "", false
	}
	switch value := value.(type) {
	case nil:
		return "null", true
	case string:
		return "string:" + value, true
	case json.Number:
		return "number:" + value.String(), true
	}
	return "", false
}

// Handle accepts one JSON-RPC object. Notifications can schedule asynchronous
// analysis; send must support concurrent calls. Wait drains currently scheduled
// jobs when a caller needs a deterministic barrier (for example in tests).
func (s *Server) Handle(ctx context.Context, body []byte, send func(any) error) error {
	if send == nil {
		return fmt.Errorf("LSP response writer is required")
	}
	body = bytes.TrimSpace(body)
	var message struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if !json.Valid(body) {
		return respond(send, nil, nil, &ResponseError{Code: -32700, Message: "Parse error"})
	}
	if len(body) == 0 || body[0] != '{' || json.Unmarshal(body, &message) != nil || message.JSONRPC != "2.0" || message.Method == "" {
		return respond(send, nil, nil, &ResponseError{Code: -32600, Message: "Invalid Request"})
	}
	if message.ID != nil {
		if _, valid := requestKey(message.ID); !valid {
			return respond(send, nil, nil, &ResponseError{Code: -32600, Message: "Invalid request id"})
		}
	}
	fail := func(code int, text string) error {
		if message.ID == nil {
			return notify(send, "window/logMessage", struct {
				Type    int    `json:"type"`
				Message string `json:"message"`
			}{1, text})
		}
		return respond(send, message.ID, nil, &ResponseError{Code: code, Message: text})
	}
	s.mu.Lock()
	initialized, shutdown := s.initialized, s.shutdown
	s.mu.Unlock()
	if message.Method == "exit" {
		s.mu.Lock()
		s.exited = true
		s.mu.Unlock()
		return errExit
	}
	if message.Method == "initialize" {
		if message.ID == nil {
			return nil
		}
		if initialized {
			return fail(-32600, "Server is already initialized")
		}
		s.mu.Lock()
		s.initialized = true
		s.mu.Unlock()
		return respond(send, message.ID, map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "textDocumentSync": map[string]any{"openClose": true, "change": 2, "save": map[string]any{"includeText": false}}, "hoverProvider": true, "definitionProvider": true, "referencesProvider": true, "documentSymbolProvider": true, "completionProvider": map[string]any{"resolveProvider": false, "triggerCharacters": []string{"."}}, "renameProvider": map[string]any{"prepareProvider": true}}, "serverInfo": map[string]any{"name": "quint-go"}}, nil)
	}
	if !initialized {
		return fail(-32002, "Server is not initialized")
	}
	if shutdown {
		return fail(-32600, "Server has shut down")
	}
	switch message.Method {
	case "initialized", "$/setTrace":
		return nil
	case "shutdown":
		s.mu.Lock()
		s.shutdown = true
		s.cancelLocked()
		s.mu.Unlock()
		if message.ID != nil {
			return respond(send, message.ID, nil, nil)
		}
		return nil
	case "$/cancelRequest":
		var params struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(message.Params, &params) != nil {
			return nil
		}
		s.mu.Lock()
		key, _ := requestKey(params.ID)
		if request := s.requests[key]; request != nil {
			request.cancel()
		}
		s.mu.Unlock()
		return nil
	case "textDocument/didOpen":
		var params struct {
			TextDocument struct {
				URI        string `json:"uri"`
				LanguageID string `json:"languageId"`
				Version    int    `json:"version"`
				Text       string `json:"text"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(message.Params, &params) != nil || params.TextDocument.URI == "" {
			return fail(-32602, "Invalid didOpen parameters")
		}
		if _, err := sourceForURI(params.TextDocument.URI); err != nil {
			return fail(-32602, err.Error())
		}
		doc := Document{URI: params.TextDocument.URI, LanguageID: params.TextDocument.LanguageID, Version: params.TextDocument.Version, Text: params.TextDocument.Text}
		if _, err := applyChanges(doc.Text, nil); err != nil {
			return fail(-32602, err.Error())
		}
		s.mu.Lock()
		if previous := s.documents.Get(doc.URI); previous != nil && previous.cancel != nil {
			previous.cancel()
		}
		s.documents.Set(doc.URI, &documentState{Document: doc})
		s.scheduleAllLocked(ctx, send)
		s.mu.Unlock()
		return nil
	case "textDocument/didChange":
		var params struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
			} `json:"textDocument"`
			ContentChanges []contentChange `json:"contentChanges"`
		}
		if json.Unmarshal(message.Params, &params) != nil {
			return fail(-32602, "Invalid didChange parameters")
		}
		s.mu.Lock()
		state := s.documents.Get(params.TextDocument.URI)
		if state == nil {
			s.mu.Unlock()
			return fail(-32602, "Document is not open")
		}
		if params.TextDocument.Version <= state.Version {
			s.mu.Unlock()
			return nil
		}
		text, err := applyChanges(state.Text, params.ContentChanges)
		if err != nil {
			s.mu.Unlock()
			return fail(-32602, err.Error())
		}
		state.Text = text
		state.Version = params.TextDocument.Version
		s.scheduleAllLocked(ctx, send)
		s.mu.Unlock()
		return nil
	case "textDocument/didSave":
		s.mu.Lock()
		s.scheduleAllLocked(ctx, send)
		s.mu.Unlock()
		return nil
	case "textDocument/didClose":
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(message.Params, &params) != nil {
			return fail(-32602, "Invalid didClose parameters")
		}
		s.mu.Lock()
		if state := s.documents.Get(params.TextDocument.URI); state != nil {
			if state.cancel != nil {
				state.cancel()
			}
			s.documents.Del(params.TextDocument.URI)
		}
		s.scheduleAllLocked(ctx, send)
		s.mu.Unlock()
		return publish(send, params.TextDocument.URI, nil, []Diagnostic{})
	default:
		if message.ID == nil {
			return nil
		}
		switch message.Method {
		case "textDocument/hover", "textDocument/definition", "textDocument/references", "textDocument/documentSymbol", "textDocument/completion", "textDocument/rename", "textDocument/prepareRename":
		default:
			return fail(-32601, "Method not found")
		}
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(message.Params, &params) != nil {
			return fail(-32602, "Invalid request parameters")
		}
		s.mu.Lock()
		state := s.documents.Get(params.TextDocument.URI)
		var snapshot *Snapshot
		if state != nil {
			snapshot = state.snapshot
			if snapshot != nil && snapshot.Version != state.Version {
				snapshot = nil
			}
		}
		s.mu.Unlock()
		if snapshot == nil {
			return respond(send, message.ID, nil, nil)
		}
		requestCtx, cancel := context.WithCancel(ctx)
		request := &requestState{cancel: cancel}
		key, _ := requestKey(message.ID)
		s.mu.Lock()
		if s.requests[key] != nil {
			s.mu.Unlock()
			cancel()
			return fail(-32600, "Duplicate request id")
		}
		s.requests[key] = request
		s.jobs.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.jobs.Done()
			defer cancel()
			result, rpcError := semanticRequest(message.Method, message.Params, snapshot)
			if requestCtx.Err() != nil {
				result = nil
				rpcError = &ResponseError{Code: -32800, Message: "Request cancelled"}
			}
			s.mu.Lock()
			delete(s.requests, key)
			exited := s.exited
			s.mu.Unlock()
			if !exited {
				_ = respond(send, message.ID, result, rpcError)
			}
		}()
		return nil
	}
}
func publish(send func(any) error, uri string, version *int, diagnostics []Diagnostic) error {
	return notify(send, "textDocument/publishDiagnostics", struct {
		URI         string       `json:"uri"`
		Version     *int         `json:"version,omitempty"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}{uri, version, diagnostics})
}
func (s *Server) scheduleAllLocked(ctx context.Context, send func(any) error) {
	overlays := insdict.NewDict[string, string]()
	for _, state := range s.documents.All() {
		path, err := sourceForURI(state.URI)
		if err == nil {
			overlays.Set(path, state.Text)
		}
	}
	for _, state := range s.documents.All() {
		if state.cancel != nil {
			state.cancel()
		}
		state.snapshot = nil
		state.generation++
		generation := state.generation
		document := state.Document
		analysisCtx, cancel := context.WithCancel(ctx)
		state.cancel = cancel
		s.jobs.Add(1)
		go func() {
			defer s.jobs.Done()
			defer cancel()
			var snapshot *Snapshot
			var err error
			if s.options.Analyze != nil {
				snapshot, err = s.options.Analyze(analysisCtx, document)
			} else {
				snapshot, err = analyzeDocument(analysisCtx, document, overlays)
			}
			if analysisCtx.Err() != nil {
				return
			}
			if snapshot == nil {
				snapshot = &Snapshot{URI: document.URI, Version: document.Version, Text: document.Text, Diagnostics: []Diagnostic{}}
			}
			if err != nil {
				snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Severity: 1, Source: "quint", Message: err.Error()})
			}
			if snapshot.Diagnostics == nil {
				snapshot.Diagnostics = []Diagnostic{}
			}
			s.mu.Lock()
			current := s.documents.Get(document.URI)
			if current == nil || current != state || current.generation != generation || s.shutdown || s.exited {
				s.mu.Unlock()
				return
			}
			current.snapshot = snapshot
			// Publication shares the lock with version changes, preventing an old
			// result from being emitted after a newer didChange is accepted.
			_ = publish(send, document.URI, &document.Version, snapshot.Diagnostics)
			s.mu.Unlock()
		}()
	}
}
func (s *Server) cancelLocked() {
	for _, state := range s.documents.All() {
		if state.cancel != nil {
			state.cancel()
		}
	}
	for _, request := range s.requests {
		request.cancel()
	}
}

// Close cancels outstanding analyses and requests. It does not own caller I/O.
func (s *Server) Close() { s.mu.Lock(); s.exited = true; s.cancelLocked(); s.mu.Unlock() }
func (s *Server) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() { s.jobs.Wait(); close(done) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}
