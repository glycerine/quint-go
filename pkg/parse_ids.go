package quint

import "github.com/antlr4-go/antlr/v4"

type parserIDAllocator interface{ Next() ID }

type locatedParserIDs struct {
	generator *IDGenerator
	file      *SourceFile
	sources   SourceMap
	current   *Location
	err       error
}

func (p *locatedParserIDs) Next() ID {
	id := p.generator.Next()
	if p.current != nil {
		p.sources[id] = *p.current
	}
	return id
}
func parserScope(ids parserIDAllocator, ctx antlr.ParserRuleContext) func() {
	p, ok := ids.(*locatedParserIDs)
	if !ok {
		return func() {}
	}
	previous := p.current
	loc, err := parserLocation(p.file, ctx)
	if err != nil {
		if p.err == nil {
			p.err = err
		}
		p.current = nil
	} else {
		p.current = &loc
	}
	return func() { p.current = previous }
}

// ParseExpressionWithSource returns half-open source spans for expression,
// parameter, and local-definition IDs. Synthesized nodes inherit the enclosing
// syntax span. Type IDs and upstream's inclusive span format remain separate.
func ParseExpressionWithSource(name, text string) (Expr, SourceMap, error) {
	file, err := NewSourceFile(name, text)
	if err != nil {
		return Expr{}, nil, err
	}
	ids := &locatedParserIDs{generator: NewIDGenerator(), file: file, sources: SourceMap{}}
	expr, err := parseExpressionWithIDs(text, ids)
	if err != nil {
		return Expr{}, nil, err
	}
	if ids.err != nil {
		return Expr{}, nil, ids.err
	}
	return expr, ids.sources, nil
}
