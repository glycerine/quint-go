package quint

import (
	"github.com/glycerine/insdict"
	"hash/fnv"
)

func EntityStateVariables(e Entity) []StateVariable {
	result := []StateVariable{}
	switch e.Kind {
	case "concrete":
		result = append(result, e.StateVariables...)
	case "union":
		for _, child := range e.Entities {
			result = append(result, EntityStateVariables(child)...)
		}
	}
	return result
}

// MultipleUpdatesChecker accumulates diagnostics across calls, like upstream.
// Both variable grouping and diagnostic iteration preserve insertion order.
type MultipleUpdatesChecker struct{ Errors *insdict.Dict[ID, Diagnostic] }

func (c *MultipleUpdatesChecker) Check(effects []EffectScheme) *insdict.Dict[ID, Diagnostic] {
	if c.Errors == nil {
		c.Errors = insdict.NewDictFunc[ID, Diagnostic](func(id ID) uint64 { h := fnv.New64a(); _, _ = h.Write([]byte(id.String())); return h.Sum64() })
	}
	var visit func(Effect)
	visit = func(e Effect) {
		switch e.Kind {
		case "arrow":
			for _, p := range e.Params {
				visit(p)
			}
			if e.Result != nil {
				visit(*e.Result)
			}
		case "concrete":
			groups := insdict.NewDict[string, []StateVariable]()
			for _, component := range e.Components {
				if component.Kind != "update" {
					continue
				}
				for _, v := range EntityStateVariables(component.Entity) {
					groups.Set(v.Name, append(groups.Get(v.Name), v))
				}
			}
			for _, group := range groups.All() {
				if len(group) < 2 {
					continue
				}
				for _, v := range group {
					id := v.Reference
					c.Errors.Set(id, Diagnostic{Code: QNT202, Message: "Multiple updates of variable " + v.Name, Reference: &id, Data: &DiagnosticData{}})
				}
			}
		}
	}
	for _, scheme := range effects {
		visit(scheme.Effect)
	}
	return c.Errors
}
