package cli

import (
	"io"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

func runPrettyExpression(expression quint.Expr, width int) string {
	if width <= 0 {
		width = 80
	}
	formatted, err := quint.FormatValueExpression(expression, width, 0)
	if err != nil {
		return quint.ExpressionString(expression)
	}
	return formatted
}

// runPrintTrace renders numbered states without changing their record fields.
// Execution-frame capture and debug messages are supplied by separate hooks.
func runPrintTrace(stdout io.Writer, states []quint.Expr, hide []string) error {
	hidden := insdict.NewDict[string, bool]()
	for _, name := range hide {
		hidden.Set(name, true)
	}
	for index, state := range states {
		if len(hide) > 0 {
			filtered := []quint.Expr{}
			for i := 0; i+1 < len(state.Args); i += 2 {
				if state.Args[i].Kind == "str" && !hidden.Get(state.Args[i].Str) {
					filtered = append(filtered, state.Args[i], state.Args[i+1])
				}
			}
			state.Args = filtered
		}
		formatted, err := quint.FormatTraceStateExpression(index, state, 80)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(stdout, formatted+"\n\n"); err != nil {
			return err
		}
	}
	return nil
}
