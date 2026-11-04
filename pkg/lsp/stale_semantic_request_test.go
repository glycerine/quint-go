package lsp

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glycerine/insdict"
)

func TestServerSemanticRequestsRejectStaleSnapshots(t *testing.T) {
	for _, event := range []string{"change", "close"} {
		t.Run(event, func(t *testing.T) {
			server := NewServer(Options{})
			defer server.Close()
			capture := &capturedMessages{}
			initializeTest(t, server, capture)
			uri := "untitled:stale-semantic"
			text := `module M { pure val target = 1; pure val use = target }`
			handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, text, 1))
			waitTest(t, server)
			capture.take()

			entered := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			server.serverSnapshotSourcesForTest(uri, entered, release, &once)
			position := semanticTestPosition(t, text, "target", 0)
			params := map[string]any{
				"textDocument": map[string]any{"uri": uri},
				"position":     position,
				"newName":      "renamed",
			}
			handleTest(t, server, capture, "textDocument/rename", 44, params)
			<-entered
			if event == "change" {
				handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(uri, strings.Replace(text, "target = 1", "target = 2", 1), 2))
			} else {
				handleTest(t, server, capture, "textDocument/didClose", nil, map[string]any{"textDocument": map[string]any{"uri": uri}})
			}
			close(release)
			waitTest(t, server)
			responses := capture.take()
			var response struct {
				ID    int `json:"id"`
				Error *struct {
					Code int `json:"code"`
				} `json:"error"`
				Result json.RawMessage `json:"result"`
			}
			found := false
			for _, body := range responses {
				var message struct {
					ID json.RawMessage `json:"id"`
				}
				if json.Unmarshal(body, &message) != nil || len(message.ID) == 0 {
					continue
				}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatal(err)
				}
				if response.ID == 44 {
					found = true
					break
				}
			}
			if !found || response.Error == nil || response.Error.Code != lspContentModified || len(response.Result) != 0 {
				t.Fatalf("stale rename response missing ContentModified: %s", responses)
			}
		})
	}
}

func (s *Server) serverSnapshotSourcesForTest(uri string, entered, release chan struct{}, once *sync.Once) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.documents.Get(uri)
	if state == nil || state.snapshot == nil {
		panic("test document snapshot missing")
	}
	var enabled atomic.Bool
	state.snapshot.Sources = insdict.NewDictFunc[string, string](func(key string) uint64 {
		if enabled.Load() {
			once.Do(func() {
				close(entered)
				<-release
			})
		}
		return insdict.EasyHashString(key)
	})
	state.snapshot.Sources.Set(uri, state.Text)
	enabled.Store(true)
}
