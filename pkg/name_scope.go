package quint

import "github.com/glycerine/insdict"

// LookupDefinition retains a definition's identity and lexical visibility.
// Declaration and Parameter are mutually exclusive payloads; the scope does not
// mutate either payload. Import provenance is added by module resolution.
type LookupDefinition struct {
	identity     *byte // Identity of the upstream lookup object, distinct from its IR ID.
	Hidden       bool
	Namespaces   []string
	ImportedFrom *Declaration
	ID           ID
	Name         string
	Depth        int
	Shadowing    bool
	Declaration  *Declaration
	Parameter    *Parameter
}

// NameScope stores definition stacks in insertion order, matching the collector.
// Different depths may shadow; different identities at the same depth conflict.
type NameScope struct {
	Module      string
	Definitions *insdict.Dict[string, []LookupDefinition]
	Errors      []Diagnostic
}

func NewNameScope(module string) *NameScope {
	return &NameScope{Module: module, Definitions: insdict.NewDict[string, []LookupDefinition](), Errors: []Diagnostic{}}
}
func (s *NameScope) Add(def LookupDefinition) LookupDefinition {
	def.identity = new(byte)
	if def.Name == "_" {
		return def
	}
	if builtinResolutionNames[def.Name] {
		s.conflict(def.Name, nil, def.ID)
		return def
	}
	existing := s.Definitions.Get(def.Name)
	for _, entry := range existing {
		if entry.ID != def.ID && entry.Depth == def.Depth {
			id := entry.ID
			s.conflict(def.Name, &id, def.ID)
		}
	}
	entries := make([]LookupDefinition, 0, len(existing)+1)
	for _, entry := range existing {
		if entry.ID != def.ID {
			entries = append(entries, entry)
		}
	}
	if len(existing) > 0 {
		def.Shadowing = true
	}
	s.Definitions.Set(def.Name, append(entries, def))
	return def
}
func (s *NameScope) Lookup(name string) (LookupDefinition, bool) {
	entries := s.Definitions.Get(name)
	if len(entries) == 0 {
		return LookupDefinition{}, false
	}
	return entries[len(entries)-1], true
}
func (s *NameScope) TopLevel(name string) (LookupDefinition, bool) {
	entries := s.Definitions.Get(name)
	if len(entries) == 0 {
		return LookupDefinition{}, false
	}
	return entries[0], true
}

// Pop restores the previous visible definition and retains the name's order slot.
func (s *NameScope) Pop(name string) {
	entries := s.Definitions.Get(name)
	if len(entries) > 0 {
		s.Definitions.Set(name, entries[:len(entries)-1])
	}
}
func (s *NameScope) conflict(name string, old *ID, next ID) {
	message := "Built-in name '" + name + "' is redefined in module '" + s.Module + "'"
	if old != nil {
		message = "Conflicting definitions found for name '" + name + "' in module '" + s.Module + "'"
		s.Errors = append(s.Errors, Diagnostic{Code: QNT101, Message: message, Reference: old, Data: &DiagnosticData{}})
	}
	s.Errors = append(s.Errors, Diagnostic{Code: QNT101, Message: message, Reference: &next, Data: &DiagnosticData{}})
}
