package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func loggerClock() time.Time {
	return time.Date(2001, 2, 3, 4, 5, 6, 789000000, time.UTC)
}

func TestUpstreamLoggerConfiguration(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Thresholds []struct {
			Level   string
			Enabled map[string]bool
			Lines   []string
			Opened  []string
		}
		Paths []struct {
			Env  map[string]string
			Path string
		}
		Formatted string
	}
	data, err := os.ReadFile("testdata/logger.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Thresholds) != 11 || len(fixture.Paths) != 6 {
		t.Fatal("upstream logger fixture inventory")
	}
	for _, example := range fixture.Thresholds {
		t.Run(example.Level, func(t *testing.T) {
			var output bytes.Buffer
			level := example.Level
			if level == "" {
				level = "NONE"
			}
			logger := NewLogger(level, &output)
			logger.now = loggerClock
			for level, want := range example.Enabled {
				if got := logger.Enabled(level); got != want {
					t.Fatalf("%s enabled=%v; upstream=%v", level, got, want)
				}
			}
			for _, level := range []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR"} {
				logger.Log(level, "%s message", level)
			}
			if output.String() != strings.Join(example.Lines, "") {
				t.Fatal(output.String(), example.Lines)
			}
		})
	}
	for _, example := range fixture.Paths {
		got := environmentLogPath(func(key string) (string, bool) {
			value, exists := example.Env[key]
			return value, exists
		}, "temp")
		if want := filepath.FromSlash(example.Path); got != want {
			t.Fatal(example.Env, got, want)
		}
	}
	var output bytes.Buffer
	logger := NewLogger("TRACE", &output)
	logger.now = loggerClock
	logger.Log("INFO", "module=%s count=%d", "😀/M", 7)
	if output.String() != fixture.Formatted {
		t.Fatal(output.String(), fixture.Formatted)
	}
}

type loggerWriter struct {
	bytes.Buffer
	writes, closes int
	writeErr       error
	closeErr       error
}

func (w *loggerWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.Buffer.Write(p)
}
func (w *loggerWriter) Close() error { w.closes++; return w.closeErr }

func TestLoggerLazyFileAndOwnership(t *testing.T) {
	t.Parallel()
	output := &loggerWriter{}
	logger := NewLogger("DEBUG", nil)
	logger.now = loggerClock
	opens := 0
	logger.filename = "owned.log"
	logger.open = func(filename string) (io.WriteCloser, error) {
		opens++
		if filename != "owned.log" {
			t.Fatal(filename)
		}
		return output, nil
	}
	logger.Log("TRACE", "filtered")
	if opens != 0 {
		t.Fatal("filtered message opened a file")
	}
	logger.Log("DEBUG", "first")
	logger.Log("INFO", "second")
	if opens != 1 || output.writes != 2 {
		t.Fatal(opens, output.writes)
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	logger.Log("ERROR", "after close")
	if err := logger.Close(); err != nil || output.closes != 1 || output.writes != 2 {
		t.Fatal(err, output.closes, output.writes)
	}
	caller := &loggerWriter{}
	logger = NewLogger("INFO", caller)
	logger.Log("INFO", "caller")
	if err := logger.Close(); err != nil || caller.closes != 0 {
		t.Fatal("closed caller-owned output", err, caller.closes)
	}
	if err := (*Logger)(nil).Close(); err != nil || (*Logger)(nil).Enabled("INFO") {
		t.Fatal("nil logger", err)
	}
	(*Logger)(nil).Log("ERROR", "ignored")
	NewLogger("INFO", nil).Log("INFO", "discard")
}

func TestLoggerFailuresDisableOnlyLogging(t *testing.T) {
	t.Parallel()
	failure := errors.New("logging unavailable")
	for _, openFails := range []bool{false, true} {
		output := &loggerWriter{writeErr: failure}
		logger := NewLogger("INFO", nil)
		opens := 0
		logger.open = func(string) (io.WriteCloser, error) {
			opens++
			if openFails {
				return nil, failure
			}
			return output, nil
		}
		logger.Log("INFO", "first")
		logger.Log("INFO", "second")
		if opens != 1 || !logger.unavailable || output.writes > 1 {
			t.Fatal(opens, output.writes, logger.unavailable)
		}
		_ = logger.Close()
	}
	output := &loggerWriter{closeErr: failure}
	logger := NewLogger("INFO", nil)
	logger.open = func(string) (io.WriteCloser, error) { return output, nil }
	logger.Log("INFO", "open")
	if err := logger.Close(); !errors.Is(err, failure) {
		t.Fatal("lost owned close error", err)
	}
}

func TestEnvironmentLoggerCapturesConfiguration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("LOG_LEVEL", "debug")
	logger := NewEnvironmentLogger()
	defer logger.Close()
	filename := filepath.Join(dir, "quint-lsp.log")
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatal("construction created log", err)
	}
	logger.Log("TRACE", "filtered")
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatal("filtered message created log", err)
	}
	t.Setenv("LOG_LEVEL", "NONE")
	t.Setenv("HOME", filepath.Join(dir, "different"))
	logger.Log("DEBUG", "captured")
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filename)
	if err != nil || !strings.Contains(string(data), "[DEBUG] captured\n") {
		t.Fatal(string(data), err)
	}
	logger = NewEnvironmentLogger()
	logger.Log("INFO", "disabled")
	_ = logger.Close()
	if _, err := os.Stat(filepath.Join(dir, "different")); !os.IsNotExist(err) {
		t.Fatal("disabled logging touched unavailable home", err)
	}
	t.Setenv("LOG_LEVEL", "")
	logger = NewEnvironmentLogger()
	if logger.Enabled("INFO") {
		t.Fatal("empty environment level did not default to NONE")
	}
	logger.Log("ERROR", "default disabled")
	_ = logger.Close()
}

