package quint

import "github.com/glycerine/insdict"

// ModuleNameResolver resolves additional expressions against already linked
// modules. Table and module scopes are shared with the linked result. Resolve's
// depth argument is explicit because expression analysis may run at different
// surrounding definition depths. Callers must allocate globally unique IR IDs.
type ModuleNameResolver struct {
	*ExpressionNameResolver
	modules *insdict.Dict[string, *NameScope]
}

// ExpressionResolver starts expression analysis in the named module. An unknown
// module has an empty transient scope, matching upstream switchToModule.
func (r LinkedNameResolution) ExpressionResolver(module string) *ModuleNameResolver {
	session := &ModuleNameResolver{ExpressionNameResolver: NewExpressionNameResolver(nil), modules: r.Modules}
	session.Table = r.Table
	session.SwitchToModule(module)
	return session
}

// SwitchToModule changes the scope without discarding lookup entries or errors.
// An unknown module is not inserted into the linked module registry.
func (r *ModuleNameResolver) SwitchToModule(module string) {
	r.Scope = r.modules.Get(module)
	if r.Scope == nil {
		r.Scope = NewNameScope(module)
	}
}

// ClearErrors clears expression diagnostics while retaining resolved references.
// Failed expressions may have added valid reference entries; callers decide
// whether to retain them or discard the corresponding input.
func (r *ModuleNameResolver) ClearErrors() {
	r.Errors = []Diagnostic{}
}
