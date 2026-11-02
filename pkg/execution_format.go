package quint

import (
	"fmt"
	"strings"
)

// ExecutionFrame is an already captured operator invocation. Err marks an
// unsuccessful invocation; its displayed result is none. Formatting does not
// invoke operators or collect new frames.
type ExecutionFrame struct {
	App       Expr
	Args      []RuntimeValue
	Result    RuntimeValue
	Err       error
	Subframes []ExecutionFrame
}

// FormatTraceStateExpression formats one numbered state, including the line
// break after its prefix when the complete state does not fit the remaining
// width. State expressions must be records. Width counts UTF-16 units.
func FormatTraceStateExpression(index int, expression Expr, width int) (string, error) {
	if index < 0 || width < 0 {
		return "", fmt.Errorf("trace formatting requires a nonnegative index and width")
	}
	if expression.Kind != "app" || expression.Opcode != "Rec" || len(expression.Args)%2 != 0 {
		return "", fmt.Errorf("trace formatting requires a record state")
	}
	value, err := valueExpressionDoc(expression)
	if err != nil {
		return "", err
	}
	doc := vdConcat(vdGroup(vdText(fmt.Sprintf("[State %d]", index)), vdLine()), value)
	return formatValueDoc(doc, width, 0), nil
}

// FormatExecutionFrame formats an already captured call tree with the reference
// branch glyphs and value layout. The returned text ends with a newline. Runtime
// values are serialized for display; no evaluator state is modified.
func FormatExecutionFrame(frame ExecutionFrame, width int) (string, error) {
	if width < 0 {
		return "", fmt.Errorf("execution formatting requires a nonnegative width")
	}
	var output strings.Builder
	objects := &RuntimeObjectHashState{}
	var visit func(ExecutionFrame, []bool) error
	visit = func(frame ExecutionFrame, last []bool) error {
		args := make([]valueDoc, len(frame.Args))
		for i, arg := range frame.Args {
			expression, err := SerializeRuntimeValue(arg, objects)
			if err != nil {
				return err
			}
			args[i], err = valueExpressionDoc(expression)
			if err != nil {
				return err
			}
		}
		result := vdText("none")
		if frame.Err == nil {
			expression, err := SerializeRuntimeValue(frame.Result, objects)
			if err != nil {
				return err
			}
			result, err = valueExpressionDoc(expression)
			if err != nil {
				return err
			}
		}
		var branches, indent strings.Builder
		for i, isLast := range last {
			if isLast {
				indent.WriteString("   ")
			} else {
				indent.WriteString("│  ")
			}
			if i < len(last)-1 {
				if isLast {
					branches.WriteString("   ")
				} else {
					branches.WriteString("│  ")
				}
			} else if isLast {
				branches.WriteString("└─ ")
			} else {
				branches.WriteString("├─ ")
			}
		}
		argsDoc := vdText("")
		if len(args) > 0 {
			argsDoc = vdGroup(vdText("("), vdNest("  ", vdBreak(), vdGroup(vdJoin(vdConcat(vdText(","), vdLine()), args))), vdBreak(), vdText(")"))
		}
		call := vdGroup(vdNest(indent.String(), vdText(frame.App.Opcode), vdGroup(argsDoc, vdNest("  ", vdGroup(vdText(" =>"), vdLine(), result)))))
		output.WriteString(branches.String())
		output.WriteString(formatValueDoc(call, width, diagnosticRenderUTF16Len(branches.String())))
		output.WriteByte('\n')
		for i, child := range frame.Subframes {
			path := append(append([]bool(nil), last...), i == len(frame.Subframes)-1)
			if err := visit(child, path); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(frame, nil); err != nil {
		return "", err
	}
	return output.String(), nil
}
