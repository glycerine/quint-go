package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

func executionWrite(ctx context.Context, out io.Writer, text string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := io.WriteString(out, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	return err
}

// Private CLI entry definitions retain imported tests and REPL requests during
// flattening. They are absent from the source evaluator's visible call tree.
func executionUnwrapFrames(frames []quint.ExecutionFrame, wrapper string) []quint.ExecutionFrame {
	if wrapper == "" {
		return frames
	}
	result := make([]quint.ExecutionFrame, 0, len(frames))
	for _, frame := range frames {
		if frame.App.Opcode == wrapper {
			result = append(result, executionUnwrapFrames(frame.Subframes, wrapper)...)
		} else {
			result = append(result, frame)
		}
	}
	return result
}

func executionBestFrames(recorder *quint.TraceRecorder, wrapper string) []quint.ExecutionFrame {
	if recorder == nil {
		return nil
	}
	traces := recorder.BestTraces()
	if len(traces) == 0 {
		return nil
	}
	return executionUnwrapFrames(traces[0].Frame.Subframes, wrapper)
}

// A frame block has the same spacing in verbose test errors and the REPL.
// Tests explicitly display an empty execution; the REPL leaves it silent.
func executionPrintFrames(ctx context.Context, out io.Writer, frames []quint.ExecutionFrame, objects *quint.RuntimeObjectHashState, showEmpty bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(frames) == 0 && !showEmpty {
		return nil
	}
	if err := executionWrite(ctx, out, "\n"); err != nil {
		return err
	}
	if len(frames) == 0 {
		return executionWrite(ctx, out, "    [No execution]\n")
	}
	for index, frame := range frames {
		if err := executionPrintFrame(ctx, out, index, frame, objects); err != nil {
			return err
		}
	}
	return nil
}

func executionPrintFrame(ctx context.Context, out io.Writer, index int, frame quint.ExecutionFrame, objects *quint.RuntimeObjectHashState) error {
	text, err := quint.FormatExecutionFrameContext(ctx, frame, 80, objects)
	if err != nil {
		return err
	}
	return executionWrite(ctx, out, fmt.Sprintf("[Frame %d]\n%s\n", index, text))
}

func executionPrintTrace(ctx context.Context, out io.Writer, states []quint.Expr, frames []quint.ExecutionFrame, hide []string, objects *quint.RuntimeObjectHashState) error {
	hidden := insdict.NewDict[string, bool]()
	for _, name := range hide {
		hidden.Set(name, true)
	}
	for index, state := range states {
		if err := ctx.Err(); err != nil {
			return err
		}
		if index < len(frames) {
			if err := executionPrintFrame(ctx, out, index, frames[index], objects); err != nil {
				return err
			}
		}
		if len(hide) > 0 {
			filtered := []quint.Expr{}
			for i := 0; i+1 < len(state.Args); i += 2 {
				if state.Args[i].Kind == "str" && !hidden.Get(state.Args[i].Str) {
					filtered = append(filtered, state.Args[i], state.Args[i+1])
				}
			}
			state.Args = filtered
		}
		text, err := quint.FormatTraceStateExpression(index, state, 80)
		if err != nil {
			return err
		}
		if err := executionWrite(ctx, out, text+"\n\n"); err != nil {
			return err
		}
	}
	return nil
}

func executionPrintCounterExample(ctx context.Context, out io.Writer, verbosity int, states []quint.Expr, frames []quint.ExecutionFrame, hide []string, objects *quint.RuntimeObjectHashState) error {
	if verbosity < 2 {
		return ctx.Err()
	}
	if err := executionWrite(ctx, out, "An example execution:\n\n"); err != nil {
		return err
	}
	if verbosity < 3 {
		frames = nil
	}
	return executionPrintTrace(ctx, out, states, frames, hide, objects)
}

type executionTestFailure struct {
	Name     string
	Seed     quint.Integer
	Messages []quint.ErrorMessage
	Frames   []quint.ExecutionFrame
	// Repeat may keep a caller's Go-specific command flags and selection name.
	// The default is the source CLI's replay instruction.
	Repeat string
}

func executionPrintTestErrors(ctx context.Context, out io.Writer, verbosity int, input string, code *insdict.Dict[string, string], failures []executionTestFailure, objects *quint.RuntimeObjectHashState, options ...quint.DiagnosticRenderOptions) error {
	if verbosity < 2 {
		return ctx.Err()
	}
	if err := executionWrite(ctx, out, "\n"); err != nil {
		return err
	}
	number := 0
	for _, failure := range failures {
		for _, message := range failure.Messages {
			number++
			if err := executionWrite(ctx, out, fmt.Sprintf("  %d) %s:\n", number, failure.Name)); err != nil {
				return err
			}
			for _, line := range strings.Split(quint.RenderErrorMessage(message, code, options...), "\n") {
				if strings.TrimSpace(line) != "" {
					if err := executionWrite(ctx, out, "      "+line+"\n"); err != nil {
						return err
					}
				}
			}
			if verbosity >= 3 {
				if err := executionPrintFrames(ctx, out, failure.Frames, objects, true); err != nil {
					return err
				}
			}
			repeat := failure.Repeat
			if repeat == "" {
				repeat = fmt.Sprintf("Use --seed=0x%s --match=%s to repeat.", failure.Seed.Big().Text(16), failure.Name)
			}
			if err := executionWrite(ctx, out, "    "+repeat+"\n"); err != nil {
				return err
			}
		}
	}
	if err := executionWrite(ctx, out, "\n"); err != nil {
		return err
	}
	if verbosity < 3 {
		return executionWrite(ctx, out, "\n  Use --verbosity=3 to show executions.\n  Further debug with: quint test --verbosity=3 "+input+"\n")
	}
	return nil
}
