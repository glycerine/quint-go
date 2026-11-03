package quint

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func runtimePreparedSourceValues(t *testing.T, source, selected string) (*runtimeSourcePreparation, []Module) {
	t.Helper()
	modules, err := ParseModules(source)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := newRuntimeSourceGraph(modules)
	if err != nil {
		t.Fatal(err)
	}
	preparation := newRuntimeSourcePreparation(&runtimeSourceModule{graph: graph, name: "M"})
	if handled, err := preparation.PrepareDefinition(context.Background(), selected, NewVariableStorage()); !handled || err != nil {
		t.Fatal(handled, err)
	}
	return preparation, modules
}

func TestRuntimeSourceValueTargetsAndGlobalParameterIDs(t *testing.T) {
	t.Parallel()
	preparation, modules := runtimePreparedSourceValues(t, "module A {const N:int def f(x)=N+x} module M {import A(N=1) as I import A(N=2) as J def result=I::f(3)}", "result")
	root, found := preparation.RootIdentity("result")
	if !found || root.ID != modules[1].Declarations[2].ID || root.InstanceID != (ID{}) {
		t.Fatal(root, found)
	}
	definition, found := preparation.SourceDefinition(root)
	if !found || definition.Expr == nil {
		t.Fatal(definition, found)
	}
	target, found := preparation.ValueReference(*definition.Expr)
	instance := modules[1].Declarations[0]
	sourceDefinition := modules[0].Declarations[1]
	if !found || target.Kind != "def" || target.Name != "f" || target.Identity.ID != sourceDefinition.ID || target.Identity.InstanceID != instance.ID {
		t.Fatal(target, found)
	}
	if len(target.Overrides) != 1 {
		t.Fatal(target.Overrides)
	}
	override := target.Overrides[0]
	if override.ConstantID != modules[0].Declarations[0].ID || override.Value.Identity.ID != instance.Overrides[0].Parameter.ID || override.Value.Identity.InstanceID != instance.ID || override.Name != "N" {
		t.Fatal(override)
	}
	value, found := preparation.SourceDefinition(override.Value.Identity)
	if !found || value.Qualifier != PureVal || value.Expr.ID != instance.Overrides[0].Expr.ID || value.Depth == nil || *value.Depth != 0 {
		t.Fatal(value, found)
	}
	operator, found := preparation.SourceDefinition(target.Identity)
	if !found || operator.Expr == nil {
		t.Fatal(operator, found)
	}
	replacement := Declaration{Kind: "def", ID: target.Identity.ID, Name: "changed", Qualifier: Def, Expr: operator.Expr}
	preparation.captureSourceDefinition(LookupDefinition{ID: target.Identity.ID, Declaration: &replacement}, runtimeSourceCompileContext{instance: target.Identity.InstanceID})
	if retained, _ := preparation.SourceDefinition(target.Identity); retained.Name != "f" {
		t.Fatal("memoized definition was replaced", retained.Name)
	}
	if identity, found := preparation.DefinitionIdentity(operator); !found || identity != target.Identity {
		t.Fatal(identity, found)
	}
	parameters, found := preparation.ParameterIDs(*operator.Expr)
	if !found || len(parameters) != 1 || parameters[0] != sourceDefinition.Expr.Params[0].ID {
		t.Fatal(parameters, found)
	}
	if identity, found := preparation.ExpressionIdentity(*operator.Expr); !found || identity.ID != sourceDefinition.Expr.ID || identity.InstanceID != instance.ID {
		t.Fatal(identity, found)
	}
	constant, found := preparation.ValueReference(operator.Expr.Body.Args[0])
	if !found || constant.Kind != "const" || constant.Identity.ID != override.ConstantID || constant.Identity.InstanceID != instance.ID {
		t.Fatal(constant, found)
	}
	parameter, found := preparation.ValueReference(operator.Expr.Body.Args[1])
	if !found || parameter.Kind != "param" || parameter.Identity.ID != parameters[0] || parameter.Name != "x" {
		t.Fatal(parameter, found)
	}
	// A new source graph changes future compilation, while old source targets
	// and definition bodies remain tied to their original IDs and memo context.
	preparation.UpdateSource(nil)
	if _, found := preparation.RootIdentity("result"); found {
		t.Fatal("old root name survived source replacement")
	}
	if retained, found := preparation.ValueReference(*definition.Expr); !found || retained.Identity != target.Identity {
		t.Fatal(retained, found)
	}
	if retained, found := preparation.SourceDefinition(target.Identity); !found || retained.Expr.ID != operator.Expr.ID {
		t.Fatal(retained, found)
	}
	if retained, found := preparation.ParameterIDs(*operator.Expr); !found || retained[0] != parameters[0] {
		t.Fatal(retained, found)
	}
}

