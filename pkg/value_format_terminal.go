package quint

import (
	"context"
	"fmt"

	"github.com/glycerine/quint-go/pkg/terminal"
)

// FormatValueExpressionContext uses the context's optional terminal colors.
// ANSI styles do not contribute to layout width. With no terminal policy it
// produces the same plain output as FormatValueExpression.
func FormatValueExpressionContext(ctx context.Context, expression Expr, width, firstColumn int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if width < 0 || firstColumn < 0 {
		return "", fmt.Errorf("value formatting requires nonnegative width and first column")
	}
	doc, err := valueExpressionDocStyled(expression, terminalValueDecorator(ctx))
	if err != nil {
		return "", err
	}
	return formatValueDoc(doc, width, firstColumn), nil
}

func terminalValueDecorator(ctx context.Context) func(string, ...string) string {
	policy := terminal.PolicyFromContext(ctx)
	if policy.ColorLevel() <= 0 {
		return nil
	}
	return policy.Style
}

// FormatTraceStateExpressionContext applies optional context colors to the
// numbered state header and values, retaining visible-width layout.
func FormatTraceStateExpressionContext(ctx context.Context, index int, expression Expr, width int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return formatTraceStateExpression(index, expression, width, terminalValueDecorator(ctx))
}