func TestLoggerSerializesConcurrentWorkers(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := NewLogger("TRACE", &output)
	logger.now = loggerClock
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			for j := 0; j < 10; j++ {
				logger.Log("DEBUG", "worker=%d message=%d", i, j)
			}
		}(i)
	}
	workers.Wait()
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 80 {
		t.Fatal("lost concurrent log messages", len(lines))
	}
	seen := map[string]bool{}
	for _, line := range lines {
		seen[line] = true
	}
	for i := 0; i < 8; i++ {
		for j := 0; j < 10; j++ {
			if !seen[fmt.Sprintf("[2001-02-03 04:05:06.789] [DEBUG] worker=%d message=%d", i, j)] {
				t.Fatal("interleaved concurrent write", i, j)
			}
		}
	}
}

func TestServerLoggingStaysOutsideProtocol(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := NewLogger("TRACE", &logs)
	logger.now = loggerClock
	server := NewServer(Options{Logger: logger, Analyze: func(_ context.Context, document Document) (*Snapshot, error) {
		return &Snapshot{URI: document.URI, Version: document.Version, Text: document.Text}, errors.New("analysis failure")
	}})
	defer server.Close()
	capture := &capturedMessages{}
	initializeTest(t, server, capture)
	handleTest(t, server, capture, "textDocument/didOpen", nil, openParams("file:///logging/main.qnt", "module M {}", 1))
	waitTest(t, server)
	handleTest(t, server, capture, "unsupported", 2, nil)
	for _, want := range []string{"[INFO] 🚀 Quint Language Server Started!", "[TRACE] Received request: method=initialize", "[DEBUG] Document changed: uri=file:///logging/main.qnt, version=1", "[ERROR] Analysis failed for: file:///logging/main.qnt, error=analysis failure", "[DEBUG] Analysis completed for: file:///logging/main.qnt, errors found: 1", "[ERROR] Request failed: method=unsupported"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatal("missing server diagnostic", want, logs.String())
		}
	}
	messages := capture.take()
	if len(messages) != 2 {
		t.Fatal("logging changed protocol response count", len(messages))
	}
	for _, message := range messages {
		if !json.Valid(message) || bytes.Contains(message, []byte("[DEBUG]")) || bytes.Contains(message, []byte("Received request")) {
			t.Fatal("log leaked into protocol", string(message))
		}
	}
	// Closing the server leaves the caller's shared logger usable.
	server.Close()
	logger.Log("INFO", "caller still owns logger")
	if !strings.Contains(logs.String(), "caller still owns logger") {
		t.Fatal("server closed caller logger")
	}
	_ = logger.Close()
}

func TestLoggerDisabledServerHasSameResponses(t *testing.T) {
	t.Parallel()
	var expected []json.RawMessage
	for i, logger := range []*Logger{nil, NewLogger("NONE", &loggerWriter{}), NewLogger("invalid", &loggerWriter{})} {
		server := NewServer(Options{Logger: logger})
		capture := &capturedMessages{}
		handleTest(t, server, capture, "initialize", 1, map[string]any{})
		got := capture.take()
		server.Close()
		_ = logger.Close()
		if i == 0 {
			expected = got
		} else if !reflect.DeepEqual(got, expected) {
			t.Fatal("disabled logger changed response", i)
		}
	}
}
