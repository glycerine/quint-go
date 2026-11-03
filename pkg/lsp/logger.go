package lsp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger writes optional language-server diagnostics separately from JSON-RPC.
// It is safe for concurrent workers. Output supplied to NewLogger stays owned
// by the caller; only an environment logger owns its lazily opened file.
type Logger struct {
	mu          sync.Mutex
	rank        int
	valid       bool
	output      io.Writer
	filename    string
	file        io.Closer
	unavailable bool
	closed      bool
	now         func() time.Time
	open        func(string) (io.WriteCloser, error)
}

// NewLogger captures the configured level and a caller-owned output. Levels
// follow upstream: NONE disables messages, INFO/WARN/ERROR share a threshold,
// DEBUG adds debug messages, and TRACE enables every message. Unknown levels
// disable logging. A nil output discards messages without opening any files.
func NewLogger(level string, output io.Writer) *Logger {
	rank, valid := logLevelRank(strings.ToUpper(level))
	return &Logger{rank: rank, valid: valid, output: output, now: time.Now}
}

// NewEnvironmentLogger captures LOG_LEVEL (default NONE) and the upstream
// HOME, USERPROFILE, temporary-directory path precedence. It opens
// quint-lsp.log only for the first enabled message. File failures silently
// disable logging, so read-only homes never corrupt the protocol stream.
// Close releases an opened file; no file is created by construction or Close.
func NewEnvironmentLogger() *Logger {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "NONE"
	}
	logger := NewLogger(level, nil)
	logger.filename = environmentLogPath(os.LookupEnv, os.TempDir())
	logger.open = func(filename string) (io.WriteCloser, error) {
		return os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	}
	return logger
}

func environmentLogPath(lookup func(string) (string, bool), temp string) string {
	home, found := lookup("HOME")
	if !found {
		home, found = lookup("USERPROFILE")
	}
	if !found {
		home = temp
	}
	return filepath.Join(home, "quint-lsp.log")
}

func logLevelRank(level string) (int, bool) {
	switch level {
	case "NONE":
		return 0, true
	case "INFO", "WARN", "ERROR":
		return 1, true
	case "DEBUG":
		return 2, true
	case "TRACE":
		return 3, true
	default:
		return 0, false
	}
}

// Enabled reports the captured upstream threshold for an uppercase level.
func (l *Logger) Enabled(level string) bool {
	if l == nil {
		return false
	}
	rank, valid := logLevelRank(level)
	return l.valid && valid && l.rank >= rank
}

// Log formats a diagnostic using fmt.Sprintf and the upstream UTC millisecond
// prefix. Logging errors are deliberately nonfatal and never reach stdout.
func (l *Logger) Log(level, format string, arguments ...any) {
	if !l.Enabled(level) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.unavailable {
		return
	}
	if l.output == nil && l.open != nil {
		file, err := l.open(l.filename)
		if err != nil {
			l.unavailable = true
			return
		}
		l.output, l.file = file, file
	}
	if l.output == nil {
		return
	}
	stamp := l.now().UTC().Format("2006-01-02 15:04:05.000")
	_, err := fmt.Fprintf(l.output, "[%s] [%s] %s\n", stamp, level, fmt.Sprintf(format, arguments...))
	if err != nil {
		l.unavailable = true
	}
}

// Close stops further logging and closes only a file owned by an environment
// logger. It is idempotent and safe while workers finish their log messages.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
