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
	decorate := func(role quint.ValueDiffRole, text string) string {
		style := "gray"
		if role == quint.ValueDiffAdded {
			style = "green"
		} else if role == quint.ValueDiffRemoved {
			style = "red"
		}
		return terminalStyle(ctx, text, style)
	}
	text, err := quint.FormatRuntimeValueDiffContext(ctx, result.OldState, result.NewState, terminalWidth(ctx), quint.ValueDiffOptions{CollapseThreshold: 2, Decorate: decorate})
	if err != nil {
		return err
	}
	return executionWrite(ctx, out, text+"\n")
}
