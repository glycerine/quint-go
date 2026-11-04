package lsp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Deadlines are failure watchdogs; channels establish every test ordering.
func lifecycleAwait(t *testing.T, event <-chan struct{}) {
	t.Helper()
	select {
	case <-event:
	case <-time.After(5 * time.Second):
		t.Fatal("resource lifecycle did not complete")
	}
}

type lifecycleReader struct {
	io.Reader
	reads, closes atomic.Int32
}

func (reader *lifecycleReader) Read(p []byte) (int, error) {
	reader.reads.Add(1)
	return reader.Reader.Read(p)
}
func (reader *lifecycleReader) Close() error {
	reader.closes.Add(1)
	if closer, ok := reader.Reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

type lifecycleBlockedReader struct {
	started, released, returned chan struct{}
	start, release              sync.Once
	closes                      atomic.Int32
}

func (reader *lifecycleBlockedReader) Read([]byte) (int, error) {
	reader.start.Do(func() { close(reader.started) })
	<-reader.released
	close(reader.returned)
	return 0, io.EOF
}
func (reader *lifecycleBlockedReader) Close() error {
	reader.closes.Add(1)
	reader.release.Do(func() { close(reader.released) })
	return nil
}

func TestServeCancellationClosesInputOnce(t *testing.T) {
	t.Parallel()
	t.Run("already-cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		input := &lifecycleReader{Reader: strings.NewReader("invalid input must not be inspected")}
		var output bytes.Buffer
		if err := NewServer(Options{}).Serve(ctx, input, &output); !errors.Is(err, context.Canceled) {
			t.Fatal("pre-cancelled Serve", err)
		}
		if input.closes.Load() != 1 || input.reads.Load() != 0 || output.Len() != 0 {
			t.Fatal("pre-cancelled Serve acquired input or emitted output", input.closes.Load(), input.reads.Load(), output.String())
		}
	})
	t.Run("blocked-read", func(t *testing.T) {
		input := &lifecycleBlockedReader{started: make(chan struct{}), released: make(chan struct{}), returned: make(chan struct{})}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer input.release.Do(func() { close(input.released) })
		finished := make(chan struct{})
		var serveErr error
		go func() {
			serveErr = NewServer(Options{}).Serve(ctx, input, io.Discard)
			close(finished)
		}()
		lifecycleAwait(t, input.started)
		cancel()
		lifecycleAwait(t, finished)
		lifecycleAwait(t, input.returned)
		if !errors.Is(serveErr, context.Canceled) || input.closes.Load() != 1 {
			t.Fatal("cancellation did not release the blocked reader once", serveErr, input.closes.Load())
		}
	})
}

func lifecycleFrames(t *testing.T, messages ...any) *bytes.Reader {
	t.Helper()
	var stream bytes.Buffer
	for _, message := range messages {
		if err := WriteMessage(&stream, message); err != nil {
			t.Fatal(err)
		}
	}
	return bytes.NewReader(stream.Bytes())
}

func TestServeStreamEndsReleaseInputAndWorkers(t *testing.T) {
	t.Parallel()
	initialize := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"}
	for _, example := range []struct {
		name   string
		input  io.Reader
		output io.Writer
		want   error
	}{
		{"empty-eof", strings.NewReader(""), io.Discard, nil},
		{"truncated-frame", strings.NewReader("Content-Length: 10\r\n\r\n{}"), io.Discard, io.ErrUnexpectedEOF},
		{"request-write-error", lifecycleFrames(t, initialize), failWriter{}, errWrite},
	} {
		t.Run(example.name, func(t *testing.T) {
			input := &lifecycleReader{Reader: example.input}
			err := NewServer(Options{}).Serve(context.Background(), input, example.output)
			if !errors.Is(err, example.want) || input.closes.Load() != 1 {
				t.Fatal("stream error/ownership", err, example.want, input.closes.Load())
			}
		})
	}
	started, stopped := make(chan struct{}), make(chan struct{})
	server := NewServer(Options{Analyze: func(ctx context.Context, document Document) (*Snapshot, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	}})
	reader, writer := io.Pipe()
	defer writer.Close()
	input := &lifecycleReader{Reader: reader}
	var output bytes.Buffer
	finished := make(chan struct{})
	var serveErr error
	go func() {
		serveErr = server.Serve(context.Background(), input, &output)
		close(finished)
	}()
	if err := WriteMessage(writer, initialize); err != nil {
		t.Fatal(err)
	}
	if err := WriteMessage(writer, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": openParams("untitled:eof-worker", "pending", 1),
	}); err != nil {
		t.Fatal(err)
	}
	lifecycleAwait(t, started)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	lifecycleAwait(t, finished)
	if serveErr != nil {
		t.Fatal(serveErr)
	}
	lifecycleAwait(t, stopped)
	if input.closes.Load() != 1 || bytes.Contains(output.Bytes(), []byte("publishDiagnostics")) {
		t.Fatal("EOF failed to drain its cancelled analysis", input.closes.Load(), output.String())
	}
}

