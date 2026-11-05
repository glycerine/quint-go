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

// valueFitsFrame is a cursor over the children of one pending container. Its
// children are visited lazily, so a fit check never copies the layout stack or
// expands a large group beyond the text it actually scans.
type valueFitsFrame struct {
	layout   valueLayout
	children []valueDoc
	next     int
}

// valueGroupFits reports whether the children of group, followed by the
// pending layouts in stack, fit within budget columns when laid out flat. It
// reads stack in place and never modifies it. The walk order matches a
// valueDocPush of group's children onto a copy of stack.
func valueGroupFits(budget int, stack []valueLayout, group valueLayout) bool {
	frames := []valueFitsFrame{{layout: group, children: group.doc.children}}
	stackIndex := len(stack)
	for budget >= 0 {
		var cur valueLayout
		if n := len(frames); n > 0 {
			top := &frames[n-1]
			if top.next == len(top.children) {
				frames = frames[:n-1]
				continue
			}
			cur = valueLayout{doc: top.children[top.next], flat: top.layout.flat, indent: top.layout.indent}
			top.next++
		} else if stackIndex > 0 {
			stackIndex--
			cur = stack[stackIndex]
		} else {
			break
		}
		switch cur.doc.kind {
		case valueText:
			budget -= cur.doc.width
		case valueLine:
			if !cur.flat {
				return true
			}
			budget -= diagnosticRenderUTF16Len(cur.doc.flat)
		case valueNest:
			cur.indent += cur.doc.text
			frames = append(frames, valueFitsFrame{layout: cur, children: cur.doc.children})
		case valueGroup:
			cur.flat = true
			frames = append(frames, valueFitsFrame{layout: cur, children: cur.doc.children})
		case valueConcat:
			frames = append(frames, valueFitsFrame{layout: cur, children: cur.doc.children})
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
			element.flat = valueGroupFits(width-column, stack, element)
			stack = valueDocPush(stack, element, element.doc.children)
		case valueConcat:
			stack = valueDocPush(stack, element, element.doc.children)
		}
	}
	return output.String()
}

func valueExpressionDoc(expression Expr) (valueDoc, error) {
	return valueExpressionDocStyled(expression, nil)
}

func valueExpressionDocStyled(expression Expr, decorate func(string, ...string) string) (valueDoc, error) {
	text := func(value, style string) valueDoc {
		doc := vdText(value)
		if decorate != nil {
			doc.text = decorate(value, style)
		}
		return doc
	}
	switch expression.Kind {
	case "bool":
		return text(fmt.Sprint(expression.Bool), "yellow"), nil
	case "int":
		return text(expression.Int.String(), "yellow"), nil
	case "str":
		return text("\""+expression.Str+"\"", "green"), nil
	case "lambda":
		params := make([]valueDoc, len(expression.Params))
		for i, param := range expression.Params {
			params[i] = vdText(param.Name)
		}
		return vdConcat(vdText("{"), vdGroup(vdText("("), vdJoin(vdConcat(vdText(","), vdLine()), params), vdText(")"), vdLine(), vdText("=>"), vdLine(), vdText("...")), vdText("}")), nil
	case "app":
	default:
		return text("unsupported operator: "+expression.Kind, "red"), nil
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
			value, err := valueExpressionDocStyled(expression.Args[i+1], decorate)
			if err != nil {
				return valueDoc{}, err
			}
			args = append(args, vdGroup(vdText(key.Str), text(":", "gray"), vdNest("  ", vdLine(), value)))
		}
		return vdNary("{", args, "}", vdLine()), nil
	case "Map":
		for _, pair := range expression.Args {
			if pair.Kind != "app" || pair.Opcode != "Tup" || len(pair.Args) != 2 {
				args = append(args, vdText("<expected-pair>"))
				continue
			}
			key, err := valueExpressionDocStyled(pair.Args[0], decorate)
			if err != nil {
				return valueDoc{}, err
			}
			value, err := valueExpressionDocStyled(pair.Args[1], decorate)
			if err != nil {
				return valueDoc{}, err
			}
			args = append(args, vdGroup(key, text(" ->", "gray"), vdNest("  ", vdLine(), value)))
		}
		return vdGroup(text("Map", "green"), vdNary("(", args, ")", vdBreak())), nil
	case "variant":
		if len(expression.Args) != 2 || expression.Args[0].Kind != "str" {
			return valueDoc{}, fmt.Errorf("malformed variant value")
		}
		label := text(expression.Args[0].Str, "green")
		payload := expression.Args[1]
		if payload.Kind == "app" && payload.Opcode == "Tup" && len(payload.Args) == 0 {
			return vdGroup(label), nil
		}
		value, err := valueExpressionDocStyled(payload, decorate)
		if err != nil {
			return valueDoc{}, err
		}
		return vdGroup(label, vdText("("), value, vdText(")")), nil
	case "Set", "List", "Tup":
		for _, arg := range expression.Args {
			value, err := valueExpressionDocStyled(arg, decorate)
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
		return vdGroup(text("Set", "green"), vdNary("(", args, ")", vdBreak())), nil
	}
	return text("unsupported operator: "+expression.Opcode+"(...)", "red"), nil
}
