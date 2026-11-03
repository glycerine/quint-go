package quint

import (
	"fmt"
	"strings"
)

// DeclarationPrintOptions controls diagnostic IR and plain pretty printing.
// The zero value includes definition bodies. A source annotation takes
// precedence over Type. Printers borrow their inputs and never modify them.
type DeclarationPrintOptions struct {
	OmitBody bool
	Type     *TypeScheme
}

// ModuleString prints a module in Quint's diagnostic IR notation. This is not
// a source-preserving formatter: comments and declaration metadata are omitted.
func ModuleString(module Module) (string, error) {
	if module.Name == "" {
		return "", fmt.Errorf("module missing name")
	}
	declarations := make([]string, len(module.Declarations))
	for i, declaration := range module.Declarations {
		text, err := DeclarationString(declaration)
		if err != nil {
			return "", err
		}
		declarations[i] = text
	}
	return "module " + module.Name + " {\n  " + strings.Join(declarations, "\n  ") + "\n}", nil
}

// DeclarationString renders a declaration in the upstream diagnostic IR
// notation. At most one options value may be supplied. Malformed IR returns an
// error rather than causing the underlying expression/type printer to panic.
func DeclarationString(declaration Declaration, options ...DeclarationPrintOptions) (string, error) {
	option, err := declarationPrintOption(options)
	if err != nil {
		return "", err
	}
	if err = validatePrintDeclaration(declaration); err != nil {
		return "", err
	}
	switch declaration.Kind {
	case "def", "var", "const", "assume", "typedef":
		return definitionString(declaration, option)
	case "import", "export":
		text := declaration.Kind + " " + declaration.ProtoName
		if declaration.DefName != nil && *declaration.DefName != "" {
			text += "." + *declaration.DefName
		}
		if declaration.QualifiedName != nil && *declaration.QualifiedName != "" {
			text += " as " + *declaration.QualifiedName
		}
		if declaration.Kind == "import" && declaration.FromSource != nil && *declaration.FromSource != "" {
			text += " from \"" + *declaration.FromSource + "\""
		}
		return text, nil
	case "instance":
		overrides := make([]string, len(declaration.Overrides))
		for i, override := range declaration.Overrides {
			overrides[i] = override.Parameter.Name + " = " + ExpressionString(override.Expr)
		}
		text := "import " + declaration.ProtoName + "(" + strings.Join(overrides, ", ") + ")"
		if declaration.QualifiedName != nil && *declaration.QualifiedName != "" {
			text += " as " + *declaration.QualifiedName
		} else {
			text += ".*"
		}
		if declaration.FromSource != nil && *declaration.FromSource != "" {
			text += " from \"" + *declaration.FromSource + "\""
		}
		return text, nil
	}
	return "", fmt.Errorf("unknown declaration kind %q", declaration.Kind)
}

// DefinitionString prints a def, const, var, assume or typedef declaration.
// Imports, exports and instances return an error; DeclarationString covers them.
func DefinitionString(definition Declaration, options ...DeclarationPrintOptions) (string, error) {
	option, err := declarationPrintOption(options)
	if err != nil {
		return "", err
	}
	if !isDocumentationDefinition(definition) {
		return "", fmt.Errorf("%q is not a definition", definition.Kind)
	}
	if err = validatePrintDeclaration(definition); err != nil {
		return "", err
	}
	return definitionString(definition, option)
}

func declarationPrintOption(options []DeclarationPrintOptions) (DeclarationPrintOptions, error) {
	if len(options) > 1 {
		return DeclarationPrintOptions{}, fmt.Errorf("at most one declaration printing option is supported")
	}
	if len(options) == 1 {
		return options[0], nil
	}
	return DeclarationPrintOptions{}, nil
}

func definitionString(definition Declaration, option DeclarationPrintOptions) (string, error) {
	annotation := ""
	if definition.TypeAnnotation != nil {
		annotation = ": " + TypeString(*definition.TypeAnnotation)
	} else if option.Type != nil {
		if err := option.Type.Type.Validate(); err != nil {
			return "", err
		}
		typ, tree := TypeSchemeString(*option.Type)
		if tree != nil {
			return "", fmt.Errorf("%s", tree.String())
		}
		annotation = ": " + typ
	}
	switch definition.Kind {
	case "def":
		header := QualifierString(definition.Qualifier) + " " + definition.Name + annotation
		if option.OmitBody {
			return header, nil
		}
		return header + " = " + ExpressionString(*definition.Expr), nil
	case "var", "const":
		return definition.Kind + " " + definition.Name + annotation, nil
	case "assume":
		return "assume " + definition.Name + " = " + ExpressionString(*definition.Assumption), nil
	case "typedef":
		text := "type " + definition.Name
		if definition.Type != nil {
			if len(definition.Params) > 0 {
				text += "[" + strings.Join(definition.Params, ", ") + "]"
			}
			text += " = " + TypeString(*definition.Type)
		}
		return text, nil
	}
	return "", fmt.Errorf("unknown definition kind %q", definition.Kind)
}

func validatePrintDeclaration(declaration Declaration) error {
	if err := declaration.Validate(); err != nil {
		return err
	}
	for _, typ := range []*Type{declaration.TypeAnnotation, declaration.Type} {
		if typ != nil {
			if err := typ.Validate(); err != nil {
				return err
			}
		}
	}
	for _, expression := range []*Expr{declaration.Expr, declaration.Assumption} {
		if err := validatePrintExpression(expression); err != nil {
			return err
		}
	}
	for _, override := range declaration.Overrides {
		if override.Parameter.Name == "" {
			return fmt.Errorf("instance override missing parameter name")
		}
		if err := validatePrintExpression(&override.Expr); err != nil {
			return err
		}
	}
	return nil
}

func validatePrintExpression(expression *Expr) error {
	return WalkExpr(expression, func(current *Expr) (bool, error) {
		if current.Kind == "lambda" && (current.Body == nil || !current.Qualifier.Valid()) {
			return false, fmt.Errorf("invalid lambda")
		}
		if current.Kind == "let" {
			if current.Body == nil || current.Opdef == nil || current.Opdef.Kind != "def" || current.Opdef.Expr == nil || !current.Opdef.Qualifier.Valid() {
				return false, fmt.Errorf("invalid let")
			}
			if current.Opdef.TypeAnnotation != nil {
				if err := current.Opdef.TypeAnnotation.Validate(); err != nil {
					return false, err
				}
			}
		}
		return true, nil
	})
}
