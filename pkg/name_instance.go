package quint

// InstanceDefinitions copies a prototype without mutating it. Overridden
// constants become visible in the alias scope so that exports can expose them.
func (s *NameScope) InstanceDefinitions(source *NameScope, decl Declaration) *NameScope {
	if source == nil || decl.ProtoName == s.Module {
		s.copyModuleDefinitions(source, decl, true)
		return nil
	}
	instance := NewNameScope(source.Module)
	for name, entries := range source.Definitions.All() {
		instance.Definitions.Set(name, append([]LookupDefinition(nil), entries...))
	}
	for _, override := range decl.Overrides {
		name := override.Parameter.Name
		entries := instance.Definitions.Get(name)
		message := ""
		if len(entries) == 0 {
			message = "Instantiation error: '" + name + "' not found in '" + decl.ProtoName + "'"
		} else if entries[0].Declaration == nil || entries[0].Declaration.Kind != "const" {
			message = "Instantiation error: '" + name + "' is not a constant in '" + decl.ProtoName + "'"
		} else {
			entries[0].Hidden = false
		}
		if message != "" {
			id := decl.ID
			s.Errors = append(s.Errors, Diagnostic{Code: QNT406, Message: message, Reference: &id, Data: &DiagnosticData{}})
		}
	}
	s.copyModuleDefinitions(instance, decl, true)
	return instance
}
