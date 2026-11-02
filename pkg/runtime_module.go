package quint

import (
	"context"
	"github.com/glycerine/insdict"
)

// ModuleEvaluator retains registers and RNG for a standalone/flattened module.
// Callers use Shift to commit and record states, or recover Storage between actions.
// Each instance is owned by one execution and must be used sequentially.
type ModuleEvaluator struct {
	Storage     *VariableStorage
	RNG         *RNG
	Trace       RuntimeTrace
	module      Module
	definitions *insdict.Dict[string, Declaration]
}

func NewModuleEvaluator(module Module, rng *RNG, metadata bool) (*ModuleEvaluator, error) {
	for _, d := range module.Declarations {
		if d.Kind == "import" || d.Kind == "export" || d.Kind == "instance" {
			id := d.ID
			return nil, Diagnostic{Code: "QNT501", Message: "Runtime module requires flattened imports and instances", Reference: &id}
		}
	}
	linked, err := ResolveLinkedModules([]Module{module})
	if err != nil {
		return nil, err
	}
	if len(linked.Errors) > 0 {
		return nil, linked.Errors[0]
	}
	sorted, diagnostics := SortModuleDeclarations([]Module{module}, linked.Table)
	if len(diagnostics) > 0 {
		return nil, diagnostics[0]
	}
	if rng == nil {
		rng = NewRNG(Integer{})
	}
	evaluator := &ModuleEvaluator{Storage: NewVariableStorageWithMetadata(metadata), RNG: rng, module: sorted[0], definitions: insdict.NewDict[string, Declaration]()}
	evaluator.module.Declarations = append([]Declaration{}, sorted[0].Declarations...)
	for i, d := range evaluator.module.Declarations {
		if d.Expr != nil {
			body := copyRuntimeExpression(*d.Expr)
			d.Expr = &body
			evaluator.module.Declarations[i] = d
		}
		switch d.Kind {
		case "var":
			evaluator.Storage.Register(d.ID.String(), d.Name)
		case "def":
			evaluator.definitions.Set(d.Name, d)
		}
	}
	return evaluator, nil
}

// Evaluate requests a named value/action, or invokes a named operator when args
// is non-nil. Definition IR is owned by the evaluator; returned closures retain
// its state storage and can be invoked while this instance remains alive.
func (e *ModuleEvaluator) Evaluate(ctx context.Context, name string, args []RuntimeValue) (RuntimeValue, error) {
	declaration, ok := e.definitions.Get2(name)
	if !ok {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Definition not found: " + name}
	}
	body := Expr{ID: declaration.ID, Kind: "name", Name: name}
	for i := len(e.module.Declarations) - 1; i >= 0; i-- {
		d := e.module.Declarations[i]
		if d.Kind != "def" {
			continue
		}
		depth := 0
		definition := OperatorDef{Depth: &depth, ID: d.ID, Name: d.Name, Qualifier: d.Qualifier, Expr: d.Expr}
		nested := body
		body = Expr{ID: d.ID, Kind: "let", Opdef: &definition, Body: &nested}
	}
	value, err := evaluateExpression(ctx, body, nil, e.Storage, e.RNG, &e.Trace)
	if err != nil {
		return RuntimeValue{}, err
	}
	if args != nil {
		return value.Apply(ctx, args)
	}
	return value, nil
}

// Shift commits a pending state and appends it to this execution's trace.
func (e *ModuleEvaluator) Shift() error { return e.Trace.Shift(e.Storage) }
