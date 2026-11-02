package quint

import (
	"github.com/glycerine/insdict"
	"hash/fnv"
)

// ExpressionNameResolver resolves expressions against an existing module scope.
// It retains lookup entries across calls and restores scoped bindings on exit.
type ExpressionNameResolver struct {
	Scope  *NameScope
	Table  *insdict.Dict[ID, LookupDefinition]
	Errors []Diagnostic
}

func NewExpressionNameResolver(scope *NameScope) *ExpressionNameResolver {
	return &ExpressionNameResolver{Scope: scope, Table: insdict.NewDictFunc[ID, LookupDefinition](func(id ID) uint64 { h := fnv.New64a(); _, _ = h.Write([]byte(id.String())); return h.Sum64() }), Errors: []Diagnostic{}}
}
func (r *ExpressionNameResolver) resolve(name string, id ID, typeName bool) {
	if !typeName && builtinResolutionNames[name] {
		return
	}
	def, ok := r.Scope.Lookup(name)
	isType := def.Declaration != nil && def.Declaration.Kind == "typedef"
	if !ok || isType != typeName {
		description := "Name"
		if typeName {
			description = "Type alias"
		}
		r.Errors = append(r.Errors, Diagnostic{Code: QNT404, Message: description + " '" + name + "' not found", Reference: &id, Data: &DiagnosticData{}})
		return
	}
	r.Table.Set(id, def)
}
func (r *ExpressionNameResolver) resolveType(typ *Type) {
	if typ == nil {
		return
	}
	_ = WalkType(typ, func(t *Type) (bool, error) {
		if t.Kind == "const" && t.ID != nil {
			r.resolve(t.Name, *t.ID, true)
		}
		return true, nil
	})
}

// Resolve uses the surrounding definition's depth (zero for top-level bodies).
// Input expressions are expected to be validated IR.
func (r *ExpressionNameResolver) Resolve(expr *Expr, depth int) {
	if expr == nil {
		return
	}
	switch expr.Kind {
	case "name":
		r.resolve(expr.Name, expr.ID, false)
	case "app":
		r.resolve(expr.Opcode, expr.ID, false)
		for i := range expr.Args {
			r.Resolve(&expr.Args[i], depth)
		}
	case "lambda":
		for i := range expr.Params {
			p := expr.Params[i]
			r.Scope.Add(LookupDefinition{ID: p.ID, Name: p.Name, Depth: depth + 1, Parameter: &p})
		}
		for i := range expr.Params {
			r.resolveType(expr.Params[i].TypeAnnotation)
		}
		r.Resolve(expr.Body, depth+1)
		for _, p := range expr.Params {
			r.Scope.Pop(p.Name)
		}
	case "let":
		op := expr.Opdef
		declaration := Declaration{ID: op.ID, Kind: "def", Name: op.Name, Qualifier: op.Qualifier, Expr: op.Expr, TypeAnnotation: op.TypeAnnotation}
		def := r.Scope.Add(LookupDefinition{ID: op.ID, Name: op.Name, Depth: depth + 2, Declaration: &declaration})
		r.Table.Set(op.ID, def)
		r.resolveType(op.TypeAnnotation)
		r.Resolve(op.Expr, depth+2)
		r.Resolve(expr.Body, depth+1)
		r.Scope.Pop(op.Name)
	}
}
