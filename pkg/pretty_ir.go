package quint

import "fmt"

// FormatType renders a type using Quint's plain, width-aware graphics layout.
// Width and firstColumn count UTF-16 units. Inputs are borrowed and unchanged.
// Type applications return an error: the upstream pretty renderer recurses on
// itself for that variant. TypeString supports their diagnostic IR notation.
// Closed sums retain the upstream graphics display of variant labels only.
func FormatType(typ Type, width, firstColumn int) (string, error) {
	if err := prettyIRDimensions(width, firstColumn); err != nil {
		return "", err
	}
	if err := typ.Validate(); err != nil {
		return "", err
	}
	doc, err := prettyIRType(typ)
	if err != nil {
		return "", err
	}
	return formatValueDoc(doc, width, firstColumn), nil
}

// FormatTypeScheme canonically renames quantified variables and renders the
// result with Quint's plain graphics layout. See FormatType for width and type
// application limits. No part of the supplied scheme is modified.
func FormatTypeScheme(scheme TypeScheme, width, firstColumn int) (string, error) {
	if err := prettyIRDimensions(width, firstColumn); err != nil {
		return "", err
	}
	if err := scheme.Type.Validate(); err != nil {
		return "", err
	}
	doc, err := prettyIRScheme(scheme)
	if err != nil {
		return "", err
	}
	return formatValueDoc(doc, width, firstColumn), nil
}

// FormatDeclaration renders a declaration using Quint's plain graphics layout.
// A def body uses the value pretty printer, which visibly labels expressions
// that are not runtime values. Other declaration kinds retain upstream's
// diagnostic-print fallback; typedef graphics omit type parameters as upstream
// does. Inputs are borrowed and unchanged. At most one options value is allowed.
func FormatDeclaration(declaration Declaration, width, firstColumn int, options ...DeclarationPrintOptions) (string, error) {
	if err := prettyIRDimensions(width, firstColumn); err != nil {
		return "", err
	}
	option, err := declarationPrintOption(options)
	if err != nil {
		return "", err
	}
	if err = validatePrintDeclaration(declaration); err != nil {
		return "", err
	}
	var annotation valueDoc
	if declaration.TypeAnnotation != nil {
		annotation, err = prettyIRType(*declaration.TypeAnnotation)
		annotation = vdConcat(vdText(": "), annotation)
	} else if option.Type != nil {
		if err = option.Type.Type.Validate(); err == nil {
			annotation, err = prettyIRScheme(*option.Type)
		}
		annotation = vdConcat(vdText(": "), annotation)
	}
	if err != nil {
		return "", err
	}
	var doc valueDoc
	switch declaration.Kind {
	case "def":
		doc = vdGroup(vdText(QualifierString(declaration.Qualifier)), vdText(" "), vdText(declaration.Name), annotation)
		if !option.OmitBody {
			body, bodyErr := valueExpressionDoc(*declaration.Expr)
			if bodyErr != nil {
				return "", bodyErr
			}
			doc = vdGroup(doc, vdText(" = "), body)
		}
	case "typedef":
		doc = vdGroup(vdText("type "), vdText(declaration.Name))
		if declaration.Type != nil {
			typ, typeErr := prettyIRType(*declaration.Type)
			if typeErr != nil {
				return "", typeErr
			}
			doc = vdGroup(doc, vdText(" = "), typ)
		}
	default:
		text, textErr := DeclarationString(declaration)
		if textErr != nil {
			return "", textErr
		}
		doc = vdText(text)
	}
	return formatValueDoc(doc, width, firstColumn), nil
}

func prettyIRDimensions(width, firstColumn int) error {
	if width < 0 || firstColumn < 0 {
		return fmt.Errorf("IR formatting requires nonnegative width and first column")
	}
	return nil
}

