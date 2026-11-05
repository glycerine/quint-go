package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/glycerine/quint-go/pkg/lsp"
)

func TestRunWithEOFOnEmptyInputExitsZero(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runWith(context.Background(), lsp.NewLogger("NONE", nil), strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Fatal("empty input exited", code, errOut.String())
	}
}

func TestRunWithCancelledContextExitsZero(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	code := runWith(ctx, lsp.NewLogger("NONE", nil), strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Fatal("cancelled context exited", code, errOut.String())
	}
	if errOut.Len() != 0 {
		t.Fatal("cancellation wrote to stderr", errOut.String())
	}
}

func TestRunWithTruncatedMessageExitsOneWithStderr(t *testing.T) {
	// A header that promises more body bytes than arrive is a framing failure,
	// not a clean end of input.
	input := "Content-Length: 100\r\n\r\n{}"
	var out, errOut bytes.Buffer
	code := runWith(context.Background(), lsp.NewLogger("NONE", nil), strings.NewReader(input), &out, &errOut)
	if code != 1 {
		t.Fatal("truncated message exited", code)
	}
	if errOut.Len() == 0 {
		t.Fatal("failure was not reported on stderr")
	}
}
