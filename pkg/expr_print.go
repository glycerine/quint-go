package quint

import (
	"strconv"
	"strings"
)

// ExpressionString renders validated IR in the upstream diagnostic format.
func ExpressionString(e Expr) string {
	switch e.Kind {
	case "name":
		return e.Name
	case "bool":
		return strconv.FormatBool(e.Bool)
	case "int":
		return e.Int.String()
	case "str":
		return "\"" + e.Str + "\""
	case "app":
		args := []string{}
		for _, a := range e.Args {
			args = append(args, ExpressionString(a))
		}
		return e.Opcode + "(" + strings.Join(args, ", ") + ")"
	case "lambda":
		params := []string{}
		for _, p := range e.Params {
			params = append(params, p.Name)
		}
		return "((" + strings.Join(params, ", ") + ") => " + ExpressionString(*e.Body) + ")"
	case "let":
		d := e.Opdef
		q := string(d.Qualifier)
		if q == "pureval" {
			q = "pure val"
		}
		if q == "puredef" {
			q = "pure def"
		}
		annotation := ""
		if d.TypeAnnotation != nil {
			annotation = ": " + TypeString(*d.TypeAnnotation)
		}
		return q + " " + d.Name + annotation + " = " + ExpressionString(*d.Expr) + " { " + ExpressionString(*e.Body) + " }"
	}
	return ""
}
