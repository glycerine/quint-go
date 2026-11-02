package quint

import "github.com/glycerine/insdict"

// EffectSignatureContext holds the retained inference tables and shared fresh
// names used while traversing declarations.
type EffectSignatureContext struct {
	Table    *insdict.Dict[ID, LookupDefinition]
	Effects  *insdict.Dict[string, EffectScheme]
	Errors   *insdict.Dict[string, ErrorTree]
	Fresh    FreshNames
	Location string
}

func NewEffectSignatureContext(table *insdict.Dict[ID, LookupDefinition]) *EffectSignatureContext {
	return &EffectSignatureContext{Table: table, Effects: insdict.NewDict[string, EffectScheme](), Errors: insdict.NewDict[string, ErrorTree]()}
}
func (c *EffectSignatureContext) Fetch(id ID) (EffectScheme, *ErrorTree) {
	if c.Errors != nil {
		if e, ok := c.Errors.Get2(id.String()); ok {
			return EffectScheme{}, &e
		}
	}
	if c.Effects != nil {
		if s, ok := c.Effects.Get2(id.String()); ok {
			return s, nil
		}
	}
	e := ErrorLeaf(c.Location, "Couldn't find any result for id "+id.String()+" while "+c.Location)
	return EffectScheme{}, &e
}
func (c *EffectSignatureContext) Save(id ID, s EffectScheme, e *ErrorTree) {
	if e != nil {
		c.Errors.Set(id.String(), WrapError(c.Location, *e))
	} else {
		c.Effects.Set(id.String(), s)
	}
}
func (c *EffectSignatureContext) EffectForName(name string, id ID, arity int) (Effect, *ErrorTree) {
	fail := func(message string) (Effect, *ErrorTree) { e := ErrorLeaf(c.Location, message); return Effect{}, &e }
	_, fixed := fixedEffectSignatures[name]
	_, prop := fixedEffectPropagation[name]
	builtin := fixed || prop
	switch name {
	case "List", "Set", "Map", "Rec", "Tup", "tuples", "and", "or", "matchVariant", "actionAll", "actionAny":
		builtin = true
	}
	var s EffectScheme
	var def LookupDefinition
	if builtin {
		var err error
		s, err = BuiltinEffectSignature(name, arity)
		if err != nil {
			return fail(err.Error())
		}
	} else {
		var found bool
		if c.Table != nil {
			def, found = c.Table.Get2(id)
		}
		if !found {
			return fail("Signature not found for name: " + name)
		}
		var e *ErrorTree
		s, e = c.Fetch(def.ID)
		if e != nil {
			return Effect{}, e
		}
	}
	out, err := InstantiateEffectScheme(s, &c.Fresh)
	if err != nil {
		return fail(err.Error())
	}
	if !builtin && def.ImportedFrom != nil && def.ImportedFrom.Kind == "instance" {
		out = AddEffectNamespaces(out, def.Namespaces)
	}
	return out, nil
}

// SeedDeclaration records constant and state-variable effects. It returns false
// for declarations whose effects must be inferred from their bodies.
func (c *EffectSignatureContext) SeedDeclaration(d Declaration) bool {
	pure := Effect{Kind: "concrete", Components: []EffectComponent{}}
	s := EffectScheme{Effect: pure, EffectVariables: []string{}, EntityVariables: []string{}}
	switch d.Kind {
	case "const":
		if d.TypeAnnotation != nil && d.TypeAnnotation.Kind == "oper" {
			s = StandardEffectPropagation(len(d.TypeAnnotation.Args))
		}
	case "var":
		s.Effect.Components = []EffectComponent{{Kind: "read", Entity: cloneEntity(Entity{Kind: "concrete", StateVariables: []StateVariable{{Name: d.Name, Reference: d.ID}}})}}
	default:
		return false
	}
	c.Save(d.ID, s, nil)
	return true
}
