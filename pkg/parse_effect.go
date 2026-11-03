package quint

import (
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/effectparser"
)

// ParseEffect parses a complete effect signature, assigning state occurrence
// references from one in encounter order, independently for each parse.
func ParseEffect(text string) (Effect, error) {
	listener := newSyntaxErrors(text)
	lexer := p.NewEffectLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	parser := p.NewEffectParser(tokens)
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	parser.SetErrorHandler(&effectEntryErrorStrategy{parser.GetErrorHandler().(*antlr.DefaultErrorStrategy)})
	tree := parser.Effect()
	if tokens.LA(1) != antlr.TokenEOF {
		listener.trailing(tokens.LT(1))
	}
	if len(listener.issues) > 0 {
		return Effect{}, listener.err("effect")
	}
	ids := NewIDGenerator()
	entity := func(ctx p.IEntityContext) Entity {
		parts := []Entity{}
		for _, name := range ctx.AllIDENTIFIER() {
			parts = append(parts, Entity{Kind: "variable", Name: name.GetText()})
		}
		vars := []StateVariable{}
		for _, v := range ctx.AllStateVarRef() {
			vars = append(vars, NewStateVariable(v.IDENTIFIER().GetText(), ids.Next()))
		}
		if len(vars) > 0 {
			parts = append(parts, Entity{Kind: "concrete", StateVariables: vars})
		}
		if len(parts) == 0 {
			return Entity{Kind: "concrete", StateVariables: []StateVariable{}}
		}
		if len(parts) == 1 {
			return parts[0]
		}
		return Entity{Kind: "union", Entities: parts}
	}
	var convert func(p.IEffectContext) Effect
	convert = func(ctx p.IEffectContext) Effect {
		switch c := ctx.(type) {
		case *p.VariableEffectContext:
			return Effect{Kind: "variable", Name: c.IDENTIFIER().GetText()}
		case *p.ArrowEffectContext:
			children := c.AllEffect()
			params := make([]Effect, len(children)-1)
			for i := range params {
				params[i] = convert(children[i])
			}
			result := convert(children[len(children)-1])
			return Effect{Kind: "arrow", Params: params, Result: &result}
		case *p.ConcreteEffectContext:
			components := []EffectComponent{}
			if concrete, ok := c.Concrete().(*p.ConcreteComponentsContext); ok {
				for _, component := range concrete.AllComponent() {
					switch v := component.(type) {
					case *p.ReadComponentContext:
						components = append(components, EffectComponent{Kind: "read", Entity: entity(v.Read().Entity())})
					case *p.UpdateComponentContext:
						components = append(components, EffectComponent{Kind: "update", Entity: entity(v.Update().Entity())})
					case *p.TemporalComponentContext:
						components = append(components, EffectComponent{Kind: "temporal", Entity: entity(v.Temporal().Entity())})
					}
				}
			}
			return Effect{Kind: "concrete", Components: components}
		}
		panic("unreachable effect parse node")
	}
	return convert(tree), nil
}

// The reference standalone entry allows arrow delimiters to follow a concrete
// effect when recovering a missing component bracket. The Go runtime's follow
// set can omit those delimiters outside an arrow argument and consume the rest
// of the input instead. Keep this compatibility rule local to the effect entry.
type effectEntryErrorStrategy struct{ *antlr.DefaultErrorStrategy }

func (strategy *effectEntryErrorStrategy) RecoverInline(parser antlr.Parser) antlr.Token {
	switch parser.GetParserRuleContext().(type) {
	case *p.ReadContext, *p.UpdateContext, *p.TemporalContext:
		current := parser.GetTokenStream().LA(1)
		expected := parser.GetExpectedTokens().GetIntervals()
		if (current == p.EffectParserT__1 || current == p.EffectParserT__2) &&
			len(expected) == 1 && expected[0].Start == p.EffectParserT__6 && expected[0].Stop == p.EffectParserT__6+1 {
			// Preserve the default deletion rule for Read[,] and its siblings.
			if matched := strategy.SingleTokenDeletion(parser); matched != nil {
				parser.Consume()
				return matched
			}
			strategy.ReportMissingToken(parser)
			return strategy.GetMissingSymbol(parser)
		}
	}
	return strategy.DefaultErrorStrategy.RecoverInline(parser)
}
