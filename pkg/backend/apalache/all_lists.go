package apalache

import (
	"fmt"
	"math/big"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

// adaptAllListsUpTo lowers finite bounded lists to a set fold. The fold starts
// with the empty list; each of n steps adds every one-element extension of the
// lists accumulated so far. The same adapter is used for CHECK and TLA input.
func adaptAllListsUpTo(compiled quint.CompilationResult) (quint.CompilationResult, error) {
	if compiled.Analysis.Types == nil {
		return compiled, nil
	}
	maximum := new(big.Int)
	for key := range compiled.Analysis.Types.All() {
		if value, ok := new(big.Int).SetString(key, 10); ok && value.Cmp(maximum) > 0 {
			maximum.Set(value)
		}
	}
	last, err := quint.ParseID(maximum.String())
	if err != nil {
		return compiled, err
	}
	ids := quint.NewIDGeneratorAfter(last)
	types := insdict.NewDict[string, quint.TypeScheme]()
	for key, scheme := range compiled.Analysis.Types.All() {
		types.Set(key, scheme)
	}
	addType := func(id quint.ID, typ quint.Type) {
		types.Set(id.String(), quint.TypeScheme{Type: typ, TypeVariables: []string{}, RowVariables: []string{}})
	}
	app := func(op string, args []quint.Expr, typ quint.Type) quint.Expr {
		id := ids.Next()
		addType(id, typ)
		return quint.Expr{ID: id, Kind: "app", Opcode: op, Args: args}
	}
	name := func(value string, typ quint.Type) (quint.Expr, quint.Parameter) {
		id := ids.Next()
		addType(id, typ)
		return quint.Expr{ID: id, Kind: "name", Name: value}, quint.Parameter{ID: id, Name: value}
	}
	makeOper := func(result quint.Type, args ...quint.Type) quint.Type {
		return quint.Type{Kind: "oper", Res: &result, Args: args}
	}
	transformer := &quint.IRTransformer{ExitExpression: func(expression quint.Expr) (quint.Expr, error) {
		if expression.Kind != "app" || expression.Opcode != "allListsUpTo" {
			return expression, nil
		}
		if len(expression.Args) != 2 {
			return quint.Expr{}, fmt.Errorf("TLA allListsUpTo lowering requires two arguments")
		}
		setScheme, found := types.Get2(expression.Args[0].ID.String())
		if !found || setScheme.Type.Kind != "set" || setScheme.Type.Elem == nil {
			return quint.Expr{}, fmt.Errorf("TLA allListsUpTo lowering requires a typed set")
		}
		elemType := *setScheme.Type.Elem
		listType := quint.Type{Kind: "list", Elem: &elemType}
		setOfListsType := quint.Type{Kind: "set", Elem: &listType}
		intType := quint.Type{Kind: "int"}
		setOfIntsType := quint.Type{Kind: "set", Elem: &intType}

		// For each source element, add extensions of the unchanged outer set.
		extensions, extensionsParam := name("__quint_lists_extensions", setOfListsType)
		element, elementParam := name("__quint_lists_element", elemType)
		prefix, prefixParam := name("__quint_lists_prefix", listType)
		appended := app("append", []quint.Expr{prefix, element}, listType)
		appendedCallback := quint.Expr{ID: ids.Next(), Kind: "lambda", Qualifier: quint.Def, Params: []quint.Parameter{prefixParam}, Body: &appended}
		addType(appendedCallback.ID, makeOper(listType, listType))
		outerLists, outerListsParam := name("__quint_lists_outer", setOfListsType)
		mapped := app("map", []quint.Expr{outerLists, appendedCallback}, setOfListsType)
		union := app("union", []quint.Expr{extensions, mapped}, setOfListsType)
		innerCallback := quint.Expr{ID: ids.Next(), Kind: "lambda", Qualifier: quint.Def, Params: []quint.Parameter{extensionsParam, elementParam}, Body: &union}
		addType(innerCallback.ID, makeOper(setOfListsType, setOfListsType, elemType))
		innerEmptySet := app("Set", []quint.Expr{}, setOfListsType)
		innerFold := app("fold", []quint.Expr{expression.Args[0], innerEmptySet, innerCallback}, setOfListsType)

		// Fold over 1..n; the integer parameter is intentionally unused.
		iterationID := ids.Next()
		addType(iterationID, intType)
		iterationParam := quint.Parameter{ID: iterationID, Name: "__quint_lists_iteration"}
		outerBody := app("union", []quint.Expr{outerLists, innerFold}, setOfListsType)
		outerCallback := quint.Expr{ID: ids.Next(), Kind: "lambda", Qualifier: quint.Def, Params: []quint.Parameter{outerListsParam, iterationParam}, Body: &outerBody}
		addType(outerCallback.ID, makeOper(setOfListsType, setOfListsType, intType))
		oneValue, _ := quint.ParseInteger("1")
		one := quint.Expr{ID: ids.Next(), Kind: "int", Int: oneValue}
		addType(one.ID, intType)
		rangeExpr := app("to", []quint.Expr{one, expression.Args[1]}, setOfIntsType)
		emptyList := app("List", []quint.Expr{}, listType)
		emptySet := app("Set", []quint.Expr{emptyList}, setOfListsType)
		foldScheme, found := types.Get2(expression.ID.String())
		if !found {
			return quint.Expr{}, fmt.Errorf("TLA allListsUpTo lowering requires a typed result")
		}
		outerFold := app("fold", []quint.Expr{rangeExpr, emptySet, outerCallback}, foldScheme.Type)
		types.Del(outerFold.ID.String())
		outerFold.ID = expression.ID
		return outerFold, nil
	}}
	module, err := transformer.TransformModule(compiled.MainModule)
	if err != nil {
		return compiled, err
	}
	compiled.MainModule = module
	compiled.Analysis.Types = types
	return compiled, nil
}
