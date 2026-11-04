package quint

import "github.com/glycerine/insdict"

// Recorded applications retain their source expression, including names inside
// callback arguments. Unshadowing and namespace expansion change those names
// even in a pure project without variables or instances. A resolved application
// has a table entry exactly when it calls a user definition or parameter.
func runtimeHasSourceCall(expression Expr, table *insdict.Dict[ID, LookupDefinition]) bool {
	if expression.Kind == "app" && table != nil {
		if _, exists := table.Get2(expression.ID); exists {
			return true
		}
	}
	if expression.Opdef != nil && expression.Opdef.Expr != nil && runtimeHasSourceCall(*expression.Opdef.Expr, table) {
		return true
	}
	for _, argument := range expression.Args {
		if runtimeHasSourceCall(argument, table) {
			return true
		}
	}
	return expression.Body != nil && runtimeHasSourceCall(*expression.Body, table)
}
