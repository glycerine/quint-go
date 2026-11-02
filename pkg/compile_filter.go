package quint

// RemoveRunDefinitions prepares a compiled module for the TLA output path.
// Local definitions and other declarations retain their upstream behavior.
func RemoveRunDefinitions(module Module) Module {
	out := module
	out.Declarations = make([]Declaration, 0, len(module.Declarations))
	for _, d := range module.Declarations {
		if d.Kind == "def" && d.Qualifier == Run {
			continue
		}
		out.Declarations = append(out.Declarations, d)
	}
	return out
}
