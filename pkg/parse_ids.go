package quint

import "github.com/antlr4-go/antlr/v4"

type parserIDAllocator interface{ Next() ID }

type locatedParserIDs struct {
	generator       *IDGenerator
	file            *SourceFile
	sources         SourceMap
	current         *Location
	upstreamSources UpstreamSourceMap
	upstreamCurrent *UpstreamLocation
	err             error
}

func (p *locatedParserIDs) Next() ID {
	id := p.generator.Next()
	if p.current != nil {
		p.sources[id] = *p.current
		if p.upstreamSources != nil && p.upstreamCurrent != nil {
			p.upstreamSources[id] = *p.upstreamCurrent
		}
	}
	return id
}
func parserScope(ids parserIDAllocator, ctx antlr.ParserRuleContext) func() {
	p, ok := ids.(*locatedParserIDs)
	if !ok {
		return func() {}
	}
	previous := p.current
	previousUpstream := p.upstreamCurrent
	loc, err := parserLocation(p.file, ctx)
	if err != nil {
		if p.err == nil {
			p.err = err
		}
		p.current = nil
		p.upstreamCurrent = nil
	} else {
		p.current = &loc
		if p.upstreamSources != nil {
			legacy := upstreamParserLocation(p.file, ctx)
			p.upstreamCurrent = &legacy
		}
	}
	return func() { p.current = previous; p.upstreamCurrent = previousUpstream }
}

// ParseExpressionWithSource returns half-open source spans for expression,
// parameter, and local-definition IDs. Synthesized nodes inherit the enclosing
// syntax span. Local annotations also carry type IDs; locations are half-open.
func ParseExpressionWithSource(name, text string) (Expr, SourceMap, error) {
	file, err := NewSourceFile(name, text)
	if err != nil {
		return Expr{}, nil, withSyntaxSource(err, name)
	}
	ids := &locatedParserIDs{generator: NewIDGenerator(), file: file, sources: SourceMap{}}
	expr, err := parseExpressionWithIDs(text, ids)
	if err != nil {
		return Expr{}, nil, withSyntaxSource(err, name)
	}
	if ids.err != nil {
		return Expr{}, nil, ids.err
	}
	return expr, ids.sources, nil
}

func parserIDAt(ids parserIDAllocator, ctx antlr.ParserRuleContext) ID {
	defer parserScope(ids, ctx)()
	return ids.Next()
}

// ParseModulesWithSource records source spans for module/declaration/expression
// and parsed type IDs. Errors return no partial module list
// or source map; use ordinary parsing when source positions are unnecessary.
func ParseModulesWithSource(name, text string) ([]Module, SourceMap, error) {
	file, err := NewSourceFile(name, text)
	if err != nil {
		return nil, nil, withSyntaxSource(err, name)
	}
	ids := &locatedParserIDs{generator: NewIDGenerator(), file: file, sources: SourceMap{}}
	modules, err := parseModulesWithIDs(text, ids)
	if err != nil {
		return nil, nil, withSyntaxSource(err, name)
	}
	if ids.err != nil {
		return nil, nil, ids.err
	}
	return modules, ids.sources, nil
}

// ParseTypeWithSource assigns IDs and half-open spans to every type node. Rows
// are structural containers without IDs; their field types carry source spans.
func ParseTypeWithSource(name, text string) (Type, SourceMap, error) {
	file, err := NewSourceFile(name, text)
	if err != nil {
		return Type{}, nil, withSyntaxSource(err, name)
	}
	ids := &locatedParserIDs{generator: NewIDGenerator(), file: file, sources: SourceMap{}}
	typ, err := parseTypeWithIDs(text, ids)
	if err != nil {
		return Type{}, nil, withSyntaxSource(err, name)
	}
	if ids.err != nil {
		return Type{}, nil, ids.err
	}
	return typ, ids.sources, nil
}

// ParseDeclarationWithSource maps a standalone declaration and its expansions.
// Errors return no partial IR or map, matching ParseModulesWithSource.
func ParseDeclarationWithSource(name, text string) ([]Declaration, SourceMap, error) {
	file, err := NewSourceFile(name, text)
	if err != nil {
		return nil, nil, err
	}
	ids := &locatedParserIDs{generator: NewIDGenerator(), file: file, sources: SourceMap{}}
	ds, err := parseDeclarationWithIDs(text, ids)
	if err != nil {
		return nil, nil, withSyntaxSource(err, name)
	}
	if ids.err != nil {
		return nil, nil, ids.err
	}
	return ds, ids.sources, nil
}

// ParseREPLWithSource parses one REPL input, optionally extending a caller-owned
// ID sequence. The returned map contains only this input's spans and is retained
// on semantic failure for diagnostic references. No partial IR is returned.
// Callers can merge maps from successive commands using their globally unique IDs.
func ParseREPLWithSource(name, text string, generator *IDGenerator) (ExpressionOrDeclaration, SourceMap, error) {
	file, err := NewSourceFile(name, text)
	if err != nil {
		return ExpressionOrDeclaration{}, nil, err
	}
	if generator == nil {
		generator = NewIDGenerator()
	}
	ids := &locatedParserIDs{generator: generator, file: file, sources: SourceMap{}}
	result, err := parseREPLWithIDs(text, ids)
	if err != nil {
		return ExpressionOrDeclaration{}, ids.sources, withSyntaxSource(err, name)
	}
	if ids.err != nil {
		return ExpressionOrDeclaration{}, ids.sources, ids.err
	}
	return result, ids.sources, nil
}
