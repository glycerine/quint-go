package cli

import (
	"context"
	"io"

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
// Verbose execution frames are supplied by executionPrintCounterExample.
func runPrintTrace(stdout io.Writer, states []quint.Expr, hide []string) error {
	return executionPrintTrace(context.Background(), stdout, states, nil, hide, nil)
}