func TestRuntimeSourceValueAssignmentWrapperAndRoot(t *testing.T) {
	t.Parallel()
	preparation, modules := runtimePreparedSourceValues(t, "module A {const N:int var x:int action init=x'=N} module M {import A(N=2) as I action result=I::x'=3}", "result")
	root, _ := preparation.RootIdentity("result")
	definition, _ := preparation.SourceDefinition(root)
	target, found := preparation.ValueReference(definition.Expr.Args[0])
	if !found || target.Kind != "var" || target.Identity.ID != modules[0].Declarations[1].ID || len(target.Overrides) != 1 {
		t.Fatal(target, found)
	}
	storage := NewVariableStorage()
	if handled, err := preparation.PrepareDefinition(context.Background(), "M::I::init", storage); !handled || err != nil {
		t.Fatal(handled, err)
	}
	identity, found := preparation.RootIdentity("M::I::init")
	if !found || identity.ID != modules[0].Declarations[2].ID || identity.InstanceID != modules[1].Declarations[0].ID {
		t.Fatal(identity, found)
	}
	reference, found := preparation.ValueReference(Expr{Kind: "name", runtimeSource: &identity})
	if !found || reference.Identity != identity || len(reference.Overrides) != 1 {
		t.Fatal(reference, found)
	}
}

func TestRuntimeSourceValueFallbackAndOwnership(t *testing.T) {
	t.Parallel()
	preparation, modules := runtimePreparedSourceValues(t, "module M {def f(x)=x def result=f(1)}", "result")
	root, _ := preparation.RootIdentity("result")
	definition, _ := preparation.SourceDefinition(root)
	target, _ := preparation.ValueReference(*definition.Expr)
	modules[0].Declarations[0].Name = "mutated"
	modules[0].Declarations[0].Expr.Params[0].Name = "mutated"
	operator, _ := preparation.SourceDefinition(target.Identity)
	if operator.Name != "f" || operator.Expr.Params[0].Name != "x" {
		t.Fatal(operator)
	}
	var transported OperatorDef
	encoded, err := json.Marshal(operator)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &transported); err != nil {
		t.Fatal(err)
	}
	if _, found := preparation.DefinitionIdentity(transported); found {
		t.Fatal("transport retained private source identity")
	}
	unknown, err := ParseID("999999")
	if err != nil {
		t.Fatal(err)
	}
	expression := Expr{Kind: "name", runtimeSource: &runtimeSourceIdentity{ID: unknown}}
	for _, source := range []*runtimeSourcePreparation{nil, preparation} {
		if _, found := source.ValueReference(Expr{}); found {
			t.Fatal("unannotated reference")
		}
		if _, found := source.ValueReference(expression); found {
			t.Fatal("unknown reference")
		}
		if _, found := source.DefinitionIdentity(OperatorDef{}); found {
			t.Fatal("unannotated definition")
		}
		if _, found := source.DefinitionIdentity(OperatorDef{Expr: &expression}); found {
			t.Fatal("unknown owner")
		}
		if _, found := source.ExpressionIdentity(Expr{}); found {
			t.Fatal("unannotated expression")
		}
		if _, found := source.ParameterIDs(Expr{}); found {
			t.Fatal("unannotated lambda")
		}
		if _, found := source.ParameterIDs(expression); found {
			t.Fatal("unknown lambda")
		}
		if _, found := source.RootIdentity("missing"); found {
			t.Fatal("unknown root")
		}
		if _, found := source.SourceDefinition(runtimeSourceIdentity{ID: unknown}); found {
			t.Fatal("unknown definition")
		}
	}
}

func TestRuntimeSourceValueOverrideErrorsAndCancellation(t *testing.T) {
	t.Parallel()
	preparation, modules := runtimePreparedSourceValues(t, "module A {const N:int def f(x)=N+x} module M {import A(N=1) as I def result=I::f(3)}", "result")
	instance := modules[1].Declarations[0]
	preparation.source.graph.table.Del(instance.Overrides[0].Parameter.ID)
	_, err := preparation.sourceValueTarget(LookupDefinition{ID: modules[0].Declarations[1].ID, ImportedFrom: &instance}, runtimeSourceCompileContext{})
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code != "QNT501" || diagnostic.Reference == nil || *diagnostic.Reference != instance.Overrides[0].Parameter.ID {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if handled, err := preparation.PrepareDefinition(canceled, "result", NewVariableStorage()); handled || !errors.Is(err, context.Canceled) {
		t.Fatal(handled, err)
	}
}
