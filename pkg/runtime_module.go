package quint

import (
	"context"
	"github.com/glycerine/insdict"
)

// ModuleEvaluator retains registers and RNG for a standalone/flattened module.
// Callers use Shift to commit and record states, or recover Storage between actions.
// Each instance is owned by one execution and must be used sequentially.
type ModuleEvaluator struct {
	Storage            *VariableStorage
	RNG                *RNG
	Trace              RuntimeTrace
	module             Module
	definitions        *insdict.Dict[string, Declaration]
	execution          *runtimeEvaluation
	program            *Expr
	request            *Expr
	prepared           map[string]bool
	programDefinitions *insdict.Dict[string, Declaration]
	expressions        map[string]Expr
}

func NewModuleEvaluator(module Module, rng *RNG, metadata bool) (*ModuleEvaluator, error) {
	return NewModuleEvaluatorWithRuntimeState(module, rng, metadata, nil)
}

// NewModuleEvaluatorWithRuntimeState shares caller-owned object-hash history
// across module executions. Other runtime registers and caches remain private.
func NewModuleEvaluatorWithRuntimeState(module Module, rng *RNG, metadata bool, objects *RuntimeObjectHashState) (*ModuleEvaluator, error) {
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
	evaluator := &ModuleEvaluator{Storage: NewVariableStorageWithMetadata(metadata), RNG: rng, module: sorted[0], definitions: insdict.NewDict[string, Declaration](), prepared: map[string]bool{}}
	evaluator.Storage.objectHashes = objects
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
	evaluator.execution = newRuntimeEvaluation(nil, evaluator.Storage, evaluator.RNG, &evaluator.Trace, evaluator.Storage.ObjectHashState())
	evaluator.rebuildProgram()
	return evaluator, nil
}

func (e *ModuleEvaluator) rebuildProgram() {
	e.request = &Expr{Kind: "name"}
	e.program = e.request
	e.prepared = map[string]bool{}
	e.expressions = map[string]Expr{}
	e.execution.constants = insdict.NewDict[string, ID]()
	for i := len(e.module.Declarations) - 1; i >= 0; i-- {
		d := e.module.Declarations[i]
		if d.Kind == "const" {
			e.execution.constants.Set(d.Name, d.ID)
		}
		if d.Kind != "def" {
			continue
		}
		depth := 0
		definition := &OperatorDef{Depth: &depth, ID: d.ID, Name: d.Name, Qualifier: d.Qualifier, Expr: d.Expr}
		e.program = &Expr{ID: d.ID, Kind: "let", Opdef: definition, Body: e.program}
	}
	e.programDefinitions = e.definitions
}

// ObjectHashState exposes the execution's shared compatibility history so
// callers can serialize values in the same sequence as runtime operations.
func (e *ModuleEvaluator) ObjectHashState() *RuntimeObjectHashState {
	return e.Storage.ObjectHashState()
}

// Evaluate requests a named value/action, or invokes a named operator when args
// is non-nil. Definition IR is owned by the evaluator; returned closures retain
// its state storage and can be invoked while this instance remains alive.
func (e *ModuleEvaluator) Evaluate(ctx context.Context, name string, args []RuntimeValue) (RuntimeValue, error) {
	if e.programDefinitions != e.definitions {
		e.rebuildProgram()
	}
	declaration, ok := e.definitions.Get2(name)
	if !ok {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Definition not found: " + name}
	}
	*e.request = Expr{ID: declaration.ID, Kind: "name", Name: name}
	if !e.prepared[name] {
		if err := e.execution.prepare(ctx, *e.program); err != nil {
			return RuntimeValue{}, err
		}
		e.prepared[name] = true
	}
	value, err := e.execution.evaluate(ctx, *e.program)
	if err != nil {
		return RuntimeValue{}, err
	}
	if args != nil {
		return value.Apply(ctx, args)
	}
	return value, nil
}

// evaluateExpression executes an owned request under this module's persistent
// lexical definitions. It is also used by the top-level simulation adapter;
// recursive expression evaluation does not interpret simulation combinators.
func (e *ModuleEvaluator) evaluateExpression(ctx context.Context, expression Expr) (RuntimeValue, error) {
	if err := e.prepareExpression(ctx, expression); err != nil {
		return RuntimeValue{}, err
	}
	return e.execution.evaluate(ctx, *e.program)
}

// prepareExpression compiles a request's lexical dependencies and metadata
// before any simulation sample runs, without evaluating its runtime effects.
func (e *ModuleEvaluator) prepareExpression(ctx context.Context, expression Expr) error {
	if e.programDefinitions != e.definitions {
		e.rebuildProgram()
	}
	key := "expr:" + expression.ID.String()
	if expression.ID == (ID{}) {
		// Hand-built expressions can omit IDs; keep distinct requests separate.
		key += ":" + ExpressionString(expression)
	}
	owned, exists := e.expressions[key]
	if !exists {
		owned = copyRuntimeExpression(expression)
		e.expressions[key] = owned
	}
	*e.request = owned
	if !e.prepared[key] {
		if err := e.execution.prepare(ctx, *e.program); err != nil {
			return err
		}
		e.prepared[key] = true
	}
	return nil
}

// Shift commits a pending state and appends it to this execution's trace.
func (e *ModuleEvaluator) Shift() error { return e.Trace.Shift(e.Storage) }
