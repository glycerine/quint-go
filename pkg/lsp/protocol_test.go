package lsp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestMessageFraming(t *testing.T) {
	t.Parallel()
	var encoded bytes.Buffer
	for _, message := range []any{map[string]any{"jsonrpc": "2.0", "id": 1, "result": "😀"}, map[string]any{"jsonrpc": "2.0", "id": "next", "result": nil}} {
		if err := WriteMessage(&encoded, message); err != nil {
			t.Fatal(err)
		}
	}
	reader := bufio.NewReader(&encoded)
	first, err := ReadMessage(reader)
	if err != nil || !bytes.Contains(first, []byte("😀")) {
		t.Fatal(string(first), err)
	}
	second, err := ReadMessage(reader)
	if err != nil || !bytes.Contains(second, []byte(`"result":null`)) {
		t.Fatal(string(second), err)
	}
	if _, err = ReadMessage(reader); err != io.EOF {
		t.Fatal(err)
	}
	for _, input := range []string{"Content-Length: 2\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n{}", "content-length: 2\r\n\r\n{}"} {
		body, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))
		if err != nil || string(body) != "{}" {
			t.Fatal(input, string(body), err)
		}
	}
}
func TestMessageFramingFailures(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"Content-Length: 2\r\n\r\n{", "Content-Length: 2\r\n", "Content-Length: 2\n\n{}", "Content-Length 2\r\n\r\n{}",
		"Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}", "Content-Type: text/json\r\n\r\n{}", "Content-Length: 0\r\n\r\n", "Content-Length: -1\r\n\r\n", "Content-Length: 16777217\r\n\r\n", "Content-Length: words\r\n\r\n", strings.Repeat("a", 8193) + "\r\n",
	} {
		if _, err := ReadMessage(bufio.NewReader(strings.NewReader(input))); err == nil {
			t.Fatal("accepted", input)
		}
	}
	if err := WriteMessage(io.Discard, make(chan int)); err == nil {
		t.Fatal("unsupported JSON accepted")
	}
	if err := WriteMessage(zeroWriter{}, true); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if err := WriteMessage(failWriter{}, true); !errors.Is(err, errWrite) {
		t.Fatal(err)
	}
	writer := &shortWriter{}
	if err := WriteMessage(writer, map[string]string{"text": "value"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadMessage(bufio.NewReader(&writer.Buffer)); err != nil {
		t.Fatal(err)
	}
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

var errWrite = errors.New("write failure")

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errWrite }

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(data []byte) (int, error) {
	if len(data) > 3 {
		data = data[:3]
	}
	return w.Buffer.Write(data)
}
