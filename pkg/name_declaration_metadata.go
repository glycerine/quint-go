package quint

import "encoding/json"

// declarationLookup retains the lookup fields carried by flattened IR. Fresh
// source declarations omit these fields, but imported definitions can retain
// visibility and namespaces when they become local flattened declarations.
func declarationLookup(declaration Declaration) LookupDefinition {
	copy := declaration
	definition := LookupDefinition{
		ID: declaration.ID, Name: declaration.Name, Declaration: &copy,
		Namespaces: append([]string(nil), declaration.Namespaces...),
	}
	if declaration.Depth != nil {
		definition.Depth = *declaration.Depth
	}
	if declaration.Hidden != nil {
		definition.Hidden = *declaration.Hidden
	}
	if len(declaration.Shadowing) != 0 {
		_ = json.Unmarshal(declaration.Shadowing, &definition.Shadowing)
	}
	if len(declaration.ImportedFrom) != 0 {
		var imported Declaration
		if err := json.Unmarshal(declaration.ImportedFrom, &imported); err == nil {
			definition.ImportedFrom = &imported
		}
	}
	return definition
}
