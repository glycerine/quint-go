package quint

import "github.com/glycerine/insdict"

// TypeInferrer retains inference state across dependency-ordered batches.
// Name resolution and type-application expansion must precede inference.
type TypeInferrer struct{ *TypeConstraintGenerator }
type TypeInferenceResult struct {
	Types  *insdict.Dict[string, TypeScheme]
	Errors *insdict.Dict[string, ErrorTree]
}

// NewTypeInferrer optionally reuses an existing type table, as upstream does.
// Saved schemes are available for references but are not added to active scope.
func NewTypeInferrer(table *insdict.Dict[ID, LookupDefinition], types *insdict.Dict[string, TypeScheme]) *TypeInferrer {
	g := NewTypeConstraintGenerator(TypeUnifier{Table: table})
	if types != nil {
		g.Context.Types = types
	}
	return &TypeInferrer{g}
}

// InferTypes traverses declarations and simplifies newly created type entries.
// Like upstream, it does not force a final solve of pending assumption constraints.
// Result dictionaries are live shared state. Structural failures return partial
// results and a Go error; type failures appear in Errors.
func (i *TypeInferrer) InferTypes(declarations []Declaration) (TypeInferenceResult, error) {
	result := TypeInferenceResult{Types: i.Context.Types, Errors: i.Context.Errors}
	before := map[string]bool{}
	for id := range i.Context.Types.All() {
		before[id] = true
	}
	for _, d := range declarations {
		if err := i.GenerateDeclaration(d); err != nil {
			return result, err
		}
	}
	for id, s := range i.Context.Types.All() {
		if !before[id] {
			s.Type = SimplifyType(s.Type)
			i.Context.Types.Set(id, s)
		}
	}
	return result, nil
}
