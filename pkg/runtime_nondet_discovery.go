package quint

import (
	"context"

	"github.com/glycerine/insdict"
)

// RuntimeNondetDiscovery models the compiler's registration of pick names.
// Keep it with one evaluator so compiling a new nondet binding registers an
// unset pick, while revisiting already compiled bindings preserves their picks.
// Input IDs must be unique within the evaluator, as for reference compilation.
// The zero value is ready to use. It is not safe for concurrent use.
type RuntimeNondetDiscovery struct {
	compiled map[ID]bool
}

type runtimeDiscoveryDefinition struct {
	expression *Expr
	scopes     []*insdict.Dict[string, *runtimeDiscoveryDefinition]
	visited    bool
}

// Prepare follows compilation order without executing expressions or drawing
// random values. It discovers lazy branches and lambda bodies, but does not
// compile an unused ordinary let definition. A referenced definition is compiled
// under its lexical scope; application arguments precede the operator's body.
// storage may be nil when metadata is not needed by a pure evaluation.
func (d *RuntimeNondetDiscovery) Prepare(ctx context.Context, expression Expr, storage *VariableStorage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if storage == nil {
		return nil
	}
	if d.compiled == nil {
		d.compiled = make(map[ID]bool)
	}
	var visit func(*Expr, []*insdict.Dict[string, *runtimeDiscoveryDefinition]) error
	lookup := func(name string, scopes []*insdict.Dict[string, *runtimeDiscoveryDefinition]) *runtimeDiscoveryDefinition {
		for i := len(scopes) - 1; i >= 0; i-- {
			if definition, exists := scopes[i].Get2(name); exists {
				return definition
			}
		}
		return nil
	}
	compile := func(definition *runtimeDiscoveryDefinition) error {
		if definition == nil || definition.visited {
			return nil
		}
		definition.visited = true
		return visit(definition.expression, definition.scopes)
	}
	visit = func(expression *Expr, scopes []*insdict.Dict[string, *runtimeDiscoveryDefinition]) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if expression == nil {
			return nil
		}
		switch expression.Kind {
		case "name":
			return compile(lookup(expression.Name, scopes))
		case "lambda":
			parameters := insdict.NewDict[string, *runtimeDiscoveryDefinition]()
			for _, parameter := range expression.Params {
				parameters.Set(parameter.Name, &runtimeDiscoveryDefinition{visited: true})
			}
			return visit(expression.Body, append(append([]*insdict.Dict[string, *runtimeDiscoveryDefinition](nil), scopes...), parameters))
		case "app":
			// Assignment compiles only its RHS; the left name is a variable
			// reference, not an operator/value dependency.
			if expression.Opcode == "assign" {
				if len(expression.Args) > 1 {
					return visit(&expression.Args[1], scopes)
				}
				return nil
			}
			for i := range expression.Args {
				if err := visit(&expression.Args[i], scopes); err != nil {
					return err
				}
			}
			return compile(lookup(expression.Opcode, scopes))
		case "let":
			if expression.Opdef == nil {
				return nil
			}
			definition := expression.Opdef
			frame := insdict.NewDict[string, *runtimeDiscoveryDefinition]()
			bound := &runtimeDiscoveryDefinition{expression: definition.Expr}
			frame.Set(definition.Name, bound)
			local := append(append([]*insdict.Dict[string, *runtimeDiscoveryDefinition](nil), scopes...), frame)
			bound.scopes = local
			if definition.Qualifier == Nondet {
				// The reference installs the nondet cache before compiling the
				// sampled set and body. Its definition expression is not built
				// as a normal oneOf application.
				bound.visited = true
				if !d.compiled[definition.ID] {
					storage.NondetPicks.Set(definition.Name, RuntimeValue{})
					d.compiled[definition.ID] = true
				}
				if definition.Expr != nil && len(definition.Expr.Args) > 0 {
					if err := visit(&definition.Expr.Args[0], local); err != nil {
						return err
					}
				}
			}
			return visit(expression.Body, local)
		}
		return nil
	}
	return visit(&expression, nil)
}
