package quint

import "encoding/json"

// RawModuleRecovery is a caller-owned snapshot of malformed upstream parser
// output. Modules may contain null operands and is deliberately not typed IR:
// strict Module/Expr decoders reject it. It must not be analyzed, evaluated,
// compiled, or sent to a checker. Diagnostics and source maps refer to the raw
// snapshot's IDs; NextID is the reference listener's continuing ID.
//
// This snapshot is available on SyntaxRecoveryError.Raw when the listener can
// preserve malformed record operands. It never changes the valid typed prefix
// or continuing generator returned alongside that error.
type RawModuleRecovery struct {
	Modules         json.RawMessage   `json:"modules"`
	Diagnostics     []Diagnostic      `json:"diagnostics"`
	Sources         SourceMap         `json:"sources"`
	UpstreamSources UpstreamSourceMap `json:"upstreamSources"`
	NextID          ID                `json:"nextID"`
}

type nullRecordOperandError struct{ message string }

func (e *nullRecordOperandError) Error() string { return e.message }

func rawModuleRecovery(file *SourceFile, generator *IDGenerator) (*RawModuleRecovery, error) {
	located := &locatedParserIDs{generator: generator, file: file, sources: SourceMap{}, upstreamSources: UpstreamSourceMap{}}
	ids := &recoveringParserIDs{parserIDAllocator: located, rawNullOperands: true}
	modules, err := parseModulesWithIDs(file.text, ids)
	if err != nil {
		return nil, err
	}
	if located.err != nil {
		return nil, located.err
	}
	wire, err := marshalRawRecoveredModules(modules)
	if err != nil {
		return nil, err
	}
	return &RawModuleRecovery{Modules: wire, Diagnostics: append([]Diagnostic{}, ids.diagnostics...),
		Sources: located.sources, UpstreamSources: located.upstreamSources, NextID: located.generator.Next()}, nil
}
