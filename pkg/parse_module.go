package quint

import (
	"fmt"
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
	"strings"
)

// ParseModules performs syntax parsing and IR conversion only; imports, names,
// types, and effects are resolved by subsequent passes.
func ParseModules(text string) ([]Module, error) {
	return parseModulesWithIDs(text, NewIDGenerator())
}

func parseModulesWithIDs(text string, ids parserIDAllocator) (modules []Module, err error) {
	listener := newSyntaxErrors(text)
	if recovery, ok := ids.(*recoveringParserIDs); ok {
		listener.onIssue = recovery.syntaxIssue
	}
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	parser := p.NewQuintParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	tree := parser.Modules()
	if recoveringSyntax(ids) {
		defer func() {
			if failure := recover(); failure != nil {
				err = syntaxRecoveryFailure(ids, fmt.Errorf("malformed parse tree: %v", failure))
			}
		}()
	}
	if len(listener.issues) > 0 && !recoveringSyntax(ids) {
		return nil, listener.err("module")
	}
	modules = []Module{}
	for _, ctx := range tree.AllModule() {
		module := Module{Name: ctx.QualId().GetText(), Declarations: []Declaration{}}
		docs := []string{}
		for _, doc := range ctx.AllDOCCOMMENT() {
			docs = append(docs, doc.GetText())
		}
		module.Doc = joinedDocs(docs)
		docs = nil
		for _, item := range ctx.AllModuleItem() {
			if item.DOCCOMMENT() != nil {
				docs = append(docs, item.DOCCOMMENT().GetText())
				continue
			}
			decls, err := declarationsFromContext(item.Declaration(), ids)
			if err != nil {
				if recoveringSyntax(ids) {
					return modules, syntaxRecoveryFailure(ids, err)
				}
				return nil, err
			}
			if len(decls) == 0 {
				docs = nil
				continue
			}
			decls[len(decls)-1].Doc = joinedDocs(docs)
			docs = nil
			module.Declarations = append(module.Declarations, decls...)
		}
		module.ID = parserIDAt(ids, ctx)
		modules = append(modules, module)
	}
	return modules, nil
}
func joinedDocs(lines []string) *string {
	if len(lines) == 0 {
		return nil
	}
	parts := make([]string, len(lines))
	for i, line := range lines {
		// Match JavaScript slice(4, -1): remove the prefix plus one character
		// and the terminating LF, preserving all other whitespace.
		runes := []rune(line)
		if len(runes) > 4 {
			parts[i] = string(runes[4 : len(runes)-1])
		}
	}
	text := strings.Join(parts, "\n")
	if text == "" {
		return nil
	}
	return &text
}
func declarationFromContext(ctx p.IDeclarationContext, ids parserIDAllocator) (Declaration, error) {
	d := Declaration{}
	switch c := ctx.(type) {
	case *p.OperContext:
		op, err := operatorFromContext(c.OperDef(), ids)
		if err != nil {
			return d, err
		}
		return Declaration{ID: op.ID, Kind: op.Kind, Name: op.Name, Qualifier: op.Qualifier, Expr: op.Expr, TypeAnnotation: op.TypeAnnotation}, nil
	case *p.ImportDefContext:
		imp := c.ImportMod()
		d.Kind, d.ProtoName = "import", imp.Name(0).GetText()
		if imp.IdentOrStar() != nil {
			value := imp.IdentOrStar().GetText()
			d.DefName = &value
		}
		if len(imp.AllName()) > 1 {
			value := imp.Name(1).GetText()
			d.QualifiedName = &value
		}
		if imp.FromSource() != nil {
			value := imp.FromSource().GetText()
			value = value[1 : len(value)-1]
			d.FromSource = &value
		}
	case *p.ExportDefContext:
		exp := c.ExportMod()
		d.Kind, d.ProtoName = "export", exp.Name(0).GetText()
		if exp.IdentOrStar() != nil {
			value := exp.IdentOrStar().GetText()
			d.DefName = &value
		}
		if len(exp.AllName()) > 1 {
			value := exp.Name(1).GetText()
			d.QualifiedName = &value
		}
	case *p.InstanceContext:
		inst := c.InstanceMod()
		d.Kind, d.ProtoName = "instance", inst.ModuleName().GetText()
		d.IdentityOverride = inst.MUL() != nil
		d.Overrides = []Override{}
		if inst.QualifiedName() != nil {
			value := inst.QualifiedName().GetText()
			d.QualifiedName = &value
		}
		if inst.FromSource() != nil {
			value := inst.FromSource().GetText()
			value = value[1 : len(value)-1]
			d.FromSource = &value
		}
		for i, ctx := range inst.AllExpr() {
			expr, err := expressionFromContext(ctx, ids)
			if err != nil {
				return d, err
			}
			d.Overrides = append(d.Overrides, Override{Parameter: Parameter{ID: parserIDAt(ids, inst.Name(i)), Name: inst.Name(i).GetText()}, Expr: expr})
		}
	case *p.ConstContext:
		typ, err := typeFromContextWithIDs(c.Type_(), ids)
		if err != nil {
			return d, err
		}
		d.Kind = "const"
		d.Name = c.QualId().GetText()
		d.TypeAnnotation = &typ
	case *p.VarContext:
		typ, err := typeFromContextWithIDs(c.Type_(), ids)
		if err != nil {
			return d, err
		}
		d.Kind = "var"
		d.Name = c.QualId().GetText()
		d.TypeAnnotation = &typ
	case *p.AssumeContext:
		expr, err := expressionFromContext(c.Expr(), ids)
		if err != nil {
			return d, err
		}
		d.Kind = "assume"
		d.Name = c.GetAssumeName().GetText()
		d.Assumption = &expr
	case *p.TypeDefsContext:
		d.Kind = "typedef"
		switch def := c.TypeDef().(type) {
		case *p.TypeAbstractDefContext:
			d.Name = def.QualId().GetText()
		case *p.TypeAliasDefContext:
			d.Name = def.TypeDefHead().GetTypeName().GetText()
			for _, param := range def.TypeDefHead().AllLOW_ID() {
				d.Params = append(d.Params, param.GetText())
			}
			typ, err := typeFromContextWithIDs(def.Type_(), ids)
			if err != nil {
				return d, err
			}
			d.Type = &typ
		default:
			return d, fmt.Errorf("type declaration conversion not implemented for %T", def)
		}
	default:
		return d, fmt.Errorf("declaration conversion not implemented for %T", ctx)
	}
	d.ID = ids.Next()
	if d.Kind == "typedef" {
		if err := checkParsedTypeDeclaration(d, true, ids); err != nil {
			return Declaration{}, err
		}
	}
	return d, nil
}
