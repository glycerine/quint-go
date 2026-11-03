package quint

import (
	"context"
	"fmt"
	"strings"

	"github.com/glycerine/insdict"
)

// TraceFormatOptions supplies optional trace annotations. Diagnostics are
// indexed exactly as supplied, including sparse rows returned by ITFDiagnostics.
// PendingDiagnostics describe a failed step after the last recorded state.
// Frames beyond the number of states are ignored, as in the source renderer.
type TraceFormatOptions struct {
	Diagnostics        [][]DebugMessage
	PendingDiagnostics []DebugMessage
	Frames             []ExecutionFrame
	HideVariables      []string
	ObjectHashState    *RuntimeObjectHashState
}

// FormatTrace formats numbered states, captured calls and debug messages.
// Width counts visible UTF-16 units, and zero requests the narrowest layout.
// Input expressions and annotations remain owned by the caller.
func FormatTrace(states []Expr, width int, options ...TraceFormatOptions) (string, error) {
	return FormatTraceContext(context.Background(), states, width, options...)
}

// FormatTraceContext applies the context's optional terminal styles and checked
// captured-value serialization. Cancellation and malformed input return an
// error with no partial rendering. An object-hash state may share display-order
// history with the evaluator; omitting it starts one history for this trace.
func FormatTraceContext(ctx context.Context, states []Expr, width int, options ...TraceFormatOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if width < 0 {
		return "", fmt.Errorf("trace formatting requires nonnegative width")
	}
	var option TraceFormatOptions
	if len(options) > 0 {
		option = options[0]
	}
	objects := option.ObjectHashState
	if objects == nil {
		objects = &RuntimeObjectHashState{}
	}
	decorate := terminalValueDecorator(ctx)
	hidden := insdict.NewDict[string, bool]()
	for _, name := range option.HideVariables {
		hidden.Set(name, true)
	}
	var output strings.Builder
	for index, state := range states {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if state.Kind != "app" || state.Opcode != "Rec" || len(state.Args)%2 != 0 {
			return "", fmt.Errorf("trace formatting requires a record state")
		}
		if index < len(option.Frames) {
			text, err := FormatExecutionFrameContext(ctx, option.Frames[index], width, objects)
			if err != nil {
				return "", err
			}
			header := fmt.Sprintf("Frame %d", index)
			if decorate != nil {
				header = decorate(header, "bold")
			}
			fmt.Fprintf(&output, "[%s]\n%s\n", header, text)
		}
		if len(option.HideVariables) > 0 {
			args := make([]Expr, 0, len(state.Args))
			for i := 0; i < len(state.Args); i += 2 {
				if key := state.Args[i]; key.Kind == "str" && !hidden.Get(key.Str) {
					args = append(args, key, state.Args[i+1])
				}
			}
			state.Args = args
		}
		if index < len(option.Diagnostics) && len(option.Diagnostics[index]) > 0 {
			for _, message := range option.Diagnostics[index] {
				text, err := formatTraceDebug(ctx, message, width, false, decorate)
				if err != nil {
					return "", err
				}
				output.WriteString(text + "\n")
			}
			output.WriteByte('\n')
		}
		text, err := FormatTraceStateExpressionContext(ctx, index, state, width)
		if err != nil {
			return "", err
		}
		output.WriteString(text + "\n\n")
	}
	if len(option.PendingDiagnostics) > 0 {
		for _, message := range option.PendingDiagnostics {
			text, err := formatTraceDebug(ctx, message, width, true, decorate)
			if err != nil {
				return "", err
			}
			output.WriteString(text + "\n")
		}
		output.WriteByte('\n')
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return output.String(), nil
}

func formatTraceDebug(ctx context.Context, message DebugMessage, width int, pending bool, decorate func(string, ...string) string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	value, err := valueExpressionDocStyled(message.Value, decorate)
	if err != nil {
		return "", err
	}
	header, label := vdText("[DEBUG]"), vdText(message.Label)
	if pending && decorate != nil {
		header.text = decorate(header.text, "yellow")
		label.text = decorate(label.text, "yellow")
	}
	doc := vdGroup(header, vdText(" "), label, vdLine(), value)
	return formatValueDoc(doc, width, 0), nil
}
