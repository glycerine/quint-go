package cli

import (
	"context"
	"io"

	quint "github.com/glycerine/quint-go/pkg"
)

func verifyPrintCounterExample(ctx context.Context, out io.Writer, options verifyOptions, stage verifyStage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if options.verbosity < 2 || stage.Status != "violation" || len(stage.Trace) == 0 {
		return nil
	}
	trace, err := quint.DecodeNormalizedITFTrace(stage.Trace)
	if err != nil {
		return err
	}
	// The upstream verification path displays normalized states without the
	// runtime-only frame/debug annotations used by simulation reporting.
	return executionPrintCounterExample(ctx, out, options.verbosity, trace.States, nil, nil, nil)
}
