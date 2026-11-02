package quint

// ModuleRecoveryResult contains source-mapped IR even when semantic conversion
// diagnostics were found. Such modules are for editor/diagnostic use and must
// not be evaluated or compiled until Diagnostics is empty.
type ModuleRecoveryResult struct {
	ModuleParseLocations
	Diagnostics []Diagnostic
}

// ParseModulesRecovering accumulates semantic conversion diagnostics while
// retaining the declarations around them. It corrects deprecated Map types to
// function types and retains duplicate record fields for subsequent analysis.
// Syntax/lexical failures still return an error without recovered modules.
// Existing ParseModules entry points retain their fail-fast contracts.
func ParseModulesRecovering(name, text string) (ModuleRecoveryResult, error) {
	result := ModuleRecoveryResult{Diagnostics: []Diagnostic{}}
	file, err := NewSourceFile(name, text)
	if err != nil {
		return result, err
	}
	located := &locatedParserIDs{generator: NewIDGenerator(), file: file, sources: SourceMap{}, upstreamSources: UpstreamSourceMap{}}
	ids := &recoveringParserIDs{parserIDAllocator: located}
	modules, err := parseModulesWithIDs(text, ids)
	result.ModuleParseLocations = ModuleParseLocations{Modules: modules, Sources: located.sources, UpstreamSources: located.upstreamSources, IDs: located.generator}
	result.Diagnostics = append(result.Diagnostics, ids.diagnostics...)
	if err != nil {
		return result, withSyntaxSource(err, name)
	}
	return result, located.err
}

type recoveringParserIDs struct {
	parserIDAllocator
	diagnostics []Diagnostic
}

// reportParserDiagnostic returns whether conversion should continue after a
// semantic diagnostic. Ordinary parser allocators preserve fail-fast behavior.
func reportParserDiagnostic(ids parserIDAllocator, diagnostic Diagnostic) bool {
	if recovery, ok := ids.(*recoveringParserIDs); ok {
		recovery.diagnostics = append(recovery.diagnostics, diagnostic)
		return true
	}
	return false
}

func checkParsedTypeDeclaration(d Declaration, checkName bool, ids parserIDAllocator) error {
	if _, ok := ids.(*recoveringParserIDs); !ok {
		return checkTypeDeclaration(d, checkName)
	}
	// Type-name and unbound-variable checks are independent in the reference.
	if err := checkTypeDeclaration(d, checkName); err != nil {
		if diagnostic, ok := err.(Diagnostic); ok {
			reportParserDiagnostic(ids, diagnostic)
			if diagnostic.Code != QNT007 {
				return nil
			}
		} else {
			return err
		}
	} else {
		return nil
	}
	if err := checkTypeDeclaration(d, false); err != nil {
		if diagnostic, ok := err.(Diagnostic); ok {
			reportParserDiagnostic(ids, diagnostic)
			return nil
		}
		return err
	}
	return nil
}

// The reference fills a missing expression with a valid, visibly synthetic
// local binding. This keeps later declarations available without inventing a
// value for the malformed expression.
func parserUndefinedExpression(ids parserIDAllocator) Expr {
	name := "__undefinedExprGenerated"
	id, definitionID, bodyID, referenceID := ids.Next(), ids.Next(), ids.Next(), ids.Next()
	body := Expr{ID: bodyID, Kind: "bool", Bool: true}
	reference := Expr{ID: referenceID, Kind: "name", Name: name}
	return Expr{ID: id, Kind: "let", Opdef: &OperatorDef{ID: definitionID, Kind: "def", Qualifier: Val, Name: name, Expr: &body}, Body: &reference}
}
