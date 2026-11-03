package quint

import (
	"context"
	"fmt"
	"math/big"

	"github.com/glycerine/insdict"
)

// EvaluatePureExpression evaluates concrete constructors, strict integer/Boolean
// operations, and lazy Boolean/conditional expressions. Bindings contain already
// evaluated values. Local definitions use compiled lambda objects, shared
// parameter cells, and scoped value caches. ModuleEvaluator adds persistent
// module definitions, state storage, RNG, and traces.
func EvaluatePureExpression(ctx context.Context, expression Expr, bindings *insdict.Dict[string, RuntimeValue]) (RuntimeValue, error) {
	return evaluateExpression(ctx, expression, bindings, nil, nil)
}

// EvaluatePureExpressionWithRuntimeState shares the reference's object-hash
// sequence with other evaluations and explicit runtime serialization. A nil
// state starts a fresh sequence. Returned closures retain this execution state.
func EvaluatePureExpressionWithRuntimeState(ctx context.Context, expression Expr, bindings *insdict.Dict[string, RuntimeValue], state *RuntimeObjectHashState) (RuntimeValue, error) {
	return newRuntimeEvaluation(bindings, nil, nil, nil, state).evaluateOwned(ctx, copyRuntimeExpression(expression))
}

// EvaluateStateExpression adds registered variable reads and assignments to the
// pure evaluator. Assignments update next registers; callers explicitly shift
// or recover storage when composing actions.
func EvaluateStateExpression(ctx context.Context, expression Expr, storage *VariableStorage) (RuntimeValue, error) {
	if storage == nil {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "State evaluation requires variable storage"}
	}
	return evaluateExpression(ctx, expression, nil, storage, nil)
}

// EvaluateStateExpressionWithRNG enables randomized action choice with caller-owned
// state and RNG. Both retain changes/counter advancement across invocations.
func EvaluateStateExpressionWithRNG(ctx context.Context, expression Expr, storage *VariableStorage, rng *RNG) (RuntimeValue, error) {
	if storage == nil {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "State evaluation requires variable storage"}
	}
	return evaluateExpression(ctx, expression, nil, storage, rng)
}

func evaluateExpression(ctx context.Context, expression Expr, bindings *insdict.Dict[string, RuntimeValue], storage *VariableStorage, rng *RNG, traces ...*RuntimeTrace) (RuntimeValue, error) {
	var trace *RuntimeTrace
	if len(traces) > 0 {
		trace = traces[0]
	}
	var objects *RuntimeObjectHashState
	if storage != nil {
		objects = storage.ObjectHashState()
	}
	return newRuntimeEvaluation(bindings, storage, rng, trace, objects).evaluateOwned(ctx, copyRuntimeExpression(expression))
}

type runtimeLocalValue func([]RuntimeValue) (RuntimeValue, error)
type runtimeEvaluation struct {
	evaluate                func(context.Context, Expr) (RuntimeValue, error)
	prepare                 func(context.Context, Expr) error
	prepareWithoutVariables func(context.Context, Expr) error
	objects                 *RuntimeObjectHashState
	constants               *insdict.Dict[string, ID]
	variables               *insdict.Dict[string, RuntimeVariableBinding]
	source                  *runtimeSourcePreparation
}

func (e *runtimeEvaluation) variableBinding(expression Expr) (RuntimeVariableBinding, bool) {
	if e.source != nil && expression.runtimeSource != nil {
		if binding, exists := e.source.Binding(*expression.runtimeSource); exists {
			return binding, true
		}
	}
	if e.variables != nil {
		return e.variables.Get2(expression.Name)
	}
	return RuntimeVariableBinding{}, false
}

func (e *runtimeEvaluation) evaluateOwned(ctx context.Context, expression Expr) (RuntimeValue, error) {
	if err := e.prepare(ctx, expression); err != nil {
		return RuntimeValue{}, err
	}
	return e.evaluate(ctx, expression)
}

type runtimeParameter struct{ value RuntimeValue }
type runtimeLocalCache struct {
	value      RuntimeValue
	err        error
	evaluated  bool
	generation uint64
}
type runtimeLocalDefinition struct {
	cache *runtimeLocalCache
	scope *insdict.Dict[string, runtimeLocalValue]
	value runtimeLocalValue
}
type runtimeLambdaKey struct {
	id       ID
	instance ID
	source   bool
	body     *Expr
}
type runtimeDefinitionKey struct {
	id         ID
	instance   ID
	source     bool
	definition *OperatorDef
}
type runtimeParameterKey struct {
	id     ID
	source bool
}

