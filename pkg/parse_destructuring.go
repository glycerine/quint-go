package quint

import (
	"fmt"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
)

// destructuredOperators returns nil for ordinary operator definitions.
func destructuredOperators(ctx p.IOperDefContext, ids parserIDAllocator) ([]OperatorDef, error) {
	defer parserScope(ids, ctx)()
	var rhs p.IExprContext
	qualifier := Val
	switch c := ctx.(type) {
	case *p.ValDestructuringContext:
		rhs = c.Expr()
	case *p.PureValDestructuringContext:
		rhs = c.Expr()
		qualifier = PureVal
	default:
		return nil, nil
	}
	value, err := expressionFromContext(rhs, ids)
	if err != nil {
		return nil, err
	}
	id := ids.Next()
	temp := OperatorDef{ID: id, Kind: "def", Name: "quintDestructTemp" + id.String(), Qualifier: qualifier, Expr: &value}
	defs := []OperatorDef{temp}
	if _, local := ctx.GetParent().(*p.LetInContext); !local {
		destructuredFields(ctx, temp, ids, func(op OperatorDef) { defs = append(defs, op) })
	}
	return defs, nil
}

// Local field bindings are created by exitLetIn, after walking the body;
// module-level bindings are created immediately by exitValDestructuring.
func destructuredFields(ctx p.IOperDefContext, temp OperatorDef, ids parserIDAllocator, visit func(OperatorDef)) {
	var pattern p.IDestructuringPatternContext
	switch c := ctx.(type) {
	case *p.ValDestructuringContext:
		pattern = c.DestructuringPattern()
	case *p.PureValDestructuringContext:
		pattern = c.DestructuringPattern()
	}
	names := []string{}
	tuple := pattern.TuplePattern() != nil
	if tuple {
		for _, n := range pattern.TuplePattern().AllIdentOrHole() {
			names = append(names, n.GetText())
		}
	} else {
		for _, n := range pattern.RecordPattern().AllSimpleId() {
			names = append(names, n.GetText())
		}
	}
	for i, name := range names {
		if name == "_" {
			continue
		}
		restore := parserScope(ids, ctx)
		defID := parserSyntheticID(ids)
		appID := parserSyntheticID(ids)
		target := Expr{ID: parserSyntheticID(ids), Kind: "name", Name: temp.Name}
		accessor := Expr{ID: parserSyntheticID(ids), Kind: "str", Str: name}
		restore()
		opcode := "field"
		if tuple {
			opcode = "item"
			accessor.Kind = "int"
			accessor.Str = ""
			accessor.Int, _ = ParseInteger(fmt.Sprint(i + 1))
		}
		access := Expr{ID: appID, Kind: "app", Opcode: opcode, Args: []Expr{target, accessor}}
		visit(OperatorDef{ID: defID, Kind: "def", Name: name, Qualifier: temp.Qualifier, Expr: &access})
	}
}
