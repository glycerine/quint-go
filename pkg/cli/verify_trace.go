package cli

import (
	"context"
	"fmt"
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
	if err := executionPrintCounterExample(ctx, out, options.verbosity, trace.States, nil, nil, nil); err != nil {
		return err
	}
	// A lasso counterexample's final transition returns to an earlier state.
	// Show that target, which ITF records as a zero-based loop index. Upstream's
	// default output omits it, so it appears only at verbosity 3 and above.
	if trace.Loop != nil && options.verbosity >= 3 {
		text := fmt.Sprintf("The final transition returns to [State %d].\n", *trace.Loop)
		return executionWrite(ctx, out, terminalStyle(ctx, text, "gray"))
	}
	return nil
}
