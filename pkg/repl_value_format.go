package quint

import "fmt"

// FormatValue renders a REPL result with the reference value printer and the
// session's object identity history. A nonpositive width uses 80 columns.
// Like evaluation, formatting belongs to the sequential session owner.
func (s *REPLSession) FormatValue(value RuntimeValue, width int) (string, error) {
	if s == nil || s.runtime == nil {
		return "", fmt.Errorf("REPL session is not initialized")
	}
	expression, err := SerializeRuntimeValue(value, s.runtime.ObjectHashState())
	if err != nil {
		return "", err
	}
	if width <= 0 {
		width = 80
	}
	return FormatValueExpression(expression, width, 0)
}
