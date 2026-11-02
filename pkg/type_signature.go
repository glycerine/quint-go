package quint

import "github.com/glycerine/insdict"

// TypeSignatureContext supplies inferred schemes and previous failures while
// resolving a name's type. Fresh is shared across a constraint-generation run.
type TypeSignatureContext struct {
	Unifier  TypeUnifier
	Types    *insdict.Dict[string, TypeScheme]
	Errors   *insdict.Dict[string, ErrorTree]
	Fresh    FreshNames
	Location string
}

func (c *TypeSignatureContext) TypeForName(name string, id ID, arity int) (Type, *ErrorTree) {
	fail := func(message string) (Type, *ErrorTree) { e := ErrorLeaf(c.Location, message); return Type{}, &e }
	_, builtin := fixedBuiltinTypes[name]
	switch name {
	case "List", "Set", "Map", "and", "or", "actionAll", "actionAny", "matchVariant", "tuples":
		builtin = true
	}
	if builtin {
		signature, err := BuiltinTypeSignature(name, arity)
		if err != nil {
			return fail(err.Error())
		}
		return c.Unifier.InstantiateScheme(signature, &c.Fresh)
	}
	var def LookupDefinition
	var found bool
	if c.Unifier.Table != nil {
		def, found = c.Unifier.Table.Get2(id)
	}
	if !found {
		return fail("Signature not found for name: " + name)
	}
	fetch := func() (TypeScheme, *ErrorTree) {
		if c.Errors != nil {
			if e, ok := c.Errors.Get2(def.ID.String()); ok {
				return TypeScheme{}, &e
			}
		}
		if c.Types != nil {
			if scheme, ok := c.Types.Get2(def.ID.String()); ok {
				return scheme, nil
			}
		}
		e := ErrorLeaf(c.Location, "Couldn't find any result for id "+def.ID.String()+" while "+c.Location)
		return TypeScheme{}, &e
	}
	copyType := func(t Type) (Type, *ErrorTree) {
		out, err := ApplySubstitution(nil, t)
		if err != nil {
			return fail(err.Error())
		}
		return out, nil
	}
	if def.Declaration != nil && (def.Declaration.Kind == "var" || def.Declaration.Kind == "const") {
		scheme, e := fetch()
		if e != nil {
			return Type{}, e
		}
		return copyType(scheme.Type)
	}
	if def.Declaration != nil && def.Declaration.TypeAnnotation != nil {
		return copyType(*def.Declaration.TypeAnnotation)
	}
	if def.Parameter != nil && def.Parameter.TypeAnnotation != nil {
		return copyType(*def.Parameter.TypeAnnotation)
	}
	scheme, e := fetch()
	if e != nil {
		return Type{}, e
	}
	return c.Unifier.InstantiateScheme(scheme, &c.Fresh)
}
