package quint

import (
	"context"
	"fmt"
)

// FormatValue renders a REPL result with the reference value printer and the
// session's object identity history. A nonpositive width uses 80 columns.
// Like evaluation, formatting belongs to the sequential session owner.
func (s *REPLSession) FormatValue(value RuntimeValue, width int) (string, error) {
	return s.FormatValueContext(context.Background(), value, width)
}

// FormatValueContext renders a result while preserving context cancellation,
// including when a compact domain must be enumerated for display.
func (s *REPLSession) FormatValueContext(ctx context.Context, value RuntimeValue, width int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s == nil || s.runtime == nil {
		return "", fmt.Errorf("REPL session is not initialized")
	}
	expression, err := SerializeRuntimeValueContext(ctx, value, s.runtime.ObjectHashState())
	if err != nil {
		return "", err
	}
	if width <= 0 {
		width = 80
	}
	return FormatValueExpressionContext(ctx, expression, width, 0)
}
