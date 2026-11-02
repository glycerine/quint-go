package quint

import (
	"fmt"
	"strings"
)

// FormatValueExpression renders a value expression using Quint's plain pretty
// printer. Width and the already occupied firstColumn count UTF-16 units, as
// in the TypeScript printer. Expressions need not be executable: unsupported
// operators get the reference's visible placeholder, and lambdas show only
// parameter names. The function never writes to a terminal.
func FormatValueExpression(expression Expr, width, firstColumn int) (string, error) {
	if width < 0 || firstColumn < 0 {
		return "", fmt.Errorf("value formatting requires nonnegative width and first column")
	}
	doc, err := valueExpressionDoc(expression)
	if err != nil {
		return "", err
	}
	return formatValueDoc(doc, width, firstColumn), nil
}

// valueDoc is the strict pretty-printer document algebra. Group decisions look
// ahead through the remaining stack, so the following text affects wrapping.
type valueDoc struct {
	kind     uint8
	text     string
	flat     string
	width    int
	children []valueDoc
}

const (
	valueText uint8 = iota
	valueLine
	valueNest
	valueGroup
	valueConcat
)

func vdText(text string) valueDoc {
	return valueDoc{kind: valueText, text: text, width: diagnosticRenderUTF16Len(text)}
}
func vdLine() valueDoc                       { return valueDoc{kind: valueLine, text: "\n", flat: " "} }
func vdBreak() valueDoc                      { return valueDoc{kind: valueLine, text: "\n"} }
func vdConcat(children ...valueDoc) valueDoc { return valueDoc{kind: valueConcat, children: children} }
func vdGroup(children ...valueDoc) valueDoc  { return valueDoc{kind: valueGroup, children: children} }
func vdNest(indent string, children ...valueDoc) valueDoc {
	return valueDoc{kind: valueNest, text: indent, children: children}
}
func vdJoin(separator valueDoc, children []valueDoc) valueDoc {
	result := make([]valueDoc, 0, len(children)*2)
	for i, child := range children {
		if i > 0 {
			result = append(result, separator)
		}
		result = append(result, child)
	}
	return vdConcat(result...)
}
func vdNary(left string, args []valueDoc, right string, padding valueDoc) valueDoc {
	return vdGroup(vdText(left), vdGroup(vdNest("  ", padding, vdJoin(vdConcat(vdText(","), vdLine()), args)), padding), vdText(right))
}

type valueLayout struct {
	doc    valueDoc
	indent string
	flat   bool
}

func valueDocPush(stack []valueLayout, element valueLayout, children []valueDoc) []valueLayout {
	for i := len(children) - 1; i >= 0; i-- {
		element.doc = children[i]
		stack = append(stack, element)
	}
	return stack
}
func valueDocFits(budget int, stack []valueLayout) bool {
	for len(stack) > 0 && budget >= 0 {
		element := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch element.doc.kind {
		case valueText:
			budget -= element.doc.width
		case valueLine:
			if !element.flat {
				return true
			}
			budget -= diagnosticRenderUTF16Len(element.doc.flat)
		case valueNest:
			element.indent += element.doc.text
			stack = valueDocPush(stack, element, element.doc.children)
		case valueGroup:
			element.flat = true
			stack = valueDocPush(stack, element, element.doc.children)
		case valueConcat:
			stack = valueDocPush(stack, element, element.doc.children)
		}
	}
	return budget >= 0
}
func formatValueDoc(doc valueDoc, width, firstColumn int) string {
	var output strings.Builder
	column := firstColumn
	stack := []valueLayout{{doc: vdGroup(doc), flat: true}}
	for len(stack) > 0 {
		element := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch element.doc.kind {
		case valueText:
			output.WriteString(element.doc.text)
			column += element.doc.width
		case valueLine:
			if element.flat {
				output.WriteString(element.doc.flat)
				column += diagnosticRenderUTF16Len(element.doc.flat)
			} else {
				output.WriteString(element.doc.text)
				output.WriteString(element.indent)
				column = diagnosticRenderUTF16Len(element.indent)
			}
		case valueNest:
			element.indent += element.doc.text
			stack = valueDocPush(stack, element, element.doc.children)
		case valueGroup:
			element.flat = true
			trial := append([]valueLayout(nil), stack...)
			trial = valueDocPush(trial, element, element.doc.children)
			element.flat = valueDocFits(width-column, trial)
			stack = valueDocPush(stack, element, element.doc.children)
		case valueConcat:
			stack = valueDocPush(stack, element, element.doc.children)
		}
	}
	return output.String()
}

