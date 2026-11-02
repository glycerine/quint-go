package cli

import (
	"context"
	"io"

	quint "github.com/glycerine/quint-go/pkg"
)

func executionREPLContext(ctx context.Context, session *quint.REPLSession, level int) (context.Context, *quint.TraceRecorder, error) {
	if err := ctx.Err(); err != nil {
		return ctx, nil, err
	}
	if level < 3 {
		return ctx, nil, nil
	}
	recorder, err := session.NewTraceRecorder(level)
	if err != nil {
		return ctx, nil, err
	}
	return quint.WithExecutionListener(ctx, recorder), recorder, nil
}

func executionPrintREPLFrames(ctx context.Context, out io.Writer, session *quint.REPLSession, recorder *quint.TraceRecorder) error {
	if recorder == nil {
		return ctx.Err()
	}
	frames := executionUnwrapFrames(recorder.CurrentFrame().Subframes, "q::input")
	return executionPrintFrames(ctx, out, frames, session.ObjectHashState(), false)
}

func executionREPLReplay(ctx context.Context, session *quint.REPLSession, level func() int, out func(string), writer io.Writer, report func(error), render func(quint.REPLResult) error, writeErr *error) (context.Context, quint.REPLReplayHooks, error) {
	replayContext, recorder, err := executionREPLContext(ctx, session, level())
	if err != nil {
		return ctx, quint.REPLReplayHooks{}, err
	}
	hooks := replReplayOutput(level, out, report, func(result quint.REPLResult) {
		if err := render(result); err != nil {
			report(err)
		}
	}, writeErr)
	if recorder == nil {
		return replayContext, hooks, nil
	}
	before := hooks.Before
	hooks.Before = func(filename, input string) error {
		// Loading adopts a new runtime before replay starts. Rebind to its RNG
		// and clear between saved expressions, as the interactive path does.
		bound, err := session.NewTraceRecorder(level())
		if err != nil {
			return err
		}
		recorder.Clear()
		recorder.RNG, recorder.VerbosityLevel = bound.RNG, level()
		return before(filename, input)
	}
	hooks.After = func(entry quint.REPLReplayResult) error {
		if entry.Err == nil {
			if err := render(entry.Result); err != nil {
				report(err)
				return *writeErr
			}
		}
		if err := executionPrintREPLFrames(ctx, writer, session, recorder); err != nil {
			report(err)
			return *writeErr
		}
		if entry.Err != nil {
			report(entry.Err)
		}
		return *writeErr
	}
	return replayContext, hooks, nil
}
