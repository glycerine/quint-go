package lsp

import (
	"encoding/json"
	"strings"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

// Hover inference selects the smallest analyzed source span, independently of
// lexical name navigation. Field selectors and anonymous lambdas have types but
// are not declarations or name references. Documentation is a separate section
// and remains available on applications that have no inferred-expression hover.
func semanticHoverForPosition(snapshot *Snapshot, index semanticIndex, position Position) any {
	sections := []string{}
	if inferred := hoverInferredSection(snapshot, position); inferred != "" {
		sections = append(sections, inferred)
	}
	params, _ := json.Marshal(struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position Position `json:"position"`
	}{TextDocument: struct {
		URI string `json:"uri"`
	}{URI: snapshot.URI}, Position: position})
	if result, problem := signatureHelpRequest(params, snapshot); problem == nil {
		if signatures, ok := result.(signatureHelp); ok && len(signatures.Signatures) > 0 {
			signature := signatures.Signatures[0]
			text := "```quint\n" + signature.Label + "\n```\n"
			if signature.Documentation != nil && signature.Documentation.Value != "" {
				text += "\n" + signature.Documentation.Value
			}
			sections = append(sections, text)
		}
	}
	if len(sections) == 0 {
		return nil
	}
	result := semanticHover{Contents: semanticMarkup{Kind: "markdown", Value: strings.Join(sections, "\n-----\n")}}
	if occurrence, ok := index.at(snapshot.URI, position); ok {
		result.Range = occurrence.Location.Range
	} else if _, _, location, ok := hoverBestResult(snapshot, position, snapshot.Analysis.Types); ok {
		result.Range = semanticRange(location)
	} else if _, id, found := signatureHelpName(snapshot, position); found {
		result.Range = semanticRange(snapshot.Frontend.Sources[id])
	}
	return result
}

func hoverBestResult[T any](snapshot *Snapshot, position Position, table *insdict.Dict[string, T]) (quint.ID, T, quint.Location, bool) {
	var id quint.ID
	var value T
	var location quint.Location
	found := false
	bestSize := int(^uint(0) >> 1)
	if table == nil {
		return id, value, location, false
	}
	for key, candidate := range table.All() {
		candidateID, err := quint.ParseID(key)
		if err != nil {
			continue
		}
		span, ok := snapshot.Frontend.Sources[candidateID]
		if !ok || uriForSource(span.Source) != snapshot.URI || !signatureLocationContains(span, position) {
			continue
		}
		size := 0
		if span.End != nil {
			size = span.End.Index - span.Start.Index
		}
		// The upstream span sorter prefers the later result on equal spans.
		if !found || size <= bestSize {
			id, value, location, found, bestSize = candidateID, candidate, span, true, size
		}
	}
	return id, value, location, found
}

func hoverInferredSection(snapshot *Snapshot, position Position) string {
	id, scheme, location, found := hoverBestResult(snapshot, position, snapshot.Analysis.Types)
	if !found {
		return ""
	}
	expression, definition := hoverNode(snapshot, id)
	if expression == nil && definition == nil {
		return ""
	}
	typeText := hoverTypeScheme(scheme)
	text := ""
	if definition != nil {
		text = hoverDeclaration(*definition, typeText)
	} else {
		switch expression.Kind {
		case "app":
			if expression.Opcode == "field" && len(expression.Args) == 2 && expression.Args[1].Kind == "str" {
				text = "(field) " + expression.Args[1].Str + ": " + typeText
			}
		case "name":
			if table := snapshot.Frontend.Table; table != nil {
				if binding, ok := table.Get2(expression.ID); ok {
					_, declared := hoverNode(snapshot, binding.ID)
					qualifier := "(parameter)"
					if declared != nil {
						qualifier = declared.Kind
						if declared.Kind == "def" {
							qualifier = quint.QualifierString(declared.Qualifier)
						}
					}
					text = qualifier + " " + expression.Name + ": " + typeText
				}
			}
			if text == "" {
				text = hoverSourceText(snapshot, location) + ": " + typeText
			}
		case "lambda":
			text = hoverSourceText(snapshot, location) + ": " + typeText
		}
	}
	if text == "" {
		return ""
	}
	section := "```quint\n" + text + "\n```\n"
	if _, effect, _, ok := hoverBestResult(snapshot, position, snapshot.Analysis.Effects); ok {
		if effectText, err := quint.EffectSchemeString(effect); err == nil {
			section += "\n**effect**: `" + effectText + "`\n"
		}
	}
	return section
}