func prettyIRScheme(scheme TypeScheme) (valueDoc, error) {
	names, typ, tree := CanonicalTypeScheme(scheme)
	if tree != nil {
		return valueDoc{}, fmt.Errorf("%s", tree.String())
	}
	var variables valueDoc
	if len(names) > 0 {
		docs := make([]valueDoc, len(names))
		for i, name := range names {
			docs[i] = vdText(name)
		}
		variables = vdGroup(vdText("∀ "), vdJoin(vdConcat(vdText(","), vdLine()), docs), vdText(" . "))
	}
	doc, err := prettyIRType(typ)
	if err != nil {
		return valueDoc{}, err
	}
	return vdGroup(variables, doc), nil
}

func prettyIRType(typ Type) (valueDoc, error) {
	switch typ.Kind {
	case "bool", "int", "str":
		return vdText(typ.Kind), nil
	case "const", "var":
		return vdText(typ.Name), nil
	case "set", "list":
		name := "Set"
		if typ.Kind == "list" {
			name = "List"
		}
		element, err := prettyIRType(*typ.Elem)
		return vdGroup(vdText(name), vdText("["), element, vdText("]")), err
	case "fun":
		arg, err := prettyIRType(*typ.Arg)
		if err != nil {
			return valueDoc{}, err
		}
		res, err := prettyIRType(*typ.Res)
		return vdGroup(arg, vdText(" -> "), res), err
	case "oper":
		args := make([]valueDoc, len(typ.Args))
		for i, arg := range typ.Args {
			var err error
			args[i], err = prettyIRType(arg)
			if err != nil {
				return valueDoc{}, err
			}
		}
		res, err := prettyIRType(*typ.Res)
		return vdGroup(vdNary("(", args, ")", vdBreak()), vdText(" => "), res), err
	case "tup":
		row, err := prettyIRRow(*typ.Fields, false)
		return vdGroup(vdText("("), row, vdText(")")), err
	case "rec":
		if RowString(*typ.Fields) == "{}" {
			return vdText("{}"), nil
		}
		row, err := prettyIRRow(*typ.Fields, true)
		return vdGroup(vdText("{ "), row, vdText("}")), err
	case "sum":
		return prettyIRSumRow(*typ.Fields)
	case "app":
		return valueDoc{}, fmt.Errorf("pretty formatting of type applications is unsupported by the upstream renderer; use TypeString")
	}
	return valueDoc{}, fmt.Errorf("unknown type kind %q", typ.Kind)
}

func prettyIRRow(input Row, showFieldName bool) (valueDoc, error) {
	row := SimplifyRow(input)
	fields := []valueDoc{}
	var tail valueDoc
	if row.Kind == "row" {
		for _, field := range row.Fields {
			prefix := ""
			if showFieldName {
				prefix = field.Name + ": "
			}
			typ, err := prettyIRType(field.Type)
			if err != nil {
				return valueDoc{}, err
			}
			fields = append(fields, vdGroup(vdText(prefix), typ))
		}
		if row.Other.Kind == "var" {
			tail = vdText(" | " + row.Other.Name)
		}
	}
	return vdGroup(vdNest("  ", vdBreak(), vdJoin(vdConcat(vdText(","), vdLine()), fields)), tail, vdBreak()), nil
}

func prettyIRSumRow(input Row) (valueDoc, error) {
	row := SimplifyRow(input)
	fields := []valueDoc{}
	if row.Kind == "row" {
		for _, field := range row.Fields {
			if row.Other.Kind == "empty" || field.Type.IsUnit() {
				fields = append(fields, vdGroup(vdText(field.Name)))
				continue
			}
			typ, err := prettyIRType(field.Type)
			if err != nil {
				return valueDoc{}, err
			}
			fields = append(fields, vdGroup(vdText(field.Name+"("), typ, vdText(")")))
		}
	}
	return vdGroup(vdNest("  ", vdBreak(), vdJoin(vdConcat(vdText("|"), vdLine()), fields)), vdBreak()), nil
}
