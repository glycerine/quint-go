package cli

import (
	"context"
	"io"

	quint "github.com/glycerine/quint-go/pkg"
)

func replPrintStateDiff(ctx context.Context, out io.Writer, result quint.REPLResult, level int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if level < 2 || !result.Shifted {
		return nil
	}
	text, err := quint.FormatRuntimeValueDiffContext(ctx, result.OldState, result.NewState, 80, quint.ValueDiffOptions{CollapseThreshold: 2})
	if err != nil {
		return err
	}
	return executionWrite(ctx, out, text+"\n")
}