func hoverNode(snapshot *Snapshot, id quint.ID) (*quint.Expr, *quint.Declaration) {
	var expression *quint.Expr
	var definition *quint.Declaration
	visit := func(root *quint.Expr) {
		_ = quint.WalkExpr(root, func(node *quint.Expr) (bool, error) {
			if node.ID == id {
				expression = node
			}
			if node.Kind == "let" && node.Opdef != nil && node.Opdef.ID == id {
				op := node.Opdef
				definition = &quint.Declaration{Kind: "def", ID: op.ID, Name: op.Name, Qualifier: op.Qualifier, Expr: op.Expr, TypeAnnotation: op.TypeAnnotation}
			}
			return true, nil
		})
	}
	for _, module := range snapshot.Frontend.Modules {
		for i := range module.Declarations {
			declaration := &module.Declarations[i]
			if declaration.ID == id {
				definition = declaration
			}
			visit(declaration.Expr)
			visit(declaration.Assumption)
			for j := range declaration.Overrides {
				visit(&declaration.Overrides[j].Expr)
			}
		}
	}
	return expression, definition
}

func hoverSourceText(snapshot *Snapshot, location quint.Location) string {
	text, ok := semanticSource(snapshot, location.Source)
	if !ok || location.End == nil {
		return ""
	}
	file, err := quint.NewSourceFile(location.Source, text)
	if err != nil {
		return ""
	}
	start, err := file.ByteOffset(location.Start.Line, location.Start.Col)
	if err != nil {
		return ""
	}
	end, err := file.ByteOffset(location.End.Line, location.End.Col)
	if err != nil || end < start {
		return ""
	}
	return text[start:end]
}

func hoverDeclaration(declaration quint.Declaration, inferred string) string {
	annotation := inferred
	if declaration.TypeAnnotation != nil {
		annotation = hoverPrettyType(*declaration.TypeAnnotation)
	}
	switch declaration.Kind {
	case "def":
		return quint.QualifierString(declaration.Qualifier) + " " + declaration.Name + ": " + annotation
	case "const", "var":
		if declaration.TypeAnnotation != nil {
			annotation = quint.TypeString(*declaration.TypeAnnotation)
		}
		return declaration.Kind + " " + declaration.Name + ": " + annotation
	case "typedef":
		text := "type " + declaration.Name
		if declaration.Type != nil {
			text += " = " + hoverPrettyType(*declaration.Type)
		}
		return text
	case "assume":
		if declaration.Assumption != nil {
			return "assume " + declaration.Name + " = " + quint.ExpressionString(*declaration.Assumption)
		}
	}
	return ""
}

func hoverTypeScheme(scheme quint.TypeScheme) string {
	names, typ, err := quint.CanonicalTypeScheme(scheme)
	if err != nil {
		return quint.TypeString(scheme.Type)
	}
	prefix := ""
	if len(names) > 0 {
		prefix = "∀ " + strings.Join(names, ", ") + " . "
	}
	return prefix + hoverPrettyType(typ)
}

// The editor uses graphics.ts's compact type layout, which differs from the
// diagnostic IR printer around record closers and function arrows.
func hoverPrettyType(typ quint.Type) string {
	switch typ.Kind {
	case "bool", "int", "str":
		return typ.Kind
	case "const", "var":
		return typ.Name
	case "set", "list":
		name := "Set"
		if typ.Kind == "list" {
			name = "List"
		}
		if typ.Elem != nil {
			return name + "[" + hoverPrettyType(*typ.Elem) + "]"
		}
	case "fun":
		if typ.Arg != nil && typ.Res != nil {
			return hoverPrettyType(*typ.Arg) + " -> " + hoverPrettyType(*typ.Res)
		}
	case "oper":
		arguments := make([]string, len(typ.Args))
		for i, argument := range typ.Args {
			arguments[i] = hoverPrettyType(argument)
		}
		if typ.Res != nil {
			return "(" + strings.Join(arguments, ", ") + ") => " + hoverPrettyType(*typ.Res)
		}
	case "tup", "rec":
		if typ.Fields != nil {
			row := hoverPrettyRow(*typ.Fields, typ.Kind == "rec")
			if typ.Kind == "tup" {
				return "(" + row + ")"
			}
			if row == "" {
				return "{}"
			}
			return "{ " + row + "}"
		}
	case "sum":
		if typ.Fields != nil {
			fields := []quint.RowField{}
			current := typ.Fields
			for current != nil && current.Kind == "row" {
				fields = append(fields, current.Fields...)
				current = current.Other
			}
			closed := current != nil && current.Kind == "empty"
			variants := make([]string, len(fields))
			for i, field := range fields {
				variants[i] = field.Name
				// graphics.ts displays just the labels of a closed sum row.
				if !closed && !field.Type.IsUnit() {
					variants[i] += "(" + hoverPrettyType(field.Type) + ")"
				}
			}
			return strings.Join(variants, "| ")
		}
	}
	return quint.TypeString(typ)
}
func hoverPrettyRow(row quint.Row, names bool) string {
	fields := []string{}
	current := &row
	for current != nil && current.Kind == "row" {
		for _, field := range current.Fields {
			prefix := ""
			if names {
				prefix = field.Name + ": "
			}
			fields = append(fields, prefix+hoverPrettyType(field.Type))
		}
		current = current.Other
	}
	text := strings.Join(fields, ", ")
	if current != nil && current.Kind == "var" {
		text += " | " + current.Name
	}
	return text
}