// The reference catches domain exceptions at the expression that invokes an
// operation. Checked value/serialization APIs retain those exceptions directly.
func runtimeExpressionError(expression Expr, err error) error {
	if exception, ok := err.(RuntimeDomainException); ok {
		err = Diagnostic{Code: "QNT501", Message: exception.Message}
	}
	if diagnostic, ok := err.(Diagnostic); ok {
		if diagnostic.Reference == nil {
			id := expression.ID
			diagnostic.Reference = &id
		}
		return diagnostic
	}
	return err
}

func newRuntimeEvaluation(bindings *insdict.Dict[string, RuntimeValue], storage *VariableStorage, rng *RNG, trace *RuntimeTrace, objects *RuntimeObjectHashState) *runtimeEvaluation {
	if objects == nil {
		objects = &RuntimeObjectHashState{}
	}
	session := &runtimeEvaluation{objects: objects}
	discovery := &RuntimeNondetDiscovery{trackDefinitions: true}
	session.prepare = func(ctx context.Context, expression Expr) error {
		return discovery.PrepareWithVariableBindings(ctx, expression, storage, session.variables)
	}
	session.prepareWithoutVariables = func(ctx context.Context, expression Expr) error {
		return discovery.Prepare(ctx, expression, storage)
	}
	ctx := context.Background()
	var listener ExecutionListener
	activeEvaluations := 0
	if bindings != nil {
		owned := insdict.NewDict[string, RuntimeValue]()
		for name, value := range bindings.All() {
			owned.Set(name, value)
		}
		bindings = owned
	}
	shift := func() error {
		if trace != nil {
			return trace.Shift(storage)
		}
		storage.Shift()
		storage.ClearMetadata()
		return nil
	}
	var eval func(Expr) (RuntimeValue, error)
	scopes := []*insdict.Dict[string, runtimeLocalValue]{}
	lambdas := map[runtimeLambdaKey]RuntimeValue{}
	literals := map[runtimeLambdaKey]RuntimeValue{}
	parameters := map[runtimeParameterKey]*runtimeParameter{}
	definitions := map[runtimeDefinitionKey]*runtimeLocalDefinition{}
	scopedValues := map[ID]*runtimeLocalCache{}
	constantValues := map[ID]runtimeLocalCache{}
	lookup := func(name string) (runtimeLocalValue, bool) {
		for i := len(scopes) - 1; i >= 0; i-- {
			if value, ok := scopes[i].Get2(name); ok {
				return value, true
			}
		}
		return nil, false
	}
	fail := func(e Expr, message string) (RuntimeValue, error) {
		id := e.ID
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message, Reference: &id}
	}
	boolValue := func(b bool) (RuntimeValue, error) { return NewRuntimeValue(Expr{Kind: "bool", Bool: b}) }
	var sourceValue func(runtimeSourceValueTarget) (RuntimeValue, error)
	applySourceOverrides := func(target runtimeSourceValueTarget) {
		for _, override := range target.Overrides {
			value, err := sourceValue(override.Value)
			constantValues[override.ConstantID] = runtimeLocalCache{value: value, err: err, evaluated: true}
		}
	}
	sourceValue = func(target runtimeSourceValueTarget) (RuntimeValue, error) {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		applySourceOverrides(target)
		switch target.Kind {
		case "const":
			if cell, exists := constantValues[target.Identity.ID]; exists {
				return cell.value, cell.err
			}
			return RuntimeValue{}, Diagnostic{Code: "QNT500", Message: "Uninitialized const " + target.Name + ". Use: import <moduleName>(" + target.Name + "=<value>).*"}
		case "param":
			if register := parameters[runtimeParameterKey{id: target.Identity.ID, source: true}]; register != nil && register.value.Valid() {
				return register.value, nil
			}
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Parameter " + target.Name + " not set"}
		case "def":
			key := runtimeDefinitionKey{id: target.Identity.ID, instance: target.Identity.InstanceID, source: true}
			definition, exists := definitions[key]
			if !exists {
				original, found := session.source.SourceDefinition(target.Identity)
				if !found {
					return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Runtime source definition not compiled: " + target.Name}
				}
				// Flattening can omit an unused override and its dependencies.
				// Install the owned source body through the ordinary evaluator.
				body := Expr{Kind: "bool", Bool: true}
				if _, err := eval(Expr{Kind: "let", Opdef: &original, Body: &body}); err != nil {
					return RuntimeValue{}, err
				}
				definition = definitions[key]
				if definition == nil {
					return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Runtime source definition not installed: " + target.Name}
				}
			}
			return definition.value(nil)
		default:
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Unsupported runtime source reference: " + target.Name}
		}
	}
	lookupExpression := func(expression Expr) (runtimeLocalValue, bool) {
		if target, exists := session.source.ValueReference(expression); exists {
			return func(arguments []RuntimeValue) (RuntimeValue, error) {
				var value RuntimeValue
				var err error
				if target.Kind == "var" {
					applySourceOverrides(target)
					binding, bound := session.variableBinding(expression)
					if storage != nil && bound {
						if register, registered := storage.current.Get2(binding.Key); registered {
							value, err = register.Read()
						} else {
							err = Diagnostic{Code: "QNT501", Message: "Runtime source variable not registered: " + target.Name}
						}
					} else {
						err = Diagnostic{Code: "QNT501", Message: "Runtime source variable unavailable: " + target.Name}
					}
				} else {
					value, err = sourceValue(target)
				}
				if err == nil && arguments != nil {
					return value.Apply(ctx, arguments)
				}
				return value, err
			}, true
		}
		if expression.Kind == "app" {
			return lookup(expression.Opcode)
		}
		return lookup(expression.Name)
	}
	eval = func(e Expr) (RuntimeValue, error) {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		switch e.Kind {
		case "int", "bool", "str":
			if e.ID != (ID{}) {
				key := runtimeLambdaKey{id: e.ID}
				if identity, source := session.source.ExpressionIdentity(e); source {
					key.id, key.instance, key.source = identity.ID, identity.InstanceID, true
				}
				if value, exists := literals[key]; exists {
					return value, nil
				}
				value, err := NewRuntimeValue(e)
				if err == nil {
					literals[key] = value
				}
				return value, err
			}
			return NewRuntimeValue(e)
		case "name":
			if value, ok := lookupExpression(e); ok {
				result, err := value(nil)
				return result, runtimeExpressionError(e, err)
			}
			if bindings != nil {
				if value, ok := bindings.Get2(e.Name); ok && value.Valid() {
					return value, nil
				}
			}
			if session.constants != nil {
				if _, exists := session.constants.Get2(e.Name); exists {
					return RuntimeValue{}, Diagnostic{Code: "QNT500", Message: "Uninitialized const " + e.Name + ". Use: import <moduleName>(" + e.Name + "=<value>).*", Reference: &e.ID}
				}
			}
			if storage != nil {
				var register *RuntimeRegister
				binding, bound := session.variableBinding(e)
				if bound {
					register, _ = storage.current.Get2(binding.Key)
				} else {
					for _, candidate := range storage.current.All() {
						if candidate.Name == e.Name {
							register = candidate
							break
						}
					}
				}
				if register != nil {
					value, err := register.Read()
					if d, ok := err.(Diagnostic); ok {
						id := e.ID
						d.Reference = &id
						err = d
					}
					return value, err
				}
			}
			if e.Name == "Bool" {
				return NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: []Expr{{Kind: "bool", Bool: false}, {Kind: "bool", Bool: true}}})
			}
			if e.Name == "Int" || e.Name == "Nat" {
				domain, err := NewRuntimeInfiniteSet(e.Name)
				if err != nil {
					return RuntimeValue{}, runtimeExpressionError(e, err)
				}
				return RuntimeValueFromDomain(domain)
			}
			if e.Name == "q::lastTrace" {
				args := []RuntimeValue{}
				if trace != nil {
					args = trace.States()
				}
				return newRuntimeComposite("List", args)
			}
			return fail(e, "Unbound runtime name: "+e.Name)
		case "lambda":
			if e.Body == nil {
				return fail(e, "Malformed local operator")
			}
			key := runtimeLambdaKey{id: e.ID}
			if identity, source := session.source.ExpressionIdentity(e); source {
				key.id, key.instance, key.source = identity.ID, identity.InstanceID, true
			}
			if e.ID == (ID{}) {
				key.body = e.Body
			}
			if value, ok := lambdas[key]; ok {
				return value, nil
			}
			captured := append([]*insdict.Dict[string, runtimeLocalValue](nil), scopes...)
			parameterScope := insdict.NewDict[string, runtimeLocalValue]()
			registers := make([]*runtimeParameter, len(e.Params))
			sourceParameters, hasSourceParameters := session.source.ParameterIDs(e)
			for i, param := range e.Params {
				parameterKey := runtimeParameterKey{id: param.ID}
				if hasSourceParameters && i < len(sourceParameters) {
					parameterKey.id, parameterKey.source = sourceParameters[i], true
				}
				register := parameters[parameterKey]
				// Syntax APIs also accept hand-built IR with omitted IDs. Keep
				// those independent rather than alias every unnamed parameter.
				if register == nil || param.ID == (ID{}) {
					register = &runtimeParameter{}
					if param.ID != (ID{}) {
						parameters[parameterKey] = register
					}
				}
				registers[i] = register
				parameterScope.Set(param.Name, func(extra []RuntimeValue) (RuntimeValue, error) {
					if !register.value.Valid() {
						return fail(e, "Parameter "+param.Name+" not set")
					}
					if extra != nil {
						return register.value.Apply(ctx, extra)
					}
					return register.value, nil
				})
			}
			value := newRuntimeClosure(len(e.Params), func(callContext context.Context, arguments []RuntimeValue) (RuntimeValue, error) {
				if len(arguments) != len(e.Params) {
					return RuntimeValue{}, Diagnostic{Code: "QNT506", Message: fmt.Sprintf("Lambda expects %d arguments, but got %d", len(e.Params), len(arguments))}
				}
				for i, register := range registers {
					register.value = arguments[i]
				}
				saved, savedContext, savedListener := scopes, ctx, listener
				if activeEvaluations == 0 {
					listener = ExecutionListenerFromContext(callContext)
				}
				activeEvaluations++
				scopes = append(append([]*insdict.Dict[string, runtimeLocalValue](nil), captured...), parameterScope)
				ctx = callContext
				result, err := eval(*e.Body)
				scopes, ctx, listener = saved, savedContext, savedListener
				activeEvaluations--
				return result, err
			})
			lambdas[key] = value
			return value, nil

		case "let":
			if e.Opdef == nil || e.Opdef.Expr == nil || e.Body == nil {
				return fail(e, "Malformed local definition")
			}
			key := runtimeDefinitionKey{id: e.Opdef.ID}
			alias := e.Opdef.Name
			if identity, source := session.source.DefinitionIdentity(*e.Opdef); source {
				if original, found := session.source.SourceDefinition(identity); found {
					key.id, key.instance, key.source = identity.ID, identity.InstanceID, true
					e.Opdef = &original
				} else if e.Opdef.Qualifier == Nondet {
					// A nondeterministic let installs its sampled-value getter
					// directly instead of compiling an ordinary definition body.
					key.id, key.instance, key.source = identity.ID, identity.InstanceID, true
				}
			}
			if e.Opdef.ID == (ID{}) {
				key.definition = e.Opdef
			}
			actionName := alias
			if !key.source && e.Opdef.Qualifier != Nondet && e.Opdef.ID != (ID{}) {
				compiled, exists := discovery.definitions[e.Opdef.ID]
				if !exists {
					// Entering a module's lexical wrappers does not compile its
					// unused definitions. The first selected reference freezes
					// the getter, including an ordinary import alias's name.
					return eval(*e.Body)
				}
				e.Opdef = compiled
				actionName = compiled.Name
			}
			if e.Opdef.Qualifier == Nondet {
				if e.Opdef.Expr.Kind != "app" || e.Opdef.Expr.Opcode != "oneOf" || len(e.Opdef.Expr.Args) != 1 {
					return fail(e, "Nondeterministic definitions require a oneOf application")
				}
				binding, exists := definitions[key]
				if !exists {
					binding = &runtimeLocalDefinition{scope: insdict.NewDict[string, runtimeLocalValue](), cache: &runtimeLocalCache{}}
					definitions[key] = binding
					binding.value = func(arguments []RuntimeValue) (RuntimeValue, error) {
						if binding.cache.err != nil {
							return RuntimeValue{}, binding.cache.err
						}
						if arguments != nil {
							return binding.cache.value.Apply(ctx, arguments)
						}
						return binding.cache.value, nil
					}
				}
				binding.scope.Set(e.Opdef.Name, binding.value)
				value, err := evalSourceNondet(ctx, e.Opdef.Name, storage, rng,
					func(callContext context.Context) (RuntimeValue, error) {
						saved := ctx
						ctx = callContext
						value, err := eval(e.Opdef.Expr.Args[0])
						ctx = saved
						return value, err
					},
					func(callContext context.Context, picked RuntimeValue, pickErr error) (RuntimeValue, error) {
						binding.cache.value, binding.cache.err = picked, pickErr
						saved := ctx
						ctx = callContext
						scopes = append(scopes, binding.scope)
						value, err := eval(*e.Body)
						scopes = scopes[:len(scopes)-1]
						ctx = saved
						return value, err
					})
				*binding.cache = runtimeLocalCache{}
				return value, runtimeExpressionError(e, err)
			}
			definition, exists := definitions[key]
			topLevel := e.Opdef.Depth != nil && *e.Opdef.Depth == 0
			if !exists {
				definition = &runtimeLocalDefinition{scope: insdict.NewDict[string, runtimeLocalValue](), cache: &runtimeLocalCache{}}
				if key.source && !topLevel && e.Opdef.Expr.Kind != "lambda" && e.Opdef.Qualifier != Action {
					if cell := scopedValues[key.id]; cell != nil {
						definition.cache = cell
					} else {
						scopedValues[key.id] = definition.cache
					}
				}
				definitions[key] = definition
				captured := append([]*insdict.Dict[string, runtimeLocalValue](nil), scopes...)
				var actionApp Expr
				if e.Opdef.Qualifier == Action && e.Opdef.Expr.Kind != "lambda" {
					actionApp = session.source.ActionApp(*e.Opdef)
				}
				definition.value = func(arguments []RuntimeValue) (RuntimeValue, error) {
					var actionListener ExecutionListener
					if listener != nil && e.Opdef.Qualifier == Action && e.Opdef.Expr.Kind != "lambda" {
						actionListener = listener
						actionListener.OnUserOperatorCall(actionApp)
					}
					if storage != nil && e.Opdef.Qualifier == Action && storage.ActionTaken == "" {
						storage.ActionTaken = actionName
					}
					cache := definition.cache
					if storage != nil && topLevel && e.Opdef.Qualifier == Val && cache.generation != storage.generation {
						cache.evaluated = false
					}
					cacheValue := e.Opdef.Expr.Kind == "lambda" || (e.Opdef.Qualifier != Action && (!topLevel || e.Opdef.Qualifier == Val || e.Opdef.Qualifier == PureVal))
					if !cache.evaluated || !cacheValue {
						saved := scopes
						scopes = captured
						cache.value, cache.err = eval(*e.Opdef.Expr)
						scopes = saved
						cache.evaluated = true
						if storage != nil {
							cache.generation = storage.generation
						}
					}
					if actionListener != nil {
						actionListener.OnUserOperatorReturn(actionApp, nil, cache.value, cache.err)
					}
					if cache.err != nil {
						return RuntimeValue{}, cache.err
					}
					if arguments != nil {
						return cache.value.Apply(ctx, arguments)
					}
					return cache.value, nil
				}
			}
			// Ordinary import aliases share the compiled definition ID and
			// cache, but each flattened name must resolve to that same getter.
			definition.scope.Set(alias, definition.value)
			// Scoped value cells belong to the compiled definition. Each let
			// invocation temporarily clears its cache, then restores the saved
			// value even when a returned lambda still refers to that cell.
			savedCache := *definition.cache
			if !topLevel && e.Opdef.Expr.Kind != "lambda" {
				*definition.cache = runtimeLocalCache{}
			}
			scopes = append(scopes, definition.scope)
			result, err := eval(*e.Body)
			scopes = scopes[:len(scopes)-1]
			if !topLevel && e.Opdef.Expr.Kind != "lambda" {
				*definition.cache = savedCache
			}
			return result, err
		case "app":
			if value, ok := lookupExpression(e); ok {
				if listener != nil {
					listener.OnUserOperatorCall(e)
				}
				arguments := make([]RuntimeValue, len(e.Args))
				for i, child := range e.Args {
					argument, err := eval(child)
					if err != nil {
						return RuntimeValue{}, err
					}
					arguments[i] = argument
				}
				operator, err := value(nil)
				var result RuntimeValue
				if err == nil {
					if operator.closure == nil {
						// The reference throws while converting a non-function to
						// an arrow, before issuing the application's return event.
						return RuntimeValue{}, runtimeExpressionError(e, Diagnostic{Code: "QNT501", Message: "Expected a lambda value"})
					}
					result, err = operator.Apply(ctx, arguments)
				}
				if listener != nil {
					listener.OnUserOperatorReturn(e, arguments, result, err)
				}
				if diagnostic, ok := err.(Diagnostic); ok && diagnostic.Reference == nil {
					diagnostic.Reference = &e.ID
					err = diagnostic
				}
				return result, err
			}
		default:
			return fail(e, "Unsupported pure expression kind: "+e.Kind)
		}
		boolean := func(child Expr) (bool, error) {
			v, err := eval(child)
			if err != nil {
				return false, err
			}
			if v.expression.Kind != "bool" {
				_, err = fail(child, "Expected a Boolean value")
				return false, err
			}
			return v.expression.Bool, nil
		}
		switch e.Opcode {
		case "next", "oneOf":
			id := e.ID
			return RuntimeValue{}, Diagnostic{Code: "QNT000", Message: "Unknown stateful op", Reference: &id}
		case "then":
			if storage == nil {
				return fail(e, "Action sequencing requires variable storage")
			}
			if len(e.Args) != 2 {
				return fail(e, "then requires 2 arguments")
			}
			var oldState RuntimeValue
			if listener != nil {
				var err error
				oldState, err = storage.Record()
				if err != nil {
					return RuntimeValue{}, err
				}
			}
			enabled, err := boolean(e.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			if !enabled {
				id := e.Args[0].ID
				if e.Args[0].Kind == "app" && e.Args[0].Opcode == "then" && len(e.Args[0].Args) == 2 {
					id = e.Args[0].Args[1].ID
				}
				return RuntimeValue{}, Diagnostic{Code: "QNT513", Message: "Cannot continue in `then` because the highlighted expression evaluated to false", Reference: &id}
			}
			if err := shift(); err != nil {
				return RuntimeValue{}, err
			}
			if listener != nil {
				newState, err := storage.Record()
				if err != nil {
					return RuntimeValue{}, err
				}
				listener.OnNextState(oldState, newState)
			}
			return eval(e.Args[1])
		case "reps":
			if storage == nil {
				return fail(e, "Action repetition requires variable storage")
			}
			if len(e.Args) != 2 {
				return fail(e, "reps requires 2 arguments")
			}
			count, err := eval(e.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			if count.expression.Kind != "int" {
				return fail(e, "reps requires an integer count")
			}
			limit, _ := count.expression.Int.Big().Float64()
			result, _ := boolValue(true)
			for i := float64(0); i < limit; i++ {
				if err := ctx.Err(); err != nil {
					return RuntimeValue{}, err
				}
				callback, err := eval(e.Args[1])
				if err != nil {
					return RuntimeValue{}, err
				}
				integer, _ := new(big.Float).SetFloat64(i).Int(nil)
				n := IntegerFromBig(integer)
				iteration, err := NewRuntimeValue(Expr{Kind: "int", Int: n})
				if err != nil {
					return RuntimeValue{}, err
				}
				result, err = callback.Apply(ctx, []RuntimeValue{iteration})
				if err != nil {
					return RuntimeValue{}, err
				}
				if result.expression.Kind != "bool" {
					return fail(e, "Expected a Boolean action result")
				}
				if !result.expression.Bool {
					id := e.ID
					return RuntimeValue{}, Diagnostic{Code: "QNT513", Message: fmt.Sprintf("Reps loop could not continue after iteration #%s evaluated to false", runtimeNumberString(i+1)), Reference: &id}
				}
				if i < limit-1 {
					if err := shift(); err != nil {
						return RuntimeValue{}, err
					}
				}
			}
			return result, nil
		case "actionAny":
			if storage == nil || rng == nil {
				return fail(e, "Action choice requires variable storage and RNG")
			}
			snapshot := storage.Snapshot()
			indices := make([]int, len(e.Args))
			for i := range indices {
				indices[i] = i
			}
			for i := len(indices) - 1; i > 0; i-- {
				bound, _ := ParseInteger(fmt.Sprint(i + 1))
				draw, err := rng.Next(bound)
				if err != nil {
					return RuntimeValue{}, err
				}
				j := int(draw.Big().Int64())
				indices[i], indices[j] = indices[j], indices[i]
			}
			var anyApp Expr
			if listener != nil {
				anyApp = Expr{Kind: "app", Opcode: "actionAny", Args: []Expr{}}
			}
			for _, index := range indices {
				storage.ActionTaken = ""
				for name := range storage.NondetPicks.All() {
					storage.NondetPicks.Set(name, RuntimeValue{})
				}
				if listener != nil {
					listener.OnAnyOptionCall(anyApp, index)
				}
				enabled, err := boolean(e.Args[index])
				if listener != nil {
					listener.OnAnyOptionReturn(anyApp, index)
				}
				if err != nil {
					return RuntimeValue{}, err
				}
				if enabled {
					if listener != nil {
						listener.OnAnyReturn(len(e.Args), index)
					}
					storage.recoverSourceSnapshot(storage.Snapshot())
					return boolValue(true)
				}
				storage.recoverSourceSnapshot(snapshot)
			}
			if listener != nil {
				listener.OnAnyReturn(len(e.Args), -1)
			}
			storage.recoverSourceSnapshot(snapshot)
			return boolValue(false)
		case "actionAll":
			if storage == nil {
				return fail(e, "Action conjunction requires variable storage")
			}
			snapshot := storage.Snapshot()
			for _, action := range e.Args {
				enabled, err := boolean(action)
				if err != nil {
					return RuntimeValue{}, err
				}
				if !enabled {
					storage.recoverSourceSnapshot(snapshot)
					return boolValue(false)
				}
			}
			return boolValue(true)
		case "expect":
			if storage == nil {
				return fail(e, "Expect requires variable storage")
			}
			if len(e.Args) != 2 {
				return fail(e, "Expect requires an action and condition")
			}
			enabled, err := boolean(e.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			if !enabled {
				return RuntimeValue{}, Diagnostic{Code: "QNT508", Message: "Cannot continue to \"expect\"", Reference: &e.ID}
			}
			snapshot := storage.Snapshot()
			if err := shift(); err != nil {
				return RuntimeValue{}, err
			}
			condition, err := boolean(e.Args[1])
			if err != nil {
				return RuntimeValue{}, err
			}
			storage.recoverSourceSnapshot(snapshot)
			if trace != nil {
				trace.DropLast()
			}
			if !condition {
				return RuntimeValue{}, Diagnostic{Code: "QNT508", Message: "Expect condition does not hold true", Reference: &e.ID}
			}
			return boolValue(true)
		case "assign":
			if storage == nil {
				return fail(e, "Assignment requires variable storage")
			}
			if len(e.Args) != 2 || e.Args[0].Kind != "name" {
				return fail(e, "Assignment requires a variable and value")
			}
			sourceTarget, hasSourceTarget := session.source.ValueReference(e.Args[0])
			if !hasSourceTarget || sourceTarget.Kind != "var" {
				if _, local := lookup(e.Args[0].Name); local {
					return fail(e, "Assignment target is a local binding")
				}
			}
			var target *RuntimeRegister
			binding, bound := session.variableBinding(e.Args[0])
			if bound {
				target, _ = storage.next.Get2(binding.Key)
			} else {
				for key, register := range storage.current.All() {
					if register.Name == e.Args[0].Name {
						target = storage.next.Get(key)
						break
					}
				}
			}
			if target != nil {
				value, err := eval(e.Args[1])
				if err != nil {
					return RuntimeValue{}, err
				}
				if hasSourceTarget {
					applySourceOverrides(sourceTarget)
				}
				target.Value = value
				return boolValue(true)
			}
			return fail(e, "Unknown assignment variable: "+e.Args[0].Name)
		case "matchVariant":
			if len(e.Args) < 1 || len(e.Args)%2 != 1 {
				return fail(e, "matchVariant requires a value and label/branch pairs")
			}
			matched, err := eval(e.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			if matched.expression.Kind != "app" || matched.expression.Opcode != "variant" {
				return fail(e, "Expected a variant value")
			}
			label := matched.expression.Args[0].Str
			for i := 1; i < len(e.Args); i += 2 {
				candidate, err := eval(e.Args[i])
				if err != nil {
					return RuntimeValue{}, err
				}
				if candidate.expression.Kind != "str" {
					return fail(e.Args[i], "Expected a string variant label")
				}
				if candidate.expression.Str != "_" && candidate.expression.Str != label {
					continue
				}
				branch, err := eval(e.Args[i+1])
				if err != nil {
					return RuntimeValue{}, err
				}
				children, err := runtimeChildren(matched)
				if err != nil {
					return RuntimeValue{}, err
				}
				payload := children[1]
				result, err := branch.Apply(ctx, []RuntimeValue{payload})
				if d, ok := err.(Diagnostic); ok && d.Reference == nil {
					id := e.ID
					d.Reference = &id
					err = d
				}
				return result, err
			}
			id := e.ID
			return RuntimeValue{}, Diagnostic{Code: "QNT505", Message: "No match for variant " + label, Reference: &id}
		case "and", "or":
			for _, child := range e.Args {
				b, err := boolean(child)
				if err != nil {
					return RuntimeValue{}, err
				}
				if b == (e.Opcode == "or") {
					return boolValue(b)
				}
			}
			return boolValue(e.Opcode == "and")
		case "ite", "implies":
			arity := 3
			if e.Opcode == "implies" {
				arity = 2
			}
			if len(e.Args) != arity {
				return fail(e, fmt.Sprintf("%s requires %d arguments", e.Opcode, arity))
			}
			b, err := boolean(e.Args[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			if e.Opcode == "implies" {
				if !b {
					return boolValue(true)
				}
				value, err := boolean(e.Args[1])
				if err != nil {
					return RuntimeValue{}, err
				}
				return boolValue(value)
			}
			if b {
				return eval(e.Args[1])
			}
			return eval(e.Args[2])
		}
		args := make([]RuntimeValue, len(e.Args))
		for i, child := range e.Args {
			value, err := eval(child)
			if err != nil {
				return RuntimeValue{}, err
			}
			args[i] = value
		}
		switch e.Opcode {
		case "List", "Tup", "Set", "Map", "Rec", "variant":
			value, err := NewRuntimeCompositeContext(ctx, e.Opcode, args)
			return value, runtimeExpressionError(e, err)
		case "not", "fail", "iff", "eq", "neq", "assert":
			arity := 2
			if e.Opcode == "not" || e.Opcode == "fail" || e.Opcode == "assert" {
				arity = 1
			}
			if len(args) != arity {
				return fail(e, fmt.Sprintf("%s requires %d arguments", e.Opcode, arity))
			}
			if e.Opcode == "eq" || e.Opcode == "neq" {
				equal, err := runtimeSourceEqualContext(ctx, args[0], args[1])
				if err != nil {
					return RuntimeValue{}, runtimeExpressionError(e, err)
				}
				return boolValue(equal == (e.Opcode == "eq"))
			}
			for _, arg := range args {
				if arg.expression.Kind != "bool" {
					return fail(e, "Expected a Boolean value")
				}
			}
			switch e.Opcode {
			case "not", "fail":
				return boolValue(!args[0].expression.Bool)
			case "iff":
				return boolValue(args[0].expression.Bool == args[1].expression.Bool)
			case "assert":
				if !args[0].expression.Bool {
					id := e.ID
					return RuntimeValue{}, Diagnostic{Code: "QNT508", Message: "Assertion failed", Reference: &id}
				}
				return args[0], nil
			}
		}
		var value RuntimeValue
		var err error
		if SymbolicBuiltinArity(e.Opcode) != 0 {
			value, err = EvalSymbolicBuiltin(ctx, e.Opcode, args)
		} else if e.Opcode == "q::debug" {
			value, err = EvalDebugBuiltin(ctx, args, session.objects)
		} else if e.Opcode == "allListsUpTo" {
			value, err = EvalAllListsBuiltinContext(ctx, e.Opcode, args, session.objects)
		} else if e.Opcode == "setToMap" {
			value, err = EvalSetToMapBuiltinContext(ctx, args, session.objects)
		} else if e.Opcode == "setBy" {
			value, err = EvalMapSetByContext(ctx, args)
		} else if listHigherBuiltinArity(e.Opcode) != 0 {
			value, err = EvalListHigherBuiltinContext(ctx, e.Opcode, args)
		} else if setHigherBuiltinArity(e.Opcode) != 0 {
			value, err = EvalSetHigherBuiltinContext(ctx, e.Opcode, args)
		} else if e.Opcode == "range" {
			value, err = EvalRangeBuiltin(ctx, args)
		} else if sequenceBuiltinArity(e.Opcode) != 0 {
			value, err = EvalSequenceBuiltinContext(ctx, e.Opcode, args)
		} else if mapBuiltinArity(e.Opcode) != 0 {
			value, err = EvalMapBuiltinContext(ctx, e.Opcode, args)
		} else if setConversionBuiltinArity(e.Opcode) != 0 {
			value, err = EvalSetConversionBuiltinContext(ctx, e.Opcode, args)
		} else if setBuiltinArity(e.Opcode) != 0 {
			value, err = EvalSetBuiltinContext(ctx, e.Opcode, args)
		} else {
			if e.Opcode == "apalache::generate" || RuntimeCombinatorArity(e.Opcode) != 0 || !IsBuiltinOpcode(e.Opcode) {
				err = Diagnostic{Code: "QNT000", Message: "Unknown builtin " + e.Opcode}
			} else {
				value, err = EvalIntegerBuiltin(e.Opcode, args)
			}
		}
		return value, runtimeExpressionError(e, err)
	}
	session.evaluate = func(callContext context.Context, expression Expr) (RuntimeValue, error) {
		savedContext, savedListener := ctx, listener
		ctx = callContext
		listener = ExecutionListenerFromContext(callContext)
		activeEvaluations++
		value, err := eval(expression)
		ctx, listener = savedContext, savedListener
		activeEvaluations--
		return value, err
	}
	return session
}
