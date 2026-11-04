package apalache

import (
	"fmt"
	"math/big"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

// adaptFoldRight lowers Quint foldr to foldl over a reversed sequence for
// checker and TLA inputs. The caller-owned compilation remains unchanged.
func adaptFoldRight(compiled quint.CompilationResult) (quint.CompilationResult, error) {
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
	newExpr := func(op string, args []quint.Expr, typ quint.Type) quint.Expr {
		id := ids.Next()
		types.Set(id.String(), quint.TypeScheme{Type: typ, TypeVariables: []string{}, RowVariables: []string{}})
		return quint.Expr{ID: id, Kind: "app", Opcode: op, Args: args}
	}
	newName := func(name string, typ quint.Type) (quint.Expr, quint.Parameter) {
		id := ids.Next()
		types.Set(id.String(), quint.TypeScheme{Type: typ, TypeVariables: []string{}, RowVariables: []string{}})
		return quint.Expr{ID: id, Kind: "name", Name: name}, quint.Parameter{ID: id, Name: name}
	}
	transformer := &quint.IRTransformer{ExitExpression: func(expression quint.Expr) (quint.Expr, error) {
		if expression.Kind != "app" || expression.Opcode != "foldr" {
			return expression, nil
		}
		if len(expression.Args) != 3 {
			return quint.Expr{}, fmt.Errorf("TLA foldr lowering requires three arguments")
		}
		seqScheme, found := types.Get2(expression.Args[0].ID.String())
		if !found || seqScheme.Type.Kind != "list" || seqScheme.Type.Elem == nil {
			return quint.Expr{}, fmt.Errorf("TLA foldr lowering requires a typed sequence")
		}
		seqType := seqScheme.Type
		elemType := *seqType.Elem
		listType := seqType
		accName, accParam := newName("__quint_foldr_acc", listType)
		itemName, itemParam := newName("__quint_foldr_item", elemType)
		prepend := newExpr("List", []quint.Expr{itemName}, listType)
		concat := newExpr("concat", []quint.Expr{prepend, accName}, listType)
		reverseCallbackType := quint.Type{Kind: "oper", Res: &listType, Args: []quint.Type{listType, elemType}}
		reverseCallback := quint.Expr{ID: ids.Next(), Kind: "lambda", Params: []quint.Parameter{accParam, itemParam}, Qualifier: quint.Def, Body: &concat}
		types.Set(reverseCallback.ID.String(), quint.TypeScheme{Type: reverseCallbackType, TypeVariables: []string{}, RowVariables: []string{}})
		empty := newExpr("List", []quint.Expr{}, listType)
		reversed := newExpr("foldl", []quint.Expr{expression.Args[0], empty, reverseCallback}, listType)
		foldScheme, found := types.Get2(expression.ID.String())
		if !found {
			return quint.Expr{}, fmt.Errorf("TLA foldr lowering requires a typed result")
		}
		foldType := foldScheme.Type
		callback := expression.Args[2]
		switch callback.Kind {
		case "lambda":
			if len(callback.Params) != 2 || callback.Body == nil {
				return quint.Expr{}, fmt.Errorf("TLA foldr lowering requires a two-parameter lambda")
			}
			// foldl supplies (accumulator, element); foldr's body expects
			// (element, accumulator). Reordering parameters keeps body references
			// bound to their original IDs.
			callback.Params[0], callback.Params[1] = callback.Params[1], callback.Params[0]
		case "name":
			accType, found := types.Get2(expression.Args[1].ID.String())
			if !found {
				return quint.Expr{}, fmt.Errorf("TLA foldr lowering requires a typed accumulator")
			}
			acc, accParameter := newName("__quint_foldr_acc", accType.Type)
			item, itemParameter := newName("__quint_foldr_item", elemType)
			call := newExpr(callback.Name, []quint.Expr{item, acc}, foldType)
			callback = quint.Expr{ID: ids.Next(), Kind: "lambda", Qualifier: "def", Params: []quint.Parameter{accParameter, itemParameter}, Body: &call}
			callbackType := quint.Type{Kind: "oper", Res: &foldType, Args: []quint.Type{accType.Type, elemType}}
			types.Set(callback.ID.String(), quint.TypeScheme{Type: callbackType, TypeVariables: []string{}, RowVariables: []string{}})
		default:
			return quint.Expr{}, fmt.Errorf("TLA foldr lowering requires a lambda or named operator callback")
		}
		foldl := newExpr("foldl", []quint.Expr{reversed, expression.Args[1], callback}, foldType)
		types.Del(foldl.ID.String())
		foldl.ID = expression.ID
		return foldl, nil
	}}
	module, err := transformer.TransformModule(compiled.MainModule)
	if err != nil {
		return compiled, err
	}
	compiled.MainModule = module
	compiled.Analysis.Types = types
	return compiled, nil
}
