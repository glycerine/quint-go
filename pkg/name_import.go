package quint

import "strings"

// ImportDefinitions copies visible top-level definitions into this scope.
// The caller resolves the source module and registers aliases for later exports.
// Imports are hidden from subsequent imports except for constants, as upstream.
func (s *NameScope) ImportDefinitions(source *NameScope, decl Declaration) {
	s.copyModuleDefinitions(source, decl, true)
}

// ExportDefinitions makes copied definitions visible to subsequent importers.
// The caller resolves aliases before supplying the source scope.
func (s *NameScope) ExportDefinitions(source *NameScope, decl Declaration) {
	s.copyModuleDefinitions(source, decl, false)
}
func (s *NameScope) copyModuleDefinitions(source *NameScope, decl Declaration, hide bool) {
	report := func(code, message string) {
		id := decl.ID
		s.Errors = append(s.Errors, Diagnostic{Code: code, Message: message, Reference: &id, Data: &DiagnosticData{}})
	}
	if decl.ProtoName == s.Module {
		verb := decl.Kind
		if verb == "instance" {
			verb = "instantiate"
		}
		report(QNT407, "Cannot "+verb+" '"+decl.ProtoName+"' inside '"+decl.ProtoName+"'")
		return
	}
	if source == nil {
		report(QNT405, "Module '"+decl.ProtoName+"' not found")
		return
	}
	namespace := ""
	if decl.DefName == nil {
		namespace = decl.ProtoName
		if decl.QualifiedName != nil {
			namespace = *decl.QualifiedName
		}
	}
	if decl.Kind == "instance" && decl.QualifiedName == nil {
		namespace = ""
	}
	selective := decl.DefName != nil && *decl.DefName != "*"
	found := false
	for name, entries := range source.Definitions.All() {
		identifier := name
		if namespace != "" {
			identifier = namespace + "::" + name
		}
		if selective && identifier != *decl.DefName {
			continue
		}
		var candidate *LookupDefinition
		for _, entry := range entries {
			isConst := entry.Declaration != nil && entry.Declaration.Kind == "const"
			if !entry.Hidden || isConst {
				copy := entry
				candidate = &copy
				break
			}
		}
		if candidate == nil {
			continue
		}
		found = true
		def := *candidate
		if hide {
			def.Hidden = true
		}
		def.ImportedFrom = &decl
		def.Namespaces = append([]string(nil), def.Namespaces...)
		namespaces := []string{namespace}
		if decl.Kind == "instance" {
			instanceName := decl.ProtoName
			if decl.QualifiedName != nil {
				instanceName = *decl.QualifiedName
			}
			namespaces = []string{instanceName, s.Module}
		}
		for _, ns := range namespaces {
			if ns != "" && !strings.HasPrefix(def.Name, ns+"::") && (len(def.Namespaces) == 0 || def.Namespaces[len(def.Namespaces)-1] != ns) {
				def.Namespaces = append(def.Namespaces, ns)
			}
		}
		existing := s.Definitions.Get(identifier)
		if selective && builtinResolutionNames[identifier] {
			s.conflict(identifier, nil, decl.ID)
			return
		}
		kept := []LookupDefinition{}
		for _, old := range existing {
			if old.ID != def.ID {
				kept = append(kept, old)
				if !selective || old.Depth == def.Depth {
					id := old.ID
					reference := def.ID
					if selective {
						reference = decl.ID
					}
					s.conflict(identifier, &id, reference)
				}
			}
		}
		if selective && len(existing) > 0 {
			def.Shadowing = true
		}
		s.Definitions.Set(identifier, append(kept, def))
	}
	if selective && !found {
		report(QNT404, "Name '"+decl.ProtoName+"::"+*decl.DefName+"' not found")
	}
}
