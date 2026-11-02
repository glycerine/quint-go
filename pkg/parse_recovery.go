package quint

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
)

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
// Lexical and syntax diagnostics are collected before semantic diagnostics.
// Recoverable parser trees retain their declarations; unsupported malformed
// trees return a SyntaxRecoveryError and modules completed before that tree.
// Existing ParseModules entry points retain their fail-fast contracts.
func ParseModulesRecovering(name, text string) (ModuleRecoveryResult, error) {
	return parseModulesRecoveringWithIDs(name, text, NewIDGenerator())
}

func parseModulesRecoveringWithIDs(name, text string, generator *IDGenerator) (ModuleRecoveryResult, error) {
	result := ModuleRecoveryResult{Diagnostics: []Diagnostic{}}
	file, err := NewSourceFile(name, text)
	if err != nil {
		return result, err
	}
	if generator == nil {
		generator = NewIDGenerator()
	}
	located := &locatedParserIDs{generator: generator, file: file, sources: SourceMap{}, upstreamSources: UpstreamSourceMap{}}
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
	diagnostics     []Diagnostic
	syntaxCount     int
	expressionHoles map[antlr.Tree]bool
}

// SyntaxRecoveryError identifies a malformed parse tree whose conversion does
// not yet reproduce the reference listener. Diagnostics and any completed
// modules remain available; callers must not treat partial output as valid IR.
type SyntaxRecoveryError struct{ Cause error }

func (e *SyntaxRecoveryError) Error() string {
	return "unsupported syntax recovery: " + e.Cause.Error()
}
func (e *SyntaxRecoveryError) Unwrap() error { return e.Cause }

var parserDiagnosticCode = regexp.MustCompile(`QNT[0-9]{3}`)

func (ids *recoveringParserIDs) syntaxIssue(offending interface{}, line, column int, message string, position Position) {
	located := ids.parserIDAllocator.(*locatedParserIDs)
	id := ids.Next()
	// antlr4ts escapes whitespace in lexical error excerpts; the Go lexer
	// supplies it literally. Preserve the reference diagnostic text.
	if strings.HasPrefix(message, "token recognition error at:") {
		message = strings.NewReplacer("\n", `\n`, "\t", `\t`, "\r", `\r`).Replace(message)
	}
	code := parserDiagnosticCode.FindString(message)
	if code == "" {
		code = QNT000
	}
	diagnostic := Diagnostic{Code: code, Message: strings.Replace(message, "["+code+"] ", "", 1), Reference: &id}
	ids.diagnostics = append(ids.diagnostics, diagnostic)
	ids.syntaxCount++
	index, width := 0, 0
	if token, ok := offending.(antlr.Token); ok {
		index, width = token.GetStart(), token.GetStop()-token.GetStart()
	}
	start := UpstreamPosition{Line: line - 1, Col: column, Index: index}
	end := UpstreamPosition{Line: line - 1, Col: column + width, Index: index + width}
	located.upstreamSources[id] = UpstreamLocation{Source: located.file.name, Start: start, End: &end}
	// Lexer callbacks have no offending token. Keep their true cursor position
	// in the public map; the raw map above preserves upstream's index-zero quirk.
	finish := position
	if token, ok := offending.(antlr.Token); ok && token.GetStart() >= 0 {
		runeEnd := token.GetStop() + 1
		if runeEnd < token.GetStart() {
			runeEnd = token.GetStart()
		}
		if location, err := located.file.RuneLocation(token.GetStart(), runeEnd); err == nil {
			position, finish = location.Start, *location.End
		}
	}
	located.sources[id] = Location{Source: located.file.name, Start: position, End: &finish}
}

func syntaxRecoveryFailure(ids parserIDAllocator, err error) error {
	if recovery, ok := ids.(*recoveringParserIDs); ok && recovery.syntaxCount > 0 {
		return &SyntaxRecoveryError{Cause: err}
	}
	return err
}

func recoveringSyntax(ids parserIDAllocator) bool {
	recovery, ok := ids.(*recoveringParserIDs)
	return ok && recovery.syntaxCount > 0
}

func unexpectedRecoveryNode(node any) error {
	return fmt.Errorf("cannot convert malformed %T", node)
}

func (ids *recoveringParserIDs) expressionHole(tree antlr.Tree) bool {
	if ids.expressionHoles == nil {
		ids.expressionHoles = make(map[antlr.Tree]bool)
	}
	if hole, known := ids.expressionHoles[tree]; known {
		return hole
	}
	if _, missing := tree.(*p.ExprContext); missing {
		ids.expressionHoles[tree] = true
		return true
	}
	for _, child := range tree.GetChildren() {
		if ids.expressionHole(child) {
			ids.expressionHoles[tree] = true
			return true
		}
	}
	ids.expressionHoles[tree] = false
	return false
}

func recoveryExpressionFrontier(tree antlr.Tree) int {
	if _, expression := tree.(p.IExprContext); expression {
		return 1
	}
	// Definition bodies are consumed into declarations before a let-in's
	// continuation is reduced; they are not sibling expression operands.
	if _, definition := tree.(p.IOperDefContext); definition {
		return 0
	}
	count := 0
	for _, child := range tree.GetChildren() {
		count += recoveryExpressionFrontier(child)
	}
	return count
}

func unsupportedRecoveryOperands(ctx p.IExprContext, ids parserIDAllocator) bool {
	recovery := ids.(*recoveringParserIDs)
	expressions := 0
	hole := false
	for _, child := range ctx.GetChildren() {
		expressions += recoveryExpressionFrontier(child)
		hole = hole || recovery.expressionHole(child)
	}
	return expressions > 1 && hole
}

func recoveryMissingExpressionAllowed(ctx p.IExprContext) bool {
	switch ctx.GetParent().(type) {
	case *p.DeprecatedOperDefContext, *p.AnnotatedOperDefContext, *p.LetInContext,
		*p.LambdaUnsugaredContext, *p.LambdaTupleSugarContext, *p.AsgnContext:
		return true
	}
	return false
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
