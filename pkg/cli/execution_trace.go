package cli

import (
	"context"
	"io"

	quint "github.com/glycerine/quint-go/pkg"
)

func executionPrintAnnotatedTrace(ctx context.Context, out io.Writer, states []quint.Expr, options quint.TraceFormatOptions) error {
	text, err := quint.FormatTraceContext(ctx, states, terminalWidth(ctx), options)
	if err != nil {
		return err
	}
	if text == "" {
		return ctx.Err()
	}
	return executionWrite(ctx, out, text)
}
