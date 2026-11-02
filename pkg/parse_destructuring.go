package quint

import (
	"fmt"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
)

// destructuredOperators returns nil for ordinary operator definitions.
func destructuredOperators(ctx p.IOperDefContext, ids *IDGenerator) ([]OperatorDef, error) {
	var pattern p.IDestructuringPatternContext
	var rhs p.IExprContext
	qualifier := Val
	switch c := ctx.(type) {
	case *p.ValDestructuringContext:
		pattern, rhs = c.DestructuringPattern(), c.Expr()
	case *p.PureValDestructuringContext:
		pattern, rhs = c.DestructuringPattern(), c.Expr()
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
		defID := ids.Next()
		appID := ids.Next()
		target := Expr{ID: ids.Next(), Kind: "name", Name: temp.Name}
		accessor := Expr{ID: ids.Next(), Kind: "str", Str: name}
		opcode := "field"
		if tuple {
			opcode = "item"
			accessor.Kind = "int"
			accessor.Str = ""
			accessor.Int, _ = ParseInteger(fmt.Sprint(i + 1))
		}
		access := Expr{ID: appID, Kind: "app", Opcode: opcode, Args: []Expr{target, accessor}}
		defs = append(defs, OperatorDef{ID: defID, Kind: "def", Name: name, Qualifier: qualifier, Expr: &access})
	}
	return defs, nil
}