type lifecycleOutput struct {
	writes, closes atomic.Int32
	initialized    chan struct{}
	failed         chan struct{}
}

func (output *lifecycleOutput) Write(data []byte) (int, error) {
	if output.writes.Add(1) == 1 {
		close(output.initialized)
		return len(data), nil
	}
	close(output.failed)
	return 0, errWrite
}
func (output *lifecycleOutput) Close() error { output.closes.Add(1); return nil }

func TestServeAsyncPublicationWriteErrorReleasesResources(t *testing.T) {
	t.Parallel()
	reader, writer := io.Pipe()
	defer writer.Close()
	input := &lifecycleReader{Reader: reader}
	output := &lifecycleOutput{initialized: make(chan struct{}), failed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := NewServer(Options{Analyze: func(_ context.Context, document Document) (*Snapshot, error) {
		return &Snapshot{URI: document.URI, Version: document.Version, Text: document.Text}, nil
	}})
	finished := make(chan struct{})
	var serveErr error
	go func() {
		serveErr = server.Serve(ctx, input, output)
		close(finished)
	}()
	if err := WriteMessage(writer, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"}); err != nil {
		t.Fatal(err)
	}
	lifecycleAwait(t, output.initialized)
	if err := WriteMessage(writer, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": openParams("untitled:async-write", "pending", 1)}); err != nil {
		t.Fatal(err)
	}
	lifecycleAwait(t, output.failed)
	lifecycleAwait(t, finished)
	if !errors.Is(serveErr, errWrite) || input.closes.Load() != 1 || output.closes.Load() != 0 {
		t.Fatal("async write failure ownership", serveErr, input.closes.Load(), output.closes.Load())
	}
	server.mu.Lock()
	active := len(server.requests)
	server.mu.Unlock()
	if active != 0 {
		t.Fatal("request resources survived Serve failure", active)
	}
}

func TestClosedServerRejectsNewHandleWork(t *testing.T) {
	t.Parallel()
	var analyses atomic.Int32
	server := NewServer(Options{Analyze: func(_ context.Context, document Document) (*Snapshot, error) {
		analyses.Add(1)
		return &Snapshot{URI: document.URI, Version: document.Version, Text: document.Text}, nil
	}})
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	server.Close()
	body := []byte(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"untitled:after-close","text":"module M {}","version":1}}}`)
	if err := server.Handle(context.Background(), body, capture.send); !errors.Is(err, ErrServerClosed) {
		t.Fatal("closed server accepted a document", err)
	}
	waitTest(t, server)
	if analyses.Load() != 0 || len(capture.take()) != 0 {
		t.Fatal("closed server scheduled or published new work", analyses.Load())
	}
	server.mu.Lock()
	count := server.documents.Len()
	server.mu.Unlock()
	if count != 0 {
		t.Fatal("closed server retained a newly opened document", count)
	}
	if err := server.Serve(context.Background(), strings.NewReader(""), io.Discard); !errors.Is(err, ErrServerClosed) {
		t.Fatal("closed server accepted another Serve session", err)
	}
}

type lifecycleLogFile struct {
	started, release chan struct{}
	writes, closes   atomic.Int32
}

func (file *lifecycleLogFile) Write(data []byte) (int, error) {
	file.writes.Add(1)
	close(file.started)
	<-file.release
	return len(data), nil
}
func (file *lifecycleLogFile) Close() error { file.closes.Add(1); return nil }

func TestLoggerCloseOverlapsLiveWrite(t *testing.T) {
	t.Parallel()
	file := &lifecycleLogFile{started: make(chan struct{}), release: make(chan struct{})}
	logger := NewLogger("TRACE", nil)
	logger.filename = "owned.log"
	logger.open = func(string) (io.WriteCloser, error) { return file, nil }
	written := make(chan struct{})
	go func() { logger.Log("INFO", "one live message"); close(written) }()
	lifecycleAwait(t, file.started)
	closing, closed := make(chan struct{}), make(chan struct{})
	var closeErr error
	go func() { close(closing); closeErr = logger.Close(); close(closed) }()
	lifecycleAwait(t, closing)
	if file.closes.Load() != 0 {
		t.Fatal("logger closed its file during an active write")
	}
	close(file.release)
	lifecycleAwait(t, written)
	lifecycleAwait(t, closed)
	if closeErr != nil || file.closes.Load() != 1 || file.writes.Load() != 1 {
		t.Fatal("logger live close lifecycle", closeErr, file.closes.Load(), file.writes.Load())
	}
	logger.Log("ERROR", "after close")
	if err := logger.Close(); err != nil || file.closes.Load() != 1 || file.writes.Load() != 1 {
		t.Fatal("logger reopened or rewrote after close", err, file.closes.Load(), file.writes.Load())
	}
}

type lifecycleWaitContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (ctx *lifecycleWaitContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.entered) })
	return ctx.Context.Done()
}

func TestServerWaitCancellationGenerationReuseAndConcurrentClose(t *testing.T) {
	t.Parallel()
	started := make(chan context.Context, 2)
	releaseFirst := make(chan struct{})
	server := NewServer(Options{Analyze: func(ctx context.Context, document Document) (*Snapshot, error) {
		started <- ctx
		if document.Version == 1 {
			select {
			case <-releaseFirst:
			case <-ctx.Done():
			}
		} else {
			<-ctx.Done()
		}
		return &Snapshot{URI: document.URI, Version: document.Version, Text: document.Text}, ctx.Err()
	}})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := server.Wait(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled idle wait", err)
	}
	uri := "untitled:wait-generations"
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams(uri, "first", 1))
	first := <-started
	waitCtx, cancelWait := context.WithCancel(context.Background())
	observed := &lifecycleWaitContext{Context: waitCtx, entered: make(chan struct{})}
	waited := make(chan struct{})
	var waitErr error
	go func() { waitErr = server.Wait(observed); close(waited) }()
	lifecycleAwait(t, observed.entered)
	cancelWait()
	lifecycleAwait(t, waited)
	if !errors.Is(waitErr, context.Canceled) || first.Err() != nil {
		t.Fatal("Wait cancellation affected the document analysis", waitErr, first.Err())
	}
	close(releaseFirst)
	waitTest(t, server)
	if got := diagnosticsTest(t, capture.take()); len(got) != 1 || got[0].Version == nil || *got[0].Version != 1 {
		t.Fatal("first analysis did not drain after cancelling only its waiter", got)
	}
	handleTest(t, server, capture, "textDocument/didChange", nil, changeParams(uri, "second", 2))
	second := <-started
	var waiters sync.WaitGroup
	results := make(chan error, 6)
	for range 6 {
		ctx := &lifecycleWaitContext{Context: context.Background(), entered: make(chan struct{})}
		waiters.Add(1)
		go func() { defer waiters.Done(); results <- server.Wait(ctx) }()
		lifecycleAwait(t, ctx.entered)
	}
	// Every waiter has observed the second active generation. Close cancels
	// its analysis and all waiters must share the new generation's idle event.
	server.Close()
	drained := make(chan struct{})
	go func() { waiters.Wait(); close(drained) }()
	lifecycleAwait(t, drained)
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal("concurrent drain failed", err)
		}
	}
	if !errors.Is(second.Err(), context.Canceled) {
		t.Fatal("Close did not cancel the second generation", second.Err())
	}
	if got := capture.take(); len(got) != 0 {
		t.Fatal("cancelled second generation published diagnostics", got)
	}
	waitTest(t, server)
}