func valueExpressionDoc(expression Expr) (valueDoc, error) {
	switch expression.Kind {
	case "bool":
		return vdText(fmt.Sprint(expression.Bool)), nil
	case "int":
		return vdText(expression.Int.String()), nil
	case "str":
		return vdText("\"" + expression.Str + "\""), nil
	case "lambda":
		params := make([]valueDoc, len(expression.Params))
		for i, param := range expression.Params {
			params[i] = vdText(param.Name)
		}
		return vdConcat(vdText("{"), vdGroup(vdText("("), vdJoin(vdConcat(vdText(","), vdLine()), params), vdText(")"), vdLine(), vdText("=>"), vdLine(), vdText("...")), vdText("}")), nil
	case "app":
	default:
		return vdText("unsupported operator: " + expression.Kind), nil
	}
	args := make([]valueDoc, 0, len(expression.Args))
	switch expression.Opcode {
	case "Rec":
		if len(expression.Args)%2 != 0 {
			return valueDoc{}, fmt.Errorf("malformed record value")
		}
		for i := 0; i < len(expression.Args); i += 2 {
			key := expression.Args[i]
			if key.Kind != "str" {
				continue
			}
			value, err := valueExpressionDoc(expression.Args[i+1])
			if err != nil {
				return valueDoc{}, err
			}
			args = append(args, vdGroup(vdText(key.Str), vdText(":"), vdNest("  ", vdLine(), value)))
		}
		return vdNary("{", args, "}", vdLine()), nil
	case "Map":
		for _, pair := range expression.Args {
			if pair.Kind != "app" || pair.Opcode != "Tup" || len(pair.Args) != 2 {
				args = append(args, vdText("<expected-pair>"))
				continue
			}
			key, err := valueExpressionDoc(pair.Args[0])
			if err != nil {
				return valueDoc{}, err
			}
			value, err := valueExpressionDoc(pair.Args[1])
			if err != nil {
				return valueDoc{}, err
			}
			args = append(args, vdGroup(key, vdText(" ->"), vdNest("  ", vdLine(), value)))
		}
		return vdGroup(vdText("Map"), vdNary("(", args, ")", vdBreak())), nil
	case "variant":
		if len(expression.Args) != 2 || expression.Args[0].Kind != "str" {
			return valueDoc{}, fmt.Errorf("malformed variant value")
		}
		label := vdText(expression.Args[0].Str)
		payload := expression.Args[1]
		if payload.Kind == "app" && payload.Opcode == "Tup" && len(payload.Args) == 0 {
			return vdGroup(label), nil
		}
		value, err := valueExpressionDoc(payload)
		if err != nil {
			return valueDoc{}, err
		}
		return vdGroup(label, vdText("("), value, vdText(")")), nil
	case "Set", "List", "Tup":
		for _, arg := range expression.Args {
			value, err := valueExpressionDoc(arg)
			if err != nil {
				return valueDoc{}, err
			}
			args = append(args, value)
		}
		if expression.Opcode == "List" {
			return vdNary("[", args, "]", vdBreak()), nil
		}
		if expression.Opcode == "Tup" {
			return vdNary("(", args, ")", vdBreak()), nil
		}
		return vdGroup(vdText("Set"), vdNary("(", args, ")", vdBreak())), nil
	}
	return vdText("unsupported operator: " + expression.Opcode + "(...)"), nil
}
