// Package lsp implements a Go Quint language server over the Language Server
// Protocol's JSON-RPC transport. Source positions use UTF-16 code units.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const MaxMessageBytes = 16 << 20

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
type DiagnosticRelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}
type Diagnostic struct {
	Range              Range                          `json:"range"`
	Severity           int                            `json:"severity"`
	Code               string                         `json:"code,omitempty"`
	Source             string                         `json:"source"`
	Message            string                         `json:"message"`
	RelatedInformation []DiagnosticRelatedInformation `json:"relatedInformation,omitempty"`
}

// ReadMessage reads one Content-Length framed message. EOF before any header is
// a normal stream end; a partial header or body is an error.
func ReadMessage(reader *bufio.Reader) ([]byte, error) {
	length := -1
	readHeader := false
	headerBytes := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && !readHeader && line == "" {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("reading LSP header: %w", io.ErrUnexpectedEOF)
		}
		readHeader = true
		headerBytes += len(line)
		if headerBytes > 8192 {
			return nil, fmt.Errorf("LSP headers exceed 8192 bytes")
		}
		if !strings.HasSuffix(line, "\r\n") {
			return nil, fmt.Errorf("LSP header requires CRLF")
		}
		line = strings.TrimSuffix(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid LSP header")
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if length >= 0 {
				return nil, fmt.Errorf("duplicate Content-Length")
			}
			parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
			if err != nil || parsed == 0 || parsed > MaxMessageBytes {
				return nil, fmt.Errorf("invalid LSP Content-Length")
			}
			length = int(parsed)
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, fmt.Errorf("reading LSP body: %w", err)
	}
	return body, nil
}

// WriteMessage serializes exactly one framed JSON-RPC object. Concurrent
// writers must serialize calls; Server.Serve does this for its own output.
func WriteMessage(writer io.Writer, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(body) > MaxMessageBytes {
		return fmt.Errorf("LSP message exceeds %d bytes", MaxMessageBytes)
	}
	frame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	for len(frame) > 0 {
		n, err := writer.Write(frame)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(frame) {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}
	return nil
}
