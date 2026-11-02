package quint

import "context"

// EvaluatePureDefinition evaluates a named definition from a standalone or
// flattened module. Definitions are resolved and dependency-ordered before lazy
// lexical binding, so forward references work and cycles retain parser errors.
// A nil arguments slice requests the value/operator itself; a non-nil slice
// invokes it. Imports/instances must be flattened first. State storage and
// constants without supplied values are not implemented by this pure API.
func EvaluatePureDefinition(ctx context.Context, module Module, name string, arguments []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	found := false
	for _, d := range module.Declarations {
		switch d.Kind {
		case "import", "export", "instance":
			id := d.ID
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Pure evaluation requires flattened imports and instances", Reference: &id}
		case "def":
			if d.Name == name {
				found = true
			}
		}
	}
	if !found {
		id := module.ID
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Definition not found: " + name, Reference: &id}
	}
	linked, err := ResolveLinkedModules([]Module{module})
	if err != nil {
		return RuntimeValue{}, err
	}
	if len(linked.Errors) > 0 {
		return RuntimeValue{}, linked.Errors[0]
	}
	sorted, diagnostics := SortModuleDeclarations([]Module{module}, linked.Table)
	if len(diagnostics) > 0 {
		return RuntimeValue{}, diagnostics[0]
	}
	body := Expr{Kind: "name", Name: name}
	declarations := sorted[0].Declarations
	for i := len(declarations) - 1; i >= 0; i-- {
		d := declarations[i]
		if d.Kind != "def" {
			continue
		}
		depth := 0
		definition := OperatorDef{Depth: &depth, ID: d.ID, Name: d.Name, Qualifier: d.Qualifier, Expr: d.Expr}
		nested := body
		body = Expr{ID: d.ID, Kind: "let", Opdef: &definition, Body: &nested}
	}
	value, err := EvaluatePureExpression(ctx, body, nil)
	if err != nil {
		return RuntimeValue{}, err
	}
	if arguments != nil {
		return value.Apply(ctx, arguments)
	}
	return value, nil
}
