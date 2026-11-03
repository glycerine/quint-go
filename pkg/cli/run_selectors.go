package cli

import (
	"fmt"

	quint "github.com/glycerine/quint-go/pkg"
)

// runValidateSelectors follows the CLI's independent toExpr calls. Name errors
// remain in one transient resolver session, so later valid expressions repeat
// its first error. Qualifier and parse errors do not enter that shared list.
// Scratch tables/scopes/IDs keep validation out of the published source stage.
func runValidateSelectors(front quint.FrontendResult, main, init, step, invariant string, witnesses []string) ([]quint.ErrorMessage, error) {
	if front.Table == nil || front.Scopes == nil {
		return nil, fmt.Errorf("run selector validation requires linked module scopes")
	}
	table := front.Table.Clone()
	scopes := front.Scopes.Clone()
	last := quint.ID{}
	consider := func(id quint.ID) {
		value, current := id.String(), last.String()
		if len(value) > len(current) || len(value) == len(current) && value > current {
			last = id
		}
	}
	for id := range front.Sources {
		consider(id)
	}
	for id, definition := range table.All() {
		consider(id)
		consider(definition.ID)
	}
	for name, original := range scopes.All() {
		if original == nil {
			continue
		}
		copied := *original
		copied.Definitions = original.Definitions.Clone()
		copied.Errors = append([]quint.Diagnostic{}, original.Errors...)
		for _, definitions := range copied.Definitions.All() {
			for _, definition := range definitions {
				consider(definition.ID)
			}
		}
		scopes.Set(name, &copied)
	}
	ids := quint.NewIDGeneratorAfter(last)
	session := (quint.LinkedNameResolution{Table: table, Modules: scopes}).ExpressionResolver(main)
	selectors := []struct{ flag, text string }{{"init", init}, {"step", step}, {"", invariant}}
	for _, witness := range witnesses {
		selectors = append(selectors, struct{ flag, text string }{text: witness})
	}
	problems := []quint.ErrorMessage{}
	appendProblem := func(problem quint.Diagnostic) {
		problems = append(problems, quint.ResolveDiagnostic(nil, problem))
	}
	for _, selector := range selectors {
		parsed, err := quint.ParseExpressionOrDeclaration(selector.text, ids)
		if err != nil || parsed.Kind != "expr" {
			appendProblem(quint.Diagnostic{Code: quint.QNT501, Message: fmt.Sprintf("Expected %s to be a valid expression", selector.text)})
			continue
		}
		session.Resolve(parsed.Expr, 0)
		if len(session.Errors) > 0 {
			appendProblem(session.Errors[0])
			continue
		}
		if selector.flag == "" || parsed.Expr.Kind != "name" {
			continue
		}
		definition, ok := table.Get2(parsed.Expr.ID)
		if !ok || definition.Declaration == nil {
			continue
		}
		declaration := definition.Declaration
		if declaration.Kind == "def" && declaration.Qualifier == quint.Action {
			continue
		}
		kind := declaration.Kind
		if kind == "def" {
			kind = string(declaration.Qualifier)
		}
		appendProblem(quint.Diagnostic{Code: quint.QNT501, Message: fmt.Sprintf("default --%s action name '%s' is used by a %s declaration", selector.flag, selector.text, kind)})
	}
	if len(problems) == 0 {
		return nil, nil
	}
	return problems, nil
}
