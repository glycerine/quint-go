package quint

import p "github.com/glycerine/quint-go/pkg/internal/quintparser"

// sumDeclarations expands a sum declaration into its type and variant operators.
func sumDeclarations(ctx *p.TypeSumDefContext, ids parserIDAllocator) ([]Declaration, error) {
	head, sum := ctx.TypeDefHead(), ctx.SumTypeDefinition()
	d := Declaration{ID: ids.Next(), Kind: "typedef", Name: head.GetTypeName().GetText()}
	for _, param := range head.AllLOW_ID() {
		d.Params = append(d.Params, param.GetText())
	}
	fields := []RowField{}
	for _, variant := range sum.AllTypeSumVariant() {
		typ := Type{Kind: "tup", Fields: &Row{Kind: "row", Fields: []RowField{}, Other: &Row{Kind: "empty"}}}
		if variant.Type_() != nil {
			var err error
			typ, err = typeFromContext(variant.Type_())
			if err != nil {
				return nil, err
			}
		}
		fields = append(fields, RowField{Name: variant.GetSumLabel().GetText(), Type: typ})
	}
	d.Type = &Type{Kind: "sum", Fields: &Row{Kind: "row", Fields: fields, Other: &Row{Kind: "empty"}}}
	// Upstream checks sum parameters, but applies its uppercase-name check only
	// to abstract and alias declarations.
	if err := checkTypeDeclaration(d, false); err != nil {
		return nil, err
	}
	if len(fields) == 1 && isUnitType(fields[0].Type) && sum.GetSeparator() == nil {
		d.Type = &Type{Kind: "const", Name: sum.TypeSumVariant(0).GetText()}
		d.Params = nil // Match upstream's alias disambiguation.
		return []Declaration{d}, nil
	}
	result := Type{Kind: "const", Name: d.Name}
	if len(d.Params) > 0 {
		ctor := result
		result = Type{Kind: "app", Ctor: &ctor, Args: []Type{}}
		for _, param := range d.Params {
			result.Args = append(result.Args, Type{Kind: "var", Name: param})
		}
	}
	decls := []Declaration{d}
	for _, field := range fields {
		label := Expr{ID: ids.Next(), Kind: "str", Str: field.Name}
		payload := Expr{ID: ids.Next(), Kind: "app", Opcode: "Tup", Args: []Expr{}}
		annotation, qualifier := result, Val
		var parameter Parameter
		if !isUnitType(field.Type) {
			qualifier = Def
			parameter = Parameter{ID: ids.Next(), Name: "__" + field.Name + "Param"}
			payload.Kind, payload.Name, payload.Opcode, payload.Args = "name", parameter.Name, "", nil
			annotation = Type{Kind: "oper", Args: []Type{field.Type}, Res: &result}
		}
		expr := Expr{ID: ids.Next(), Kind: "app", Opcode: "variant", Args: []Expr{label, payload}}
		if qualifier == Def {
			body := expr
			expr = Expr{ID: ids.Next(), Kind: "lambda", Params: []Parameter{parameter}, Qualifier: Def, Body: &body}
		}
		decls = append(decls, Declaration{ID: ids.Next(), Kind: "def", Name: field.Name, Qualifier: qualifier, TypeAnnotation: &annotation, Expr: &expr})
	}
	return decls, nil
}

func isUnitType(t Type) bool {
	return t.Kind == "tup" && t.Fields != nil && t.Fields.Kind == "row" && len(t.Fields.Fields) == 0 && t.Fields.Other != nil && t.Fields.Other.Kind == "empty"
}
