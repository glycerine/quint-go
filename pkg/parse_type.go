package quint

import (
	"fmt"
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
	"strconv"
)

// ParseType parses one complete Quint type. IDs are deliberately absent: the
// module parser will assign source identities when constructing annotated IR.
func ParseType(text string) (Type, error) {
	return parseTypeWithIDs(text, nil)
}
func parseTypeWithIDs(text string, ids parserIDAllocator) (Type, error) {
	listener := newSyntaxErrors(text)
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	parser := p.NewQuintParser(tokens)
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	tree := parser.Type_()
	if tokens.LA(1) != antlr.TokenEOF {
		listener.trailing(tokens.LT(1))
	}
	if len(listener.issues) > 0 {
		return Type{}, listener.err("type")
	}
	return typeFromContextWithIDs(tree, ids)
}
func typeFromContext(ctx p.ITypeContext) (Type, error) {
	return typeFromContextWithIDs(ctx, nil)
}
func typeFromContextWithIDs(ctx p.ITypeContext, ids parserIDAllocator) (result Type, err error) {
	if ids != nil {
		defer func() {
			if err == nil && result.ID == nil {
				id := parserIDAt(ids, ctx)
				result.ID = &id
			}
		}()
	}

	one := func(kind string, child p.ITypeContext) (Type, error) {
		v, err := typeFromContextWithIDs(child, ids)
		return Type{Kind: kind, Elem: &v}, err
	}
	many := func(children []p.ITypeContext) ([]Type, error) {
		out := make([]Type, len(children))
		for i, c := range children {
			v, err := typeFromContextWithIDs(c, ids)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	}
	switch c := ctx.(type) {
	case *p.TypeIntContext:
		return Type{Kind: "int"}, nil
	case *p.TypeBoolContext:
		return Type{Kind: "bool"}, nil
	case *p.TypeStrContext:
		return Type{Kind: "str"}, nil
	case *p.TypeVarCaseContext:
		return Type{Kind: "var", Name: c.GetText()}, nil
	case *p.TypeConstContext:
		return Type{Kind: "const", Name: c.GetText()}, nil
	case *p.TypeParenContext:
		return typeFromContextWithIDs(c.Type_(), ids)
	case *p.TypeSetContext:
		return one("set", c.Type_())
	case *p.TypeListContext:
		return one("list", c.Type_())
	case *p.TypeUnitContext:
		return UnitType(), nil
	case *p.TypeFunContext:
		ts, err := many(c.AllType_())
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: "fun", Arg: &ts[0], Res: &ts[1]}, nil
	case *p.TypeOperContext:
		ts, err := many(c.AllType_())
		if err != nil {
			return Type{}, err
		}
		return Type{Kind: "oper", Args: ts[:len(ts)-1], Res: &ts[len(ts)-1]}, nil
	case *p.TypeTupleContext:
		ts, err := many(c.AllType_())
		if err != nil {
			return Type{}, err
		}
		fields := make([]RowField, len(ts))
		for i, t := range ts {
			fields[i] = RowField{Name: strconv.Itoa(i), Type: t}
		}
		return Type{Kind: "tup", Fields: &Row{Kind: "row", Fields: fields, Other: &Row{Kind: "empty"}}}, nil
	case *p.TypeRecContext:
		row := &Row{Kind: "row", Fields: []RowField{}, Other: &Row{Kind: "empty"}}
		if r := c.Row(); r != nil {
			ts, err := many(r.AllType_())
			if err != nil {
				return Type{}, err
			}
			for i, t := range ts {
				row.Fields = append(row.Fields, RowField{Name: r.RowLabel(i).GetText(), Type: t})
			}
			if r.Identifier() != nil {
				row.Other = &Row{Kind: "var", Name: r.Identifier().GetText()}
			}
		}
		return Type{Kind: "rec", Fields: row}, nil
	case *p.TypeAppContext:
		app := c.TypeApplication()
		args, err := many(app.TypeArgs().AllType_())
		if err != nil {
			return Type{}, err
		}
		if app.QualId().GetText() == "Map" && len(args) == 2 {
			key, value := TypeString(args[0]), TypeString(args[1])
			original := "Map[" + key + ", " + value + "]"
			replacement := key + " -> " + value
			var reference *ID
			if ids != nil {
				id := parserIDAt(ids, ctx)
				reference = &id
			}
			return Type{}, Diagnostic{Reference: reference, Code: QNT015, Message: "Use '" + replacement + "' instead of '" + original + "' for map types", Data: &DiagnosticData{Fix: &DiagnosticFix{Kind: "replace", Original: original, Replacement: replacement}}}
		}
		ctor := Type{Kind: "const", Name: app.QualId().GetText()}
		if ids != nil {
			id := parserIDAt(ids, ctx)
			ctor.ID = &id
		}
		return Type{Kind: "app", Ctor: &ctor, Args: args}, nil
	}
	return Type{}, fmt.Errorf("unsupported type parse node %T", ctx)
}
