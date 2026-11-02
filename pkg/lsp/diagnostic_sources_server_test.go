package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerDiagnosticOwnersAcrossImportedBuffers(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	libraryPath := filepath.Join(directory, "lib.qnt")
	library := "module Lib { var state:int pure val good=1 pure val bad=state }"
	if err := os.WriteFile(libraryPath, []byte(library), 0600); err != nil {
		t.Fatal(err)
	}
	main := "module Main { import Lib.* from \"lib\" pure val use=good }"
	mainURI, libraryURI := uriForSource(filepath.Join(directory, "main.qnt")), uriForSource(libraryPath)
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(mainURI, main, 8))
	waitTest(t, server)
	publications := diagnosticsTest(t, capture.take())
	if len(publications) != 1 || publications[0].URI != mainURI || len(publications[0].Diagnostics) != 0 {
		t.Fatal("imported error was attached to the main file", publications)
	}
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(libraryURI, library, 3))
	waitTest(t, server)
	publications = diagnosticsTest(t, capture.take())
	if len(publications) != 2 {
		t.Fatal(publications)
	}
	foundMain, foundLibrary := false, false
	for _, publication := range publications {
		switch publication.URI {
		case mainURI:
			foundMain = true
			if len(publication.Diagnostics) != 0 || publication.Version == nil || *publication.Version != 8 {
				t.Fatal(publication)
			}
		case libraryURI:
			foundLibrary = true
			if len(publication.Diagnostics) != 1 || publication.Version == nil || *publication.Version != 3 {
				t.Fatal(publication)
			}
			diagnostic := publication.Diagnostics[0]
			if diagnostic.Code != "QNT200" || len(diagnostic.RelatedInformation) != 0 || !strings.Contains(string(diagnostic.Data), `"fix"`) {
				t.Fatal("origin diagnostic lost its actionable fix", diagnostic)
			}
		default:
			t.Fatal("unopened URI received diagnostics", publication)
		}
	}
	if !foundMain || !foundLibrary {
		t.Fatal(publications)
	}
	fixed := strings.Replace(library, "pure val bad", "val bad", 1)
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(libraryURI, fixed, 4))
	waitTest(t, server)
	publications = diagnosticsTest(t, capture.take())
	if len(publications) != 2 {
		t.Fatal(publications)
	}
	for _, publication := range publications {
		if len(publication.Diagnostics) != 0 {
			t.Fatal("unsaved imported fix did not clear its own diagnostics", publication)
		}
	}
	if data, err := os.ReadFile(libraryPath); err != nil || string(data) != library {
		t.Fatal("analysis wrote the unsaved imported buffer to disk", err, string(data))
	}
}

func TestServerRecoveredSyntaxRetainsCurrentNavigation(t *testing.T) {
	t.Parallel()
	server := NewServer(Options{})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	uri := "untitled:recovered-navigation"
	text := "module Main { pure val valid=1 pure val broken= pure val use=valid }\n"
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, text, 1))
	waitTest(t, server)
	publications := diagnosticsTest(t, capture.take())
	if len(publications) != 1 || len(publications[0].Diagnostics) != 1 || publications[0].Diagnostics[0].Code != "QNT000" {
		t.Fatal(publications)
	}
	server.mu.Lock()
	snapshot := server.documents.Get(uri).snapshot
	server.mu.Unlock()
	if snapshot == nil || snapshot.Frontend.Table == nil || snapshot.Analysis.Types != nil || snapshot.Documentation != nil {
		t.Fatal("recovered parsing was lost or stale analysis was retained", snapshot)
	}
	assertDefinition := func(id int, current string) {
		t.Helper()
		params := map[string]any{"textDocument": map[string]any{"uri": uri}, "position": semanticTestPosition(t, current, "valid", 1)}
		handleTest(t, server, capture, "textDocument/definition", id, params)
		waitTest(t, server)
		messages := capture.take()
		var response struct {
			Result []Location
			Error  *ResponseError
		}
		if len(messages) != 1 || json.Unmarshal(messages[0], &response) != nil || response.Error != nil || len(response.Result) != 1 {
			t.Fatal(messages)
		}
		server.mu.Lock()
		currentSnapshot := server.documents.Get(uri).snapshot
		server.mu.Unlock()
		if response.Result[0].URI != uri || semanticTestRangeText(t, currentSnapshot, response.Result[0]) != "valid" {
			t.Fatal(response)
		}
	}
	assertDefinition(2, text)
	fixed := "module Main { pure val valid=1 pure val use=valid }\n"
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(uri, fixed, 2))
	waitTest(t, server)
	publications = diagnosticsTest(t, capture.take())
	if len(publications) != 1 || len(publications[0].Diagnostics) != 0 || publications[0].Version == nil || *publications[0].Version != 2 {
		t.Fatal(publications)
	}
	server.mu.Lock()
	snapshot = server.documents.Get(uri).snapshot
	server.mu.Unlock()
	if snapshot == nil || snapshot.Analysis.Types == nil || snapshot.Documentation == nil || snapshot.Text != fixed {
		t.Fatal("fixed source did not receive fresh analysis", snapshot)
	}
	assertDefinition(3, fixed)
}
