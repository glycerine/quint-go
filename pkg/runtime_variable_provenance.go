package quint

import "strings"

func seedRuntimeVariableProvenance(declaration Declaration) Declaration {
	if declaration.Kind == "var" && declaration.runtimeVariable == nil {
		declaration.runtimeVariable = &RuntimeVariableBinding{Key: declaration.ID.String(), Name: declaration.Name}
	}
	return declaration
}

func instantiateRuntimeVariableProvenance(declaration Declaration, namespace string) Declaration {
	declaration = seedRuntimeVariableProvenance(declaration)
	if declaration.Kind != "var" {
		return declaration
	}
	binding := declaration.runtimeVariable
	// Entering an already instantiated dependency replaces the builder's
	// namespace context, so an inner instance retains its own source cell.
	if strings.Contains(binding.Key, "#") {
		return declaration
	}
	parts := strings.Split(namespace, "::")
	key := binding.Key
	for i := len(parts) - 1; i >= 0; i-- {
		key += "#" + parts[i]
	}
	declaration.runtimeVariable = &RuntimeVariableBinding{Key: key, Name: namespace + "::" + binding.Name}
	return declaration
}
